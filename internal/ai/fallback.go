package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// textTransport reaches the model through plain chat, embedding the tool set in
// the system prompt and expecting the model to reply with a JSON envelope:
//
//	{"tool":"name","args":{...}}  — invoke a tool
//	anything else                 — final answer
type textTransport struct {
	client       *Client
	systemPrompt string
}

func newTextTransport(c *Client) *textTransport {
	return &textTransport{client: c}
}

func (t *textTransport) Name() string { return "text" }

func (t *textTransport) Complete(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error) {
	prompt := buildToolPrompt(tools)
	sys := msgs
	if len(sys) > 0 && sys[0].Role == "system" {
		sys[0].Content = prompt
	} else {
		sys = append([]Message{{Role: "system", Content: prompt}}, sys...)
	}
	content, err := t.client.completeRaw(ctx, sys)
	if err != nil {
		return Response{}, err
	}
	t.systemPrompt = prompt

	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "{") {
		var env struct {
			Tool string         `json:"tool"`
			Args map[string]any `json:"args"`
		}
		if err := json.Unmarshal([]byte(content), &env); err == nil && env.Tool != "" {
			argsJSON := "{}"
			if env.Args != nil {
				b, _ := json.Marshal(env.Args)
				argsJSON = string(b)
			}
			return Response{ToolCalls: []ToolCall{{ID: "text_1", Name: env.Tool, Args: argsJSON}}}, nil
		}
	}
	return Response{Content: content}, nil
}

// completeRaw is a minimal single-shot completion that returns the raw text.
func (c *Client) completeRaw(ctx context.Context, msgs []Message) (string, error) {
	resp, err := c.CompleteTools(ctx, msgs, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func buildToolPrompt(tools []ToolDef) string {
	var b strings.Builder
	b.WriteString("You are a database agent. You can call tools by replying with exactly one JSON object:\n")
	b.WriteString(`{"tool":"<name>","args":{...}}` + "\n")
	b.WriteString("Otherwise reply with your final answer as plain text. Available tools:\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- %s: %s\n  args schema: %s\n", t.Name, t.Description, mustJSON(t.Params))
	}
	return b.String()
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
