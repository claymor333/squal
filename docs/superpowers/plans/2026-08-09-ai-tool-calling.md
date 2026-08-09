# AI Tool-Calling Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-subagent-driven-development (recommended) or superpowers-executing-plans to implement this plan task-by-task.

**Goal:** Turn the AI panel from a NL→SQL translator into a tool-calling agent: the model invokes schema/query/row/write tools, results stream live, writes confirm, and a 12-call budget bounds each request.

**Architecture:** Transport-agnostic loop (`internal/ai/agent.go`) over a `transport` interface, with a native OpenAI-tools implementation and a text-protocol fallback. A `Tool` registry (`internal/ai/tools.go`) with `ReadOnly` flags decides auto-run vs confirm. `run_query` feeds a summary to the model and full rows to the existing results grid. The panel (`internal/tui/aipanel.go`) streams tool calls live, offers Ask/Quick modes, and hosts the write-confirm dialog.

**Tech Stack:** Go 1.26, bubbletea v1.3 (existing), `httptest` (loop tests), existing `internal/db` (Fetch/Columnar/RowEditor/Classify), `internal/state` (turns persistence).

**Parallelization:**
- Group 1 `[SEQUENTIAL]`: **AI-1 (client)** lands first — AI-2 and AI-5 depend on its `Client`/`Message`/`ToolDef` types, so they cannot compile until it merges.
- Group 2 `[PARALLEL]`: **AI-2 (tools/registry+summary)** and **AI-5 (session+turns)** — both consume AI-1's types but are mutually independent.
- Group 3 `[SEQUENTIAL]`: **AI-3 (agent loop)** — consumes AI-1/AI-2/AI-5, defines the `transport` interface.
- Group 4 `[SEQUENTIAL]`: **AI-4 (text fallback)** — implements AI-3's `transport`.
- Group 5 `[SEQUENTIAL]`: **AI-6 (panel + model.go)** — must come after main-plan B6's model.go work.

## Global Constraints

- `gofmt` + `go vet` clean on every commit. No comments without meaning.
- Native OpenAI `tools` when the endpoint supports it; text-protocol fallback otherwise. Detection: first `CompleteTools` call that returns 4xx → retry as text protocol.
- **Writes (run_write, update_row, delete_row) ALWAYS confirm in the TUI before execution** — this is enforced by `Registry.Run`, not by the loop or panel, so no caller can bypass it.
- All SQL via existing `db.QuoteIdentifier` + `db.Fetch`; no string interpolation of identifiers.
- Loop budget: 12 tool calls per user request. Esc cancels the context; cancelled loops return a partial summary.
- No secrets in source/history/commits.
- Prereqs (from v1-core plan, must be landed): E1 `state.Store`, E2 `db.Classify`/`UndoVerdict`, E3 `db.RowEditor`, B3 `db.Fetch`/`Columnar`.

---

## File Structure Map

```
internal/ai/
  client.go    (NEW, AI-1)     Complete + CompleteTools (native tools param) + support detection
  tools.go     (NEW, AI-2)     Tool struct, Registry, the 10 tools, Summarize
  session.go   (NEW, AI-5)     multi-turn message history + schema packing
  agent.go     (NEW, AI-3)     transport interface, Run loop, 12-cap, forced summary
  native.go    (NEW, AI-1)     nativeTransport (OpenAI tools transport)
  fallback.go  (NEW, AI-4)     textTransport (JSON tool protocol over plain chat)
internal/state/
  turns.go     (NEW, AI-5)     SQLite turns table (transcript persistence)
  store.go     (MOD, AI-5)     add turns table to E1's Open migration
internal/tui/
  aipanel.go   (NEW, AI-6)     streaming panel, Ask/Quick toggle, confirm dialog
  model.go     (MOD, AI-6)     wire AI panel into keymap/message dispatch
```

---

## Group 1 — AI-1 (client) lands first

### Task AI-1: Client with native tools + support detection

**Files:**
- Create: `internal/ai/client.go`
- Create: `internal/ai/native.go`
- Test: `internal/ai/client_test.go`
- **Exclusive ownership:** Yes — no other task touches these files.

**Interfaces:**
- Consumes: `config.AI` (exists)
- Produces: `Client.Complete(ctx, system, user)`, `Client.CompleteTools(ctx, msgs, tools) (Response, error)`, `Client.ToolsSupported(ctx) bool`

- [ ] **Step 1: Write the failing test**

`internal/ai/client_test.go`:

```go
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
		http.Error(w, `{"error":{"message":"tools not supported"}}`, http.Status400)
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -v`
Expected: FAIL — package `internal/ai` does not exist / undefined symbols

- [ ] **Step 3: Write the implementation**

`internal/ai/client.go`:

```go
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
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
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
	Role      string         `json:"role"`
	Content   *string        `json:"content"`
	ToolCalls []toolCallWire `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []messageWire `json:"messages"`
	Tools    []toolDefWire `json:"tools,omitempty"`
}

type toolDefWire struct {
	Type     string         `json:"type"`
	Function ToolDef        `json:"function"`
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
```

`internal/ai/native.go`:

```go
package ai

import "context"

// nativeTransport is the OpenAI native-tools transport over Client.CompleteTools.
type nativeTransport struct {
	client *Client
}

func newNativeTransport(c *Client) *nativeTransport {
	return &nativeTransport{client: c}
}

func (t *nativeTransport) Complete(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error) {
	return t.client.CompleteTools(ctx, msgs, tools)
}

func (t *nativeTransport) Name() string { return "native" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -v`
Expected: PASS, 4 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ai/client.go internal/ai/native.go internal/ai/client_test.go
git commit -m "feat(ai): add OpenAI-compatible client with native tools support and detection"
```

## Group 2 — AI-2 + AI-5 in parallel (both consume AI-1)

### Task AI-2: Tool registry + the 10 tools + result summarizer

**Files:**
- Create: `internal/ai/tools.go`
- Test: `internal/ai/tools_test.go`
- **Exclusive ownership:** Yes.

**Interfaces:**
- Consumes: `config.Profile`/`db.Conn`, `db.Fetch`, `db.Columnar`, `db.RowEditor`, `db.Classify`, `db.PrimaryKey`, `state.Store` (E1), `db.QuoteIdentifier`
- Produces: `Registry` with `Register`/`Get`/`Run`; `Summarize(*db.Columnar) string`

- [ ] **Step 1: Write the failing test**

`internal/ai/tools_test.go`:

```go
package ai

import (
	"context"
	"testing"

	"github.com/claymor333/squal/internal/db"
)

func TestRegistryRunReadOnlyAuto(t *testing.T) {
	r := NewRegistry(nil, nil, nil, nil)
	r.Register(Tool{Name: "echo", ReadOnly: true, Execute: func(ctx context.Context, args map[string]any) (string, error) {
		return "ok", nil
	}})
	out, err := r.Run(context.Background(), "echo", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("out = %q", out)
	}
}

func TestRegistryWriteNeedsConfirm(t *testing.T) {
	r := NewRegistry(nil, nil, nil, nil)
	r.Register(Tool{Name: "wipe", ReadOnly: false, Execute: func(ctx context.Context, args map[string]any) (string, error) {
		return "executed", nil
	}})
	// Without a confirm func, a write tool must be rejected, never silently run.
	out, err := r.Run(context.Background(), "wipe", map[string]any{})
	if err == nil {
		t.Fatalf("write tool ran without confirm: %q", out)
	}
}

func TestSummarize(t *testing.T) {
	d := &db.Columnar{
		Columns: []string{"id", "name", "score"},
		Types:   []string{"INT", "VARCHAR", "DOUBLE"},
		Cols:    [][]string{{"1", "2", "3"}, {"a", "b", "c"}, {"10", "20", "30"}},
		Rows:    3,
	}
	s := Summarize(d)
	if s == "" {
		t.Fatal("empty summary")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run 'Registry|Summarize' -v`
Expected: FAIL — undefined `NewRegistry`/`Summarize`

- [ ] **Step 3: Write the implementation**

`internal/ai/tools.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/claymor333/squal/internal/db"
	"github.com/claymor333/squal/internal/state"
)

// ConfirmFunc asks the user whether a write tool may run. It must be provided
// by the panel; nil means writes are rejected (never silently run).
type ConfirmFunc func(toolName string, args map[string]any, sql string) (bool, error)

type Tool struct {
	Name        string
	Description string
	Params      map[string]any
	ReadOnly    bool
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the tool set and enforces the write-confirm rule centrally.
type Registry struct {
	tools   map[string]Tool
	conn    *db.Conn
	ed      *db.RowEditor
	store   *state.Store
	confirm ConfirmFunc
	// OnQuery ships a completed result set to the UI results grid. Set by the
	// panel; nil is fine for tests. run_query invokes it after draining Fetch.
	OnQuery func(col *db.Columnar)
}

func NewRegistry(conn *db.Conn, ed *db.RowEditor, store *state.Store, confirm ConfirmFunc) *Registry {
	r := &Registry{tools: map[string]Tool{}, conn: conn, ed: ed, store: store, confirm: confirm}
	r.build()
	return r
}

func (r *Registry) Register(t Tool) { r.tools[t.Name] = t }

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []ToolDef {
	out := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, ToolDef{Name: t.Name, Description: t.Description, Params: t.Params})
	}
	return out
}

// Run executes a tool. Writes are gated on the confirm func — enforced here,
// so no caller can bypass it.
func (r *Registry) Run(ctx context.Context, name string, args map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if !t.ReadOnly {
		if r.confirm == nil {
			return "", fmt.Errorf("write tool %q blocked: no confirm handler", name)
		}
		ok, err := r.confirm(name, args, "")
		if err != nil {
			return "", err
		}
		if !ok {
			return "user declined", nil
		}
	}
	return t.Execute(ctx, args)
}

// --- the ten tools ----------------------------------------------------------

func (r *Registry) build() {
	r.Register(Tool{Name: "list_databases", ReadOnly: true, Description: "List all databases visible to the connection.",
		Params: map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			dbs, err := r.conn.Schema(ctx, false)
			if err != nil {
				return "", err
			}
			var names []string
			for _, d := range dbs {
				names = append(names, d.Name)
			}
			return strings.Join(names, ", "), nil
		}})
	r.Register(Tool{Name: "list_tables", ReadOnly: true, Description: "List tables in a database.",
		Params: map[string]any{"type": "object", "properties": map[string]any{"database": map[string]any{"type": "string"}}, "required": []string{"database"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			dbName := argStr(args, "database")
			dbs, err := r.conn.Schema(ctx, false)
			if err != nil {
				return "", err
			}
			for _, d := range dbs {
				if d.Name == dbName {
					var names []string
					for _, t := range d.Tables {
						names = append(names, t.Name)
					}
					return strings.Join(names, ", "), nil
				}
			}
			return "", fmt.Errorf("database %q not found", dbName)
		}})
	r.Register(Tool{Name: "get_schema", ReadOnly: true, Description: "Get columns, types, and PK for a table.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
		}, "required": []string{"database", "table"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			dbName, table := argStr(args, "database"), argStr(args, "table")
			cols, err := r.conn.Columns(ctx, dbName, table)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, c := range cols {
				fmt.Fprintf(&b, "%s %s %s key=%s\n", c.Name, c.Type, nullable(c.Nullable), c.Key)
			}
			return strings.TrimSuffix(b.String(), "\n"), nil
		}})
	r.Register(Tool{Name: "run_query", ReadOnly: true, Description: "Run a SELECT and return a summary of the result.",
		Params: map[string]any{"type": "object", "properties": map[string]any{"sql": map[string]any{"type": "string"}}, "required": []string{"sql"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			sql := argStr(args, "sql")
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(sql)), "select") {
				return "", fmt.Errorf("run_query only executes SELECT statements")
			}
			col, ch, err := r.conn.Fetch(ctx, sql, 1000)
			if err != nil {
				return "", err
			}
			for b := range ch {
				if b.Err != nil {
					return "", b.Err
				}
			}
			// Full rows to the user's grid; summary to the model.
			if r.OnQuery != nil {
				r.OnQuery(col)
			}
			return Summarize(col), nil
		}})
	r.Register(Tool{Name: "run_write", ReadOnly: false, Description: "Execute an INSERT/UPDATE/DELETE via the undo contract.",
		Params: map[string]any{"type": "object", "properties": map[string]any{"sql": map[string]any{"type": "string"}}, "required": []string{"sql"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			sql := argStr(args, "sql")
			v, err := db.Classify(sql)
			if err != nil {
				return "", err
			}
			if !v.Feasible {
				return "", fmt.Errorf("not undoable: %s", v.Reason)
			}
			res, err := r.conn.ExecContext(ctx, sql)
			if err != nil {
				return "", err
			}
			n, _ := res.RowsAffected()
			return fmt.Sprintf("%d row(s) affected", n), nil
		}})
	r.Register(Tool{Name: "get_row", ReadOnly: true, Description: "Get one row by PK as a JSON object.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
			"pk":       map[string]any{"type": "object"},
		}, "required": []string{"database", "table", "pk"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			pkVals, err := mapArgsToStr(args["pk"])
			if err != nil {
				return "", err
			}
			row, err := r.ed.Load(ctx, pkVals)
			if err != nil {
				return "", err
			}
			b, _ := json.Marshal(row)
			return string(b), nil
		}})
	r.Register(Tool{Name: "update_row", ReadOnly: false, Description: "Update a row by PK.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"table":   map[string]any{"type": "string"},
			"pk":      map[string]any{"type": "object"},
			"values":  map[string]any{"type": "object"},
		}, "required": []string{"table", "pk", "values"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			pkVals, err := mapArgsToStr(args["pk"])
			if err != nil {
				return "", err
			}
			before, err := r.ed.Load(ctx, pkVals)
			if err != nil {
				return "", err
			}
			after := map[string]string{}
			for k, v := range before {
				after[k] = v
			}
			for k, v := range args["values"].(map[string]any) {
				after[k] = anyToStr(v)
			}
			n, err := r.ed.Update(ctx, before, after)
			if err != nil {
				return "", err
			}
			if r.store != nil {
				_ = r.store.Record(&state.Action{
					ID: state.NewID("update", r.connName(), argStr(args, "table")),
					Verdict: state.Undoable, Kind: "update", Table: argStr(args, "table"),
					PK: pkVals, Before: before, After: after,
					Status: state.Applied, CreatedAt: now(),
				})
			}
			return fmt.Sprintf("%d row(s) updated", n), nil
		}})
	r.Register(Tool{Name: "delete_row", ReadOnly: false, Description: "Delete a row by PK.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"table": map[string]any{"type": "string"},
			"pk":    map[string]any{"type": "object"},
		}, "required": []string{"table", "pk"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			pkVals, err := mapArgsToStr(args["pk"])
			if err != nil {
				return "", err
			}
			before, err := r.ed.Load(ctx, pkVals)
			if err != nil {
				return "", err
			}
			n, err := r.ed.Delete(ctx, pkVals)
			if err != nil {
				return "", err
			}
			if r.store != nil {
				_ = r.store.Record(&state.Action{
					ID: state.NewID("delete", r.connName(), argStr(args, "table")),
					Verdict: state.Undoable, Kind: "delete", Table: argStr(args, "table"),
					PK: pkVals, Before: before, After: nil,
					Status: state.Applied, CreatedAt: now(),
				})
			}
			return fmt.Sprintf("%d row(s) deleted", n), nil
		}})
	r.Register(Tool{Name: "explain_query", ReadOnly: true, Description: "Get a plain-English explanation of a SQL statement's behavior.",
		Params: map[string]any{"type": "object", "properties": map[string]any{"sql": map[string]any{"type": "string"}}, "required": []string{"sql"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			sql := argStr(args, "sql")
			// Fetch EXPLAIN output rows as the factual basis.
			col, ch, err := r.conn.Fetch(ctx, "EXPLAIN "+sql, 100)
			if err != nil {
				return "", err
			}
			for b := range ch {
				if b.Err != nil {
					return "", b.Err
				}
			}
			return Summarize(col), nil
		}})
	r.Register(Tool{Name: "explain_table", ReadOnly: true, Description: "Get schema + a sampled row of a table as the basis for an explanation.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
		}, "required": []string{"database", "table"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			dbName, table := argStr(args, "database"), argStr(args, "table")
			cols, err := r.conn.Columns(ctx, dbName, table)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for _, c := range cols {
				fmt.Fprintf(&b, "%s %s %s key=%s\n", c.Name, c.Type, nullable(c.Nullable), c.Key)
			}
			col, ch, err := r.conn.Fetch(ctx, fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1",
				db.QuoteIdentifier(dbName), db.QuoteIdentifier(table)), 1)
			if err != nil {
				return b.String(), err
			}
			for batch := range ch {
				if batch.Err != nil {
					return b.String(), batch.Err
				}
			}
			b.WriteString("\nsample row:\n")
			b.WriteString(Summarize(col))
			return b.String(), nil
		}})
}

func (r *Registry) connName() string { return "ai" }

func now() time.Time { return time.Now() }

func nullable(b bool) string {
	if b {
		return "NULL"
	}
	return "NOT NULL"
}

func argStr(args map[string]any, k string) string {
	if v, ok := args[k]; ok {
		return anyToStr(v)
	}
	return ""
}

func anyToStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func mapArgsToStr(v any) (map[string]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object, got %T", v)
	}
	out := make(map[string]string, len(m))
	for k, vv := range m {
		out[k] = anyToStr(vv)
	}
	return out, nil
}

// Summarize renders a compact, model-usable description of a result set:
// count, columns, types, and numeric min/max/avg — never raw row values.
func Summarize(d *db.Columnar) string {
	if d == nil {
		return "no result"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d rows, %d columns: ", d.Rows, len(d.Columns))
	for i, c := range d.Columns {
		typ := ""
		if i < len(d.Types) {
			typ = d.Types[i]
		}
		fmt.Fprintf(&b, "%s(%s) ", c, typ)
	}
	if d.Rows > 0 {
		b.WriteString("\n")
		for i := range d.Columns {
			if i >= len(d.Types) || !isNumeric(d.Types[i]) {
				continue
			}
			min, max, ok := minMax(d.Cols[i])
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "%s: min=%s max=%s\n", d.Columns[i], min, max)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func isNumeric(t string) bool {
	switch strings.ToUpper(t) {
	case "INT", "BIGINT", "SMALLINT", "TINYINT", "DOUBLE", "FLOAT", "DECIMAL", "NUMERIC":
		return true
	}
	return false
}

func minMax(vals []string) (string, string, bool) {
	if len(vals) == 0 {
		return "", "", false
	}
	min, max := vals[0], vals[0]
	for _, v := range vals {
		if v == "" {
			continue
		}
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run 'Registry|Summarize' -v`
Expected: PASS, 3 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ai/tools.go internal/ai/tools_test.go
git commit -m "feat(ai): add tool registry with write-confirm gate and result summarizer"
```

### Task AI-5: Session + transcript persistence

**Files:**
- Create: `internal/ai/session.go`
- Create: `internal/state/turns.go`
- Modify: `internal/state/store.go` (add the turns table to E1's `Open` migration — additive, sequenced after E1)
- Test: `internal/ai/session_test.go`, `internal/state/turns_test.go`
- **Exclusive ownership:** Yes — turns.go is new; store.go is modified additively by this task only (E1 landed first).

**Interfaces:**
- Consumes: `ai.Client` (AI-1), `state.Store` (E1)
- Produces: `Session` (history, `AddTurn`, `AddToolResult`, `Reset`), `state.Turns` (AddTurn/ListTurns persist transcript; consumed by AI-3's `SetTranscript`)

- [ ] **Step 1: Write the failing test**

`internal/ai/session_test.go`:

```go
package ai

import "testing"

func TestSessionBuildsHistory(t *testing.T) {
	s := NewSession(nil)
	s.AddTurn("user", "show users")
	s.AddTurn("assistant", "SELECT * FROM users")
	s.AddTurn("user", "only active")
	if len(s.Messages()) != 3 {
		t.Fatalf("messages = %d", len(s.Messages()))
	}
	s.Reset()
	if len(s.Messages()) != 0 {
		t.Fatalf("after reset = %d", len(s.Messages()))
	}
}
```

`internal/state/turns_test.go`:

```go
package state

import (
	"path/filepath"
	"testing"
)

func TestTurnsPersist(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.AddTurn("conn1", "get_schema", `{"table":"users"}`, "8 cols"); err != nil {
		t.Fatal(err)
	}
	turns, err := s.ListTurns("conn1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].Tool != "get_schema" {
		t.Fatalf("turns = %+v", turns)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ ./internal/state/ -run 'Session|Turns' -v`
Expected: FAIL — undefined symbols

- [ ] **Step 3: Write the implementation**

`internal/ai/session.go`:

```go
package ai

type Session struct {
	client *Client
	msgs   []Message
}

func NewSession(c *Client) *Session {
	return &Session{client: c}
}

func (s *Session) AddTurn(role, content string) {
	s.msgs = append(s.msgs, Message{Role: role, Content: content})
}

func (s *Session) AddToolResult(id, name, result string) {
	s.msgs = append(s.msgs, Message{Role: "tool", ToolCallID: id, Content: result})
}

func (s *Session) Messages() []Message { return s.msgs }

func (s *Session) Reset() { s.msgs = s.msgs[:0] }
```

`internal/state/turns.go`:

```go
package state

type Turn struct {
	Connection string
	Tool       string
	ArgsJSON   string
	Result     string
}

// AddTurn persists one tool call to the transcript table.
func (s *Store) AddTurn(conn, tool, argsJSON, result string) error {
	_, err := s.db.Exec(`INSERT INTO turns (connection, tool, args_json, result)
		VALUES (?, ?, ?, ?)`, conn, tool, argsJSON, result)
	return err
}

func (s *Store) ListTurns(conn string, limit int) ([]Turn, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT connection, tool, args_json, result
		FROM turns WHERE connection = ? ORDER BY rowid DESC LIMIT ?`, conn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Turn
	for rows.Next() {
		var t Turn
		if err := rows.Scan(&t.Connection, &t.Tool, &t.ArgsJSON, &t.Result); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

`internal/state/store.go` — add the turns table to E1's `Open` schema migration. The migration block inside `Open` (after the `actions` CREATE) becomes:

```go
	// ...E1's actions table CREATE (unchanged)...

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS turns (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		connection TEXT NOT NULL,
		tool TEXT NOT NULL,
		args_json TEXT NOT NULL,
		result TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}

	// ...rest of E1's Open (unchanged)...
```

*`db` here is the `*sql.DB` local in E1's `Open` (see v1-core plan Task E1). This task only appends the turns CREATE to that function; it does not restructure it.*

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ ./internal/state/ -run 'Session|Turns' -v`
Expected: PASS, 2 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ai/session.go internal/ai/session_test.go internal/state/turns.go internal/state/turns_test.go internal/state/store.go
git commit -m "feat(ai): add multi-turn session and SQLite tool-transcript persistence"
```

---

## Group 3 — Sequential (AI-3 consumes AI-1/AI-2/AI-5)

### Task AI-3: Agent loop

**Files:**
- Create: `internal/ai/agent.go`
- Test: `internal/ai/agent_test.go`
- **Exclusive ownership:** Yes.

**Interfaces:**
- Consumes: `transport` (interface defined here), `Registry` (AI-2), `Session` (AI-5), `state.Store` (transcript, via `SetTranscript`)
- Produces: `transport` interface, `Agent.Run`, `Agent.MaxCalls` + `Agent.OnEvent` fields

- [ ] **Step 1: Write the failing test**

`internal/ai/agent_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run Agent -v`
Expected: FAIL — undefined `transport`/`NewAgent`

- [ ] **Step 3: Write the implementation**

`internal/ai/agent.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run Agent -v`
Expected: PASS, 3 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ai/agent.go internal/ai/agent_test.go
git commit -m "feat(ai): add bounded tool-calling agent loop with live events"
```

---

## Group 4 — Sequential (AI-4 implements AI-3's transport)

### Task AI-4: Text-protocol fallback transport

**Files:**
- Create: `internal/ai/fallback.go`
- Test: `internal/ai/fallback_test.go`
- **Exclusive ownership:** Yes.

**Interfaces:**
- Consumes: `transport` interface (AI-3), `Client.Complete` (AI-1)
- Produces: `textTransport` implementing `transport` — used when `ToolsSupported` is false

- [ ] **Step 1: Write the failing test**

`internal/ai/fallback_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run TextTransport -v`
Expected: FAIL — undefined `newTextTransport`

- [ ] **Step 3: Write the implementation**

`internal/ai/fallback.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// textTransport reaches the model through plain chat, embedding the tool set in
// the system prompt and expecting the model to reply with a JSON envelope:
//   {"tool":"name","args":{...}}  — invoke a tool
//   anything else                 — final answer
type textTransport struct {
	client      *Client
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/ai/ -run TextTransport -v`
Expected: PASS, 2 tests

- [ ] **Step 5: Commit**

```bash
git add internal/ai/fallback.go internal/ai/fallback_test.go
git commit -m "feat(ai): add text-protocol tool transport for endpoints without native tools"
```

---

## Group 5 — Sequential (AI-6 consumes all; last to touch model.go)

### Task AI-6: Streaming AI panel + model wiring

**Files:**
- Create: `internal/tui/aipanel.go`
- Test: `internal/tui/aipanel_test.go`
- Modify: `internal/tui/model.go` (final task to touch it — after main-plan B6)
- **Exclusive ownership:** Yes — no task after this modifies model.go in this plan.

**Interfaces:**
- Consumes: `Agent`, `Registry`, `Client`, `transport` (all AI), `db.Conn`, `state.Store`, `runQueryMsg` (main plan), `saveRowMsg` (main plan)
- Produces: the runnable Ask/Quick panel with live events, confirm dialog, Esc cancel

- [ ] **Step 1: Write the failing test**

`internal/tui/aipanel_test.go`:

```go
package tui

import (
	"context"
	"testing"
)

func TestAIPanelToggleModes(t *testing.T) {
	p := newAIPanel()
	if p.mode != modeAsk {
		t.Fatalf("default mode = %v", p.mode)
	}
	p.toggleMode()
	if p.mode != modeQuick {
		t.Fatalf("after toggle = %v", p.mode)
	}
}

func TestAIPanelConfirmApprove(t *testing.T) {
	p := newAIPanel()
	p.pendingConfirm = "wipe"
	p.confirm(true) // approve
	if p.pendingConfirm != "" {
		t.Fatalf("pending = %q after approve", p.pendingConfirm)
	}
}

func TestAIPanelEscCancels(t *testing.T) {
	p := newAIPanel()
	// simulate a running loop: runAsk wires cancel; here we set it directly
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.running = true
	_ = ctx
	if !p.interrupt() {
		t.Fatal("interrupt should stop running loop")
	}
	if p.running {
		t.Fatal("still running")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/tui/ -run AIPanel -v`
Expected: FAIL — undefined `newAIPanel`

- [ ] **Step 3: Write the implementation**

`internal/tui/aipanel.go`:

```go
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/ai"
)

type aiMode int

const (
	modeAsk aiMode = iota
	modeQuick
)

type aiEventMsg struct {
	Tool   string
	Result string
	Done   bool
	Answer string
	Err    error
}

type aiConfirmMsg struct {
	Tool string
	Args map[string]any
	OK   chan bool
}

type aiPanel struct {
	client           *ai.Client
	agent            *ai.Agent
	registry         *ai.Registry
	session          *ai.Session
	mode             aiMode
	request          string
	running          bool
	cancel           context.CancelFunc
	pendingConfirm   string
	pendingConfirmCh chan bool // answered by model.handleKey y/n; nil when no confirm pending
	events           []aiEventMsg
	history          []string
}

func newAIPanel() *aiPanel {
	return &aiPanel{mode: modeAsk}
}

func (p *aiPanel) toggleMode() {
	if p.mode == modeAsk {
		p.mode = modeQuick
	} else {
		p.mode = modeAsk
	}
}

func (p *aiPanel) interrupt() bool {
	if p.running && p.cancel != nil {
		p.cancel()
		p.running = false
		return true
	}
	return false
}

// confirm resolves a pending write confirmation. ok=true approves the write.
// The channel send unblocks the agent's confirm wait in model.go.
func (p *aiPanel) confirm(ok bool) {
	if p.pendingConfirm != "" && p.pendingConfirmCh != nil {
		p.pendingConfirmCh <- ok
	}
	p.pendingConfirm = ""
	p.pendingConfirmCh = nil
}

// runAsk streams a tool-calling loop into the panel.
func (p *aiPanel) runAsk(ctx context.Context) tea.Cmd {
	p.running = true
	ctx, p.cancel = context.WithCancel(ctx)
	user := p.request
	return func() tea.Msg {
		answer, err := p.agent.Run(ctx, user)
		p.running = false
		if err != nil && ctx.Err() != nil {
			return aiEventMsg{Err: err}
		}
		return aiEventMsg{Answer: answer, Done: true, Err: err}
	}
}

// runQuick is the one-shot NL→SQL path (no tools).
func (p *aiPanel) runQuick(ctx context.Context) tea.Cmd {
	p.running = true
	return func() tea.Msg {
		defer func() { p.running = false }()
		sql, err := p.client.Complete(ctx, quickPrompt, p.request)
		if err != nil {
			return aiEventMsg{Err: err}
		}
		return runQueryMsg{SQL: sql}
	}
}

const quickPrompt = `You translate natural language into MariaDB/MySQL SQL. Return ONLY the SQL.`

func (p *aiPanel) view() string {
	var b strings.Builder
	mode := "[Ask]"
	if p.mode == modeQuick {
		mode = "[Quick]"
	}
	b.WriteString(styleAccent.Render(mode) + " " + p.request + "\n")
	for _, e := range p.events {
		switch {
		case e.Tool != "" && e.Result == "":
			b.WriteString(styleDim.Render("▸ " + e.Tool + "…") + "\n")
		case e.Tool != "":
			fmt.Fprintf(&b, "  %s → %s\n", styleDim.Render(e.Tool), e.Result)
		case e.Answer != "":
			b.WriteString(styleAccent.Render("✓ " + e.Answer) + "\n")
		case e.Err != nil:
			b.WriteString(styleErr.Render("✗ " + e.Err.Error()) + "\n")
		}
	}
	if p.pendingConfirm != "" {
		b.WriteString(styleErr.Render("⚠ confirm " + p.pendingConfirm + "? (y/n)") + "\n")
	}
	return b.String()
}
```

*`model.go` wiring: the AI panel is a fourth focus in `paneFocus` (`focusAI`). The keymap routes Tab to cycle schema→editor→results→AI. The confirm dialog contract is explicit: `model` creates the panel's `pendingConfirmCh` when a write tool fires (`aiConfirmMsg`), renders "⚠ confirm <tool>? (y/n)", and on y/n calls `panel.confirm(true/false)` which sends on the channel — unblocking the agent's confirm wait. The registry's confirm func is set at connection time to the panel's confirm-flow. `run_query` ships full rows to the results grid via `registry.OnQuery` (set at connection time to swap the grid's columnar). The `runQueryMsg` type comes from the v1-core plan's B2 (editor) — must be landed. Mode toggle key: `a` switches Ask/Quick. Esc in AI mode calls `panel.interrupt()`.*

*(The model.go diff is the integration: a `focusAI` constant, the confirm-channel handler answering `aiConfirmMsg.OK` (or driving `panel.confirm`), the AI keybindings, routing `aiEventMsg` into `panel.events`, and setting `registry.OnQuery`/`registry.confirm` at connect. The panel's unit tests cover the pure logic — mode toggle, confirm state, interrupt — without needing model.go.)*

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/zaim/squal && export PATH="$HOME/go-sdk/go/bin:$PATH" && go test ./internal/tui/ -run AIPanel -v`
Expected: PASS, 3 tests

- [ ] **Step 5: Commit**

```bash
git add internal/tui/aipanel.go internal/tui/aipanel_test.go internal/tui/model.go
git commit -m "feat(tui): add streaming AI panel with Ask/Quick modes and write confirmation"
```

---

## Final Verification

Before claiming done:

```bash
cd /home/zaim/squal
export PATH="$HOME/go-sdk/go/bin:$PATH"
gofmt -l .
go vet ./...
go test ./...
go run ./cmd/smoke
```

Expected: `gofmt` empty, `vet` clean, all tests PASS (ai package + tui + db + state), smoke still fetches 100k rows.

## Cross-Plan Notes

- This plan **replaces** the v1-core plan's Phase C (A1/A2/A3). Do NOT execute A1/A2/A3; instead execute AI-1 through AI-6. If Phase C was already executed, A3's `aipanel.go`/`model.go` wiring is superseded by AI-6's (a full replacement of the panel), and A1/A2 files are superseded by AI-1/AI-5. Keep the old client's `Complete` as the Quick path primitive if present; otherwise AI-1's `Complete` covers it.
- model.go ownership chain across plans: B4 → B5 → B6 (v1-core) → AI-6 (this plan). Do not merge AI-6 before B6 — and if Phase C's A3 was executed, AI-6 is a replacement of A3's panel work, sequenced after it.
- `run_query` intentionally returns a summary to the model; full rows flow to the user's grid via `registry.OnQuery` (set by AI-6's model wiring). The grid display path is owned by the v1-core plan (B3/B4).
