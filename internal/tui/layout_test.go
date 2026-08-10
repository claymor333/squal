package tui

import "testing"

func TestLayoutFits(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, false, false, 1)
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

func TestLayoutWidthsConsistentAcrossFocus(t *testing.T) {
	l := newLayout()
	// focus must never change pane widths — only heights (editor) move.
	base := l.rects(120, 40, focusResults, false, false, 1)
	schema := l.rects(120, 40, focusSchema, false, false, 1)
	if base.schema.W != schema.schema.W {
		t.Fatalf("schema width changed with focus: %d vs %d", base.schema.W, schema.schema.W)
	}
	if base.editor.W != schema.editor.W || base.results.W != schema.results.W {
		t.Fatalf("stage width changed with focus: editor %d vs %d", base.editor.W, schema.editor.W)
	}
}

func TestLayoutRailFixedWidth(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, true, false, 1)
	// rightW = 120 - schemaW(28) = 92; the rail holds a fixed 42.
	if rs.rail.W != railColWidth {
		t.Fatalf("rail width = %d, want %d", rs.rail.W, railColWidth)
	}
	if rs.rail.X+rs.rail.W > 120 {
		t.Fatalf("rail escapes the window: %+v", rs.rail)
	}
	if rs.rail.H != rs.schema.H {
		t.Fatalf("rail should span the body height: %d vs %d", rs.rail.H, rs.schema.H)
	}
	// editor/results shrink to make room for the rail
	if rs.editor.W != 92-railColWidth || rs.results.W != 92-railColWidth {
		t.Fatalf("stage panes not shrunk: editor.W=%d results.W=%d", rs.editor.W, rs.results.W)
	}
}

func TestLayoutRailShrinksOnNarrowWindow(t *testing.T) {
	l := newLayout()
	rs := l.rects(80, 40, focusResults, true, false, 1)
	if rs.rail.W >= railColWidth {
		t.Fatalf("narrow window should shrink the rail: %d", rs.rail.W)
	}
	if rs.editor.W < 15 {
		t.Fatalf("stage collapsed: editor.W=%d", rs.editor.W)
	}
}

func TestLayoutCollapseGivesSchemaWidthToStage(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, true, true, 1)
	if rs.schema.W != 0 {
		t.Fatalf("collapsed schema width = %d, want 0", rs.schema.W)
	}
	if rs.results.X+rs.results.W > 120 {
		t.Fatalf("stage should use the freed width: %+v", rs.results)
	}
}

func TestLayoutTinyWindowNeverBreaks(t *testing.T) {
	l := newLayout()
	rs := l.rects(60, 12, focusEditor, false, false, 1)
	if rs.editor.H < 1 || rs.results.H < 1 || rs.schema.H < 1 {
		t.Fatalf("tiny window collapsed a pane: %+v", rs)
	}
}

func TestLayoutToastFooter(t *testing.T) {
	l := newLayout()
	rs := l.rects(120, 40, focusResults, false, false, 2)
	if rs.toast.Y != 38 || rs.status.Y != 39 {
		t.Fatalf("footer rows misplaced: toast=%+v status=%+v", rs.toast, rs.status)
	}
	if rs.results.Y+rs.results.H > 38 {
		t.Fatalf("body overflows with 2 footer rows: %+v", rs.results)
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
