package tui

import "fmt"

type undoActionMsg struct {
	ID string
}

type historyRow struct {
	ID      string
	Label   string
	Verdict string
	Status  string
}

type historyView struct {
	rows   []historyRow
	cursor int
}

func newHistoryView(rows []historyRow) *historyView {
	return &historyView{rows: rows}
}

func (h *historyView) moveDown() {
	if h.cursor < len(h.rows)-1 {
		h.cursor++
	}
}

func (h *historyView) moveUp() {
	if h.cursor > 0 {
		h.cursor--
	}
}

func (h *historyView) selectRow() any {
	if len(h.rows) == 0 {
		return nil
	}
	return undoActionMsg{ID: h.rows[h.cursor].ID}
}

func (h *historyView) view() string {
	out := ""
	for i, r := range h.rows {
		mark := " "
		if i == h.cursor {
			mark = "▸"
		}
		out += fmt.Sprintf("%s %s [%s/%s] %s\n", mark, r.ID, r.Verdict, r.Status, r.Label)
	}
	return out
}
