package tui

import (
	"errors"
	"strings"
	"testing"
)

func TestStatusStreamingCounter(t *testing.T) {
	r := newResultsView(colsData())
	r.total = 3
	r.loading = true
	s := statusView(statusInfo{Conn: "webstack", Results: r, Elapsed: "12ms", Focus: focusResults})
	if !strings.Contains(s, "3 rows…") {
		t.Fatalf("streaming counter missing: %q", s)
	}
}

func TestStatusScopedError(t *testing.T) {
	r := newResultsView(colsData())
	r.err = errors.New("boom")
	s := statusView(statusInfo{Conn: "webstack", Results: r, Focus: focusResults})
	if !strings.Contains(s, "✗ boom") {
		t.Fatalf("scoped error not rendered: %q", s)
	}
}

func TestStatusShowsKeysPerFocus(t *testing.T) {
	s := statusView(statusInfo{Conn: "webstack", Focus: focusResults})
	if !strings.Contains(s, "sort") || !strings.Contains(s, "filter") {
		t.Fatalf("results keys missing: %q", s)
	}
}

func TestToastLine(t *testing.T) {
	got := toastLine("saved — undoable", 40)
	if !strings.Contains(got, "saved — undoable") {
		t.Fatalf("toast missing text: %q", got)
	}
	if lineCount(got) > 1 {
		t.Fatalf("toast must be one line: %q", got)
	}
	if n := len([]rune(got)); n > 40 {
		t.Fatalf("toast exceeds width: %d > 40", n)
	}
}
