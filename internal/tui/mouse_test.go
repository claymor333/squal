package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/db"
)

// mouseModel returns a model with one connection, a 10-row results grid, and a
// fixed 80x30 window so layout hit-testing is deterministic. For that window
// with focus=results: tabs 0, schema 1..8, editor 9, results 10..27, ai 28,
// status 29; data row r is at Y = 13+r.
func mouseModel(t *testing.T) *model {
	t.Helper()
	m := newModelForTest(1)
	m.width, m.height = 80, 30
	m.focus = focusResults
	m.conns[0].results = newResultsView(&db.Columnar{
		Columns: []string{"id"},
		Cols:    [][]string{{"0"}, {"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"7"}, {"8"}, {"9"}},
		Rows:    10,
	})
	return m
}

func TestMouseClickFocusesSchema(t *testing.T) {
	m := mouseModel(t)
	m.handleMouse(tea.MouseMsg{X: 5, Y: 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.focus != focusSchema {
		t.Fatalf("focus = %v, want schema", m.focus)
	}
}

func TestMouseClickSelectsResultsRow(t *testing.T) {
	m := mouseModel(t)
	m.handleMouse(tea.MouseMsg{X: 5, Y: 16, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.focus != focusResults {
		t.Fatalf("focus = %v, want results", m.focus)
	}
	if got := m.conns[0].results.selRow; got != 3 {
		t.Fatalf("selRow = %d, want 3", got)
	}
}

func TestMouseWheelScrollsResults(t *testing.T) {
	m := mouseModel(t)
	cur := m.conns[0]
	m.handleMouse(tea.MouseMsg{X: 5, Y: 15, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if cur.results.top != 0 {
		t.Fatalf("wheel up at top: top = %d, want 0", cur.results.top)
	}
	cur.results.top = 10
	m.handleMouse(tea.MouseMsg{X: 5, Y: 15, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if cur.results.top != 9 {
		t.Fatalf("wheel up: top = %d, want 9", cur.results.top)
	}
	m.handleMouse(tea.MouseMsg{X: 5, Y: 15, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if cur.results.top != 10 {
		t.Fatalf("wheel down: top = %d, want 10", cur.results.top)
	}
}

func TestMouseWheelOutsideResultsIgnored(t *testing.T) {
	m := mouseModel(t)
	cur := m.conns[0]
	cur.results.top = 5
	m.handleMouse(tea.MouseMsg{X: 5, Y: 3, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if cur.results.top != 5 {
		t.Fatalf("wheel over schema must not scroll results: top = %d", cur.results.top)
	}
}
