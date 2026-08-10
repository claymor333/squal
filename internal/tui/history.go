package tui

import (
	"fmt"

	"github.com/claymor333/squal/internal/state"
)

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

// SetActions rebuilds the list from the action store.
func (h *historyView) SetActions(actions []*state.Action) {
	h.rows = h.rows[:0]
	for _, a := range actions {
		h.rows = append(h.rows, historyRow{
			ID:      a.ID,
			Label:   a.Kind + " " + a.Table,
			Verdict: a.Verdict.String(),
			Status:  a.Status.String(),
		})
	}
	if h.cursor > len(h.rows)-1 {
		h.cursor = len(h.rows) - 1
	}
	if h.cursor < 0 {
		h.cursor = 0
	}
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
	out += styleDim.Render("enter undo · esc close")
	return out
}
