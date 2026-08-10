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

// rects is the per-surface geometry for one frame. rail is a zero rect while
// the context rail is closed; toast is a zero rect when no toast line is shown.
type rects struct {
	tabs    rect
	schema  rect
	editor  rect
	results rect
	rail    rect
	toast   rect
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

// Fixed pane widths so the layout never reflows as focus moves. The left and
// context rails keep these widths on any normal-size terminal; they degrade
// only when the window is too narrow to leave the stage a usable minimum.
const (
	schemaColWidth = 28
	railColWidth   = 42
	stageMinWidth  = 48
)

// rects computes pane bounds for a w×h window with the given focus, whether the
// context rail is open, whether the left rail is collapsed, and how many footer
// rows are in use (1 normally, 2 while a toast is active). Row 0 is the tab bar
// and the last footerH rows are the footer. The body between them is
// horizontal: the schema tree is a left column (fixed width), the editor and
// results stack in the stage column, and the context rail (row/hist/ai tabs) is
// a right-hand strip of the stage's full height. Focus changes height (editor
// grows when focused) but never width, so pane edges stay put. Collapsing the
// left rail gives its width back to the stage. Every pane keeps at least one
// body row so a tiny window never collapses a surface; the results floor of 8
// degrades to whatever space remains.
func (l *layout) rects(w, h int, focus paneFocus, railOpen, railCollapsed bool, footerH int) rects {
	if w < 1 {
		w = 1
	}
	if h < footerH+2 {
		h = footerH + 2 // tab bar + footer + one body row
	}
	if footerH < 1 {
		footerH = 1
	}
	bodyH := h - 1 - footerH

	schemaW := schemaColWidth
	if railCollapsed {
		schemaW = 0
	} else if w-schemaW < stageMinWidth {
		schemaW = w - stageMinWidth // keep the stage usable on narrow windows
		if schemaW < 1 {
			schemaW = 1
		}
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
	resH := bodyH - edH
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
	if footerH > 1 {
		rs.toast = rect{X: 0, Y: h - 2, W: w, H: 1}
	}
	rs.editor = rect{X: rightX, Y: 1, W: rightW, H: edH}
	rs.results = rect{X: rightX, Y: 1 + edH, W: rightW, H: resH}

	if railOpen {
		rw := railColWidth
		if rw > rightW-stageMinWidth {
			rw = rightW / 3 // narrow window: give the rail a third instead
		}
		if rw < 15 {
			rw = 15
		}
		if rw > rightW-1 {
			rw = rightW - 1
		}
		rs.rail = rect{X: rightX + rightW - rw, Y: 1, W: rw, H: bodyH}
		rs.editor.W = rightW - rw
		rs.results.W = rightW - rw
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
// Width pads but never truncates — it wraps — so the string is first
// MaxWidth-truncated (ANSI-aware) and then padded, keeping single-line bars on
// one line.
func fit(s string, w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	s = clipHeight(s, h)
	s = lipgloss.NewStyle().MaxWidth(w).Render(s)
	return lipgloss.NewStyle().Width(w).Render(s)
}

// paneBox wraps a pane title + body in a rounded border sized to a rect. The
// focused border is the accent color; idle borders are dim. The title line is
// the first content line. lipgloss Width pads but never truncates (it wraps),
// so the body is height-clipped here and MaxWidth-truncated before the border
// pads — otherwise a long line would grow the box past its rect. A rect too
// short for a border (h < 3) renders a bare title line instead.
func paneBox(title string, focused bool, body string, w, h int) string {
	if h < 3 {
		head := "  " + title
		if focused {
			head = styleAccent.Render("▸ " + title)
		}
		return fit(head, w, h)
	}
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
		Width(innerW).
		Height(innerH) // pad the box to the rect so it never shrinks below h
	if focused {
		style = style.BorderForeground(styleAccent.GetForeground())
	}
	return style.Render(content)
}
