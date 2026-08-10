package tui

import "testing"

func TestWriterLogsUndoableTypedSQL(t *testing.T) {
	_ = newWriter(nil, nil, nil) // store + editor injected later; verdict logic is pure
	// classify: single-table UPDATE is feasible
	v, err := classifyForTest("UPDATE users SET name='x' WHERE id=7")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Feasible {
		t.Fatalf("expected feasible, got %s", v.Reason)
	}
	if v.Kind != "update" {
		t.Fatalf("kind = %q", v.Kind)
	}
}

func TestWriterRejectsJoinSQL(t *testing.T) {
	v, err := classifyForTest("UPDATE users u JOIN orders o ON o.user_id=u.id SET u.x=1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Feasible {
		t.Fatal("join update must not be undoable")
	}
}
