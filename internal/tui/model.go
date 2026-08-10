package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/claymor333/squal/internal/ai"
	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/db"
	"github.com/claymor333/squal/internal/db/mutate"
	"github.com/claymor333/squal/internal/state"
)

// --- messages ---------------------------------------------------------------

type schemaLoadedMsg struct {
	idx  int
	conn *db.Conn
	dbs  []db.Database
	err  error
}

type connClosedMsg struct {
	idx int
}

// --- connection state --------------------------------------------------------

type connData struct {
	profile     config.Profile
	conn        *db.Conn
	loading     bool
	dbs         []db.Database
	loadErr     error
	pane        *schemaPane
	ed          *editor
	wr          *writer
	results     *resultsView
	row         *rowPanel // right-side row editor; nil when closed
	hist        *historyView
	browse      *browseRequestMsg // table browsed by the current results, for load-next
	currentDB   string            // database selected in the tree; default schema for unqualified SQL
	job         *fetchJob
	queryStart  time.Time
	lastElapsed string
}

func openConnection(idx int, p config.Profile) tea.Cmd {
	return func() tea.Msg {
		c, err := db.Open(context.Background(), p)
		if err != nil {
			return schemaLoadedMsg{idx: idx, err: err}
		}
		dbs, err := c.Schema(context.Background(), false)
		if err != nil {
			c.Close()
			return schemaLoadedMsg{idx: idx, err: err}
		}
		return schemaLoadedMsg{idx: idx, conn: c, dbs: dbs}
	}
}

func closeConnection(idx int, c *db.Conn) tea.Cmd {
	return func() tea.Msg {
		if c != nil {
			c.Close()
		}
		return connClosedMsg{idx: idx}
	}
}

// quitCmd closes every live connection and the app-side store so the process
// exits cleanly. It runs alongside tea.Quit on ctrl+c.
func (m *model) quitCmd() tea.Cmd {
	conns := m.conns
	store := m.store
	return func() tea.Msg {
		for _, c := range conns {
			if c.conn != nil {
				c.conn.Close()
			}
		}
		if store != nil {
			store.Close()
		}
		return nil
	}
}

// wireAI binds the AI panel to a live connection: client, session, registry
// (with the confirm-flow and OnQuery), and the tool-calling agent.
func (m *model) wireAI(c *connData, conn *db.Conn) {
	if m.ai == nil {
		m.ai = newAIPanel()
	}
	client := ai.New(m.cfg.AI)
	m.ai.client = client
	m.ai.session = ai.NewSession(client)
	confirm := func(toolName string, args map[string]any, sql string) (bool, error) {
		ch := make(chan bool)
		m.ai.pendingConfirmCh = ch
		m.ai.pendingConfirm = toolName
		return <-ch, nil
	}
	m.ai.registry = ai.NewRegistry(conn, c.wr.ed, c.wr.store, confirm)
	m.ai.registry.OnQuery = func(col *db.Columnar) {
		if col == nil {
			return
		}
		c.results = newResultsView(col)
		c.results.done = true
		c.results.loading = false
		c.results.total = col.Rows
		c.job = nil
		c.browse = nil
	}
	// Transport selection lives in the ai package: probes ToolsSupported and
	// falls back to the text protocol when the endpoint lacks native tools.
	m.ai.agent = ai.NewAgentForClient(client, m.ai.registry, m.ai.session)
	if c.wr.store != nil {
		m.ai.agent.SetTranscript(c.wr.store, c.profile.Name)
	}
}

// --- model -------------------------------------------------------------------

type model struct {
	cfg          *config.Config
	conns        []*connData
	active       int
	focus        paneFocus
	connect      *connectView
	ai           *aiPanel
	store        *state.Store
	width        int
	height       int
	transientErr error // last transient error (config save, store read); cleared on the next key
	confirm      *confirmModal
	help         bool
	quitting     bool
}

// confirmModal is a y/n prompt overlaying the layout. yes builds the command to
// run once confirmed; the model executes it only on an explicit 'y'.
type confirmModal struct {
	prompt string
	yes    func(*model) tea.Cmd
}

func (c *confirmModal) view() string {
	return styleErr.Render("⚠ " + c.prompt + " (y/n)")
}

func New(cfg *config.Config, profiles []config.Profile) *model {
	m := &model{cfg: cfg, focus: focusSchema, ai: newAIPanel()}
	for i, p := range profiles {
		m.conns = append(m.conns, &connData{profile: p, loading: true, ed: newEditor()})
		m.active = i
	}
	return m
}

// ensureStore opens the app-side SQLite store (action log + AI transcript)
// once and reuses it across connections. Failure is non-fatal: the app runs
// without persistence rather than refusing to start.
func (m *model) ensureStore() {
	if m.store != nil {
		return
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "squal", "state.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	s, err := state.Open(path)
	if err != nil {
		return
	}
	m.store = s
}

func (m *model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i, c := range m.conns {
		if c.conn == nil {
			cmds = append(cmds, openConnection(i, c.profile))
		}
	}
	return tea.Batch(cmds...)
}

// --- tab navigation ----------------------------------------------------------

func (m *model) nextTab() {
	if len(m.conns) > 1 {
		m.active = (m.active + 1) % len(m.conns)
	}
}

func (m *model) prevTab() {
	if len(m.conns) > 1 {
		m.active = (m.active - 1 + len(m.conns)) % len(m.conns)
	}
}

// focusCycle returns the tab order for the active connection. The row and
// history panes only join the cycle while they are open.
func (m *model) focusCycle() []paneFocus {
	cyc := []paneFocus{focusSchema, focusEditor, focusResults}
	if m.active >= 0 && m.active < len(m.conns) {
		cur := m.conns[m.active]
		if cur.row != nil {
			cyc = append(cyc, focusRow)
		}
		if cur.hist != nil {
			cyc = append(cyc, focusHistory)
		}
	}
	return append(cyc, focusAI)
}

// nextFocus advances the key focus through the open panes in tab order.
func (m *model) nextFocus() paneFocus {
	cyc := m.focusCycle()
	for i, f := range cyc {
		if f == m.focus {
			return cyc[(i+1)%len(cyc)]
		}
	}
	return cyc[0]
}

// prevFocus steps the key focus back through the open panes.
func (m *model) prevFocus() paneFocus {
	cyc := m.focusCycle()
	for i, f := range cyc {
		if f == m.focus {
			return cyc[(i-1+len(cyc))%len(cyc)]
		}
	}
	return cyc[len(cyc)-1]
}

// onBrowse converts a schema selection into a browse query ready to run.
func (m *model) onBrowse(req browseRequestMsg) any {
	return runQueryMsg{SQL: browseQuery(req.Database, req.Table, req.PK)}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		m.handleMouse(msg)
		return m, nil

	case schemaLoadedMsg:
		if msg.idx < 0 || msg.idx >= len(m.conns) {
			if msg.conn != nil {
				msg.conn.Close()
			}
			return m, nil
		}
		c := m.conns[msg.idx]
		c.loading = false
		if msg.err != nil {
			c.loadErr = msg.err
			if msg.conn != nil {
				msg.conn.Close()
			}
		} else {
			c.conn = msg.conn
			c.dbs = msg.dbs
			c.loadErr = nil
			c.pane = newSchemaPane(msg.dbs)
			m.ensureStore()
			c.wr = newWriter(msg.conn, m.store, nil)
			m.wireAI(c, msg.conn)
		}
		return m, nil

	case connClosedMsg:
		if msg.idx >= 0 && msg.idx < len(m.conns) {
			m.conns[msg.idx].conn = nil
		}
		if len(m.conns) > 0 {
			// drop the tab entirely
			m.conns = append(m.conns[:msg.idx], m.conns[msg.idx+1:]...)
		}
		if m.active >= len(m.conns) {
			m.active = len(m.conns) - 1
		}
		if len(m.conns) == 0 {
			m.quitting = true
		}
		return m, nil

	case batchMsg:
		return m, m.handleBatch(msg)

	case queryDoneMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if cur.results != nil {
			cur.results.loading = false
			cur.results.done = true
			if msg.Err != nil {
				cur.results.err = msg.Err
			} else {
				cur.lastElapsed = msg.Elapsed
			}
		} else if msg.Err != nil {
			m.transientErr = msg.Err
		}
		return m, nil

	case aiEventMsg:
		if m.ai != nil {
			m.ai.events = append(m.ai.events, msg)
		}
		return m, nil

	case runQueryMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		cur.browse = nil
		return m, m.startFetch(context.Background(), msg.SQL)

	case saveRowMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if cur.wr == nil || cur.browse == nil {
			return m, nil
		}
		return m, m.doSaveRow(cur, msg.Before, msg.After, msg.OrigIdx)

	case deleteRowMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if cur.wr == nil || cur.browse == nil {
			return m, nil
		}
		return m, m.doDeleteRow(cur, msg.Row, msg.PK)

	case rowWriteDoneMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if msg.Err != nil {
			if cur.results != nil {
				cur.results.err = msg.Err
			}
			return m, nil
		}
		if msg.Deleted && cur.browse != nil {
			// the row is gone; re-browse so the grid reflects the table
			if rq, ok := m.onBrowse(*cur.browse).(runQueryMsg); ok {
				return m, m.startFetch(context.Background(), rq.SQL)
			}
		}
		if cur.results != nil && msg.OrigIdx >= 0 && msg.After != nil {
			patchRow(cur.results, msg.OrigIdx, msg.After)
		}
		cur.row = nil
		if m.focus == focusRow {
			m.focus = focusResults
		}
		return m, nil

	case historyLoadedMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if cur.hist == nil {
			return m, nil
		}
		if msg.Err != nil {
			m.transientErr = msg.Err
			return m, nil
		}
		cur.hist.SetActions(msg.Rows)
		return m, nil

	case undoDoneMsg:
		if m.active < 0 || m.active >= len(m.conns) {
			return m, nil
		}
		cur := m.conns[m.active]
		if msg.Err != nil {
			m.transientErr = msg.Err
			return m, nil
		}
		var cmds []tea.Cmd
		if m.store != nil {
			cmds = append(cmds, func() tea.Msg {
				rows, err := m.store.List(50)
				return historyLoadedMsg{Rows: rows, Err: err}
			})
		}
		if cur.browse != nil && cur.browse.Database == msg.DB && cur.browse.Table == msg.Table {
			if rq, ok := m.onBrowse(*cur.browse).(runQueryMsg); ok {
				cmds = append(cmds, m.startFetch(context.Background(), rq.SQL))
			}
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// doSaveRow runs the structured UPDATE off the event loop.
func (m *model) doSaveRow(cur *connData, before, after map[string]string, orig int) tea.Cmd {
	b := *cur.browse
	connName := cur.profile.Name
	return func() tea.Msg {
		err := cur.wr.saveRow(context.Background(), connName, b.Database, b.Table, before, after)
		return rowWriteDoneMsg{Err: err, OrigIdx: orig, After: after}
	}
}

// doDeleteRow runs the structured DELETE off the event loop.
func (m *model) doDeleteRow(cur *connData, row map[string]string, pk []string) tea.Cmd {
	b := *cur.browse
	connName := cur.profile.Name
	return func() tea.Msg {
		err := cur.wr.deleteRow(context.Background(), connName, b.Database, b.Table, pk, row)
		return rowWriteDoneMsg{Err: err, Deleted: true}
	}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.connect != nil {
		return m.handleConnectKey(msg)
	}
	if m.help {
		if msg.String() == "esc" {
			m.help = false
		}
		if msg.String() != "ctrl+c" {
			return m, nil
		}
		// ctrl+c quits even from the help overlay
	}
	if m.confirm != nil {
		// modal: y approves, n/esc cancels, ctrl+c falls through to quit.
		switch msg.String() {
		case "y":
			yes := m.confirm.yes
			m.confirm = nil
			if yes != nil {
				return m, yes(m)
			}
		case "n", "esc":
			m.confirm = nil
		}
		if msg.String() != "ctrl+c" {
			return m, nil
		}
	}
	m.transientErr = nil // the status error slot is one-shot; any key clears it
	switch msg.String() {
	case "ctrl+c":
		// immediately exit; never just drop a tab (that is q/ctrl+d's job)
		return m, tea.Batch(m.quitCmd(), tea.Quit)
	case "ctrl+d":
		if m.active >= 0 && m.active < len(m.conns) {
			return m, closeConnection(m.active, m.conns[m.active].conn)
		}
		return m, nil
	case "f1":
		m.help = true
		return m, nil
	case "?":
		if m.focus == focusEditor || m.focus == focusAI || m.focus == focusRow {
			break // '?' is SQL/input text in these panes; use F1 for help
		}
		m.help = true
		return m, nil
	case "tab":
		m.focus = m.nextFocus()
		return m, nil
	case "shift+tab":
		m.focus = m.prevFocus()
		return m, nil
	case "u":
		return m, m.toggleHistory(m.conns[m.active])
	case "q":
		if m.focus == focusEditor || m.focus == focusAI || m.focus == focusRow || m.focus == focusHistory || m.active < 0 || m.active >= len(m.conns) {
			break // 'q' is text input in the editor/AI panel; ignore it elsewhere
		}
		cur := m.conns[m.active]
		m.confirm = &confirmModal{
			prompt: "close connection " + cur.profile.Name + "?",
			yes: func(m *model) tea.Cmd {
				return closeConnection(m.active, m.conns[m.active].conn)
			},
		}
		return m, nil
	}
	if m.active < 0 || m.active >= len(m.conns) {
		return m, nil
	}
	cur := m.conns[m.active]

	switch m.focus {
	case focusSchema:
		switch msg.String() {
		case "c":
			m.connect = newConnectView()
		case "up":
			if cur.pane != nil {
				cur.pane.moveUp()
				cur.currentDB = cur.pane.currentDatabase()
			}
		case "down":
			if cur.pane != nil {
				cur.pane.moveDown()
				cur.currentDB = cur.pane.currentDatabase()
			}
		case "enter":
			if cur.pane != nil && cur.conn != nil {
				cur.currentDB = cur.pane.currentDatabase()
				if req, ok := cur.pane.selectCurrent().(browseRequestMsg); ok {
					if rq, ok := m.onBrowse(req).(runQueryMsg); ok {
						cur.browse = &req
						cur.currentDB = req.Database
						return m, m.startFetch(context.Background(), rq.SQL)
					}
				}
			}
		}
	case focusEditor:
		switch msg.String() {
		case "up":
			if cur.ed != nil {
				cur.ed.moveUp()
			}
		case "down":
			if cur.ed != nil {
				cur.ed.moveDown()
			}
		case "left":
			if cur.ed != nil {
				cur.ed.moveLeft()
			}
		case "right":
			if cur.ed != nil {
				cur.ed.moveRight()
			}
		case "backspace":
			if cur.ed != nil {
				cur.ed.backspace()
			}
		case "ctrl+p":
			if cur.ed != nil {
				cur.ed.historyBack()
			}
		case "ctrl+n":
			if cur.ed != nil {
				cur.ed.historyForward()
			}
		case "enter":
			// Enter inserts a newline; running the query moved to alt+enter /
			// ctrl+r so multi-line SQL is possible (U3).
			if cur.ed != nil {
				cur.ed.newline()
			}
		case "alt+enter", "ctrl+r":
			if cur.ed != nil {
				if rq, ok := cur.ed.runQuery().(runQueryMsg); ok {
					if cur.conn != nil {
						cur.browse = nil
						return m, m.startFetch(context.Background(), rq.SQL)
					}
				}
			}
		default:
			runes := []rune(msg.String())
			if len(runes) == 1 && cur.ed != nil {
				cur.ed.insert(runes[0])
			}
		}
	case focusResults:
		r := cur.results
		switch msg.String() {
		case "up":
			scrollResults(r, -1)
		case "down":
			scrollResults(r, 1)
		case "left":
			if r != nil {
				r.moveCol(-1)
			}
		case "right":
			if r != nil {
				r.moveCol(1)
			}
		case "s":
			if r != nil {
				r.sortCursor()
			}
		case "f":
			if r != nil {
				r.startFilter()
			}
		case "esc":
			if r != nil {
				r.endFilter()
			}
		case "backspace":
			if r != nil {
				r.popFilter()
			}
		case "n":
			return m, m.loadNext(cur)
		case "enter", "o":
			if cur.browse != nil && r != nil && len(r.order) > 0 {
				idx := r.order[r.top+r.selRow]
				p := newRowPanel()
				p.SetRow(rowAt(r, idx))
				cur.row = p
				m.focus = focusRow
			}
		case "delete":
			if cur.browse != nil && r != nil && len(r.order) > 0 {
				idx := r.order[r.top+r.selRow]
				row := rowAt(r, idx)
				b := *cur.browse
				m.confirm = &confirmModal{
					prompt: "delete " + b.Table + " row (" + fmtPK(row, b.PK) + ")?",
					yes: func(m *model) tea.Cmd {
						return m.doDeleteRow(cur, row, b.PK)
					},
				}
			}
		default:
			runes := []rune(msg.String())
			if r != nil && r.filterMode && len(runes) == 1 {
				r.appendFilter(runes[0])
			}
		}
	case focusRow:
		if cur.row == nil {
			break
		}
		p := cur.row
		switch {
		case p.editing:
			switch msg.String() {
			case "enter":
				p.commitEdit()
			case "backspace":
				p.backspaceEdit()
			case "esc":
				p.cancelEdit()
			default:
				runes := []rune(msg.String())
				if len(runes) == 1 {
					p.appendEdit(runes[0])
				}
			}
		case p.raw:
			switch msg.String() {
			case "esc", "enter":
				p.toggleRaw()
			case "backspace":
				if len(p.rawBuf) > 0 {
					p.rawBuf = p.rawBuf[:len(p.rawBuf)-1]
				}
			default:
				runes := []rune(msg.String())
				if len(runes) == 1 {
					p.rawBuf += string(runes[0])
				}
			}
		default:
			switch msg.String() {
			case "up":
				p.moveUp()
			case "down":
				p.moveDown()
			case "enter":
				p.startEdit()
			case "esc":
				cur.row = nil
				m.focus = focusResults
			case "r":
				p.toggleRaw()
			case "ctrl+s", "s":
				r := cur.results
				if r == nil || cur.browse == nil || len(r.order) == 0 {
					break
				}
				idx := r.order[r.top+r.selRow]
				b := *cur.browse
				before := rowAt(r, idx)
				return m, func() tea.Msg {
					after, err := p.Values()
					if err != nil {
						return rowWriteDoneMsg{Err: err}
					}
					return saveRowMsg{Before: before, After: after, PK: b.PK, OrigIdx: idx}
				}
			}
		}
	case focusHistory:
		if cur.hist == nil {
			break
		}
		switch msg.String() {
		case "up":
			cur.hist.moveUp()
		case "down":
			cur.hist.moveDown()
		case "enter":
			if ua, ok := cur.hist.selectRow().(undoActionMsg); ok {
				return m, m.doUndo(cur, ua.ID)
			}
		case "esc":
			cur.hist = nil
			m.focus = focusResults
		}
	case focusAI:
		if m.ai == nil {
			break
		}
		switch msg.String() {
		case "a":
			m.ai.toggleMode()
		case "esc":
			m.ai.interrupt()
		case "enter":
			if m.ai.mode == modeAsk {
				return m, m.ai.runAsk(context.Background())
			}
			return m, m.ai.runQuick(context.Background())
		case "y":
			if m.ai.pendingConfirm != "" {
				m.ai.confirm(true)
			}
		case "n":
			if m.ai.pendingConfirm != "" {
				m.ai.confirm(false)
			}
		default:
			runes := []rune(msg.String())
			if len(runes) == 1 {
				m.ai.request += string(runes[0])
			}
		}
	}
	return m, nil
}

// handleConnectKey routes keys while the new-connection modal is open.
func (m *model) handleConnectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.connect = nil
		return m, nil
	case "ctrl+c":
		// ctrl+c always exits the app, even mid-dialog
		return m, tea.Batch(m.quitCmd(), tea.Quit)
	case "enter":
		name := m.connect.value(hostField)
		if name == "" {
			name = "conn"
		}
		p, ok := m.connect.buildProfile(name)
		m.connect = nil
		if !ok {
			return m, nil
		}
		m.cfg.AddProfile(p)
		if err := m.cfg.Save(); err != nil {
			m.transientErr = err
			return m, nil
		}
		idx := len(m.conns)
		m.conns = append(m.conns, &connData{profile: p, loading: true, ed: newEditor()})
		m.active = idx
		return m, openConnection(idx, p)
	case "tab":
		m.connect.cur = (m.connect.cur + 1) % numConnectFields
		return m, nil
	case "shift+tab":
		m.connect.cur = (m.connect.cur - 1 + numConnectFields) % numConnectFields
		return m, nil
	default:
		runes := []rune(msg.String())
		if len(runes) == 1 {
			m.connect.setField(m.connect.cur, m.connect.value(m.connect.cur)+string(runes[0]))
		}
		return m, nil
	}
}

// toggleHistory opens or closes the action-history panel. Opening kicks off a
// store.List so the panel fills asynchronously.
func (m *model) toggleHistory(cur *connData) tea.Cmd {
	if m.store == nil {
		return nil
	}
	if cur.hist != nil {
		cur.hist = nil
		if m.focus == focusHistory {
			m.focus = focusResults
		}
		return nil
	}
	cur.hist = newHistoryView(nil)
	m.focus = focusHistory
	return func() tea.Msg {
		rows, err := m.store.List(50)
		return historyLoadedMsg{Rows: rows, Err: err}
	}
}

// doUndo restores an action from the log via RowEditor.Undo, marks it Undone,
// and returns a message so the model can refresh both the list and the grid.
func (m *model) doUndo(cur *connData, id string) tea.Cmd {
	store := m.store
	conn := cur.conn
	wr := cur.wr
	return func() tea.Msg {
		act, err := store.Find(id)
		if err != nil {
			return undoDoneMsg{Err: err}
		}
		ctx := context.Background()
		pk, err := conn.PrimaryKey(ctx, act.Database, act.Table)
		if err != nil {
			return undoDoneMsg{Err: err}
		}
		pkVals := make(map[string]string, len(pk))
		for _, p := range pk {
			pkVals[p] = act.Before[p]
		}
		ed, err := wr.editorFor(act.Database, act.Table)
		if err != nil {
			return undoDoneMsg{Err: err}
		}
		if err := ed.Undo(ctx, act.Kind, pkVals, act.Before, act.After); err != nil {
			return undoDoneMsg{Err: err}
		}
		_ = store.SetStatus(id, state.Undone)
		return undoDoneMsg{DB: act.Database, Table: act.Table}
	}
}

// --- query pipeline ----------------------------------------------------------

// startFetch launches a query on a goroutine and returns the cmd that pumps
// batches into the tea event loop as batchMsg values. When the active tab has a
// currentDB selected in the tree, the query runs with that default schema
// (FetchOn) so unqualified SQL resolves against the selected database.
func (m *model) startFetch(ctx context.Context, sql string) tea.Cmd {
	cur := m.conns[m.active]
	var col *db.Columnar
	var ch <-chan db.Batch
	var err error
	if cur.browse == nil && cur.currentDB != "" {
		col, ch, err = cur.conn.FetchOn(ctx, cur.currentDB, sql, 1000)
	} else {
		col, ch, err = cur.conn.Fetch(ctx, sql, 1000)
	}
	if err != nil {
		cur.job = nil
		return func() tea.Msg { return queryDoneMsg{Err: err} }
	}
	cur.job = &fetchJob{col: col, ch: ch, sql: sql}
	cur.results = newResultsView(col)
	cur.results.loading = true
	cur.queryStart = time.Now()
	return func() tea.Msg {
		b, ok := <-ch
		if !ok {
			return queryDoneMsg{Elapsed: time.Since(cur.queryStart).Round(time.Millisecond).String()}
		}
		return batchMsg{Rows: b.Rows, Done: b.Done, Err: b.Err}
	}
}

// pumpBatch re-arms the receive loop for the next chunk.
func (m *model) pumpBatch(cur *connData) tea.Cmd {
	if cur.job == nil {
		return nil
	}
	ch := cur.job.ch
	return func() tea.Msg {
		b, ok := <-ch
		if !ok {
			return queryDoneMsg{Elapsed: time.Since(cur.queryStart).Round(time.Millisecond).String()}
		}
		return batchMsg{Rows: b.Rows, Done: b.Done, Err: b.Err}
	}
}

// handleBatch applies a chunk to the active results view and re-arms the pump
// unless the fetch is done or errored.
func (m *model) handleBatch(msg batchMsg) tea.Cmd {
	if m.active < 0 || m.active >= len(m.conns) {
		return nil
	}
	cur := m.conns[m.active]
	if msg.Err != nil {
		cur.results.err = msg.Err
		cur.results.loading = false
		return nil
	}
	cur.results.appendBatch(msg.Rows)
	if msg.Done {
		cur.results.loading = false
		cur.results.done = true
		return nil
	}
	return m.pumpBatch(cur)
}

// loadNext keyset-paginates the current browse via mutate.LoadNextSQL. It is a
// no-op unless the active results are a finished fetch of a browsed table with
// a resolvable primary key.
func (m *model) loadNext(cur *connData) tea.Cmd {
	if cur == nil || cur.conn == nil || cur.browse == nil || cur.results == nil {
		return nil
	}
	if !cur.results.done || cur.results.err != nil {
		return nil
	}
	req := *cur.browse
	ctx := context.Background()
	pk, err := cur.conn.PrimaryKey(ctx, req.Database, req.Table)
	if err != nil || len(pk) == 0 {
		return nil
	}
	lastVals, ok := lastPKVals(cur.results, pk)
	if !ok {
		return nil
	}
	sql, err := mutate.LoadNextSQL(req.Database, req.Table, pk, lastVals, browseLimit)
	if err != nil {
		return nil
	}
	return m.startFetch(ctx, sql)
}

func lastPKVals(r *resultsView, pk []string) ([]string, bool) {
	if r.data == nil || r.data.Rows == 0 {
		return nil, false
	}
	cols := make([]int, len(pk))
	for i, name := range pk {
		idx := -1
		for c, col := range r.data.Columns {
			if col == name {
				idx = c
				break
			}
		}
		if idx < 0 {
			return nil, false
		}
		cols[i] = idx
	}
	last := r.data.Rows - 1
	vals := make([]string, len(pk))
	for i, idx := range cols {
		vals[i] = r.data.Value(idx, last)
	}
	return vals, true
}

// --- results interaction -----------------------------------------------------

// scrollResults moves the selection by one logical row, sliding the window
// when the selection would leave the viewport.
func scrollResults(r *resultsView, dir int) {
	if r == nil || len(r.order) == 0 {
		return
	}
	if dir > 0 {
		if r.top+r.selRow < len(r.order)-1 {
			if r.selRow < r.viewport-1 {
				r.selRow++
			} else {
				r.top++
			}
		}
		return
	}
	if r.top+r.selRow > 0 {
		if r.selRow > 0 {
			r.selRow--
		} else {
			r.top--
		}
	}
}

// sort/column/filter interaction moved into results.go (sortCursor, moveCol,
// startFilter, ...) in U2; the model only routes keys to them.

// fmtPK renders pk=value pairs for confirm prompts and status text.
func fmtPK(row map[string]string, pk []string) string {
	vals := make([]string, len(pk))
	for i, p := range pk {
		vals[i] = p + "=" + row[p]
	}
	return strings.Join(vals, ",")
}

// --- view --------------------------------------------------------------------

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	stylePane   = lipgloss.NewStyle().Padding(0, 1)
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

func (m *model) View() string {
	if m.quitting {
		return "bye\n"
	}
	if len(m.conns) == 0 {
		return "No connections configured.\n"
	}

	cur := m.conns[m.active]
	rs := newLayout().rects(m.width, m.height, m.focus, cur.row != nil, cur.hist != nil)

	var out strings.Builder
	out.WriteString(fit(renderTabs(m.conns, m.active), rs.tabs.W, rs.tabs.H))

	schemaBody := renderSchemaTree(cur.dbs)
	switch {
	case cur.loading:
		schemaBody = styleDim.Render("connecting to " + cur.profile.Name + "…")
	case cur.loadErr != nil:
		schemaBody = styleErr.Render("connection failed: " + cur.loadErr.Error())
	case cur.pane != nil:
		cur.pane.SetLines(rs.schema.H - 3) // title + borders
		schemaBody = cur.pane.view()
	}
	schemaBox := paneBox("schema", m.focus == focusSchema, schemaBody, rs.schema.W, rs.schema.H)

	editorBody := styleDim.Render("(empty)")
	if cur.ed != nil {
		cur.ed.SetViewport(rs.editor.W-2, rs.editor.H-2)
		editorBody = cur.ed.view()
	}
	resultsBody := styleDim.Render("(no query)")
	if cur.results != nil {
		cur.results.SetViewport(rs.results.H - 4) // title + borders + column header
		resultsBody = cur.results.view(rs.results.W - 2)
	}
	aiBody := renderAI(m.ai)
	if m.ai != nil {
		m.ai.SetLines(rs.ai.H - 3)
	}

	// Right column: editor, results, [history], ai stacked, with the row panel
	// as a right-hand strip when open.
	var right []string
	right = append(right, paneBox("editor", m.focus == focusEditor, editorBody, rs.editor.W, rs.editor.H))
	right = append(right, paneBox("results", m.focus == focusResults, resultsBody, rs.results.W, rs.results.H))
	if cur.hist != nil {
		right = append(right, paneBox("history", m.focus == focusHistory, cur.hist.view(), rs.hist.W, rs.hist.H))
	}
	right = append(right, paneBox("ai", m.focus == focusAI, aiBody, rs.ai.W, rs.ai.H))

	rightCol := lipgloss.JoinVertical(lipgloss.Left, right...)
	if cur.row != nil {
		rightCol = lipgloss.JoinHorizontal(lipgloss.Top, rightCol, paneBox("row", m.focus == focusRow, cur.row.view(), rs.row.W, rs.row.H))
	}
	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, schemaBox, rightCol))

	var elapsed string
	if cur.results != nil && cur.job != nil {
		if cur.results.done {
			elapsed = cur.lastElapsed
		} else {
			elapsed = time.Since(cur.queryStart).Round(time.Millisecond).String() + "…"
		}
	}
	if elapsed == "" {
		elapsed = "–"
	}
	statusErr := error(nil)
	if cur.results != nil {
		statusErr = cur.results.err
	}
	if statusErr == nil {
		statusErr = m.transientErr
	}
	if m.confirm != nil {
		out.WriteString(fit(m.confirm.view(), rs.status.W, rs.status.H))
	} else {
		out.WriteString(fit(statusView(statusInfo{
			Conn:    cur.profile.Name,
			Results: cur.results,
			Elapsed: elapsed,
			Err:     statusErr,
			Focus:   m.focus,
		}), rs.status.W, rs.status.H))
	}

	base := out.String()
	if m.help {
		box := renderHelp(m.focus, m.width, m.height)
		pad := (m.height - lineCount(box)) / 2
		if pad > 0 {
			return strings.Repeat("\n", pad) + box + "\n"
		}
		return box + "\n"
	}
	if m.connect != nil {
		return renderConnectModal(m.connect) + "\n" + base
	}
	return base
}

// renderTabs draws the connection tabs, highlighting the active one.
func renderTabs(conns []*connData, active int) string {
	var tabs string
	for i, c := range conns {
		label := c.profile.Name
		if i == active {
			tabs += styleTitle.Render(fmt.Sprintf(" %s ", label)) + " "
		} else {
			tabs += styleDim.Render(" "+label+" ") + " "
		}
	}
	return tabs
}

// renderAI renders the streaming AI panel, or an idle hint when unwired.
func renderAI(p *aiPanel) string {
	if p == nil {
		return styleDim.Render("(ai panel unconfigured)")
	}
	return p.view()
}

var connectFieldLabels = [numConnectFields]string{"host", "port", "user", "pass", "db"}

func renderConnectModal(c *connectView) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("new connection") + "\n")
	for f := connectField(0); f < numConnectFields; f++ {
		mark := "  "
		if f == c.cur {
			mark = "▸ "
		}
		fmt.Fprintf(&b, "%s%s: %s\n", mark, connectFieldLabels[f], c.value(f))
	}
	b.WriteString(styleDim.Render("enter connect · esc cancel"))
	return b.String()
}

func renderSchemaTree(dbs []db.Database) string {
	var out string
	for _, d := range dbs {
		out += styleAccent.Render("▸ "+d.Name) + " " + styleDim.Render(fmt.Sprintf("(%d tables)", len(d.Tables))) + "\n"
		for _, t := range d.Tables {
			out += "   " + t.Name + "\n"
		}
	}
	if out == "" {
		return styleDim.Render("(no databases)")
	}
	return out
}

// renderResults moved to results.go as (*resultsView).view(w) in U2; the grid
// owns its own rendering so sort/filter/column-cursor stay in one file.

var _ tea.Model = (*model)(nil)
