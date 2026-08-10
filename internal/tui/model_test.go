package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/db"
	"github.com/claymor333/squal/internal/state"
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

func TestEditorEnterInsertsNewline(t *testing.T) {
	// Enter must insert a newline, not run the query (multiline SQL, U3).
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.ed = newEditor()
	for _, r := range "SELECT 1" {
		cur.ed.insert(r)
	}
	m.focus = focusEditor

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter should not dispatch a query cmd")
	}
	if len(cur.ed.history) != 0 {
		t.Fatalf("Enter must not push history: %v", cur.ed.history)
	}
	if lineCount(cur.ed.view()) != 2 {
		t.Fatalf("Enter should insert a newline (editor now %d lines)", lineCount(cur.ed.view()))
	}
}

func TestEditorCtrlRRunsQuery(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.ed = newEditor()
	for _, r := range "SELECT 1" {
		cur.ed.insert(r)
	}
	m.focus = focusEditor

	// handleKey needs a live conn for startFetch; without one it still must
	// consume the run (clear the editor + push history) rather than swallow it.
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if cmd != nil {
		t.Fatal("no conn: expected nil cmd (query can't run without a connection)")
	}
	if got := cur.ed.text(); got != "" {
		t.Fatalf("editor not cleared after run: %q", got)
	}
	if len(cur.ed.history) != 1 || cur.ed.history[0] != "SELECT 1" {
		t.Fatalf("history = %v", cur.ed.history)
	}
}

func TestSchemaNavigationSetsCurrentDB(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.pane = newSchemaPane([]db.Database{
		{Name: "app", Tables: []db.Table{{Name: "users"}}},
		{Name: "blog", Tables: []db.Table{{Name: "posts"}}},
	})
	m.focus = focusSchema

	// cursor starts on the first db header
	if cur.currentDB != "" {
		t.Fatalf("initial currentDB = %q, want empty", cur.currentDB)
	}
	// move down -> second db
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if cur.currentDB != "blog" {
		t.Fatalf("currentDB = %q, want blog", cur.currentDB)
	}
	// move up -> back to first db
	m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if cur.currentDB != "app" {
		t.Fatalf("currentDB = %q, want app", cur.currentDB)
	}
}

func newModelForTest(count int) *model {
	m := &model{focus: focusSchema}
	for i := 0; i < count; i++ {
		m.conns = append(m.conns, &connData{profile: config.Profile{Name: "t"}})
	}
	return m
}

func TestResultsKeysSortCursorColumn(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].results = newResultsView(colsData())
	m.focus = focusResults
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.conns[m.active].results.sortCol != 1 {
		t.Fatalf("sortCol = %d, want 1", m.conns[m.active].results.sortCol)
	}
}

func TestResultsKeysFilterIncremental(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].results = newResultsView(colsData())
	m.focus = focusResults

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	r := m.conns[m.active].results
	if r.filter != "b" {
		t.Fatalf("filter = %q, want b", r.filter)
	}
	if len(r.order) != 1 {
		t.Fatalf("filtered rows = %d, want 1", len(r.order))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if r.filterMode {
		t.Fatal("esc should leave filter mode")
	}
}

// TestEditorCtrlRRunsQuery covers the editor run binding (U3); this one asserts
// the history keys route through the model.
func TestEditorHistoryKeysRouted(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].ed = newEditor()
	m.focus = focusEditor
	e := m.conns[m.active].ed
	for _, r := range "one" {
		e.insert(r)
	}
	e.runQuery()
	for _, r := range "two" {
		e.insert(r)
	}
	e.runQuery()
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})
	if got := e.text(); got != "two" {
		t.Fatalf("ctrl+p text = %q, want two", got)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	if got := e.text(); got != "" {
		t.Fatalf("ctrl+n text = %q, want empty", got)
	}
}

// TestHistoryToggleLoadsActions opens the app-side SQLite store on a temp file,
// records an action, presses 'u', and asserts the panel fills from it.
func TestHistoryToggleLoadsActions(t *testing.T) {
	dir := t.TempDir()
	s, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Record(&state.Action{
		ID: "a1", Verdict: state.Undoable, Kind: "update",
		Connection: "t", Database: "app", Table: "users",
		Status: state.Applied, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	m := newModelForTest(1)
	m.store = s
	m.focus = focusResults
	cur := m.conns[0]

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil || cur.hist == nil {
		t.Fatal("u should open the history panel and dispatch a load")
	}
	if m.focus != focusHistory {
		t.Fatalf("focus = %v, want history", m.focus)
	}
	msg := cmd()
	hl, ok := msg.(historyLoadedMsg)
	if !ok || hl.Err != nil {
		t.Fatalf("load cmd = %#v", msg)
	}
	m.Update(hl)
	if len(cur.hist.rows) != 1 || cur.hist.rows[0].ID != "a1" {
		t.Fatalf("history rows = %+v", cur.hist.rows)
	}
	// esc closes
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cur.hist != nil {
		t.Fatal("esc should close history")
	}
}

// TestQuestionMarkTypesInEditor guards the ?-in-editor regression: '?' is valid
// SQL, so it must insert in the editor instead of opening help.
func TestQuestionMarkTypesInEditor(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].ed = newEditor()
	m.focus = focusEditor
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if m.help {
		t.Fatal("? in the editor should not open help")
	}
	if got := m.conns[m.active].ed.text(); got != "?" {
		t.Fatalf("editor text = %q, want ?", got)
	}
}

// TestQuestionMarkOpensHelp covers the ? binding and the help overlay render.
func TestQuestionMarkOpensHelp(t *testing.T) {
	m := newModelForTest(1)
	m.width, m.height = 80, 30
	m.focus = focusSchema
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.help {
		t.Fatal("? should open the help overlay")
	}
	if !strings.Contains(m.View(), "keys") {
		t.Fatal("help overlay should render")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.help {
		t.Fatal("esc should close help")
	}
}

// TestCloseRequiresConfirm covers the q -> confirm -> y flow.
func TestCloseRequiresConfirm(t *testing.T) {
	m := newModelForTest(1)
	m.focus = focusSchema
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.confirm == nil {
		t.Fatal("q should open a confirm modal, not close immediately")
	}
	before := len(m.conns)
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirming should dispatch a close cmd")
	}
	m.Update(cmd()) // connClosedMsg
	if len(m.conns) != before-1 {
		t.Fatalf("confirming close should drop the tab: %d -> %d", before, len(m.conns))
	}
}

func TestCtrlDClosesDirectly(t *testing.T) {
	m := newModelForTest(2)
	m.focus = focusSchema
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("ctrl+d should dispatch a close cmd")
	}
	m.Update(cmd())
	if len(m.conns) != 1 {
		t.Fatalf("conns = %d, want 1", len(m.conns))
	}
}

// TestCtrlCQuits: ^c must exit the app, not drop a tab (q/ctrl+d own closing).
func TestCtrlCQuits(t *testing.T) {
	m := newModelForTest(2)
	m.focus = focusSchema
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should dispatch a quit cmd")
	}
	if len(m.conns) != 2 {
		t.Fatalf("ctrl+c must not close a tab: conns = %d", len(m.conns))
	}
}

func TestCtrlCQuitsFromHelp(t *testing.T) {
	m := newModelForTest(1)
	m.help = true
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even while help is open")
	}
}

func TestCtrlCQuitsFromConnectDialog(t *testing.T) {
	m := newModelForTest(1)
	m.connect = newConnectView()
	_, cmd := m.handleConnectKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even from the connect dialog")
	}
}

func TestTransientErrorClearedOnKey(t *testing.T) {
	m := newModelForTest(1)
	m.transientErr = errSave
	m.focus = focusSchema
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.transientErr != nil {
		t.Fatal("any key should clear the transient status error")
	}
}

// TestViewNeverOverflows renders every pane (a large expanded schema tree, a
// loaded results grid) into a small window and asserts the output fits.
func TestViewNeverOverflows(t *testing.T) {
	m := newModelForTest(1)
	m.width, m.height = 80, 20
	cur := m.conns[0]

	tables := make([]db.Table, 40) // a fat tree would blow up the old fixed stack
	for i := range tables {
		tables[i] = db.Table{Name: "t"}
	}
	cur.pane = newSchemaPane([]db.Database{{Name: "app", Tables: tables}})
	cur.pane.toggleDB(0)
	cur.pane.SetLines(6)

	cur.ed = newEditor()
	cur.results = newResultsView(&db.Columnar{
		Columns: []string{"id", "name"},
		Cols:    [][]string{{"1", "2"}, {"a", "b"}},
		Rows:    2,
	})

	out := m.View()
	if lineCount(out) > m.height {
		t.Fatalf("View overflowed: %d lines > %d", lineCount(out), m.height)
	}
	for _, want := range []string{"schema", "editor", "results", "ai"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View missing pane title %q", want)
		}
	}
}
