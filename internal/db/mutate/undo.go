package mutate

import (
	"fmt"

	"github.com/claymor333/squal/internal/db"
	"github.com/xwb1989/sqlparser"
)

type UndoVerdict struct {
	Feasible     bool
	Kind         string // insert | update | delete
	Table        string
	ExecSQL      string // the statement to run (B5's writer executes this)
	BeforeSelect string // the SELECT that captures the before-image ("" when infeasible)
	Reason       string
}

// Classify decides whether a raw statement can be made undoable by capturing a
// before-image. Only single-table UPDATE/DELETE with a translatable WHERE (no
// joins, no subqueries, no INTO, no ORDER BY/LIMIT) and plain INSERT qualify.
func Classify(sql string) (*UndoVerdict, error) {
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		return nil, err
	}

	// INSERT: before-image is empty; undo = delete inserted PKs (logged at runtime).
	if ins, ok := stmt.(*sqlparser.Insert); ok {
		switch ins.Rows.(type) {
		case *sqlparser.Select, *sqlparser.Union, *sqlparser.ParenSelect:
			return &UndoVerdict{Feasible: false, Reason: "INSERT ... SELECT cannot be safely undone"}, nil
		default:
			return &UndoVerdict{Feasible: true, Kind: "insert", Table: ins.Table.Name.String(), ExecSQL: sql}, nil
		}
	}

	if upd, ok := stmt.(*sqlparser.Update); ok {
		if len(upd.TableExprs) != 1 {
			return &UndoVerdict{Feasible: false, Reason: "multi-table UPDATE is not undoable"}, nil
		}
		expr, ok := upd.TableExprs[0].(*sqlparser.AliasedTableExpr)
		if !ok || !expr.As.IsEmpty() {
			return &UndoVerdict{Feasible: false, Reason: "aliased/joined UPDATE is not undoable"}, nil
		}
		if upd.Limit != nil {
			return &UndoVerdict{Feasible: false, Reason: "UPDATE ... LIMIT is not undoable"}, nil
		}
		tableName, ok := expr.Expr.(sqlparser.TableName)
		if !ok {
			return &UndoVerdict{Feasible: false, Reason: "subquery table is not undoable"}, nil
		}
		table := tableName.Name.String()
		where := upd.Where
		sel := fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(table))
		if where != nil {
			sel += " WHERE " + sqlparser.String(where.Expr)
		}
		return &UndoVerdict{Feasible: true, Kind: "update", Table: table, ExecSQL: sql, BeforeSelect: sel}, nil
	}

	if del, ok := stmt.(*sqlparser.Delete); ok {
		if len(del.TableExprs) != 1 {
			return &UndoVerdict{Feasible: false, Reason: "multi-table DELETE is not undoable"}, nil
		}
		expr, ok := del.TableExprs[0].(*sqlparser.AliasedTableExpr)
		if !ok || !expr.As.IsEmpty() {
			return &UndoVerdict{Feasible: false, Reason: "aliased/joined DELETE is not undoable"}, nil
		}
		if del.Limit != nil {
			return &UndoVerdict{Feasible: false, Reason: "DELETE ... LIMIT is not undoable"}, nil
		}
		tableName, ok := expr.Expr.(sqlparser.TableName)
		if !ok {
			return &UndoVerdict{Feasible: false, Reason: "subquery table is not undoable"}, nil
		}
		table := tableName.Name.String()
		sel := fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(table))
		if del.Where != nil {
			sel += " WHERE " + sqlparser.String(del.Where.Expr)
		}
		return &UndoVerdict{Feasible: true, Kind: "delete", Table: table, ExecSQL: sql, BeforeSelect: sel}, nil
	}

	return &UndoVerdict{Feasible: false, Reason: "only single-table INSERT/UPDATE/DELETE can be undone"}, nil
}
