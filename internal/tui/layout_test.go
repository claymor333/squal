package tui

import "testing"

func TestLayoutFits(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, false, false)
	if rs.schema.Y < 1 {
		t.Fatalf("schema starts above the tab bar: Y=%d", rs.schema.Y)
	}
	if rs.results.Y+rs.results.H > 39 {
		t.Fatalf("results escape the window: %+v", rs.results)
	}
	if rs.results.H < 8 {
		t.Fatalf("results floor 8 violated: h=%d", rs.results.H)
	}
	if rs.status.Y+rs.status.H > 40 || rs.status.Y < 0 {
		t.Fatalf("status bar out of bounds: %+v", rs.status)
	}
}

func TestLayoutFocusExpands(t *testing.T) {
	l := newLayout()
	idle := l.rects(120, 40, focusResults, false, false).schema
	focused := l.rects(120, 40, focusSchema, false, false).schema
	if focused.W <= idle.W {
		t.Fatalf("focused schema should be wider: idle=%d focus=%d", idle.W, focused.W)
	}
}

func TestLayoutRowPanelTakesRightThird(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, true, false)
	// rightW = 120 - schemaW(26) = 94; row = 94/3
	if rs.row.W != 31 {
		t.Fatalf("row panel width = %d, want 31", rs.row.W)
	}
	if rs.row.X+rs.row.W > 120 {
		t.Fatalf("row panel escapes the window: %+v", rs.row)
	}
	// editor/results/ai shrink to make room for the row strip
	if rs.editor.W != 94-31 || rs.results.W != 94-31 || rs.ai.W != 94-31 {
		t.Fatalf("right panes not shrunk: editor.W=%d results.W=%d ai.W=%d",
			rs.editor.W, rs.results.W, rs.ai.W)
	}
}

func TestLayoutTinyWindowNeverBreaks(t *testing.T) {
	l := newLayout()
	rs := l.rects(60, 12, focusEditor, false, false)
	if rs.editor.H < 1 || rs.results.H < 1 || rs.schema.H < 1 {
		t.Fatalf("tiny window collapsed a pane: %+v", rs)
	}
}

func TestLayoutHistorySplitsResults(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, false, true)
	if rs.hist.H < 3 {
		t.Fatalf("history band too small: %d", rs.hist.H)
	}
	if rs.hist.Y != rs.results.Y+rs.results.H {
		t.Fatalf("history should sit directly under results: hist.Y=%d res.Y+H=%d",
			rs.hist.Y, rs.results.Y+rs.results.H)
	}
	if rs.hist.Y+rs.hist.H > rs.ai.Y {
		t.Fatalf("history overlaps ai: %+v", rs.hist)
	}
}

func TestLayoutRectContains(t *testing.T) {
	r := rect{X: 2, Y: 3, W: 10, H: 5}
	if !r.contains(2, 3) || !r.contains(11, 7) {
		t.Fatal("boundary points should be inside")
	}
	if r.contains(1, 3) || r.contains(12, 7) || r.contains(2, 8) {
		t.Fatal("points outside the rect should not match")
	}
}

func TestClipCutsToCell(t *testing.T) {
	if got := clip("abcdef\ngh", 3, 1); got != "abc" {
		t.Fatalf("clip = %q, want abc", got)
	}
	if got := clip("abcdef", 10, 2); got != "abcdef" {
		t.Fatalf("clip should not pad: %q", got)
	}
}

func TestPaneBoxLongLineNeverGrows(t *testing.T) {
	// lipgloss Width pads but wraps long lines, which would grow the box past
	// its rect. paneBox must truncate (MaxWidth) so a long body stays inside.
	long := "this is an extremely long pane body line that must never wrap"
	out := paneBox("results", true, long, 30, 4)
	if n := lineCount(out); n > 4 {
		t.Fatalf("paneBox grew to %d lines (wrap bug), want <= 4: %q", n, out)
	}
}
