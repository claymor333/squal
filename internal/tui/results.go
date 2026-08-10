package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/claymor333/squal/internal/db"
)

type resultsView struct {
	data    *db.Columnar
	order   []int // row indices in display order
	sortCol int   // -1 = no sort
	sortAsc bool
	filter  string
	top     int // first visible row index into order
	selRow  int // selected display row (relative to top)
	loading bool
	err     error
	done    bool
	total   int // rows loaded so far
}

func newResultsView(data *db.Columnar) *resultsView {
	r := &resultsView{data: data, sortCol: -1, sortAsc: true}
	for i := 0; i < data.Rows; i++ {
		r.order = append(r.order, i)
	}
	return r
}

func (r *resultsView) rebuildOrder() {
	if r.data == nil {
		return
	}
	r.order = r.order[:0]
	for i := 0; i < r.data.Rows; i++ {
		if r.matches(i) {
			r.order = append(r.order, i)
		}
	}
	if r.sortCol >= 0 && r.sortCol < len(r.data.Columns) {
		col, asc := r.sortCol, r.sortAsc
		sort.SliceStable(r.order, func(i, j int) bool {
			a, b := r.data.Value(col, r.order[i]), r.data.Value(col, r.order[j])
			if asc {
				return a < b
			}
			return a > b
		})
	}
	if r.top > len(r.order) {
		r.top = len(r.order) - 1
	}
}

func (r *resultsView) matches(i int) bool {
	if r.filter == "" {
		return true
	}
	for c := range r.data.Columns {
		if strings.Contains(strings.ToLower(r.data.Value(c, i)), strings.ToLower(r.filter)) {
			return true
		}
	}
	return false
}

func (r *resultsView) visibleRows(height int) []int {
	if r.top < 0 || r.top >= len(r.order) {
		return nil
	}
	end := r.top + height
	if end > len(r.order) {
		end = len(r.order)
	}
	return r.order[r.top:end]
}

func (r *resultsView) appendBatch(rows int) {
	r.total += rows
	r.rebuildOrder()
}

func (r *resultsView) rowCount() string {
	if r.done {
		return fmt.Sprintf("%d rows", r.total)
	}
	return fmt.Sprintf("%d rows…", r.total)
}
