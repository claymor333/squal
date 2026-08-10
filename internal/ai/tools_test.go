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
