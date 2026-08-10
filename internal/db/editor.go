package db

import (
	"context"
	"fmt"
	"strings"
)

type RowEditor struct {
	conn  *Conn
	db    string
	table string
	cols  []Column
	pk    []string
}

func NewRowEditor(c *Conn, dbName, table string) (*RowEditor, error) {
	cols, err := c.Columns(context.Background(), dbName, table)
	if err != nil {
		return nil, err
	}
	var pk []string
	for _, col := range cols {
		if col.Key == "PRI" {
			pk = append(pk, col.Name)
		}
	}
	if len(pk) == 0 {
		return nil, fmt.Errorf("table %s.%s has no primary key; cannot edit rows", dbName, table)
	}
	return &RowEditor{conn: c, db: dbName, table: table, cols: cols, pk: pk}, nil
}

// Load returns the full row as a column->value map, keyed by the given PK values.
func (e *RowEditor) Load(ctx context.Context, pkVals map[string]string) (map[string]string, error) {
	sel, args := e.selectByPK(pkVals)
	rows, err := e.conn.QueryContext(ctx, sel, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("no row %v in %s", pkVals, e.table)
	}
	dest := make([]any, len(e.cols))
	out := make([]string, len(e.cols))
	scan := make([]any, len(e.cols))
	for i := range dest {
		scan[i] = &out[i]
	}
	if err := rows.Scan(scan...); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(e.cols))
	for i, col := range e.cols {
		m[col.Name] = out[i]
	}
	return m, nil
}

func (e *RowEditor) selectByPK(pkVals map[string]string) (string, []any) {
	conds := make([]string, 0, len(e.pk))
	args := make([]any, 0, len(e.pk))
	for _, p := range e.pk {
		conds = append(conds, QuoteIdentifier(p)+" = ?")
		args = append(args, pkVals[p])
	}
	return fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s",
		quotedCols(e.cols), QuoteIdentifier(e.db), QuoteIdentifier(e.table),
		strings.Join(conds, " AND ")), args
}

// Update sets the row to `after` (full row) where PK matches. Returns affected count.
func (e *RowEditor) Update(ctx context.Context, before, after map[string]string) (int64, error) {
	sets := make([]string, 0, len(after))
	args := make([]any, 0, len(after))
	for _, col := range e.cols {
		if isPK(e.pk, col.Name) {
			continue
		}
		v, ok := after[col.Name]
		if !ok {
			continue
		}
		sets = append(sets, QuoteIdentifier(col.Name)+" = ?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		return 0, nil
	}
	conds := make([]string, 0, len(e.pk))
	for _, p := range e.pk {
		conds = append(conds, QuoteIdentifier(p)+" = ?")
		args = append(args, after[p])
	}
	res, err := e.conn.ExecContext(ctx, fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s",
		QuoteIdentifier(e.db), QuoteIdentifier(e.table), strings.Join(sets, ", "), strings.Join(conds, " AND ")), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Delete removes the row identified by pkVals.
func (e *RowEditor) Delete(ctx context.Context, pkVals map[string]string) (int64, error) {
	conds := make([]string, 0, len(e.pk))
	args := make([]any, 0, len(e.pk))
	for _, p := range e.pk {
		conds = append(conds, QuoteIdentifier(p)+" = ?")
		args = append(args, pkVals[p])
	}
	res, err := e.conn.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s.%s WHERE %s",
		QuoteIdentifier(e.db), QuoteIdentifier(e.table), strings.Join(conds, " AND ")), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Undo restores a row to its before-image after an update/delete, or removes an insert.
func (e *RowEditor) Undo(ctx context.Context, kind string, pkVals, before, after map[string]string) error {
	switch kind {
	case "update":
		_, err := e.Update(ctx, before, before)
		return err
	case "delete":
		// re-insert the before-image
		cols := make([]string, 0, len(before))
		args := make([]any, 0, len(before))
		for _, col := range e.cols {
			if v, ok := before[col.Name]; ok {
				cols = append(cols, QuoteIdentifier(col.Name))
				args = append(args, v)
			}
		}
		_, err := e.conn.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
			QuoteIdentifier(e.db), QuoteIdentifier(e.table), strings.Join(cols, ", "),
			strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")), args...)
		return err
	case "insert":
		_, err := e.Delete(ctx, pkVals)
		return err
	}
	return fmt.Errorf("unknown undo kind %q", kind)
}

func isPK(pk []string, name string) bool {
	for _, p := range pk {
		if p == name {
			return true
		}
	}
	return false
}

func quotedCols(cols []Column) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = QuoteIdentifier(c.Name)
	}
	return strings.Join(out, ", ")
}
