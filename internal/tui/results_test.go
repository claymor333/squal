package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/claymor333/squal/internal/db"
)

// colsData is a 3-row columnar fixture: id=2,1,3 ; name=b,a,c.
func colsData() *db.Columnar {
	return &db.Columnar{
		Columns: []string{"id", "name"},
		Cols:    [][]string{{"2", "1", "3"}, {"b", "a", "c"}},
		Rows:    3,
	}
}

func TestResultsMoveColAndSortCursor(t *testing.T) {
	r := newResultsView(colsData())
	r.moveCol(1)
	if r.selCol != 1 {
		t.Fatalf("selCol = %d, want 1", r.selCol)
	}
	r.sortCursor()
	if r.sortCol != 1 || !r.sortAsc {
		t.Fatalf("sortCol=%d asc=%v, want 1/true", r.sortCol, r.sortAsc)
	}
	if r.data.Value(0, r.order[0]) != "1" { // names b,a,c → order [1,0,2], first id=1
		t.Fatalf("first row = %q, want 1", r.data.Value(0, r.order[0]))
	}
}

func TestResultsMoveColClamps(t *testing.T) {
	r := newResultsView(colsData())
	r.moveCol(-5)
	if r.selCol != 0 {
		t.Fatalf("selCol after left-clamp = %d, want 0", r.selCol)
	}
	r.moveCol(99)
	if r.selCol != 1 {
		t.Fatalf("selCol after right-clamp = %d, want 1", r.selCol)
	}
}

func TestResultsFilterIncremental(t *testing.T) {
	r := newResultsView(colsData())
	r.startFilter()
	r.appendFilter('b')
	if r.filter != "b" {
		t.Fatalf("filter = %q, want b", r.filter)
	}
	if len(r.order) != 1 { // only name=b survives
		t.Fatalf("filtered rows = %d, want 1", len(r.order))
	}
	r.endFilter()
	if r.filterMode {
		t.Fatal("endFilter should leave filter mode")
	}
}

func TestResultsFilterPop(t *testing.T) {
	r := newResultsView(colsData())
	r.startFilter()
	r.appendFilter('b')
	r.appendFilter('x') // no row matches "bx"
	if len(r.order) != 0 {
		t.Fatalf("filtered rows = %d, want 0", len(r.order))
	}
	r.popFilter()
	if len(r.order) != 1 {
		t.Fatalf("after pop, rows = %d, want 1", len(r.order))
	}
}

func TestResultsViewHighlightsCursorColumn(t *testing.T) {
	r := newResultsView(colsData())
	r.moveCol(1)
	out := r.view(40)
	if !strings.Contains(out, "name") {
		t.Fatalf("view missing header: %q", out)
	}
	if !strings.Contains(out, "▸ name") {
		t.Fatalf("cursor column not highlighted: %q", out)
	}
}

func TestResultsViewTruncatesToWidth(t *testing.T) {
	r := newResultsView(&db.Columnar{
		Columns: []string{"big"},
		Cols:    [][]string{{"this is a very long value that must be clipped"}},
		Rows:    1,
	})
	out := r.view(20)
	for _, ln := range strings.Split(out, "\n") {
		if lipgloss.Width(ln) > 20 {
			t.Fatalf("line exceeds 20 visible cells: %q", ln)
		}
	}
}

func TestRingPushFlipDrop(t *testing.T) {
	r := newResultsRing()
	r.push(newResultsView(colsData()))
	r.push(newResultsView(&db.Columnar{Columns: []string{"x"}, Cols: [][]string{{"1"}}, Rows: 1}))
	if r.len() != 2 {
		t.Fatalf("len = %d, want 2", r.len())
	}
	before := r.cur()
	r.next()
	if r.cur() == before {
		t.Fatal("next() should move the cursor")
	}
	r.next()
	if r.cur() != before {
		t.Fatal("next() past the end should wrap")
	}
	r.prev()
	r.prev()
	if r.cur() != before {
		t.Fatal("prev() past the start should wrap")
	}
	r.drop()
	if r.len() != 1 {
		t.Fatalf("after drop len = %d, want 1", r.len())
	}
}

func TestRingCapEvictsOldest(t *testing.T) {
	r := newResultsRing()
	for i := 0; i < ringCap+2; i++ {
		r.push(newResultsView(&db.Columnar{Columns: []string{"x"}, Cols: [][]string{{fmt.Sprint(i)}}, Rows: 1}))
	}
	if r.len() != ringCap {
		t.Fatalf("len = %d, want cap %d", r.len(), ringCap)
	}
}

func TestResultsSort(t *testing.T) {
	data := &db.Columnar{
		Columns: []string{"id", "name"},
		Cols:    [][]string{{"3", "1", "2"}, {"1", "b", "a"}},
		Rows:    3,
	}
	r := newResultsView(data)
	r.sortCol = 0
	r.rebuildOrder()
	want := []int{1, 2, 0} // ids 1,2,3
	for i, got := range r.order {
		if got != want[i] {
			t.Fatalf("order[%d] = %d, want %d", i, got, want[i])
		}
	}
}

func TestResultsFilter(t *testing.T) {
	data := &db.Columnar{
		Columns: []string{"name"},
		Cols:    [][]string{{"alice", "bob", "alex"}},
		Rows:    3,
	}
	r := newResultsView(data)
	r.filter = "al"
	r.rebuildOrder()
	if len(r.order) != 2 {
		t.Fatalf("filtered to %d rows, want 2", len(r.order))
	}
}

func TestResultsVisibleRows(t *testing.T) {
	data := &db.Columnar{
		Columns: []string{"id"},
		Cols:    [][]string{{"0", "1", "2", "3", "4", "5"}},
		Rows:    6,
	}
	r := newResultsView(data)
	r.top = 2
	rows := r.visibleRows(3)
	if len(rows) != 3 || rows[0] != 2 || rows[2] != 4 {
		t.Fatalf("visibleRows = %v", rows)
	}
}
