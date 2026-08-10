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
	if _, j, onTable := p.current(); !onTable || j != 5 {
		t.Fatalf("current = (onTable=%v j=%d), want t5", onTable, j)
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
