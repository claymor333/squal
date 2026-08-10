package mutate

import (
	"context"
	"os"
	"testing"

	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/db"
)

func testConn(t *testing.T) *db.Conn {
	t.Helper()
	host := os.Getenv("SQUAL_TEST_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	pass := os.Getenv("SQUAL_TEST_PASS")
	if pass == "" {
		pass = "webstack"
	}
	c, err := db.Open(context.Background(), config.Profile{
		Name: "test", Host: host, Port: 3306, User: "webstack",
		Password: pass, Database: "squal_smoke", Timeout: 5,
	})
	if err != nil {
		t.Skipf("no test DB: %v", err)
	}
	return c
}

func TestRowEditorUpdateUndo(t *testing.T) {
	c := testConn(t)
	defer c.Close()
	ctx := context.Background()
	c.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS editor_t (id INT PRIMARY KEY, name VARCHAR(64))")
	c.ExecContext(ctx, "DELETE FROM editor_t")
	c.ExecContext(ctx, "INSERT INTO editor_t VALUES (1, 'old')")

	ed, err := NewRowEditor(c, "squal_smoke", "editor_t")
	if err != nil {
		t.Fatal(err)
	}
	row, err := ed.Load(ctx, map[string]string{"id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if row["name"] != "old" {
		t.Fatalf("loaded name = %q", row["name"])
	}
	// update
	after := map[string]string{"id": "1", "name": "new"}
	if _, err := ed.Update(ctx, row, after); err != nil {
		t.Fatal(err)
	}
	// undo
	if err := ed.Undo(ctx, "update", map[string]string{"id": "1"}, row, after); err != nil {
		t.Fatal(err)
	}
	back, _ := ed.Load(ctx, map[string]string{"id": "1"})
	if back["name"] != "old" {
		t.Fatalf("after undo name = %q, want old", back["name"])
	}
}
