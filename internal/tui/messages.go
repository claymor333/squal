package tui

import (
	"github.com/claymor333/squal/internal/db"
	"github.com/claymor333/squal/internal/state"
)

// paneFocus identifies which pane receives key input. The order in this block
// is NOT the tab cycle order — the cycle is an explicit slice built by the
// model (focusCycle), because the rail only joins while open. focusRow /
// focusHistory / focusAI double as the rail's tab identifiers.
type paneFocus int

const (
	focusSchema paneFocus = iota
	focusEditor
	focusResults
	focusRail    // the tabbed context rail (row/hist/ai), only while open
	focusRow     // rail tab: right-side row editor
	focusHistory // rail tab: action history
	focusAI      // rail tab: the AI panel
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
// column->value maps; pk names the key columns. OrigIdx is the grid row index
// being edited so the grid can be patched in place after the write.
type saveRowMsg struct {
	Before  map[string]string
	After   map[string]string
	PK      []string
	OrigIdx int
}

// deleteRowMsg requests deletion of the highlighted row by its PK values.
type deleteRowMsg struct {
	Row     map[string]string
	PK      []string
	OrigIdx int
}

// rowWriteDoneMsg reports a completed structured write. OrigIdx >= 0 patches
// the grid in place (save); Deleted triggers a re-browse of the table.
type rowWriteDoneMsg struct {
	Err     error
	OrigIdx int
	After   map[string]string
	Deleted bool
}

// historyLoadedMsg carries the action-store listing into the history panel.
type historyLoadedMsg struct {
	Rows []*state.Action
	Err  error
}

// undoDoneMsg reports a completed undo and names the table so the model can
// refresh the grid if the active results are a browse of it.
type undoDoneMsg struct {
	Err   error
	DB    string
	Table string
}
