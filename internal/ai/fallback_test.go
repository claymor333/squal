package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/claymor333/squal/internal/config"
)

func TestTextTransportParsesToolJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"tool\":\"get_schema\",\"args\":{\"table\":\"users\"}}"}}]}`))
	}))
	defer srv.Close()

	c := New(config.AI{BaseURL: srv.URL, Model: "m"})
	tr := newTextTransport(c)
	resp, err := tr.Complete(context.Background(), []Message{{Role: "user", Content: "schema"}}, []ToolDef{{Name: "get_schema"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_schema" {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
}

func TestTextTransportPassesPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"just an answer"}}]}`))
	}))
	defer srv.Close()

	c := New(config.AI{BaseURL: srv.URL, Model: "m"})
	tr := newTextTransport(c)
	tools := []ToolDef{{Name: "get_schema", Description: "schema", Params: map[string]any{"type": "object"}}}
	resp, err := tr.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}}, tools)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "just an answer" {
		t.Fatalf("content = %q", resp.Content)
	}
	if !strings.Contains(tr.systemPrompt, "get_schema") {
		t.Fatal("system prompt should embed tool definitions")
	}
}
