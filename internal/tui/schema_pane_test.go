package tui

import (
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
