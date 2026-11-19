package ai

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// adkStreamRunner adapts an ADK ChatModelAgent to the message stream the
// existing gRPC/SSE layer consumes. Tool completions become Tool messages; the
// service turns those into `tool` SSE events. Token usage stays on the
// assistant messages' ResponseMeta and is accumulated by the service.
type adkStreamRunner struct {
	agent adk.Agent
}

func newADKStreamRunner(agent adk.Agent) *adkStreamRunner {
	return &adkStreamRunner{agent: agent}
}

func (r *adkStreamRunner) Stream(ctx context.Context, input []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	// DeepSeek's streamed preamble can end a turn before tool_calls are
	// dispatched. Run the ADK agent non-streaming for correct tool routing; the
	// gRPC SSE layer still emits the final answer and tool events.
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: r.agent, EnableStreaming: false})
	iter := runner.Run(ctx, input)
	out, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer writer.Close()
		for {
			event, ok := iter.Next()
			if !ok {
				return
			}
			if event.Err != nil {
				writer.Send(nil, event.Err)
				return
			}
			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}
			variant := event.Output.MessageOutput
			if variant.Role == schema.Tool {
				if variant.ToolName != "" {
					content := ""
					if variant.Message != nil {
						content = variant.Message.Content
					}
					writer.Send(schema.ToolMessage(content, "", schema.WithToolName(variant.ToolName)), nil)
				}
				continue
			}
			if variant.IsStreaming && variant.MessageStream != nil {
				for {
					chunk, err := variant.MessageStream.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						writer.Send(nil, err)
						return
					}
					if chunk != nil {
						if closed := writer.Send(chunk, nil); closed {
							return
						}
					}
				}
				continue
			}
			if variant.Message != nil {
				if closed := writer.Send(variant.Message, nil); closed {
					return
				}
			}
		}
	}()
	return out, nil
}

func buildADKAgent(ctx context.Context, cfg ProviderConfig, d Deps, admin bool) (*adkStreamRunner, error) {
	cm, err := newChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	builtTools, err := newTools(d)
	if err != nil {
		return nil, err
	}
	instruction := systemPrompt
	name := "mall-assistant"
	if admin {
		builtTools, err = newAdminTools(d)
		if err != nil {
			return nil, err
		}
		instruction = adminSystemPrompt
		name = "mall-admin"
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        name,
		Description: "Mall assistant",
		Instruction: instruction,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools:               builtTools,
				ExecuteSequentially: true,
				UnknownToolsHandler: func(context.Context, string, string) (string, error) {
					return "", fmt.Errorf("unknown tool")
				},
			},
		},
		MaxIterations: 12,
	})
	if err != nil {
		return nil, fmt.Errorf("build adk agent: %w", err)
	}
	return newADKStreamRunner(agent), nil
}
