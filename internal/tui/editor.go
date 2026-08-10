package tui

import "strings"

type runQueryMsg struct {
	SQL string
}

// editor is a small multiline text buffer. The cursor is a flat index into buf;
// lines are split on '\n'. Enter inserts a newline; running the query is
// explicitly triggered by the model (alt+enter / ctrl+r), not by Enter.
type editor struct {
	buf     []rune
	cur     int
	history []string
	histPos int // -1 when not navigating history
	vw, vh  int // viewport cells (set by the layout)
}

func newEditor() *editor {
	return &editor{histPos: -1}
}

// SetViewport records the editor's cell bounds for the multiline view.
func (e *editor) SetViewport(w, h int) {
	e.vw, e.vh = w, h
}

// insert places r at the cursor. Starting to type after a history recall begins
// a fresh buffer.
func (e *editor) insert(r rune) {
	if e.histPos >= 0 {
		e.buf = e.buf[:0]
		e.cur = 0
		e.histPos = -1
	}
	e.buf = append(e.buf[:e.cur], append([]rune{r}, e.buf[e.cur:]...)...)
	e.cur++
}

// backspace removes the rune before the cursor.
func (e *editor) backspace() {
	if e.cur > 0 {
		e.buf = append(e.buf[:e.cur-1], e.buf[e.cur:]...)
		e.cur--
	}
	e.histPos = -1
}

// newline inserts a line break at the cursor.
func (e *editor) newline() {
	e.insert('\n')
}

func (e *editor) clear() {
	e.buf = e.buf[:0]
	e.cur = 0
	e.histPos = -1
}

func (e *editor) text() string {
	return strings.TrimSpace(string(e.buf))
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
	e.buf = []rune(e.history[e.histPos])
	e.cur = len(e.buf)
}

// historyForward walks newer history entries, ending on a blank buffer.
func (e *editor) historyForward() {
	if e.histPos == -1 {
		return
	}
	e.histPos++
	if e.histPos >= len(e.history) {
		e.histPos = -1
		e.buf = e.buf[:0]
		e.cur = 0
		return
	}
	e.buf = []rune(e.history[e.histPos])
	e.cur = len(e.buf)
}

func (e *editor) moveLeft() {
	if e.cur > 0 {
		e.cur--
	}
}

func (e *editor) moveRight() {
	if e.cur < len(e.buf) {
		e.cur++
	}
}

// lineBounds returns the [start, end) rune range of the line containing pos and
// the column of pos within that line.
func lineBounds(buf []rune, pos int) (start, end, col int) {
	start = 0
	for i := pos - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			start = i + 1
			break
		}
	}
	end = len(buf)
	for i := pos; i < len(buf); i++ {
		if buf[i] == '\n' {
			end = i
			break
		}
	}
	return start, end, pos - start
}

// moveUp keeps the visual column when moving to the previous line.
func (e *editor) moveUp() {
	start, _, col := lineBounds(e.buf, e.cur)
	if start == 0 {
		return
	}
	prevEnd := start - 1 // index of the line's trailing '\n'
	prevStart := 0
	for i := prevEnd - 1; i >= 0; i-- {
		if e.buf[i] == '\n' {
			prevStart = i + 1
			break
		}
	}
	prevLen := prevEnd - prevStart
	if col > prevLen {
		col = prevLen
	}
	e.cur = prevStart + col
}

// moveDown keeps the visual column when moving to the next line.
func (e *editor) moveDown() {
	_, end, col := lineBounds(e.buf, e.cur)
	if end >= len(e.buf) {
		return
	}
	nextStart := end + 1
	nextLen := len(e.buf) - nextStart
	for i := nextStart; i < len(e.buf); i++ {
		if e.buf[i] == '\n' {
			nextLen = i - nextStart
			break
		}
	}
	if col > nextLen {
		col = nextLen
	}
	e.cur = nextStart + col
}

// view renders the buffer with a cursor block at the edit position, scrolled so
// the cursor line stays visible within the pane viewport.
func (e *editor) view() string {
	_, _, curCol := lineBounds(e.buf, e.cur)
	lines := splitLines(e.buf)
	curLine := cursorLine(e.buf, e.cur)

	top := 0
	if e.vh > 1 {
		top = curLine
		if top > e.vh-1 {
			top = curLine - e.vh + 1
		}
		if max := len(lines) - e.vh; top > max {
			top = max
		}
		if top < 0 {
			top = 0
		}
	}

	var out strings.Builder
	end := len(lines)
	if e.vh > 0 && top+e.vh < end {
		end = top + e.vh
	}
	for i := top; i < end; i++ {
		if i == curLine {
			out.WriteString(insertCursor(lines[i], curCol))
		} else {
			out.WriteString(lines[i])
		}
		if i < end-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// splitLines splits the buffer into its lines. A trailing '\n' is preserved as
// an empty final line — that is where the cursor sits after a newline.
func splitLines(buf []rune) []string {
	var out []string
	start := 0
	for i := 0; i <= len(buf); i++ {
		if i == len(buf) || buf[i] == '\n' {
			out = append(out, string(buf[start:i]))
			start = i + 1
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// cursorLine returns the zero-based line index containing the cursor.
func cursorLine(buf []rune, cur int) int {
	line := 0
	for i := 0; i < cur; i++ {
		if buf[i] == '\n' {
			line++
		}
	}
	return line
}

// insertCursor renders line with a cursor block at col.
func insertCursor(line string, col int) string {
	r := []rune(line)
	if col < 0 {
		col = 0
	}
	if col > len(r) {
		col = len(r)
	}
	return string(r[:col]) + "█" + string(r[col:])
}
