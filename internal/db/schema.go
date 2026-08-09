package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Column struct {
	Name     string
	Type     string
	Nullable bool
	Key      string // PRI, MUL, UNI, ""
	Default  sql.NullString
}

type Table struct {
	Name    string
	Type    string // BASE TABLE, VIEW, etc.
	Columns []Column
}

type Database struct {
	Name   string
	Tables []Table
}

// Schema loads databases (optionally filtered to those the user can see),
// with columns for each table. Shallow: tables list comes from information_schema
// in one query, columns in a second batch.
func (c *Conn) Schema(ctx context.Context, includeSystem bool) ([]Database, error) {
	show, err := c.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	var names []string
	for show.Next() {
		var n string
		if err := show.Scan(&n); err != nil {
			show.Close()
			return nil, err
		}
		if !includeSystem && (strings.HasPrefix(n, "information_schema") ||
			strings.HasPrefix(n, "mysql") || strings.HasPrefix(n, "performance_schema") ||
			n == "sys") {
			continue
		}
		names = append(names, n)
	}
	show.Close()
	sort.Strings(names)

	dbs := make([]Database, 0, len(names))
	for _, n := range names {
		rows, err := c.db.QueryContext(ctx, `SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, n)
		if err != nil {
			return nil, err
		}
		var tables []Table
		for rows.Next() {
			var t Table
			if err := rows.Scan(&t.Name, &t.Type); err != nil {
				rows.Close()
				return nil, err
			}
			tables = append(tables, t)
		}
		rows.Close()
		sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
		dbs = append(dbs, Database{Name: n, Tables: tables})
	}
	return dbs, nil
}

// Columns loads column metadata for one table.
func (c *Conn) Columns(ctx context.Context, dbName, table string) ([]Column, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, dbName, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		var col Column
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.Key, &col.Default); err != nil {
			return nil, err
		}
		col.Nullable = strings.EqualFold(nullable, "YES")
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// PrimaryKey returns the first column(s) marked PRI for a table, in order.
func (c *Conn) PrimaryKey(ctx context.Context, dbName, table string) ([]string, error) {
	cols, err := c.Columns(ctx, dbName, table)
	if err != nil {
		return nil, err
	}
	var pk []string
	for _, col := range cols {
		if col.Key == "PRI" {
			pk = append(pk, col.Name)
		}
	}
	return pk, nil
}

func (c *Conn) DescribeTable(ctx context.Context, dbName, table string) ([]Column, error) {
	return c.Columns(ctx, dbName, table)
}

// QuoteIdentifier safely quotes an identifier for use in generated SQL.
func QuoteIdentifier(id string) string {
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}

func (d *Database) String() string { return fmt.Sprintf("%s (%d tables)", d.Name, len(d.Tables)) }
