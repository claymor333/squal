package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// The F1 help overlay is data-driven: bindings live next to the panes they
// describe, and the help component renders them, so the menu can never drift
// from the keymap.
var (
	bindSchema = []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open db / browse table")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "expand columns")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter tree")),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "new connection")),
	}
	bindEditor = []key.Binding{
		key.NewBinding(key.WithKeys("type"), key.WithHelp("type", "SQL")),
		key.NewBinding(key.WithKeys("alt+enter", "ctrl+r"), key.WithHelp("alt+enter", "run query")),
		key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "move cursor")),
		key.NewBinding(key.WithKeys("ctrl+p", "ctrl+n"), key.WithHelp("ctrl+p/n", "history")),
	}
	bindResults = []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move row")),
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←→", "column")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort column")),
		key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
		key.NewBinding(key.WithKeys("[", "]"), key.WithHelp("[ ]", "result ring")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open row")),
		key.NewBinding(key.WithKeys("del"), key.WithHelp("del", "drop grid / delete row")),
	}
	bindRail = []key.Binding{
		key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "row tab")),
		key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "hist tab")),
		key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "ai tab")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close rail")),
	}
	bindGlobal = []key.Binding{
		key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "cycle panes")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "collapse left rail")),
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
	// clip first: lipgloss Width pads but never truncates, so the box would
	// grow if the help rows were wider than the window.
	content = clip(content, innerW, innerH)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styleAccent.GetForeground()).
		Width(innerW).
		Height(innerH).
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
