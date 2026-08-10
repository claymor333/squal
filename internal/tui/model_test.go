package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/config"
)

func TestModelTabSwitch(t *testing.T) {
	m := newModelForTest(3) // 3 tabs
	if m.active != 0 {
		t.Fatalf("active = %d", m.active)
	}
	m.nextTab()
	if m.active != 1 {
		t.Fatalf("after nextTab active = %d", m.active)
	}
	m.prevTab()
	if m.active != 0 {
		t.Fatalf("after prevTab active = %d", m.active)
	}
}

func TestModelBrowseRequestBuildsRun(t *testing.T) {
	m := newModelForTest(1)
	req := browseRequestMsg{Database: "app", Table: "users", PK: []string{"id"}}
	msg := m.onBrowse(req)
	rm, ok := msg.(runQueryMsg)
	if !ok {
		t.Fatalf("onBrowse = %T, want runQueryMsg", msg)
	}
	if rm.SQL == "" {
		t.Fatal("empty query")
	}
}

func TestEditorEnterRunsQuery(t *testing.T) {
	// Enter (not ctrl+enter — bubbletea has no such key) must run the query.
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.ed = newEditor()
	for _, r := range "SELECT 1" {
		cur.ed.insert(r)
	}
	m.focus = focusEditor

	// handleKey needs a live conn for startFetch; without one it still must
	// consume the Enter (clear the editor + push history) rather than swallow
	// it. The bug was: Enter hit the default branch and was ignored.
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("no conn: expected nil cmd (query can't run without a connection)")
	}
	if got := cur.ed.text(); got != "" {
		t.Fatalf("editor not cleared after Enter: %q", got)
	}
	if len(cur.ed.history) != 1 || cur.ed.history[0] != "SELECT 1" {
		t.Fatalf("history = %v", cur.ed.history)
	}
}

func newModelForTest(count int) *model {
	m := &model{focus: focusSchema}
	for i := 0; i < count; i++ {
		m.conns = append(m.conns, &connData{profile: config.Profile{Name: "t"}})
	}
	return m
}
