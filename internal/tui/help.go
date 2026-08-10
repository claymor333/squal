package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelp returns the key-reference overlay for a w×h window. The current
// focus's keys are included; the box is clipped so it never overflows a small
// terminal. Rendered last in View, above the layout.
func renderHelp(focus paneFocus, w, h int) string {
	rows := [][2]string{
		{"tab / shift+tab", "cycle panes"},
		{"F1 / esc", "open / close help"},
		{"ctrl+c", "exit app"},
		{"L", "collapse / restore left rail"},
		{"", ""},
		{"schema", "↑↓ move · enter open/browse · x columns · / filter · c connect"},
		{"editor", "type · alt+enter run · ↑↓←→ move · ctrl+p/n history"},
		{"results", "↑↓ move · ←→ col · s sort · f filter · n next · [ ] ring · enter row"},
		{"rail", "1 row · 2 hist · 3 ai · esc close"},
		{"row", "↑↓ field · enter edit · r raw · s save"},
		{"hist", "↑↓ move · enter undo"},
		{"ai", "type · enter run · a mode · esc interrupt · y/n confirm"},
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render(" keys "+focusedName(focus)) + "\n")
	for _, r := range rows {
		if r[0] == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, " %s %s\n", styleDim.Render(fmt.Sprintf("%-22s", r[0])), r[1])
	}
	innerW, innerH := w-2, h-2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	// clip first: lipgloss Width pads but never truncates, so the box would
	// grow if the raw rows were wider than the window.
	content := clip(b.String(), innerW, innerH)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styleAccent.GetForeground()).
		Width(innerW).
		Render(content)
}

func focusedName(f paneFocus) string {
	switch f {
	case focusSchema:
		return "schema"
	case focusEditor:
		return "editor"
	case focusResults:
		return "results"
	case focusRail:
		return "rail"
	default:
		return "rail"
	}
}
