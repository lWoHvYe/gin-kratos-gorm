package ai

import (
	"context"
	"os"
	"strings"
	"testing"

	pb "gin-kratos-gorm/api/gen/mall/v1"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

func TestPreferencesAreUserScoped(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR")
	}
	cache := redis.NewClient(&redis.Options{Addr: addr})
	defer cache.Close()
	svc := &Service{cache: cache}
	ctx := context.Background()
	if _, err := svc.SetPreferences(ctx, &pb.SetAIPreferencesRequest{
		UserId: 1, Preferences: &pb.AIPreferences{PreferredBrands: []string{"云舟"}},
	}); err != nil {
		t.Fatal(err)
	}
	one, err := svc.GetPreferences(ctx, &pb.GetAIPreferencesRequest{UserId: 1})
	if err != nil || len(one.GetPreferredBrands()) != 1 {
		t.Fatalf("user1 = %+v err=%v", one, err)
	}
	two, err := svc.GetPreferences(ctx, &pb.GetAIPreferencesRequest{UserId: 2})
	if err != nil || len(two.GetPreferredBrands()) != 0 {
		t.Fatalf("user2 = %+v err=%v", two, err)
	}
	_ = cache.Del(ctx, preferenceKey(1)).Err()
}

func TestChatEmitsOwnerScopedPreferenceAttribution(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TEST_REDIS_ADDR")
	}
	cache := redis.NewClient(&redis.Options{Addr: addr})
	defer cache.Close()
	ctx := context.Background()
	svc := newTestService(t, fakeAgent{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "为你推荐云舟 M3"},
	}}, &fakeSessions{}, fakeLimiter{allowed: true})
	svc.cache = cache
	defer cache.Del(ctx, preferenceKey(101), "ai:prefs:applied:101:s-attribution").Err()
	if _, err := svc.SetPreferences(ctx, &pb.SetAIPreferencesRequest{
		UserId: 101, Preferences: &pb.AIPreferences{
			PreferredBrands: []string{"云舟"}, PreferredCategories: []string{"手机"},
			BudgetMinCent: 100000, BudgetMaxCent: 300000,
		},
	}); err != nil {
		t.Fatal(err)
	}
	stream := &fakeChatStream{ctx: ctx}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 101, SessionId: "s-attribution", Message: "推荐手机"}, stream); err != nil {
		t.Fatal(err)
	}
	var attribution string
	for _, ev := range stream.events {
		if ev.GetType() == "attribution" {
			attribution = ev.GetAttributionJson()
		}
		if ev.GetType() == "done" && attribution == "" {
			t.Fatal("attribution was sent after done")
		}
	}
	if !strings.Contains(attribution, `"brands":["云舟"]`) || !strings.Contains(attribution, `"source":"explicit_profile"`) {
		t.Fatalf("attribution = %q", attribution)
	}
	// A different user must not receive another user's profile attribution.
	other := &fakeChatStream{ctx: ctx}
	if err := svc.Chat(&pb.AIChatRequest{UserId: 102, SessionId: "s-other", Message: "推荐手机"}, other); err != nil {
		t.Fatal(err)
	}
	for _, ev := range other.events {
		if ev.GetType() == "attribution" {
			t.Fatalf("cross-user attribution leaked: %s", ev.GetAttributionJson())
		}
	}
}
