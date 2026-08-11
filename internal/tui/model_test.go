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
	if !strings.Contains(cur.ed.ta.Value(), "\n") {
		t.Fatalf("Enter should insert a newline: %q", cur.ed.ta.Value())
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
		m.conns = append(m.conns, newConnData(config.Profile{Name: "t"}))
	}
	return m
}

// altKey builds an Alt-modified rune key; shortcuts use modifiers, so tests
// press the same combos a user would.
func altKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}
}

func TestResultsKeysSortCursorColumn(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].results = newResultsView(colsData())
	m.focus = focusResults
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	if m.conns[m.active].results.sortCol != 1 {
		t.Fatalf("sortCol = %d, want 1", m.conns[m.active].results.sortCol)
	}
}

func TestResultsKeysFilterIncremental(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].results = newResultsView(colsData())
	m.focus = focusResults

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}, Alt: true})
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

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	if cmd == nil || cur.hist == nil {
		t.Fatal("u should open the history tab and dispatch a load")
	}
	if m.focus != focusRail || cur.railTab != focusHistory {
		t.Fatalf("focus = %v railTab = %v, want rail/history", m.focus, cur.railTab)
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
	// esc closes the rail
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cur.railOpen {
		t.Fatal("esc should close the rail")
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

// TestF1OpensHelp covers the F1 help binding; the help is a popup over the live
// frame, not a full-screen replacement.
func TestF1OpensHelp(t *testing.T) {
	m := newModelForTest(1)
	m.width, m.height = 100, 26
	m.conns[0].pane = newSchemaPane([]db.Database{{Name: "app"}})
	m.focus = focusSchema
	m.handleKey(tea.KeyMsg{Type: tea.KeyF1})
	if !m.help {
		t.Fatal("F1 should open the help overlay")
	}
	out := m.View()
	if !strings.Contains(out, "keys") {
		t.Fatal("help popup should render")
	}
	if !strings.Contains(out, "▸ schema") {
		t.Fatal("the base frame should stay visible behind the popup")
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
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: true})
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

func TestRailTabSwitch(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.hist = newHistoryView(nil)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	if !cur.railOpen || cur.railTab != focusHistory {
		t.Fatalf("rail = %v/%v, want open on hist", cur.railOpen, cur.railTab)
	}
	if m.focus != focusRail {
		t.Fatalf("focus = %v, want rail", m.focus)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	if cur.railTab != focusAI {
		t.Fatalf("railTab = %v, want ai", cur.railTab)
	}
}

func TestRingFlipKeys(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	cur.ring = newResultsRing()
	cur.ring.push(newResultsView(colsData()))
	cur.ring.push(newResultsView(&db.Columnar{Columns: []string{"x"}, Cols: [][]string{{"1"}}, Rows: 1}))
	cur.results = cur.ring.cur()
	m.focus = focusResults
	m.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if cur.results.data.Columns[0] == "x" {
		t.Fatalf("pgdn should leave the newest grid: %v", cur.results.data.Columns)
	}
}

func TestRailCollapseToggles(t *testing.T) {
	m := newModelForTest(1)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}, Alt: true})
	if !m.railCollapsed {
		t.Fatal("L should collapse the left rail")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}, Alt: true})
	if m.railCollapsed {
		t.Fatal("L again should restore the left rail")
	}
}

func TestToastSetAndCleared(t *testing.T) {
	m := newModelForTest(1)
	m.width, m.height = 80, 20
	m.toast = "saved — undoable"
	if !strings.Contains(m.View(), "saved — undoable") {
		t.Fatal("toast should render in the footer")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.toast != "" {
		t.Fatal("any key should clear the toast")
	}
}

// TestHelpDismissesOnActionKey: the help overlay is a menu — any key closes it
// and is then processed, so "F1 then 1" opens the rail instead of doing nothing.
func TestHelpDismissesOnActionKey(t *testing.T) {
	m := newModelForTest(1)
	m.help = true
	m.focus = focusSchema
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	if m.help {
		t.Fatal("any key should dismiss help")
	}
	if !m.conns[0].railOpen || m.conns[0].railTab != focusHistory {
		t.Fatal("the key should then be processed (open hist tab)")
	}
}

// TestDigitsTypeInEditor: bare digits are SQL text in the editor — they type
// and must not switch the rail's active tab (which is open by default).
func TestDigitsTypeInEditor(t *testing.T) {
	m := newModelForTest(1)
	m.conns[m.active].ed = newEditor()
	m.focus = focusEditor
	cur := m.conns[m.active]
	cur.railTab = focusAI
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if got := m.conns[m.active].ed.text(); got != "1" {
		t.Fatalf("editor text = %q, want 1", got)
	}
	if cur.railTab != focusAI {
		t.Fatalf("bare 1 must not switch the rail tab: %v", cur.railTab)
	}
}

// TestRailRendersWhenOpen: opening the rail must put a visible rail box in the
// frame that stays inside the window.
func TestRailRendersWhenOpen(t *testing.T) {
	m := newModelForTest(1)
	m.width, m.height = 100, 26
	cur := m.conns[0]
	cur.results = newResultsView(colsData())
	m.focus = focusResults
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	out := m.View()
	if !strings.Contains(out, "rail") || !strings.Contains(out, "[row]") {
		t.Fatalf("rail not rendered: %q", out)
	}
	if lineCount(out) > m.height {
		t.Fatalf("rail overflowed: %d > %d", lineCount(out), m.height)
	}
}

func TestLowercaseLCollapses(t *testing.T) {
	m := newModelForTest(1)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}, Alt: true})
	if !m.railCollapsed {
		t.Fatal("lowercase l should also collapse the left rail")
	}
}

// TestAIRequestTypesBareLetters: bare keys (even y/n) type into the AI request;
// y/n only answer a pending write confirm.
func TestAIRequestTypesBareLetters(t *testing.T) {
	m := newModelForTest(1)
	m.ai = newAIPanel()
	m.focus = focusRail
	m.conns[0].railOpen = true
	m.conns[0].railTab = focusAI
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if got := m.ai.request.Value(); got != "yn" {
		t.Fatalf("request = %q, want yn (bare letters type)", got)
	}
}

// TestRailOpenByDefault: the context rail is always on screen until the user
// closes it manually (esc while focused); alt+2 reopens it on history.
func TestRailOpenByDefault(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	if !cur.railOpen {
		t.Fatal("the context rail should be open by default")
	}
	m.focus = focusRail
	m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	if cur.railOpen {
		t.Fatal("esc should close the rail")
	}
	m.handleKey(altKey('2'))
	if !cur.railOpen || cur.railTab != focusHistory {
		t.Fatal("alt+2 should reopen the rail on hist")
	}
}

// TestAltZeroTogglesRail: alt+0 opens/closes the rail from anywhere, and keeps
// the last active tab when reopening.
func TestAltZeroTogglesRail(t *testing.T) {
	m := newModelForTest(1)
	cur := m.conns[0]
	m.focus = focusResults
	cur.railTab = focusHistory

	m.handleKey(altKey('0'))
	if cur.railOpen {
		t.Fatal("alt+0 should close an open rail")
	}
	m.handleKey(altKey('0'))
	if !cur.railOpen || cur.railTab != focusHistory {
		t.Fatalf("alt+0 should reopen the rail on the last tab: %v/%v", cur.railOpen, cur.railTab)
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
	for _, want := range []string{"schema", "editor", "results"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View missing pane title %q", want)
		}
	}
}
