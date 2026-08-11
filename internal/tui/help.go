package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// The F1 help overlay is data-driven: bindings live next to the panes they
// describe, and the help component renders them, so the menu can never drift
// from the keymap. Shortcuts use modifier keys; bare keys are reserved for text
// input and navigation (arrows, enter, esc, tab, F1, delete, pgup/pgdn).
var (
	bindSchema = []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open db / browse table")),
		key.NewBinding(key.WithKeys("alt+x"), key.WithHelp("alt+x", "expand columns")),
		key.NewBinding(key.WithKeys("alt+f"), key.WithHelp("alt+f", "filter tree")),
		key.NewBinding(key.WithKeys("alt+c"), key.WithHelp("alt+c", "new connection")),
	}
	bindEditor = []key.Binding{
		key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "SQL")),
		key.NewBinding(key.WithKeys("alt+enter"), key.WithHelp("alt+enter", "run query")),
		key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "move cursor")),
		key.NewBinding(key.WithKeys("ctrl+p", "ctrl+n"), key.WithHelp("ctrl+p/n", "history")),
	}
	bindResults = []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move row")),
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←→", "scroll columns")),
		key.NewBinding(key.WithKeys("alt+s"), key.WithHelp("alt+s", "sort column")),
		key.NewBinding(key.WithKeys("alt+f"), key.WithHelp("alt+f", "filter")),
		key.NewBinding(key.WithKeys("alt+n"), key.WithHelp("alt+n", "next page")),
		key.NewBinding(key.WithKeys("pgup", "pgdn"), key.WithHelp("pgup/pgdn", "result ring")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open row")),
		key.NewBinding(key.WithKeys("del"), key.WithHelp("del", "drop grid / delete row")),
	}
	bindRail = []key.Binding{
		key.NewBinding(key.WithKeys("alt+1"), key.WithHelp("alt+1", "row tab")),
		key.NewBinding(key.WithKeys("alt+2"), key.WithHelp("alt+2", "hist tab")),
		key.NewBinding(key.WithKeys("alt+3"), key.WithHelp("alt+3", "ai tab")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close rail")),
	}
	bindGlobal = []key.Binding{
		key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "cycle panes")),
		key.NewBinding(key.WithKeys("alt+l"), key.WithHelp("alt+l", "collapse left rail")),
		key.NewBinding(key.WithKeys("alt+0"), key.WithHelp("alt+0", "open/close rail")),
		key.NewBinding(key.WithKeys("alt+q"), key.WithHelp("alt+q", "close connection")),
		key.NewBinding(key.WithKeys("F1"), key.WithHelp("F1", "help")),
		key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "exit")),
	}
)

// renderHelp returns the key-reference overlay for a w×h window, rendered with
// the bubbles help component and clipped so it never overflows the terminal.
func renderHelp(focus paneFocus, w, h int) string {
	groups := [][]key.Binding{bindGlobal}
	switch focus {
	case focusSchema:
		groups = append(groups, bindSchema)
	case focusEditor:
		groups = append(groups, bindEditor)
	case focusResults:
		groups = append(groups, bindResults)
	case focusRail:
		groups = append(groups, bindRail)
	}
	hf := help.New()
	hf.ShortSeparator = "  ·  "
	hf.FullSeparator = "\n"
	content := hf.FullHelpView(groups)

	innerW, innerH := w-2, h-2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	// clip first so a tiny window never overflows; the box keeps its natural
	// content width so overlayPopup can center it as a popup.
	content = clip(content, innerW, innerH)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styleAccent.GetForeground()).
		Render(styleTitle.Render(" keys — "+focusedName(focus)) + "\n" + content)
}

func focusedName(f paneFocus) string {
	switch f {
	case focusSchema:
		return "schema"
	case focusEditor:
		return "editor"
	case focusResults:
		return "results"
	default:
		return "rail"
	}
}
