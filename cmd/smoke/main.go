package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/db"
)

func main() {
	p := config.Profile{
		Name:     "smoke",
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "webstack",
		Password: "webstack",
		Database: "webstack",
		Timeout:  5,
	}

	c, err := db.Open(context.Background(), p)
	if err != nil {
		log.Fatal("open: ", err)
	}
	defer c.Close()

	schema, err := c.Schema(context.Background(), false)
	if err != nil {
		log.Fatal("schema: ", err)
	}
	fmt.Printf("schema: %d databases\n", len(schema))
	for _, d := range schema {
		fmt.Printf("  %s: %d tables\n", d.Name, len(d.Tables))
	}

	// force a big table for the progressive fetch test
	ctx := context.Background()
	{
		c.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS squal_smoke")
		c.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS squal_smoke.big (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			label VARCHAR(64), value DOUBLE, created DATETIME
		)`)
		c.ExecContext(ctx, "TRUNCATE squal_smoke.big")
		c.ExecContext(ctx, `INSERT INTO squal_smoke.big (label, value, created)
			SELECT CONCAT('row-', seq), seq*1.5, NOW()
			FROM (
				SELECT (@n := @n + 1) AS seq
				FROM information_schema.COLUMNS a
				CROSS JOIN information_schema.COLUMNS b
				CROSS JOIN (SELECT @n := 0) r
				LIMIT 100000
			) t`)
	}

	start := time.Now()
	col, ch, err := c.Fetch(context.Background(), "SELECT * FROM squal_smoke.big", 1000)
	if err != nil {
		log.Fatal("fetch start: ", err)
	}
	batches, rows := 0, 0
	var finalErr error
	for b := range ch {
		batches++
		rows += b.Rows
		if b.Err != nil {
			finalErr = b.Err
		}
		if batches <= 3 || b.Done {
			fmt.Printf("  batch %d: +%d rows\n", batches, b.Rows)
		}
	}
	elapsed := time.Since(start)
	fmt.Printf("fetch: %d batches, %d rows, %s, %s\n", batches, rows, elapsed.Round(time.Millisecond), col.Summary())
	fmt.Printf("sample: col1[0]=%q col1[99999]=%q\n", col.Value(1, 0), col.Value(1, rows-1))
	if finalErr != nil {
		log.Fatal("fetch error: ", finalErr)
	}

	pk, err := c.PrimaryKey(context.Background(), "squal_smoke", "big")
	if err != nil {
		log.Fatal("pk: ", err)
	}
	fmt.Printf("pk of squal_smoke.big: %v\n", pk)

	os.Exit(0)
}
