package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/claymor333/squal/internal/db"
)

type resultsView struct {
	data       *db.Columnar
	order      []int // row indices in display order
	sortCol    int   // -1 = no sort
	sortAsc    bool
	selCol     int // column cursor (header highlight)
	filter     string
	filterMode bool
	top        int // first visible row index into order
	selRow     int // selected display row (relative to top)
	viewport   int // data rows visible without scrolling
	loading    bool
	err        error
	done       bool
	total      int // rows loaded so far
}

func newResultsView(data *db.Columnar) *resultsView {
	r := &resultsView{data: data, sortCol: -1, sortAsc: true, selCol: 0, viewport: 8}
	for i := 0; i < data.Rows; i++ {
		r.order = append(r.order, i)
	}
	return r
}

// SetViewport sets the number of data rows the grid can show; the old
// fixed resultsViewport constant is gone, so a short window degrades gracefully.
func (r *resultsView) SetViewport(h int) {
	if h > 0 {
		r.viewport = h
	}
}

// moveCol steps the column cursor, clamped to the header range.
func (r *resultsView) moveCol(d int) {
	if r.data == nil || len(r.data.Columns) == 0 {
		return
	}
	r.selCol += d
	if r.selCol < 0 {
		r.selCol = 0
	}
	if r.selCol > len(r.data.Columns)-1 {
		r.selCol = len(r.data.Columns) - 1
	}
}

// sortCursor sorts by the cursor column, toggling direction if already sorted
// on it, then rebuilds the display order.
func (r *resultsView) sortCursor() {
	if r.data == nil || len(r.data.Columns) == 0 {
		return
	}
	if r.sortCol == r.selCol {
		r.sortAsc = !r.sortAsc
	} else {
		r.sortCol = r.selCol
		r.sortAsc = true
	}
	r.rebuildOrder()
}

// startFilter enters filter mode with a fresh predicate.
func (r *resultsView) startFilter() {
	r.filterMode = true
	r.filter = ""
	r.rebuildOrder()
}

// appendFilter grows the incremental filter and re-applies it.
func (r *resultsView) appendFilter(c rune) {
	if !r.filterMode {
		return
	}
	r.filter += string(c)
	r.rebuildOrder()
}

// popFilter shrinks the incremental filter and re-applies it.
func (r *resultsView) popFilter() {
	if !r.filterMode {
		return
	}
	if n := len(r.filter); n > 0 {
		r.filter = r.filter[:n-1]
	}
	r.rebuildOrder()
}

// endFilter leaves filter mode, keeping the current predicate applied.
func (r *resultsView) endFilter() {
	r.filterMode = false
}

// view renders the grid: header row (cursor column highlighted, sort arrow on
// the sorted column) then the visible window of data rows, each cell truncated
// to a width that fits the given pane width.
func (r *resultsView) view(w int) string {
	if r.err != nil {
		return styleErr.Render("✗ " + r.err.Error())
	}
	if r.data == nil || len(r.data.Columns) == 0 {
		return styleDim.Render("(no columns)")
	}
	if len(r.order) == 0 {
		if r.filterMode {
			return styleDim.Render("(no rows match filter)")
		}
		return styleDim.Render("(no rows)")
	}

	ncols := len(r.data.Columns)
	// reserve the selection marker (2 cells) and the " │ " separators.
	cellW := (w - 2 - (ncols-1)*3) / ncols
	if cellW < 1 {
		cellW = 1
	}
	if cellW > 64 {
		cellW = 64
	}

	var b strings.Builder
	for c, col := range r.data.Columns {
		if c > 0 {
			b.WriteString(" │ ")
		}
		head := col
		if r.sortCol == c {
			if r.sortAsc {
				head += " ▲"
			} else {
				head += " ▼"
			}
		}
		if c == r.selCol {
			b.WriteString(styleAccent.Render("▸ " + head))
		} else {
			b.WriteString(styleDim.Render("  " + head))
		}
	}
	if r.filterMode {
		b.WriteString("   [filter: " + r.filter + "▌]")
	}
	b.WriteString("\n")

	end := r.top + r.viewport
	if end > len(r.order) {
		end = len(r.order)
	}
	for row := r.top; row < end; row++ {
		mark := "  "
		if row == r.top+r.selRow {
			mark = "◉ "
		}
		b.WriteString(mark)
		for c := range r.data.Columns {
			if c > 0 {
				b.WriteString(" │ ")
			}
			b.WriteString(truncate(r.data.Value(c, r.order[row]), cellW))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// rowAt extracts the display row at order-index idx as a column->value map,
// for the row panel and structured writes.
func rowAt(r *resultsView, idx int) map[string]string {
	if r == nil || r.data == nil {
		return nil
	}
	row := make(map[string]string, len(r.data.Columns))
	for c, name := range r.data.Columns {
		row[name] = r.data.Value(c, idx)
	}
	return row
}

// patchRow writes a saved row's values back into the columnar grid in place so
// the visible results (sort/filter included) stay correct without a refetch.
func patchRow(r *resultsView, orig int, after map[string]string) {
	if r == nil || r.data == nil {
		return
	}
	for c, name := range r.data.Columns {
		if v, ok := after[name]; ok {
			r.data.Cols[c][orig] = v
		}
	}
	r.rebuildOrder()
}

// truncate cuts a cell to n runes, reserving one for an ellipsis when it
// truncates so the result never exceeds n cells.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
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
