package tui

import "github.com/claymor333/squal/internal/db"

// paneFocus identifies which pane receives key input.
type paneFocus int

const (
	focusSchema paneFocus = iota
	focusEditor
	focusResults
)

type queryStartedMsg struct {
	SQL string
}

type batchMsg struct {
	Rows int
	Done bool
	Err  error
}

type queryDoneMsg struct {
	Elapsed string
	Err     error
}

type loadNextMsg struct {
	LastVals []string
}

// fetchJob carries the in-flight query state into the batch pump.
type fetchJob struct {
	col *db.Columnar
	ch  <-chan db.Batch
	sql string
}

// saveRowMsg requests a structured UPDATE for the row editor. before/after are
// column->value maps; pk names the key columns. Consumed by B5's writer.
type saveRowMsg struct {
	Before map[string]string
	After  map[string]string
	PK     []string
}

// deleteRowMsg requests deletion of the highlighted row by its PK values.
// Consumed by B5's writer.
type deleteRowMsg struct {
	Row map[string]string
	PK  []string
}
