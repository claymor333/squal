package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/db"
)

func TestPaneLayoutMapsSchemaHeader(t *testing.T) {
	// schema body = 2 lines (one db, one table)
	l := newPaneLayout(2, resultsViewport)
	pane, _ := l.at(1)
	if pane != focusSchema {
		t.Fatalf("y=1 -> %v, want schema", pane)
	}
	pane, _ = l.at(2)
	if pane != focusSchema {
		t.Fatalf("y=2 -> %v, want schema", pane)
	}
}

func TestPaneLayoutMapsResultsRows(t *testing.T) {
	l := newPaneLayout(2, resultsViewport)
	// schema(0 tab,1 hdr,2-3 body) editor(4 hdr,5 body) results(6 hdr,7.. body)
	pane, row := l.at(6)
	if pane != focusResults {
		t.Fatalf("y=6 -> %v, want results", pane)
	}
	if row != -1 {
		t.Fatalf("y=6 row = %d, want -1 (header)", row)
	}
	pane, row = l.at(8)
	if pane != focusResults {
		t.Fatalf("y=8 -> %v, want results", pane)
	}
	if row != 1 {
		t.Fatalf("y=8 row = %d, want 1", row)
	}
}

func TestPaneLayoutMapsAI(t *testing.T) {
	l := newPaneLayout(2, resultsViewport)
	pane, _ := l.at(9 + resultsViewport)
	if pane != focusAI {
		t.Fatalf("after results -> %v, want ai", pane)
	}
}

func TestMouseWheelScrollsResults(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.results = newResultsView(&db.Columnar{
		Columns: []string{"id"},
		Cols:    [][]string{{"0"}, {"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"7"}, {"8"}, {"9"}},
		Rows:    10,
	})
	m.focus = focusResults

	m.handleMouse(tea.MouseMsg{X: 5, Y: 7, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if cur.results.top != 0 {
		t.Fatalf("wheel up at top: top = %d, want 0", cur.results.top)
	}
	cur.results.top = 10
	m.handleMouse(tea.MouseMsg{X: 5, Y: 7, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if cur.results.top != 9 {
		t.Fatalf("wheel up: top = %d, want 9", cur.results.top)
	}
	m.handleMouse(tea.MouseMsg{X: 5, Y: 7, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if cur.results.top != 10 {
		t.Fatalf("wheel down: top = %d, want 10", cur.results.top)
	}
}
