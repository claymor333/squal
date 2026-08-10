package tui

import "testing"

func TestEditorRunQuery(t *testing.T) {
	e := newEditor()
	for _, r := range "SELECT * FROM users" {
		e.insert(r)
	}
	msg := e.run()
	_, ok := msg.(runQueryMsg)
	if !ok {
		t.Fatalf("run() = %T, want runQueryMsg", msg)
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
	e.run()
	if len(e.history) != 1 || e.history[0] != "SELECT 1" {
		t.Fatalf("history = %v", e.history)
	}
	e.historyUp()
	if e.text() != "SELECT 1" {
		t.Fatalf("historyUp text = %q", e.text())
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
