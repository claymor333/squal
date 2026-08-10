package ai

import (
	"context"
	"testing"
)

type fakeTransport struct {
	steps []Response
}

func (f *fakeTransport) Complete(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error) {
	if len(f.steps) == 0 {
		return Response{Content: "done"}, nil
	}
	r := f.steps[0]
	f.steps = f.steps[1:]
	return r, nil
}

func (f *fakeTransport) Name() string { return "fake" }

func TestAgentLoopCallsToolThenFinishes(t *testing.T) {
	reg := NewRegistry(nil, nil, nil, nil)
	reg.Register(Tool{Name: "echo", ReadOnly: true, Execute: func(ctx context.Context, args map[string]any) (string, error) {
		return "echoed", nil
	}})
	tr := &fakeTransport{steps: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "echo", Args: "{}"}}},
	}}
	a := NewAgent(tr, reg, NewSession(nil))
	out, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if out != "done" {
		t.Fatalf("out = %q", out)
	}
}

func TestAgentLoopCapForcesSummary(t *testing.T) {
	reg := NewRegistry(nil, nil, nil, nil)
	reg.Register(Tool{Name: "spin", ReadOnly: true, Execute: func(ctx context.Context, args map[string]any) (string, error) {
		return "again", nil
	}})
	tr := &fakeTransport{steps: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "spin", Args: "{}"}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "spin", Args: "{}"}}},
		{ToolCalls: []ToolCall{{ID: "3", Name: "spin", Args: "{}"}}},
	}}
	a := NewAgent(tr, reg, NewSession(nil))
	a.MaxCalls = 2
	out, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("expected forced summary, got empty")
	}
}

func TestAgentToolErrorBecomesResult(t *testing.T) {
	reg := NewRegistry(nil, nil, nil, nil)
	reg.Register(Tool{Name: "boom", ReadOnly: true, Execute: func(ctx context.Context, args map[string]any) (string, error) {
		return "", errTool
	}})
	tr := &fakeTransport{steps: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "boom", Args: "{}"}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "boom", Args: "{}"}}},
		{ToolCalls: []ToolCall{{ID: "3", Name: "boom", Args: "{}"}}},
	}}
	a := NewAgent(tr, reg, NewSession(nil))
	a.MaxCalls = 5
	_, err := a.Run(context.Background(), "go")
	if err != nil {
		t.Fatal(err) // errors become tool results, so the loop still terminates
	}
}

var errTool = errToolFn{}

type errToolFn struct{}

func (errToolFn) Error() string { return "boom" }
