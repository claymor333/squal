package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/claymor333/squal/internal/db"
)

type resultsView struct {
	data       *db.Columnar
	order      []int // row indices in display order
	sortCol    int   // -1 = no sort
	sortAsc    bool
	selCol     int // column cursor (header highlight)
	xOff       int // first visible column (horizontal window)
	visCols    int // columns visible without horizontal scrolling
	filter     string
	filterMode bool
	top        int // first visible row index into order
	selRow     int // selected display row (relative to top)
	viewport   int // data rows visible without scrolling
	tbl        table.Model
	loading    bool
	err        error
	done       bool
	total      int // rows loaded so far
}

func newResultsView(data *db.Columnar) *resultsView {
	tbl := table.New()
	styles := table.DefaultStyles()
	styles.Header = styles.Header.Bold(true).Foreground(lipgloss.Color("214"))
	styles.Selected = styles.Selected.Background(lipgloss.Color("236")).Foreground(lipgloss.Color("39"))
	styles.Cell = styles.Cell.Padding(0, 0)
	tbl.SetStyles(styles)
	r := &resultsView{data: data, sortCol: -1, sortAsc: true, selCol: 0, visCols: 1, viewport: 8, tbl: tbl}
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

// moveCol steps the column cursor and slides the horizontal window so the
// cursor stays visible — the vertical analog of scrollResults.
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
	if r.selCol < r.xOff {
		r.xOff = r.selCol
	}
	if r.visCols > 0 && r.selCol >= r.xOff+r.visCols {
		r.xOff = r.selCol - r.visCols + 1
	}
	if r.xOff < 0 {
		r.xOff = 0
	}
}

// colWindow returns the half-open column range currently visible.
func (r *resultsView) colWindow() (start, end int) {
	if r.visCols < 1 {
		r.visCols = 1
	}
	start = r.xOff
	end = start + r.visCols
	if end > len(r.data.Columns) {
		end = len(r.data.Columns)
	}
	return start, end
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

// view renders the grid through the bubbles table component: the column
// cursor and sort arrow live in the header, the window of rows in the body.
// Sort/filter/scroll state stays in resultsView; the table is a renderer over
// the visible window, so no internal-scroll drift can hide rows.
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

	// Horizontal window: a fixed column width so wide tables pan instead of
	// squeezing every column to a couple of characters.
	const colWidth = 16
	ncols := len(r.data.Columns)
	r.visCols = (w - 2) / colWidth
	if r.visCols < 1 {
		r.visCols = 1
	}
	if r.visCols > ncols {
		r.visCols = ncols
	}
	if r.xOff > ncols-r.visCols {
		r.xOff = ncols - r.visCols
	}
	if r.xOff < 0 {
		r.xOff = 0
	}
	cStart, cEnd := r.colWindow()

	cols := make([]table.Column, cEnd-cStart)
	for i, c := cStart, 0; i < cEnd; i, c = i+1, c+1 {
		title := r.data.Columns[i]
		if r.sortCol == i {
			if r.sortAsc {
				title += " ▲"
			} else {
				title += " ▼"
			}
		}
		if i == r.selCol {
			title = styleAccent.Render("▸ " + title)
		}
		cols[c] = table.Column{Title: title, Width: colWidth}
	}

	end := r.top + r.viewport
	if end > len(r.order) {
		end = len(r.order)
	}
	rows := make([]table.Row, 0, end-r.top)
	for row := r.top; row < end; row++ {
		line := make(table.Row, cEnd-cStart)
		for i, c := cStart, 0; i < cEnd; i, c = i+1, c+1 {
			line[c] = truncate(r.data.Value(i, r.order[row]), colWidth)
		}
		rows = append(rows, line)
	}

	r.tbl.SetColumns(cols)
	r.tbl.SetRows(rows)
	r.tbl.SetHeight(r.viewport)
	r.tbl.SetWidth(w)
	if r.selRow >= len(rows) {
		r.selRow = len(rows) - 1
	}
	r.tbl.SetCursor(r.selRow)

	var b strings.Builder
	if r.filterMode || r.filter != "" {
		fmt.Fprintf(&b, "[filter: %s▌]\n", r.filter)
	}
	if cEnd-cStart < ncols {
		fmt.Fprintf(&b, "[cols %d-%d of %d]\n", cStart+1, cEnd, ncols)
	}
	b.WriteString(r.tbl.View())
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

// ringCap bounds how many result sets a connection keeps in the ring.
const ringCap = 8

// resultsRing holds the recent result sets for one connection. Each grid keeps
// its own sort/filter/scroll, so flipping through the ring preserves context.
type resultsRing struct {
	grids []*resultsView
	idx   int
}

func newResultsRing() *resultsRing {
	return &resultsRing{}
}

func (r *resultsRing) len() int { return len(r.grids) }

func (r *resultsRing) cur() *resultsView {
	if len(r.grids) == 0 {
		return nil
	}
	return r.grids[r.idx]
}

// push stages a new result set as the active grid, evicting the oldest past the
// cap.
func (r *resultsRing) push(v *resultsView) {
	r.grids = append(r.grids, v)
	r.idx = len(r.grids) - 1
	if len(r.grids) > ringCap {
		r.grids = r.grids[len(r.grids)-ringCap:]
		r.idx = len(r.grids) - 1
	}
}

func (r *resultsRing) next() {
	if len(r.grids) == 0 {
		return
	}
	r.idx = (r.idx + 1) % len(r.grids)
}

func (r *resultsRing) prev() {
	if len(r.grids) == 0 {
		return
	}
	r.idx = (r.idx - 1 + len(r.grids)) % len(r.grids)
}

// drop removes the active grid and moves the cursor to its neighbour.
func (r *resultsRing) drop() {
	if len(r.grids) == 0 {
		return
	}
	r.grids = append(r.grids[:r.idx], r.grids[r.idx+1:]...)
	if r.idx >= len(r.grids) {
		r.idx = len(r.grids) - 1
	}
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
