package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// rect is a pane's bounds in terminal cells, relative to the screen top-left.
// X is the left edge, Y the top edge, W/H the size. The bottom-right cell is
// (X+W-1, Y+H-1).
type rect struct {
	X, Y, W, H int
}

func (r rect) contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// rects is the per-surface geometry for one frame. row and hist are zero rects
// while their panels are closed.
type rects struct {
	tabs    rect
	schema  rect
	editor  rect
	results rect
	hist    rect
	ai      rect
	row     rect
	status  rect
}

type layout struct{}

func newLayout() *layout { return &layout{} }

// lineCount counts the newline-delimited lines of s, treating a trailing
// newline as the terminator of the last line, not an extra line.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	if len(s) > 0 && s[len(s)-1] == '\n' {
		n--
	}
	return n
}

// rects computes pane bounds for a w×h window with the given focus, whether the
// row panel is open, and whether the history panel is open. Row 0 is the tab
// bar and the last row is the status bar. The body between them is horizontal:
// the schema tree is a left column (wider when focused), and the editor,
// results, history, and AI panes stack in the right column. Every pane keeps at
// least one body row so a tiny window never collapses a surface; the results
// floor of 8 degrades to whatever space remains. The history panel carves a
// third out of the results band when open; the row panel is a right-hand strip
// of the whole right column.
func (l *layout) rects(w, h int, focus paneFocus, rowOpen, histOpen bool) rects {
	if w < 1 {
		w = 1
	}
	if h < 3 {
		h = 3 // tab bar + status + one body row
	}
	bodyH := h - 2

	schemaW := w * 22 / 100
	if focus == focusSchema {
		schemaW = w * 32 / 100
	}
	if schemaW < 20 {
		schemaW = 20
	}
	if schemaW > w/2 {
		schemaW = w / 2
	}

	edH := 1
	if focus == focusEditor {
		edH = bodyH / 3
		if edH > 8 {
			edH = 8
		}
		if edH < 1 {
			edH = 1
		}
	}

	aiH := 1
	if focus == focusAI {
		aiH = bodyH / 4
		if aiH > 6 {
			aiH = 6
		}
		if aiH < 1 {
			aiH = 1
		}
	}

	resH := bodyH - edH - aiH
	if resH < 1 {
		resH = 1
	}
	histH := 0
	if histOpen {
		histH = resH / 3
		if histH > resH-1 {
			histH = resH - 1
		}
		if histH < 0 {
			histH = 0
		}
	}
	resH -= histH
	if resH < 1 {
		resH = 1
	}

	rightX := schemaW
	rightW := w - schemaW

	rs := rects{
		tabs:   rect{X: 0, Y: 0, W: w, H: 1},
		status: rect{X: 0, Y: h - 1, W: w, H: 1},
		schema: rect{X: 0, Y: 1, W: schemaW, H: bodyH},
	}
	top := 1
	rs.editor = rect{X: rightX, Y: top, W: rightW, H: edH}
	top += edH
	rs.results = rect{X: rightX, Y: top, W: rightW, H: resH}
	top += resH
	if histH > 0 {
		rs.hist = rect{X: rightX, Y: top, W: rightW, H: histH}
		top += histH
	}
	rs.ai = rect{X: rightX, Y: top, W: rightW, H: aiH}

	if rowOpen {
		rw := rightW / 3
		if rw < 15 {
			rw = 15
		}
		if rw > rightW-1 {
			rw = rightW - 1
		}
		rs.row = rect{X: rightX + rightW - rw, Y: rs.editor.Y, W: rw, H: bodyH - rs.editor.Y + 1}
		rs.editor.W = rightW - rw
		rs.results.W = rightW - rw
		rs.hist.W = rightW - rw
		rs.ai.W = rightW - rw
	}
	return rs
}

// clipHeight keeps only the first h lines of a multi-line string. Width
// truncation is left to lipgloss so ANSI escapes are never split.
func clipHeight(s string, h int) string {
	if h < 1 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// clip truncates a multi-line string to at most w×h cells by runes. It is for
// ANSI-free content (tests, pure pane bodies); use fit for rendered strings.
func clip(s string, w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, ln := range lines {
		r := []rune(ln)
		if len(r) > w {
			lines[i] = string(r[:w])
		}
	}
	return strings.Join(lines, "\n")
}

// fit bounds a rendered (possibly ANSI-colored) string to w×h cells. lipgloss
// Width handles ANSI-aware truncation and pads short lines.
func fit(s string, w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	return lipgloss.NewStyle().Width(w).Render(clipHeight(s, h))
}

// paneBox wraps a pane title + body in a rounded border sized to a rect. The
// focused border is the accent color; idle borders are dim. The title line is
// the first content line. lipgloss Width pads but never truncates (it wraps),
// so the body is height-clipped here and MaxWidth-truncated before the border
// pads — otherwise a long line would grow the box past its rect.
func paneBox(title string, focused bool, body string, w, h int) string {
	innerW, innerH := w-2, h-2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	head := "  " + title
	if focused {
		head = styleAccent.Render("▸ " + title)
	} else {
		head = styleDim.Render("  " + title)
	}
	content := clipHeight(head+"\n"+body, innerH)
	content = lipgloss.NewStyle().MaxWidth(innerW).Render(content)
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styleDim.GetForeground()).
		Width(innerW)
	if focused {
		style = style.BorderForeground(styleAccent.GetForeground())
	}
	return style.Render(content)
}
