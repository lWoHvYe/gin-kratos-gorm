package ai

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// toolAwareModel buffers one DeepSeek stream. DeepSeek emits cumulative
// messages, not deltas, so the last chunk is the authoritative message. With
// tool_calls we return that one message (ADK dispatches tools); without tools
// we diff cumulative content and replay real deltas to SSE.
type toolAwareModel struct {
	inner model.ToolCallingChatModel
}

func newToolAwareModel(inner model.ToolCallingChatModel) model.ToolCallingChatModel {
	return &toolAwareModel{inner: inner}
}

func (m *toolAwareModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &toolAwareModel{inner: inner}, nil
}

func (m *toolAwareModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, in, opts...)
}

func (m *toolAwareModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := m.inner.Stream(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var chunks []*schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, recvErr
		}
		if chunk != nil {
			chunks = append(chunks, chunk)
		}
	}
	if len(chunks) == 0 {
		return schema.StreamReaderFromArray([]*schema.Message{}), nil
	}
	last := chunks[len(chunks)-1]
	combined := mergeCumulativeChunks(chunks)
	if len(combined.ToolCalls) > 0 {
		return schema.StreamReaderFromArray([]*schema.Message{combined}), nil
	}
	// Repair adapters that emit true deltas instead of cumulative messages.
	previous := ""
	out := make([]*schema.Message, 0, len(chunks))
	for _, chunk := range chunks {
		delta := chunk.Content
		if strings.HasPrefix(chunk.Content, previous) {
			delta = strings.TrimPrefix(chunk.Content, previous)
		}
		previous = chunk.Content
		if delta == "" && chunk.ResponseMeta == nil {
			continue
		}
		out = append(out, &schema.Message{
			Role: schema.Assistant, Content: delta, ResponseMeta: chunk.ResponseMeta,
		})
	}
	if len(out) == 0 {
		out = append(out, last)
	}
	return schema.StreamReaderFromArray(out), nil
}

// mergeCumulativeChunks keeps the last cumulative content but merges tool-call
// fragments by Index; DeepSeek sends id/name first and argument fragments later.
func mergeCumulativeChunks(chunks []*schema.Message) *schema.Message {
	last := chunks[len(chunks)-1]
	out := &schema.Message{Role: schema.Assistant, Content: last.Content, ResponseMeta: last.ResponseMeta}
	byIndex := map[int]*schema.ToolCall{}
	for _, chunk := range chunks {
		for i := range chunk.ToolCalls {
			call := chunk.ToolCalls[i]
			key := i
			if call.Index != nil {
				key = *call.Index
			}
			merged := byIndex[key]
			if merged == nil {
				merged = &schema.ToolCall{Index: call.Index}
				byIndex[key] = merged
			}
			if call.ID != "" {
				merged.ID = call.ID
			}
			if call.Type != "" {
				merged.Type = call.Type
			}
			if call.Function.Name != "" {
				merged.Function.Name = call.Function.Name
			}
			merged.Function.Arguments += call.Function.Arguments
		}
	}
	keys := make([]int, 0, len(byIndex))
	for key := range byIndex {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		out.ToolCalls = append(out.ToolCalls, *byIndex[key])
	}
	return out
}
