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

// statusView renders the one-line footer: connection, result count, elapsed, keys.
func statusView(name string, r *resultsView, elapsed string, err error, focus paneFocus) string {
	if err != nil {
		return styleStatusErr.Render(" ✗ " + err.Error() + " ")
	}
	count := "–"
	if r != nil {
		count = r.rowCount()
	}
	label := fmt.Sprintf(" %s | %s | %s ", name, count, elapsed)
	var keys string
	switch focus {
	case focusSchema:
		keys = "↑↓ move · enter open/select · tab focus"
	case focusEditor:
		keys = "type · ctrl+enter run · ↑ history · tab focus"
	case focusResults:
		keys = "↑↓ move · s sort · f filter · n next · tab focus"
	}
	return styleStatus.Render(label + " | " + keys)
}
