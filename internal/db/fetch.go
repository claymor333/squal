package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Columnar stores a result set as one slice per column (spec §4).
// Sorting/filtering on the client operate over these without touching the server.
type Columnar struct {
	Columns []string
	Types   []string
	// Cols[c][r] is the string value of column c, row r.
	Cols [][]string
	// Rows is the number of rows loaded so far (== len of every Cols[c]).
	Rows int
}

// Batch is a chunk of a streaming result, delivered on the channel.
type Batch struct {
	Rows int  // rows added in this batch
	Done bool // true on final batch
	Err  error
}

// Fetch streams a query into columnar storage, emitting a Batch per ~batchSize rows.
// The channel is closed when fetching finishes. ctx cancellation aborts the read.
// The goroutine appends directly into the returned *Columnar; the caller must only
// read values after receiving a Batch (channel send/receive orders the writes).
// After the channel closes, Rows is the stable total.
func (c *Conn) Fetch(ctx context.Context, query string, batchSize int, args ...any) (*Columnar, <-chan Batch, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	return fetchRows(ctx, rows, batchSize)
}

// FetchOn streams a query against a dedicated connection whose default schema is
// set to dbName (USE) for the duration of the query. Unqualified SQL in the query
// resolves against dbName. The pool is bypassed because a USE on a pooled
// connection would not persist across calls.
func (c *Conn) FetchOn(ctx context.Context, dbName, query string, batchSize int, args ...any) (*Columnar, <-chan Batch, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.ExecContext(ctx, "USE "+QuoteIdentifier(dbName)); err != nil {
		conn.Close()
		return nil, nil, err
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	// rows is bound to conn; closing rows (in fetchRows) returns conn to the pool.
	return fetchRows(ctx, rows, batchSize)
}

// fetchRows runs the shared streaming pipeline over an open *sql.Rows.
func fetchRows(ctx context.Context, rows *sql.Rows, batchSize int) (*Columnar, <-chan Batch, error) {
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return nil, nil, err
	}
	types := make([]string, len(colTypes))
	for i, ct := range colTypes {
		types[i] = ct.DatabaseTypeName()
	}

	col := make([][]string, len(cols))
	for i := range col {
		col[i] = make([]string, 0, batchSize)
	}
	data := &Columnar{Columns: cols, Types: types, Cols: col}

	out := make(chan Batch)
	go func() {
		defer rows.Close()
		defer close(out)

		raw := make([]sql.RawBytes, len(cols))
		scan := make([]any, len(cols))
		for i := range scan {
			scan[i] = &raw[i]
		}

		batch := 0
		flush := func(done bool, err error) {
			out <- Batch{Rows: batch, Done: done, Err: err}
			batch = 0
		}

		for {
			if ok := rows.Next(); !ok {
				if err := rows.Err(); err != nil {
					flush(true, err)
					return
				}
				flush(true, nil)
				return
			}
			if err := rows.Scan(scan...); err != nil {
				flush(true, err)
				return
			}
			for i, rb := range raw {
				if rb == nil {
					col[i] = append(col[i], "")
					continue
				}
				// RawBytes is invalid after Next(); copy immediately.
				col[i] = append(col[i], string(rb))
			}
			data.Rows++
			batch++
			if batch >= batchSize {
				select {
				case <-ctx.Done():
					flush(true, ctx.Err())
					return
				case out <- Batch{Rows: batch, Done: false}:
					batch = 0
				}
			}
		}
	}()

	return data, out, nil
}

// Value returns the string value at column c, row r.
func (d *Columnar) Value(c, r int) string {
	if c < 0 || c >= len(d.Cols) || r < 0 || r >= len(d.Cols[c]) {
		return ""
	}
	return d.Cols[c][r]
}

func (d *Columnar) Summary() string {
	return fmt.Sprintf("%d cols × %d rows", len(d.Columns), d.Rows)
}
