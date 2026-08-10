package tui

import "testing"

func TestHistoryListUndoSelect(t *testing.T) {
	h := newHistoryView([]historyRow{
		{ID: "a", Label: "update users", Verdict: "undoable", Status: "applied"},
		{ID: "b", Label: "DELETE FROM x", Verdict: "logged-only", Status: "applied"},
	})
	if h.cursor != 0 {
		t.Fatalf("cursor = %d", h.cursor)
	}
	h.moveDown()
	if h.cursor != 1 {
		t.Fatalf("cursor after move = %d", h.cursor)
	}
	h.moveUp()
	msg := h.selectRow()
	um, ok := msg.(undoActionMsg)
	if !ok {
		t.Fatalf("selectRow = %T, want undoActionMsg", msg)
	}
	if um.ID != "a" {
		t.Fatalf("undo id = %q", um.ID)
	}
}
