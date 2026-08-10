package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/db"
)

var errSave = errors.New("boom")

func TestRowPanelApplyField(t *testing.T) {
	p := newRowPanel()
	p.SetRow(map[string]string{"id": "1", "name": "alice"})
	if p.cur != 0 {
		t.Fatalf("cursor = %d", p.cur)
	}
	p.moveDown()
	if p.cur != 1 {
		t.Fatalf("cursor after moveDown = %d", p.cur)
	}
	// focus the name field and replace it
	p.editValue("bob")
	got, _ := p.Values()
	if got["name"] != "bob" {
		t.Fatalf("name = %q, want bob", got["name"])
	}
}

func TestRowPanelRawJSONToggle(t *testing.T) {
	p := newRowPanel()
	p.SetRow(map[string]string{"id": "1", "name": "alice"})
	p.toggleRaw()
	if !p.raw {
		t.Fatal("raw not toggled on")
	}
	p.SetRawJSON(`{"id":"1","name":"carol"}`)
	p.toggleRaw()
	got, err := p.Values()
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "carol" {
		t.Fatalf("name = %q, want carol", got["name"])
	}
}

func TestRowPanelInlineEdit(t *testing.T) {
	p := newRowPanel()
	p.SetRow(map[string]string{"id": "1", "name": "alice"})
	p.moveDown() // name
	p.startEdit()
	if string(p.editBuf) != "alice" {
		t.Fatalf("editBuf = %q, want alice", string(p.editBuf))
	}
	p.backspaceEdit()
	p.backspaceEdit()
	p.backspaceEdit()
	p.appendEdit('b')
	p.commitEdit()
	got, _ := p.Values()
	if got["name"] != "alb" {
		t.Fatalf("name = %q, want alb", got["name"])
	}
}

func TestOpenRowPanelFromGrid(t *testing.T) {
	r := newResultsView(&db.Columnar{
		Columns: []string{"id", "name"},
		Cols:    [][]string{{"7"}, {"zed"}}, // column-major: one row, id=7 name=zed
		Rows:    1,
	})
	row := rowAt(r, r.order[0])
	if row["id"] != "7" || row["name"] != "zed" {
		t.Fatalf("rowAt = %v", row)
	}
	p := newRowPanel()
	p.SetRow(row)
	if n, v := p.current(); n != "id" || v != "7" {
		t.Fatalf("current = %s=%q, want id=7", n, v)
	}
}

// browseRowModel returns a model focused on results with one browsed row, so
// the row panel and delete/save flows are testable without a live DB.
func browseRowModel(t *testing.T) *model {
	t.Helper()
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.results = newResultsView(&db.Columnar{
		Columns: []string{"id", "name"},
		Cols:    [][]string{{"7"}, {"zed"}},
		Rows:    1,
	})
	cur.browse = &browseRequestMsg{Database: "app", Table: "users", PK: []string{"id"}}
	cur.wr = newWriter(nil, nil, nil) // write cmds are built, never executed, in tests
	m.focus = focusResults
	return m
}

func TestRowPanelOpensViaEnter(t *testing.T) {
	m := browseRowModel(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	cur := m.conns[0]
	if cur.row == nil || !cur.railOpen || cur.railTab != focusRow {
		t.Fatalf("enter on a browsed row should open the row rail tab: row=%v rail=%v/%v",
			cur.row != nil, cur.railOpen, cur.railTab)
	}
	if m.focus != focusRail {
		t.Fatalf("focus = %v, want rail", m.focus)
	}
	// esc closes the rail
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cur.railOpen {
		t.Fatal("esc should close the rail")
	}
}

func TestDeleteRowRequiresConfirm(t *testing.T) {
	m := browseRowModel(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyDelete})
	if m.confirm == nil {
		t.Fatal("delete should open a confirm modal")
	}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirming should dispatch the delete cmd")
	}
	if m.confirm != nil {
		t.Fatal("confirm should clear after y")
	}
}

func TestSaveRowDispatchesWriteCmd(t *testing.T) {
	m := browseRowModel(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	cur := m.conns[0]
	cur.row.moveDown()  // name
	cur.row.startEdit() // seeds "zed"
	for i := 0; i < 3; i++ {
		cur.row.backspaceEdit()
	}
	for _, r := range "bob" {
		cur.row.appendEdit(r)
	}
	cur.row.commitEdit()

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("s should dispatch a save cmd")
	}
	// run the cmd to get the saveRowMsg, then feed it to Update
	msg := cmd()
	sm, ok := msg.(saveRowMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want saveRowMsg", msg)
	}
	if sm.After["name"] != "bob" {
		t.Fatalf("after name = %q, want bob", sm.After["name"])
	}
	_, writeCmd := m.Update(sm)
	if writeCmd == nil {
		t.Fatal("saveRowMsg should dispatch the write cmd")
	}
}

func TestSaveRowPatchesGridInPlace(t *testing.T) {
	m := browseRowModel(t)
	cur := m.conns[0]
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}) // open the panel
	if cur.row == nil {
		t.Fatal("panel did not open")
	}
	m.Update(rowWriteDoneMsg{OrigIdx: 0, After: map[string]string{"name": "bob"}})
	if got := cur.results.data.Value(1, 0); got != "bob" {
		t.Fatalf("grid name = %q, want bob after patch", got)
	}
	if cur.row != nil {
		t.Fatal("panel should close after a successful save")
	}
}

func TestSaveRowErrorStaysInResults(t *testing.T) {
	m := browseRowModel(t)
	m.Update(rowWriteDoneMsg{Err: errSave})
	if m.conns[0].results.err == nil {
		t.Fatal("save error should surface in the results pane")
	}
}
