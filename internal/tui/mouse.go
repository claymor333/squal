package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// paneLayout computes the Y-bounds of each pane from the fixed stacked layout:
// line 0 = tab bar, then per pane a header line followed by a body of known
// height. It is pure so mouse handling is testable without a live DB.
//
//	paneLayout{hdr, body} positions are:
//	  schema:  header=1, body=2..2+bodyH
//	  editor:  header=body[2+bodyH], body=+1 (single line)
//	  results: header, body = resultsViewport rows (+1 header col row inside)
//	  ai:      header, body (single line)
//	status:    last
type paneLayout struct {
	schemaHeader, schemaBodyEnd int
	editorHeader, editorBody    int
	resultsHeader, resultsEnd   int
	aiHeader                    int
	bodyH                       int
}

func newPaneLayout(schemaBodyLines, viewport int) *paneLayout {
	schemaHeader := 1
	schemaBodyEnd := schemaHeader + 1 + schemaBodyLines
	editorHeader := schemaBodyEnd
	editorBody := editorHeader + 1
	resultsHeader := editorBody + 1
	resultsEnd := resultsHeader + 1 + viewport
	aiHeader := resultsEnd
	return &paneLayout{
		schemaHeader: schemaHeader, schemaBodyEnd: schemaBodyEnd,
		editorHeader: editorHeader, editorBody: editorBody,
		resultsHeader: resultsHeader, resultsEnd: resultsEnd,
		aiHeader: aiHeader, bodyH: schemaBodyLines,
	}
}

// at maps a Y coordinate to (pane, row). row is -1 for a header line, or the
// zero-based row index inside the pane body for body lines.
func (l *paneLayout) at(y int) (paneFocus, int) {
	if y <= 0 {
		return focusSchema, -1
	}
	switch {
	case y == l.schemaHeader:
		return focusSchema, -1
	case y < l.schemaBodyEnd:
		return focusSchema, y - l.schemaHeader - 1
	case y == l.editorHeader:
		return focusEditor, -1
	case y == l.editorBody:
		return focusEditor, 0
	case y == l.resultsHeader:
		return focusResults, -1
	case y < l.resultsEnd:
		return focusResults, y - l.resultsHeader - 1
	case y == l.aiHeader:
		return focusAI, -1
	default:
		return focusAI, -1
	}
}

// schemaBodyLines returns how many lines the schema tree body occupies.
func (m *model) schemaBodyLines() int {
	cur := m.conns[m.active]
	if cur.pane != nil {
		return lineCount(cur.pane.view())
	}
	return lineCount(renderSchemaTree(cur.dbs))
}

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

// handleMouse routes a mouse event: click on a pane header focuses it; click in
// the results body selects the row; wheel over results scrolls the grid.
func (m *model) handleMouse(msg tea.MouseMsg) {
	if len(m.conns) == 0 {
		return
	}
	cur := m.conns[m.active]
	l := newPaneLayout(m.schemaBodyLines(), resultsViewport)
	pane, row := l.at(msg.Y)

	if tea.MouseEvent(msg).IsWheel() {
		if pane == focusResults && cur.results != nil {
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
	m.focus = pane
	if pane == focusResults && row >= 0 && cur.results != nil && row < resultsViewport {
		cur.results.selRow = row
	}
}
