package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/claymor333/squal/internal/state"
)

// MaxToolCalls bounds a single Run (spec §loop-budget: 12).
const MaxToolCalls = 12

// transport abstracts how the model is reached: native OpenAI tools or text
// protocol. Implementations must be stateless per-call.
type transport interface {
	Complete(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error)
	Name() string
}

// OnEvent receives live progress for the panel. Events must be delivered
// synchronously from the Run goroutine; return quickly.
type OnEvent func(ev Event)

type EventKind int

const (
	EventToolStart EventKind = iota
	EventToolResult
	EventWriteConfirm
	EventDone
)

type Event struct {
	Kind   EventKind
	Tool   string
	Result string
}

type Agent struct {
	transport transport
	registry  *Registry
	session   *Session
	store     *state.Store // optional transcript persistence (AI-5)
	connName  string
	MaxCalls  int
	OnEvent   OnEvent
}

func NewAgent(t transport, r *Registry, s *Session) *Agent {
	return &Agent{transport: t, registry: r, session: s, MaxCalls: MaxToolCalls}
}

// NewAgentForClient builds an agent over a Client, probing the endpoint for
// native tools support and selecting the transport accordingly: native when
// supported, text-protocol fallback otherwise. This is the factory the TUI
// uses so the fallback decision lives in the ai package, not the panel.
func NewAgentForClient(c *Client, r *Registry, s *Session) *Agent {
	t := transport(newNativeTransport(c))
	if !c.ToolsSupported(context.Background()) {
		t = newTextTransport(c)
	}
	return &Agent{transport: t, registry: r, session: s, MaxCalls: MaxToolCalls}
}

// SetTranscript enables persistence of each tool call into the state store.
func (a *Agent) SetTranscript(store *state.Store, connName string) {
	a.store = store
	a.connName = connName
}

// Run executes the tool-calling loop. Returns the final answer, or a forced
// summary when MaxCalls is exhausted, or ctx.Err() on cancel.
func (a *Agent) Run(ctx context.Context, user string) (string, error) {
	a.session.AddTurn("user", user)
	msgs := a.session.Messages()

	for i := 0; i < a.MaxCalls; i++ {
		resp, err := a.transport.Complete(ctx, msgs, a.registry.All())
		if err != nil {
			return "", err
		}
		if len(resp.ToolCalls) == 0 {
			a.session.AddTurn("assistant", resp.Content)
			return resp.Content, nil
		}
		for _, tc := range resp.ToolCalls {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if a.OnEvent != nil {
				a.OnEvent(Event{Kind: EventToolStart, Tool: tc.Name})
			}
			// errors-as-data: a failed tool returns its error as the result
			// string so the model sees it and can self-correct next iteration.
			result := a.runTool(ctx, tc)
			a.session.AddToolResult(tc.ID, tc.Name, result)
			if a.store != nil {
				_ = a.store.AddTurn(a.connName, tc.Name, tc.Args, result)
			}
			if a.OnEvent != nil {
				a.OnEvent(Event{Kind: EventToolResult, Tool: tc.Name, Result: result})
			}
		}
	}
	summary := "I reached my step limit without a final answer. Based on what I found: " +
		lastToolResults(a.session.Messages())
	a.session.AddTurn("assistant", summary)
	return summary, nil
}

// runTool executes one tool call and always returns a usable result string.
// Tool errors are encoded into the string ("error: ...") rather than returned,
// so they reach the model as data.
func (a *Agent) runTool(ctx context.Context, tc ToolCall) string {
	args := map[string]any{}
	if tc.Args != "" {
		if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
			return fmt.Sprintf("error: invalid tool args: %v", err)
		}
	}
	result, err := a.registry.Run(ctx, tc.Name, args)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

func lastToolResults(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "tool" && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return "no results"
}
