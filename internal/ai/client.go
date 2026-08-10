package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/claymor333/squal/internal/config"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Args string `json:"args"` // raw JSON object string
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Params      map[string]any `json:"parameters"`
}

type toolCallWire struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type messageWire struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []toolCallWire `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []messageWire `json:"messages"`
	Tools    []toolDefWire `json:"tools,omitempty"`
}

type toolDefWire struct {
	Type     string  `json:"type"`
	Function ToolDef `json:"function"`
}

type chatResponse struct {
	Choices []struct {
		Message messageWire `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Response is a normalized completion: either final text or tool calls.
type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

func New(cfg config.AI) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.Contains(base, "/v1") {
		base += "/v1"
	}
	return &Client{
		baseURL: base,
		model:   cfg.Model,
		apiKey:  cfg.APIKey,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	resp, err := c.CompleteTools(ctx, []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// CompleteTools issues a chat-completions request. When tools is non-empty it
// includes the tools parameter and parses tool_calls. Returns an error on HTTP
// failure (caller uses that to detect tools-unsupported and fall back).
func (c *Client) CompleteTools(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error) {
	var wires []messageWire
	for _, m := range msgs {
		wm := messageWire{Role: m.Role, Content: &m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			wm.ToolCalls = append(wm.ToolCalls, toolCallWire{
				ID: tc.ID, Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: tc.Name, Arguments: tc.Args},
			})
		}
		wires = append(wires, wm)
	}

	req := chatRequest{Model: c.model, Messages: wires}
	if len(tools) > 0 {
		for _, t := range tools {
			req.Tools = append(req.Tools, toolDefWire{Type: "function", Function: t})
		}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return Response{}, fmt.Errorf("ai: HTTP %d: %s", resp.StatusCode, trimErr(raw))
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Response{}, err
	}
	if len(cr.Choices) == 0 {
		return Response{}, fmt.Errorf("ai: empty response")
	}
	m := cr.Choices[0].Message
	out := Response{}
	if m.Content != nil {
		out.Content = strings.TrimSpace(*m.Content)
	}
	for _, tc := range m.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments})
	}
	return out, nil
}

// ToolsSupported probes whether the endpoint accepts the tools parameter.
func (c *Client) ToolsSupported(ctx context.Context) bool {
	_, err := c.CompleteTools(ctx, []Message{{Role: "user", Content: "ping"}}, []ToolDef{{Name: "_probe", Description: "probe", Params: map[string]any{"type": "object"}}})
	if err == nil {
		return true
	}
	// A 4xx or malformed response means the endpoint rejected tools; treat as unsupported.
	return false
}

func trimErr(raw []byte) string {
	var e chatResponse
	_ = json.Unmarshal(raw, &e)
	if e.Error != nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(raw))
}
