package ai

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/cloudwego/eino/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeAgent struct {
	chunks []*schema.Message
	err    error
}

func (f fakeAgent) Stream(context.Context, []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	if f.err != nil {
		return nil, f.err
	}
	return schema.StreamReaderFromArray(f.chunks), nil
}

type fakeLimiter struct {
	allowed bool
	err     error
}

func (f fakeLimiter) Allow(context.Context, int64) (bool, error) { return f.allowed, f.err }

type fakeSessions struct{ appended []*schema.Message }

func (f *fakeSessions) Load(context.Context, int64, string) ([]*schema.Message, error) {
	return nil, nil
}
func (f *fakeSessions) Append(_ context.Context, _ int64, _ string, msgs ...*schema.Message) error {
	f.appended = append(f.appended, msgs...)
	return nil
}

type fakeChatStream struct {
	ctx    context.Context
	events []*pb.AIChatEvent
}

func (f *fakeChatStream) Send(ev *pb.AIChatEvent) error {
	f.events = append(f.events, ev)
	return nil
}
func (f *fakeChatStream) Context() context.Context     { return f.ctx }
func (f *fakeChatStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeChatStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeChatStream) SetTrailer(metadata.MD)       {}
func (f *fakeChatStream) SendMsg(any) error            { return nil }
func (f *fakeChatStream) RecvMsg(any) error            { return io.EOF }

func newTestService(t *testing.T, a agentRunner, sessions *fakeSessions, limit limiter) *Service {
	t.Helper()
	return New(Config{Enabled: true, Timeout: time.Second},
		a, sessions, limit, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestChatStreamsDeltasAndPersistsTurn(t *testing.T) {
	sessions := &fakeSessions{}
	svc := newTestService(t, fakeAgent{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "你好"},
		{Role: schema.Assistant, Content: "，需要什么？"},
	}}, sessions, fakeLimiter{allowed: true})
	stream := &fakeChatStream{ctx: context.Background()}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 7, SessionId: "s1", Message: "在吗"}, stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.events) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(stream.events), stream.events)
	}
	if stream.events[0].GetType() != "delta" || stream.events[0].GetDelta() != "你好" {
		t.Fatalf("first event = %+v", stream.events[0])
	}
	if stream.events[2].GetType() != "done" {
		t.Fatalf("last event = %+v", stream.events[2])
	}
	if len(sessions.appended) != 2 || sessions.appended[1].Content != "你好，需要什么？" {
		t.Fatalf("session contents = %+v", sessions.appended)
	}
}

func TestChatRejectsDisabledRateLimitedAndInvalid(t *testing.T) {
	disabled := New(Config{Enabled: false}, fakeAgent{}, &fakeSessions{}, fakeLimiter{allowed: true}, nil)
	if err := disabled.Chat(&pb.AIChatRequest{UserId: 1, Message: "hi"}, &fakeChatStream{ctx: context.Background()}); status.Code(err) != codes.Unavailable {
		t.Fatalf("disabled code = %v", status.Code(err))
	}

	limited := newTestService(t, fakeAgent{}, &fakeSessions{}, fakeLimiter{allowed: false})
	if err := limited.Chat(&pb.AIChatRequest{UserId: 1, Message: "hi"}, &fakeChatStream{ctx: context.Background()}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("limited code = %v", status.Code(err))
	}

	invalid := newTestService(t, fakeAgent{}, &fakeSessions{}, fakeLimiter{allowed: true})
	if err := invalid.Chat(&pb.AIChatRequest{UserId: 0, Message: "hi"}, &fakeChatStream{ctx: context.Background()}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid code = %v", status.Code(err))
	}
}

func TestChatSendsErrorEventWhenModelFails(t *testing.T) {
	svc := newTestService(t, fakeAgent{err: context.DeadlineExceeded}, &fakeSessions{}, fakeLimiter{allowed: true})
	stream := &fakeChatStream{ctx: context.Background()}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 7, Message: "hi"}, stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.events) != 1 || stream.events[0].GetType() != "error" {
		t.Fatalf("events = %+v", stream.events)
	}
}

func TestChatSelectsAdminAgent(t *testing.T) {
	svc := newTestService(t, fakeAgent{chunks: []*schema.Message{{Role: schema.Assistant, Content: "customer"}}}, &fakeSessions{}, fakeLimiter{allowed: true})
	svc.adminAgent = fakeAgent{chunks: []*schema.Message{{Role: schema.Assistant, Content: "admin"}}}
	stream := &fakeChatStream{ctx: context.Background()}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 1, Message: "hi", Admin: true}, stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.events) == 0 || stream.events[0].GetDelta() != "admin" {
		t.Fatalf("events = %+v", stream.events)
	}
}

type fakeUsage struct {
	used   int64
	global int64
	added  int64
}

func (f *fakeUsage) Load(context.Context, int64) (int64, error) { return f.used, nil }
func (f *fakeUsage) Add(_ context.Context, _, prompt, completion, _ int64) error {
	f.added += prompt + completion
	return nil
}
func (f *fakeUsage) LoadGlobal(context.Context) (int64, error)            { return f.global, nil }
func (f *fakeUsage) AddGlobal(context.Context, int64, int64, int64) error { return nil }

func TestChatRejectsExhaustedTokenBudget(t *testing.T) {
	svc := newTestService(t, fakeAgent{chunks: []*schema.Message{{Role: schema.Assistant, Content: "hi"}}}, &fakeSessions{}, fakeLimiter{allowed: true})
	svc.usage = &fakeUsage{used: 100}
	svc.dailyBudget = 100
	stream := &fakeChatStream{ctx: context.Background()}
	err := svc.Chat(&pb.AIChatRequest{UserId: 1, Message: "hi"}, stream)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("code = %v", status.Code(err))
	}
	if len(stream.events) != 0 {
		t.Fatalf("budget rejection must not stream: %+v", stream.events)
	}
}

func TestChatEmitsDraftEventFromDraftCartTool(t *testing.T) {
	draft := `{"type":"cart_draft","draft_id":"d1","items":[],"total_cent":0}`
	svc := newTestService(t, fakeAgent{chunks: []*schema.Message{
		schema.ToolMessage(draft, "", schema.WithToolName("draft_cart")),
		{Role: schema.Assistant, Content: "请确认"},
	}}, &fakeSessions{}, fakeLimiter{allowed: true})
	stream := &fakeChatStream{ctx: context.Background()}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 7, SessionId: "s1", Message: "加入购物车"}, stream); err != nil {
		t.Fatal(err)
	}
	var draftEvent *pb.AIChatEvent
	for _, ev := range stream.events {
		if ev.GetType() == "draft" {
			draftEvent = ev
		}
	}
	if draftEvent == nil || draftEvent.GetDraftJson() != draft {
		t.Fatalf("draft event = %+v", stream.events)
	}
}
