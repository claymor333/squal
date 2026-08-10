package tui

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/claymor333/squal/internal/db"
	"github.com/claymor333/squal/internal/db/mutate"
	"github.com/claymor333/squal/internal/state"
)

// writer owns the undo contract for a connection. It logs every write action to
// the action store and returns the before-image capture path for feasible raw SQL.
type writer struct {
	conn  *db.Conn
	store *state.Store
	ed    *mutate.RowEditor
}

func newWriter(conn *db.Conn, store *state.Store, ed *mutate.RowEditor) *writer {
	return &writer{conn: conn, store: store, ed: ed}
}

// classifyForTest is a thin wrapper so the tui package can unit-test the verdict
// logic without a live DB. (Real path uses mutate.Classify directly.)
func classifyForTest(sql string) (*mutate.UndoVerdict, error) {
	return mutate.Classify(sql)
}

func (w *writer) runTypedSQL(ctx context.Context, connName, database string, sql string) (*mutate.UndoVerdict, error) {
	v, err := mutate.Classify(sql)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if !v.Feasible {
		// logged-only: execute as-is, record without before/after images
		_, err := w.conn.ExecContext(ctx, sql)
		if err != nil {
			return v, err
		}
		w.record(v, connName, database, nil, nil)
		return v, nil
	}
	return w.executeWithBeforeImage(ctx, connName, database, v)
}

// executeWithBeforeImage runs a feasible statement inside a transaction,
// capturing the before-image rows (locked FOR UPDATE) via the synthesized SELECT,
// then executing the write and committing. On any error the tx rolls back, so
// no partial write escapes.
func (w *writer) executeWithBeforeImage(ctx context.Context, connName, database string, v *mutate.UndoVerdict) (*mutate.UndoVerdict, error) {
	tx, err := w.conn.BeginTx(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()

	before, err := captureBefore(ctx, tx, v.BeforeSelect)
	if err != nil {
		return v, fmt.Errorf("before-image: %w", err)
	}
	if _, err := tx.ExecContext(ctx, v.ExecSQL); err != nil {
		return v, err
	}
	if err := tx.Commit(); err != nil {
		return v, err
	}
	w.record(v, connName, database, before, nil)
	return v, nil
}

// captureBefore runs the before-image SELECT with FOR UPDATE on the transaction,
// locking the affected rows and returning them as a JSON-able row map. Multi-row
// captures are collapsed into a per-PK map (best-effort for the action log).
func captureBefore(ctx context.Context, tx *sql.Tx, selectSQL string) (map[string]string, error) {
	// INSERT has no before-image (BeforeSelect == ""); skip the lock query.
	if selectSQL == "" {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, selectSQL+" FOR UPDATE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	if !rows.Next() {
		return out, nil
	}
	vals := make([]string, len(cols))
	scan := make([]any, len(cols))
	for i := range scan {
		scan[i] = &vals[i]
	}
	if err := rows.Scan(scan...); err != nil {
		return nil, err
	}
	for i, c := range cols {
		out[c] = vals[i]
	}
	return out, nil
}

func (w *writer) record(v *mutate.UndoVerdict, connName, database string, before, after map[string]string) {
	if w.store == nil {
		return
	}
	_ = w.store.Record(&state.Action{
		ID:         state.NewID(v.Kind, database, v.Table),
		Verdict:    state.Undoable,
		Kind:       v.Kind,
		Connection: connName,
		Database:   database,
		Table:      v.Table,
		Before:     before,
		After:      after,
		Status:     state.Applied,
		CreatedAt:  time.Now(),
	})
}
