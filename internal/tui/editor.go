package tui

import "strings"

type runQueryMsg struct {
	SQL string
}

type editor struct {
	buf     []rune
	history []string
	histPos int // -1 when not navigating history
}

func newEditor() *editor {
	return &editor{histPos: -1}
}

func (e *editor) insert(r rune) {
	if e.histPos >= 0 {
		e.buf = e.buf[:0]
		e.histPos = -1
	}
	e.buf = append(e.buf, r)
}

func (e *editor) backspace() {
	if len(e.buf) > 0 {
		e.buf = e.buf[:len(e.buf)-1]
	}
}

func (e *editor) clear() {
	e.buf = e.buf[:0]
	e.histPos = -1
}

func (e *editor) text() string {
	return strings.TrimSpace(string(e.buf))
}

func (e *editor) run() any {
	sql := e.text()
	if sql == "" {
		return nil
	}
	e.history = append(e.history, sql)
	e.clear()
	return runQueryMsg{SQL: sql}
}

func (e *editor) historyUp() {
	if len(e.history) == 0 {
		return
	}
	if e.histPos == -1 {
		e.histPos = len(e.history) - 1
	} else if e.histPos > 0 {
		e.histPos--
	}
	e.buf = []rune(e.history[e.histPos])
}

func (e *editor) view() string {
	return string(e.buf) + "█"
}
