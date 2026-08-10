package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleMouse routes a mouse event by hit-testing the live layout rects:
// click focuses the pane under the cursor, a click in the results body selects
// the row under the pointer, the wheel scrolls the results grid, and a click on
// the rail (or its tab strip) focuses the rail. The geometry comes from the
// same layout the renderer uses, so it cannot drift.
func (m *model) handleMouse(msg tea.MouseMsg) {
	if len(m.conns) == 0 {
		return
	}
	cur := m.conns[m.active]
	footerH := 1
	if m.toast != "" {
		footerH = 2
	}
	rs := newLayout().rects(m.width, m.height, m.focus, cur.railOpen, m.railCollapsed, footerH)

	if tea.MouseEvent(msg).IsWheel() {
		if rs.results.contains(msg.X, msg.Y) && cur.results != nil {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if cur.results.top > 0 {
					cur.results.top--
				}
			case tea.MouseButtonWheelDown:
				cur.results.top++
			}
		}
		return
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return
	}
	switch {
	case rs.rail.contains(msg.X, msg.Y):
		m.focus = focusRail
		if cur.railOpen {
			if msg.Y == rs.rail.Y+1 { // the tab strip line
				m.setRailTabByX(msg.X, rs.rail)
			}
		} else if msg.Y >= rs.rail.Y+2 {
			// minimized: clicking a tab row expands the rail on that tab.
			row := msg.Y - rs.rail.Y - 2
			tabs := []paneFocus{focusRow, focusHistory, focusAI}
			if row >= 0 && row < len(tabs) {
				m.activateRail(cur, tabs[row])
			}
		}
	case rs.results.contains(msg.X, msg.Y) && cur.results != nil:
		m.focus = focusResults
		// results box layout: border, title, column header, then data rows.
		row := msg.Y - rs.results.Y - 3
		if row >= 0 && row < cur.results.viewport {
			cur.results.selRow = row
		}
	case rs.schema.contains(msg.X, msg.Y):
		m.focus = focusSchema
	case rs.editor.contains(msg.X, msg.Y):
		m.focus = focusEditor
	}
}
