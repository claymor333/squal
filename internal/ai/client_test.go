package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/claymor333/squal/internal/config"
)

func TestCompletePlain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"SELECT 1"}}]}`))
	}))
	defer srv.Close()

	c := New(config.AI{BaseURL: srv.URL, Model: "m"})
	got, err := c.Complete(context.Background(), "sys", "ask")
	if err != nil {
		t.Fatal(err)
	}
	if got != "SELECT 1" {
		t.Fatalf("got %q", got)
	}
}

func TestCompleteToolsParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"get_schema","arguments":"{\"table\":\"users\"}"}}
		]}}]}`))
	}))
	defer srv.Close()

	c := New(config.AI{BaseURL: srv.URL, Model: "m"})
	resp, err := c.CompleteTools(context.Background(), []Message{{Role: "user", Content: "schema"}}, []ToolDef{{Name: "get_schema"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "get_schema" {
		t.Fatalf("name = %q", resp.ToolCalls[0].Name)
	}
}

func TestCompleteToolsErrorMeansUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"tools not supported"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := New(config.AI{BaseURL: srv.URL, Model: "m"})
	if c.ToolsSupported(context.Background()) {
		t.Fatal("expected unsupported")
	}
}

func TestClientBaseURLNormalization(t *testing.T) {
	c := New(config.AI{BaseURL: "http://localhost:11434", Model: "m"})
	if !strings.Contains(c.baseURL, "/v1") {
		t.Fatalf("baseURL = %q, want /v1", c.baseURL)
	}
}
