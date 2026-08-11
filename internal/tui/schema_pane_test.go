package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/claymor333/squal/internal/db"
)

func TestSchemaPaneSelectEmitsBrowse(t *testing.T) {
	p := newSchemaPane([]db.Database{
		{Name: "app", Tables: []db.Table{{Name: "users", Columns: []db.Column{{Name: "id", Key: "PRI"}}}}},
		{Name: "blog", Tables: []db.Table{{Name: "posts"}}},
	})
	// expand first db, move to its first table
	p.toggleDB(0)
	p.moveDown()
	msg := p.selectCurrent()
	req, ok := msg.(browseRequestMsg)
	if !ok {
		t.Fatalf("selectCurrent() = %T, want browseRequestMsg", msg)
	}
	if req.Database != "app" || req.Table != "users" {
		t.Fatalf("got %s.%s, want app.users", req.Database, req.Table)
	}
	if len(req.PK) != 1 || req.PK[0] != "id" {
		t.Fatalf("pk = %v, want [id]", req.PK)
	}
}

func TestSchemaPaneBrowseQuery(t *testing.T) {
	q := browseQuery("app", "users", nil)
	if q != "SELECT * FROM `app`.`users` LIMIT 50000" {
		t.Fatalf("browse query = %q", q)
	}
	qk := browseQuery("app", "users", []string{"id"})
	if qk != "SELECT * FROM `app`.`users` ORDER BY `id` LIMIT 50000" {
		t.Fatalf("keyset browse query = %q", qk)
	}
}

func TestSchemaPaneScrollsWindowWithCursor(t *testing.T) {
	// 1 db with 10 tables; viewport shows 3 lines
	tables := make([]db.Table, 10)
	for i := range tables {
		tables[i] = db.Table{Name: fmt.Sprintf("t%d", i)}
	}
	p := newSchemaPane([]db.Database{{Name: "app", Tables: tables}})
	p.toggleDB(0) // expand so tables are visible
	p.SetLines(3)

	// cursor starts on the db header (line 0)
	if p.top != 0 {
		t.Fatalf("initial top = %d, want 0", p.top)
	}
	// walk down to t5 (line 6); the window must follow the cursor
	for i := 0; i < 6; i++ {
		p.moveDown()
	}
	if p.top != 4 { // cursor 6, 3-line window -> top 4
		t.Fatalf("top = %d, want 4 (cursor 6)", p.top)
	}
	e, ok := p.entryAt(p.cursor)
	if !ok || e.kind != treeTable || e.table != 5 {
		t.Fatalf("entryAt = %+v, want table t5", e)
	}
	// walk back to the header; top must return to 0
	for i := 0; i < 6; i++ {
		p.moveUp()
	}
	if p.top != 0 {
		t.Fatalf("top = %d, want 0 after walking back", p.top)
	}
}

func TestSchemaPaneViewClipsToLines(t *testing.T) {
	tables := make([]db.Table, 5)
	for i := range tables {
		tables[i] = db.Table{Name: fmt.Sprintf("t%d", i)}
	}
	p := newSchemaPane([]db.Database{{Name: "app", Tables: tables}})
	p.toggleDB(0)
	p.SetLines(3)
	out := p.view()
	if n := lineCount(out); n > 3 {
		t.Fatalf("view rendered %d lines, want <= 3", n)
	}
	if !strings.Contains(out, "t0") {
		t.Fatalf("window should start at t0: %q", out)
	}
}

func TestSchemaPaneExpandsColumns(t *testing.T) {
	p := newSchemaPane([]db.Database{{Name: "app", Tables: []db.Table{
		{Name: "users", Columns: []db.Column{{Name: "id", Key: "PRI"}, {Name: "name"}}},
	}}})
	p.toggleDB(0)   // expand db
	p.moveDown()    // cursor on users
	p.toggleTable() // expand columns
	out := p.view()
	if !strings.Contains(out, "id") || !strings.Contains(out, "name") {
		t.Fatalf("columns not shown: %q", out)
	}
	// selectCurrent on a column still browses the containing table
	req, ok := p.selectCurrent().(browseRequestMsg)
	if !ok || req.Table != "users" {
		t.Fatalf("column selectCurrent = %#v, want browse of users", req)
	}
}

func TestSchemaPaneFilter(t *testing.T) {
	p := newSchemaPane([]db.Database{{Name: "app", Tables: []db.Table{{Name: "users"}, {Name: "orders"}}}})
	p.toggleDB(0)
	p.startFilter()
	p.appendFilter('u')
	out := p.view()
	if !strings.Contains(out, "users") || strings.Contains(out, "orders") {
		t.Fatalf("filter not applied: %q", out)
	}
	if p.treeLen() != 2 { // db header + users
		t.Fatalf("filtered treeLen = %d, want 2", p.treeLen())
	}
	p.popFilter()
	if !strings.Contains(p.view(), "orders") {
		t.Fatalf("popFilter should restore rows: %q", p.view())
	}
	p.endFilter()
	if p.filterMode {
		t.Fatal("endFilter should leave filter mode")
	}
}

func TestSchemaPaneCollapse(t *testing.T) {
	p := newSchemaPane([]db.Database{{Name: "app", Tables: []db.Table{{Name: "users"}}}})
	p.toggleDB(0)
	p.moveDown()
	p.SetCollapsed(true)
	if p.treeLen() != 1 { // just the db header
		t.Fatalf("collapsed treeLen = %d, want 1", p.treeLen())
	}
	p.SetCollapsed(false)
	if p.treeLen() != 2 {
		t.Fatalf("restored treeLen = %d, want 2", p.treeLen())
	}
}
