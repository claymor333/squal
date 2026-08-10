package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("236"))
	styleStatusErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Background(lipgloss.Color("236"))
)

// statusInfo carries everything the one-line footer needs to render. Errors are
// scoped to whichever pane owns them; the footer is the transient error slot.
type statusInfo struct {
	Conn    string
	Results *resultsView
	Elapsed string
	Err     error
	Focus   paneFocus
}

// toastLine renders a transient, single-line notice (a write verdict like
// "saved — undoable", a delete, or an error) clipped to the window width. It is
// the footer's second row when a toast is active.
func toastLine(t string, w int) string {
	if t == "" {
		return ""
	}
	style := styleAccent
	if strings.Contains(t, "✗") {
		style = styleErr
	}
	return fit(style.Render(t), w, 1)
}

// statusView renders the one-line footer: conn | rows | elapsed on the left,
// the focused pane's keys on the right. A scoped error replaces the left
// cluster so it is always visible.
func statusView(s statusInfo) string {
	if s.Err != nil {
		return styleStatusErr.Render(" ✗ " + s.Err.Error() + " ")
	}
	if s.Results != nil && s.Results.err != nil {
		return styleStatusErr.Render(" ✗ " + s.Results.err.Error() + " ")
	}
	count := "–"
	if s.Results != nil {
		count = s.Results.rowCount() // "N rows…" while streaming, "N rows" when done
	}
	label := fmt.Sprintf(" %s | %s | %s ", s.Conn, count, s.Elapsed)
	var keys string
	switch s.Focus {
	case focusSchema:
		keys = "↑↓ move · enter open/select · alt+x cols · alt+f filter · alt+c connect · tab focus"
	case focusEditor:
		keys = "type · alt+enter run · ↑↓←→ move · ctrl+p/n history · tab focus"
	case focusResults:
		keys = "↑↓ move · ←→ col · alt+s sort · alt+f filter · alt+n next · pgup/pgdn ring · tab focus"
	case focusRow:
		keys = "↑↓ field · enter edit · alt+r raw · alt+s save · alt+1/2/3 tab"
	case focusHistory:
		keys = "↑↓ move · enter undo · alt+1/2/3 tab · esc close"
	case focusAI:
		keys = "type · enter run · alt+a mode · esc interrupt · alt+1/2/3 tab"
	case focusRail:
		keys = "alt+1/2/3 tab · alt+0 close · esc close"
	}
	return styleStatus.Render(label + " | " + keys)
}
