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
	browse      *browseRequestMsg // table browsed by the current results, for load-next
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
	cfg      *config.Config
	conns    []*connData
	active   int
	focus    paneFocus
	connect  *connectView
	ai       *aiPanel
	store    *state.Store
	width    int
	height   int
	lastErr  error
	quitting bool
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

// focusAI is the streaming AI panel focus; it follows results in the Tab cycle.
const focusAI paneFocus = focusResults + 1

// nextFocus advances the key focus schema → editor → results → ai.
func nextFocus(f paneFocus) paneFocus {
	if f < focusAI {
		return f + 1
	}
	return focusSchema
}

func prevFocus(f paneFocus) paneFocus {
	if f > focusSchema {
		return f - 1
	}
	return focusAI
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
			m.lastErr = msg.err
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
				m.lastErr = msg.Err
			} else {
				cur.lastElapsed = msg.Elapsed
			}
		} else if msg.Err != nil {
			m.lastErr = msg.Err
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
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.connect != nil {
		return m.handleConnectKey(msg)
	}
	switch msg.String() {
	case "ctrl+c":
		if m.active >= 0 && m.active < len(m.conns) {
			return m, closeConnection(m.active, m.conns[m.active].conn)
		}
		return m, tea.Quit
	case "tab":
		m.focus = nextFocus(m.focus)
		return m, nil
	case "shift+tab":
		m.focus = prevFocus(m.focus)
		return m, nil
	case "q":
		if m.focus == focusEditor || m.focus == focusAI || m.active < 0 || m.active >= len(m.conns) {
			break // 'q' is text input in the editor/AI panel; ignore it elsewhere
		}
		return m, closeConnection(m.active, m.conns[m.active].conn)
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
			}
		case "down":
			if cur.pane != nil {
				cur.pane.moveDown()
			}
		case "enter":
			if cur.pane != nil && cur.conn != nil {
				if req, ok := cur.pane.selectCurrent().(browseRequestMsg); ok {
					if rq, ok := m.onBrowse(req).(runQueryMsg); ok {
						cur.browse = &req
						return m, m.startFetch(context.Background(), rq.SQL)
					}
				}
			}
		}
	case focusEditor:
		switch msg.String() {
		case "up":
			if cur.ed != nil {
				cur.ed.historyUp()
			}
		case "backspace":
			if cur.ed != nil {
				cur.ed.backspace()
			}
		case "ctrl+enter":
			if cur.ed != nil && cur.conn != nil {
				if rq, ok := cur.ed.run().(runQueryMsg); ok {
					cur.browse = nil
					return m, m.startFetch(context.Background(), rq.SQL)
				}
			}
		default:
			runes := []rune(msg.String())
			if len(runes) == 1 && cur.ed != nil {
				cur.ed.insert(runes[0])
			}
		}
	case focusResults:
		switch msg.String() {
		case "up":
			scrollResults(cur.results, -1)
		case "down":
			scrollResults(cur.results, 1)
		case "s":
			toggleSort(cur.results)
		case "f":
			// filter prompt deferred to a later phase; sort is the wired behavior
		case "n":
			return m, m.loadNext(cur)
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
	case "esc", "ctrl+c":
		m.connect = nil
		return m, nil
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
			m.lastErr = err
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

// --- query pipeline ----------------------------------------------------------

// startFetch launches db.Fetch on a goroutine and returns the cmd that pumps
// batches into the tea event loop as batchMsg values.
func (m *model) startFetch(ctx context.Context, sql string) tea.Cmd {
	cur := m.conns[m.active]
	col, ch, err := cur.conn.Fetch(ctx, sql, 1000)
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

const resultsViewport = 8

// scrollResults moves the selection by one logical row, sliding the window
// when the selection would leave the viewport.
func scrollResults(r *resultsView, dir int) {
	if r == nil || len(r.order) == 0 {
		return
	}
	if dir > 0 {
		if r.top+r.selRow < len(r.order)-1 {
			if r.selRow < resultsViewport-1 {
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

// toggleSort flips the sort on the current column, defaulting to column 0.
func toggleSort(r *resultsView) {
	if r == nil || r.data == nil {
		return
	}
	if r.sortCol < 0 {
		r.sortCol = 0
		r.sortAsc = true
	} else {
		r.sortAsc = !r.sortAsc
	}
	r.rebuildOrder()
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

	var tabs string
	for i, c := range m.conns {
		label := c.profile.Name
		if i == m.active {
			tabs += styleTitle.Render(fmt.Sprintf(" %s ", label)) + " "
		} else {
			tabs += styleDim.Render(" "+label+" ") + " "
		}
	}

	cur := m.conns[m.active]
	var body string
	switch {
	case cur.loading:
		body = styleDim.Render("connecting to " + cur.profile.Name + "…")
	case cur.loadErr != nil:
		body = styleErr.Render("connection failed: " + cur.loadErr.Error())
	case cur.pane != nil:
		body = cur.pane.view()
	default:
		body = renderSchemaTree(cur.dbs)
	}

	editorView := styleDim.Render("(empty)")
	if cur.ed != nil {
		editorView = cur.ed.view()
	}

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
	var rerr error
	if cur.results != nil {
		rerr = cur.results.err
	}

	var errline string
	if m.lastErr != nil {
		errline = styleErr.Render("✗ "+m.lastErr.Error()) + "\n"
	}

	base := stylePane.Render(
		tabs + "\n" +
			sectionHeader("schema", m.focus == focusSchema) + "\n" +
			body + "\n" +
			sectionHeader("editor", m.focus == focusEditor) + "\n" +
			editorView + "\n" +
			sectionHeader("results", m.focus == focusResults) + "\n" +
			renderResults(cur.results) + "\n" +
			sectionHeader("ai", m.focus == focusAI) + "\n" +
			renderAI(m.ai) + "\n" +
			errline +
			statusView(cur.profile.Name, cur.results, elapsed, rerr, m.focus) + "\n",
	)
	if m.connect != nil {
		return renderConnectModal(m.connect) + "\n" + base
	}
	return base
}

func sectionHeader(label string, focused bool) string {
	if focused {
		return styleAccent.Render("▸ " + label)
	}
	return styleDim.Render("  " + label)
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

// renderResults renders a compact grid over the loaded rows.
func renderResults(r *resultsView) string {
	if r == nil {
		return styleDim.Render("(no query)")
	}
	if r.err != nil {
		return styleErr.Render("✗ " + r.err.Error())
	}
	if r.data == nil || len(r.data.Columns) == 0 {
		return styleDim.Render("(no columns)")
	}
	if len(r.order) == 0 {
		return styleDim.Render("(no rows)")
	}
	var b strings.Builder
	for c, col := range r.data.Columns {
		if c > 0 {
			b.WriteString(" │ ")
		}
		head := col
		if r.sortCol == c {
			if r.sortAsc {
				head += " ▲"
			} else {
				head += " ▼"
			}
		}
		b.WriteString(styleAccent.Render(head))
	}
	b.WriteString("\n")
	end := r.top + resultsViewport
	if end > len(r.order) {
		end = len(r.order)
	}
	for row := r.top; row < end; row++ {
		mark := "  "
		if row == r.top+r.selRow {
			mark = "◉ "
		}
		b.WriteString(mark)
		for c := range r.data.Columns {
			if c > 0 {
				b.WriteString(" │ ")
			}
			b.WriteString(truncate(r.data.Value(c, r.order[row]), 24))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var _ tea.Model = (*model)(nil)
