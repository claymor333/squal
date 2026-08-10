package db

import (
	"context"
	"os"
	"testing"

	"github.com/claymor333/squal/internal/config"
)

func testConn(t *testing.T) *Conn {
	t.Helper()
	host := os.Getenv("SQUAL_TEST_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	pass := os.Getenv("SQUAL_TEST_PASS")
	if pass == "" {
		pass = "webstack"
	}
	c, err := Open(context.Background(), config.Profile{
		Name: "test", Host: host, Port: 3306, User: "webstack",
		Password: pass, Database: "", Timeout: 5,
	})
	if err != nil {
		t.Skipf("no test DB: %v", err)
	}
	return c
}

func TestFetchOnUsesDefaultDatabase(t *testing.T) {
	c := testConn(t)
	defer c.Close()
	ctx := context.Background()

	// squal_smoke.big exists (created by cmd/smoke). An unqualified SELECT
	// against squal_smoke must resolve only when FetchOn sets the schema.
	col, ch, err := c.FetchOn(ctx, "squal_smoke", "SELECT COUNT(*) FROM big", 10)
	if err != nil {
		t.Fatal(err)
	}
	for b := range ch {
		if b.Err != nil {
			t.Fatal(b.Err)
		}
	}
	if col.Rows != 1 {
		t.Fatalf("rows = %d, want 1", col.Rows)
	}
}
