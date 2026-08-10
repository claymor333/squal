package tui

import (
	"testing"

	"github.com/claymor333/squal/internal/db"
)

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
