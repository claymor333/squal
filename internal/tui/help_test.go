package tui

import (
	"strings"
	"testing"
)

func TestHelpListsKeysPerPane(t *testing.T) {
	out := renderHelp(focusResults, 80, 30)
	for _, want := range []string{"schema", "editor", "results", "sort", "filter", "row panel", "history", "ai"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestHelpFitsHeight(t *testing.T) {
	out := renderHelp(focusAI, 60, 10)
	if n := strings.Count(out, "\n"); n > 9 {
		t.Fatalf("help overflows 10 rows: %d", n)
	}
}
