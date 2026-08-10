package tui

import (
	"testing"

	"github.com/claymor333/squal/internal/state"
)

func TestHistorySetActions(t *testing.T) {
	h := newHistoryView(nil)
	h.SetActions([]*state.Action{
		{ID: "a1", Kind: "update", Table: "users", Verdict: state.Undoable, Status: state.Applied},
		{ID: "a2", Kind: "delete", Table: "orders", Verdict: state.LoggedOnly, Status: state.Applied},
	})
	if len(h.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(h.rows))
	}
	if h.rows[0].Label != "update users" || h.rows[1].Verdict != "logged-only" {
		t.Fatalf("rows = %+v", h.rows)
	}
	h.moveDown()
	msg := h.selectRow()
	ua, ok := msg.(undoActionMsg)
	if !ok || ua.ID != "a2" {
		t.Fatalf("selectRow = %#v, want undoActionMsg{a2}", msg)
	}
}
