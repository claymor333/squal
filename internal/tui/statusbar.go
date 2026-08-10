package tui

import (
	"fmt"

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
		keys = "↑↓ move · enter open/select · c connect · tab focus"
	case focusEditor:
		keys = "type · alt+enter run · ↑↓←→ move · ctrl+p/n history · tab focus"
	case focusResults:
		keys = "↑↓ move · ←→ col · s sort · f filter · n next · enter/o row · tab focus"
	case focusRow:
		keys = "↑↓ field · enter edit · r raw · s save · esc close"
	case focusHistory:
		keys = "↑↓ move · enter undo · esc close"
	case focusAI:
		keys = "type · enter run · a mode · esc interrupt"
	}
	return styleStatus.Render(label + " | " + keys)
}
