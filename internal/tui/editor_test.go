package tui

import (
	"strings"
	"testing"
)

func TestEditorRunQuery(t *testing.T) {
	e := newEditor()
	for _, r := range "SELECT * FROM users" {
		e.insert(r)
	}
	msg := e.runQuery()
	_, ok := msg.(runQueryMsg)
	if !ok {
		t.Fatalf("runQuery() = %T, want runQueryMsg", msg)
	}
	if got := e.text(); got != "" {
		t.Fatalf("editor not cleared after run: %q", got)
	}
}

func TestEditorHistory(t *testing.T) {
	e := newEditor()
	for _, r := range "SELECT 1" {
		e.insert(r)
	}
	e.runQuery()
	if len(e.history) != 1 || e.history[0] != "SELECT 1" {
		t.Fatalf("history = %v", e.history)
	}
	e.historyBack()
	if e.text() != "SELECT 1" {
		t.Fatalf("historyBack text = %q", e.text())
	}
}

func TestEditorBackspace(t *testing.T) {
	e := newEditor()
	for _, r := range "abc" {
		e.insert(r)
	}
	e.backspace()
	if e.text() != "ab" {
		t.Fatalf("backspace text = %q", e.text())
	}
}

func TestEditorMultilineRunKeys(t *testing.T) {
	e := newEditor()
	for _, r := range "SELECT 1;" {
		e.insert(r)
	}
	e.newline()
	e.insert('s') // second line starts with 's'
	if got := e.text(); got != "SELECT 1;\ns" {
		t.Fatalf("text = %q", got)
	}
	msg := e.runQuery()
	if msg == nil {
		t.Fatal("runQuery returned nil")
	}
}

func TestEditorCursorMovement(t *testing.T) {
	e := newEditor()
	for _, r := range "ab\ncd" {
		e.insert(r)
	}
	e.moveLeft()  // cursor lands on 'd'
	e.backspace() // deletes 'c'
	if got := e.text(); got != "ab\nd" {
		t.Fatalf("after backspace = %q, want ab\\nd", got)
	}
}

func TestEditorHistoryKeys(t *testing.T) {
	e := newEditor()
	for _, r := range "one" {
		e.insert(r)
	}
	e.runQuery()
	for _, r := range "two" {
		e.insert(r)
	}
	e.runQuery()
	e.historyBack()
	if got := e.text(); got != "two" {
		t.Fatalf("historyBack = %q, want two", got)
	}
	e.historyForward()
	if got := e.text(); got != "" {
		t.Fatalf("historyForward = %q, want empty", got)
	}
}

func TestEditorUpDownMovesLines(t *testing.T) {
	e := newEditor()
	for _, r := range "abc\nde\nf" {
		e.insert(r)
	}
	e.ta.CursorEnd() // cursor on the "f" line (line 2)
	if e.ta.Line() != 2 {
		t.Fatalf("cursor line = %d, want 2", e.ta.Line())
	}
	e.moveUp()
	if e.ta.Line() != 1 { // "de" line
		t.Fatalf("after moveUp line = %d, want 1", e.ta.Line())
	}
	e.moveDown()
	if e.ta.Line() != 2 {
		t.Fatalf("after moveDown line = %d, want 2", e.ta.Line())
	}
}

func TestEditorViewShowsValue(t *testing.T) {
	e := newEditor()
	for _, r := range "ab" {
		e.insert(r)
	}
	if got := e.ta.Value(); got != "ab" {
		t.Fatalf("value = %q, want ab", got)
	}
	if !strings.Contains(e.view(), "ab") {
		t.Fatalf("view = %q, want value rendered", e.view())
	}
	// cursor movement is exercised behaviorally by TestEditorCursorMovement;
	// the bubbles cursor renders as an inverted block, not a literal glyph.
}

func TestEditorTitleShowsDB(t *testing.T) {
	e := newEditor()
	if got := e.title("app"); !strings.Contains(got, "app") {
		t.Fatalf("editor title missing db: %q", got)
	}
	if got := e.title(""); !strings.Contains(got, "editor") {
		t.Fatalf("empty-db title should still say editor: %q", got)
	}
}
