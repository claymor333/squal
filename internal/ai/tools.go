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
			ed, err := r.ensureEditor(argStr(args, "database"), argStr(args, "table"))
			if err != nil {
				return "", err
			}
			row, err := ed.Load(ctx, pkVals)
			if err != nil {
				return "", err
			}
			b, _ := json.Marshal(row)
			return string(b), nil
		}})
	r.Register(Tool{Name: "update_row", ReadOnly: false, Description: "Update a row by PK.",
		Params: map[string]any{"type": "object", "properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
			"pk":       map[string]any{"type": "object"},
			"values":   map[string]any{"type": "object"},
		}, "required": []string{"database", "table", "pk", "values"}},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			pkVals, err := mapArgsToStr(args["pk"])
			if err != nil {
				return "", err
			}
			ed, err := r.ensureEditor(argStr(args, "database"), argStr(args, "table"))
			if err != nil {
				return "", err
			}
			before, err := ed.Load(ctx, pkVals)
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
			n, err := ed.Update(ctx, before, after)
			if err != nil {
				return "", err
			}
			if r.store != nil {
				_ = r.store.Record(&state.Action{
					ID:        state.NewID("update", r.connName(), argStr(args, "table")),
					Verdict:   state.Undoable,
					Kind:      "update",
					Table:     argStr(args, "table"),
					PK:        pkVals,
					Before:    before,
					After:     after,
					Status:    state.Applied,
					CreatedAt: now(),
				})
			}
			return fmt.Sprintf("%d row(s) updated", n), nil
		}})
	r.Register(Tool{Name: "delete_row", ReadOnly: false, Description: "Delete a row by PK.",
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
			ed, err := r.ensureEditor(argStr(args, "database"), argStr(args, "table"))
			if err != nil {
				return "", err
			}
			before, err := ed.Load(ctx, pkVals)
			if err != nil {
				return "", err
			}
			n, err := ed.Delete(ctx, pkVals)
			if err != nil {
				return "", err
			}
			if r.store != nil {
				_ = r.store.Record(&state.Action{
					ID:        state.NewID("delete", r.connName(), argStr(args, "table")),
					Verdict:   state.Undoable,
					Kind:      "delete",
					Table:     argStr(args, "table"),
					PK:        pkVals,
					Before:    before,
					After:     nil,
					Status:    state.Applied,
					CreatedAt: now(),
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

// ensureEditor lazily constructs the RowEditor for a table. The registry is
// built at connection time when no table context exists yet, so row tools
// create their editor on first use.
func (r *Registry) ensureEditor(dbName, table string) (*db.RowEditor, error) {
	if r.ed != nil {
		return r.ed, nil
	}
	if r.conn == nil {
		return nil, fmt.Errorf("row tools require a live connection")
	}
	ed, err := db.NewRowEditor(r.conn, dbName, table)
	if err != nil {
		return nil, err
	}
	r.ed = ed
	return ed, nil
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
