package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type runQueryMsg struct {
	SQL string
}

// editor wraps a bubbles textarea with query history. Enter inserts a newline;
// running the query is explicitly triggered by the model (alt+enter / ctrl+r).
// The wrapper keeps the same method surface as the hand-rolled buffer so the
// model's key routing is unchanged.
type editor struct {
	ta      textarea.Model
	history []string
	histPos int // -1 when not navigating history
	vw, vh  int // viewport cells (set by the layout)
}

func newEditor() *editor {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = "write SQL…"
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Cursor.SetMode(cursor.CursorStatic) // no blink loop needed
	ta.Focus()                             // cursor renders when focused; the model owns which pane is active
	return &editor{ta: ta, histPos: -1}
}

// SetViewport records the editor's cell bounds for the multiline view.
func (e *editor) SetViewport(w, h int) {
	e.vw, e.vh = w, h
	if h > 0 {
		e.ta.SetHeight(h)
	}
}

// title names the pane, carrying the current default database so the border
// shows what schema unqualified SQL resolves against.
func (e *editor) title(currentDB string) string {
	if currentDB == "" {
		return "editor"
	}
	return "editor — " + currentDB
}

// --- editing ---------------------------------------------------------------

func (e *editor) insert(r rune) {
	e.ta.InsertRune(r)
	e.histPos = -1
}

func (e *editor) backspace() {
	// bubbles Update returns a new model; the result must be stored back.
	ni, _ := e.ta.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	e.ta = ni
	e.histPos = -1
}

func (e *editor) newline() {
	ni, _ := e.ta.Update(tea.KeyMsg{Type: tea.KeyEnter})
	e.ta = ni
}

func (e *editor) clear() {
	e.ta.SetValue("")
	e.histPos = -1
}

func (e *editor) text() string {
	return strings.TrimSpace(e.ta.Value())
}

// runQuery pushes the buffer onto history, clears it, and returns the query
// message. It is called by the model on the run key, never on Enter.
func (e *editor) runQuery() any {
	sql := e.text()
	if sql == "" {
		return nil
	}
	e.history = append(e.history, sql)
	e.clear()
	return runQueryMsg{SQL: sql}
}

// historyBack walks older history entries into the buffer.
func (e *editor) historyBack() {
	if len(e.history) == 0 {
		return
	}
	if e.histPos == -1 {
		e.histPos = len(e.history) - 1
	} else if e.histPos > 0 {
		e.histPos--
	}
	e.ta.SetValue(e.history[e.histPos])
	e.ta.CursorEnd()
}

// historyForward walks newer history entries, ending on a blank buffer.
func (e *editor) historyForward() {
	if e.histPos == -1 {
		return
	}
	e.histPos++
	if e.histPos >= len(e.history) {
		e.histPos = -1
		e.ta.SetValue("")
		return
	}
	e.ta.SetValue(e.history[e.histPos])
	e.ta.CursorEnd()
}

// --- cursor -----------------------------------------------------------------

func (e *editor) moveLeft() {
	ni, _ := e.ta.Update(tea.KeyMsg{Type: tea.KeyLeft})
	e.ta = ni
}

func (e *editor) moveRight() {
	ni, _ := e.ta.Update(tea.KeyMsg{Type: tea.KeyRight})
	e.ta = ni
}

func (e *editor) moveUp() {
	e.ta.CursorUp()
}

func (e *editor) moveDown() {
	e.ta.CursorDown()
}

// view renders the textarea; the layout's paneBox clips it to the rect.
func (e *editor) view() string {
	return e.ta.View()
}
