package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/db"
)

// --- messages ---------------------------------------------------------------

type schemaLoadedMsg struct {
	idx int
	dbs []db.Database
	err error
}

type connClosedMsg struct {
	idx int
}

// --- connection state --------------------------------------------------------

type connData struct {
	profile config.Profile
	conn    *db.Conn
	loading bool
	dbs     []db.Database
	loadErr error
}

func openConnection(idx int, p config.Profile) tea.Cmd {
	return func() tea.Msg {
		c, err := db.Open(context.Background(), p)
		if err != nil {
			return schemaLoadedMsg{idx: idx, err: err}
		}
		dbs, err := c.Schema(context.Background(), false)
		if err != nil {
			return schemaLoadedMsg{idx: idx, err: err}
		}
		return schemaLoadedMsg{idx: idx, dbs: dbs}
	}
}

func closeConnection(idx int, c *db.Conn) tea.Cmd {
	return func() tea.Msg {
		if c != nil {
			c.Close()
		}
		return connClosedMsg{idx: idx}
	}
}

// --- model -------------------------------------------------------------------

type model struct {
	cfg      *config.Config
	conns    []*connData
	active   int
	width    int
	height   int
	lastErr  error
	quitting bool
}

func New(cfg *config.Config, profiles []config.Profile) *model {
	m := &model{cfg: cfg}
	for i, p := range profiles {
		m.conns = append(m.conns, &connData{profile: p, loading: true})
		m.active = i
	}
	return m
}

func (m *model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i, c := range m.conns {
		if c.conn == nil {
			cmds = append(cmds, openConnection(i, c.profile))
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case schemaLoadedMsg:
		if msg.idx < 0 || msg.idx >= len(m.conns) {
			return m, nil
		}
		c := m.conns[msg.idx]
		c.loading = false
		if msg.err != nil {
			c.loadErr = msg.err
			m.lastErr = msg.err
		} else {
			c.dbs = msg.dbs
			c.loadErr = nil
		}
		return m, nil

	case connClosedMsg:
		if msg.idx >= 0 && msg.idx < len(m.conns) {
			m.conns[msg.idx].conn = nil
		}
		if len(m.conns) > 0 {
			// drop the tab entirely
			m.conns = append(m.conns[:msg.idx], m.conns[msg.idx+1:]...)
		}
		if m.active >= len(m.conns) {
			m.active = len(m.conns) - 1
		}
		if len(m.conns) == 0 {
			m.quitting = true
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.active >= 0 && m.active < len(m.conns) {
			return m, closeConnection(m.active, m.conns[m.active].conn)
		}
		return m, tea.Quit
	case "tab":
		if len(m.conns) > 1 {
			m.active = (m.active + 1) % len(m.conns)
		}
	}
	return m, nil
}

// --- view --------------------------------------------------------------------

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	stylePane   = lipgloss.NewStyle().Padding(0, 1)
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

func (m *model) View() string {
	if m.quitting {
		return "bye\n"
	}
	if len(m.conns) == 0 {
		return "No connections configured.\n"
	}

	var tabs string
	for i, c := range m.conns {
		label := c.profile.Name
		if i == m.active {
			tabs += styleTitle.Render(fmt.Sprintf(" %s ", label)) + " "
		} else {
			tabs += styleDim.Render(" "+label+" ") + " "
		}
	}

	cur := m.conns[m.active]
	var body string
	switch {
	case cur.loading:
		body = styleDim.Render("connecting to " + cur.profile.Name + "…")
	case cur.loadErr != nil:
		body = styleErr.Render("connection failed: " + cur.loadErr.Error())
	default:
		body = renderSchemaTree(cur.dbs)
	}

	var errline string
	if m.lastErr != nil {
		errline = styleErr.Render("✗ "+m.lastErr.Error()) + "\n"
	}

	return stylePane.Render(
		tabs + "\n" +
			body + "\n" +
			styleDim.Render(renderHint()) + "\n" +
			errline,
	)
}

func renderSchemaTree(dbs []db.Database) string {
	var out string
	for _, d := range dbs {
		out += styleAccent.Render("▸ "+d.Name) + " " + styleDim.Render(fmt.Sprintf("(%d tables)", len(d.Tables))) + "\n"
		for _, t := range d.Tables {
			out += "   " + t.Name + "\n"
		}
	}
	if out == "" {
		return styleDim.Render("(no databases)")
	}
	return out
}

func renderHint() string {
	return "tab: next connection · q/ctrl+c: close & quit"
}

var _ tea.Model = (*model)(nil)
