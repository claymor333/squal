package tui

import (
	"strings"
	"testing"
)

func TestHelpListsKeysPerPane(t *testing.T) {
	res := renderHelp(focusResults, 80, 30)
	for _, want := range []string{"sort", "filter", "result ring", "open row", "collapse left rail"} {
		if !strings.Contains(res, want) {
			t.Fatalf("results help missing %q", want)
		}
	}
	rail := renderHelp(focusRail, 80, 30)
	for _, want := range []string{"row tab", "hist tab", "ai tab", "close rail"} {
		if !strings.Contains(rail, want) {
			t.Fatalf("rail help missing %q", want)
		}
	}
}

func TestHelpFitsHeight(t *testing.T) {
	out := renderHelp(focusAI, 60, 10)
	if n := strings.Count(out, "\n"); n > 9 {
		t.Fatalf("help overflows 10 rows: %d", n)
	}
}
