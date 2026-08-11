package tui

import (
	"fmt"
	"strings"

	"github.com/claymor333/squal/internal/db"
)

const browseLimit = 50000

type browseRequestMsg struct {
	Database string
	Table    string
	PK       []string
}

// treeKind identifies which level of the tree a cursor line points at.
type treeKind int

const (
	treeDB treeKind = iota
	treeTable
	treeCol
)

// treeEntry resolves a tree line to its db/table/column indices.
type treeEntry struct {
	kind  treeKind
	db    int
	table int
	col   int
}

// schemaPane is the DB/table/column tree. The cursor is an absolute line index
// into the *visible* tree — filtering, collapse, and column expansion all
// change what lines exist, so navigation and selection share one coordinate
// space that rebuilds from the current pane state.
type schemaPane struct {
	dbs        []db.Database
	expanded   map[int]bool    // db index -> tables visible
	expCols    map[[2]int]bool // (db, table) -> columns visible
	cursor     int
	lines      int  // viewport height; 0 = unbounded (tests)
	top        int  // first visible line
	collapsed  bool // only db headers shown (left-rail collapse, U13)
	filterMode bool
	filter     string
}

func newSchemaPane(dbs []db.Database) *schemaPane {
	return &schemaPane{dbs: dbs, expanded: map[int]bool{}, expCols: map[[2]int]bool{}}
}

// SetCollapsed shows only db headers, for the rail-collapse toggle (U13).
func (p *schemaPane) SetCollapsed(b bool) {
	p.collapsed = b
	p.clampCursor()
}

// toggleDB expands or collapses a database's table list.
func (p *schemaPane) toggleDB(i int) {
	if i < 0 || i >= len(p.dbs) {
		return
	}
	p.expanded[i] = !p.expanded[i]
	p.clampCursor()
}

// toggleTable expands or collapses the columns of the table under the cursor.
func (p *schemaPane) toggleTable() {
	e, ok := p.entryAt(p.cursor)
	if !ok || e.kind == treeDB {
		return
	}
	key := [2]int{e.db, e.table}
	p.expCols[key] = !p.expCols[key]
	p.clampCursor()
}

// SetLines bounds the viewport; view() renders only lines [top, top+lines).
func (p *schemaPane) SetLines(n int) {
	p.lines = n
	p.clampTop()
}

// --- filter ----------------------------------------------------------------

func (p *schemaPane) startFilter() {
	p.filterMode = true
	p.filter = ""
	p.clampCursor()
}

func (p *schemaPane) appendFilter(r rune) {
	if !p.filterMode {
		return
	}
	p.filter += string(r)
	p.clampCursor()
}

func (p *schemaPane) popFilter() {
	if !p.filterMode {
		return
	}
	if n := len(p.filter); n > 0 {
		p.filter = p.filter[:n-1]
	}
	p.clampCursor()
}

func (p *schemaPane) endFilter() {
	p.filterMode = false
}

// visibleDB reports whether a db has any line under the active filter.
func (p *schemaPane) visibleDB(i int) bool {
	if p.filter == "" {
		return true
	}
	if strings.Contains(strings.ToLower(p.dbs[i].Name), p.filter) {
		return true
	}
	for _, t := range p.dbs[i].Tables {
		if strings.Contains(strings.ToLower(t.Name), p.filter) {
			return true
		}
	}
	return false
}

func (p *schemaPane) visibleTable(name string) bool {
	if p.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), p.filter)
}

// --- tree navigation (absolute cursor over the visible tree) ----------------

// treeLen returns the number of visible lines.
func (p *schemaPane) treeLen() int {
	n := 0
	for i, d := range p.dbs {
		if !p.collapsed && !p.visibleDB(i) {
			continue
		}
		n++ // db header
		if p.collapsed || !p.expanded[i] {
			continue
		}
		for j, t := range d.Tables {
			if !p.visibleTable(t.Name) {
				continue
			}
			n++ // table
			if p.expCols[[2]int{i, j}] {
				n += len(t.Columns)
			}
		}
	}
	return n
}

// entryAt resolves a visible-tree line to its db/table/column indices.
func (p *schemaPane) entryAt(line int) (treeEntry, bool) {
	idx := 0
	for i, d := range p.dbs {
		if !p.collapsed && !p.visibleDB(i) {
			continue
		}
		if idx == line {
			return treeEntry{kind: treeDB, db: i}, true
		}
		idx++
		if p.collapsed || !p.expanded[i] {
			continue
		}
		for j, t := range d.Tables {
			if !p.visibleTable(t.Name) {
				continue
			}
			if idx == line {
				return treeEntry{kind: treeTable, db: i, table: j}, true
			}
			idx++
			if p.expCols[[2]int{i, j}] {
				for k := range t.Columns {
					if idx == line {
						return treeEntry{kind: treeCol, db: i, table: j, col: k}, true
					}
					idx++
				}
			}
		}
	}
	return treeEntry{}, false
}

func (p *schemaPane) clampCursor() {
	if max := p.treeLen() - 1; p.cursor > max {
		p.cursor = max
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	p.clampTop()
}

// clampTop keeps the cursor inside the viewport window when it is bounded.
func (p *schemaPane) clampTop() {
	if p.lines <= 0 {
		p.top = 0
		return
	}
	if p.cursor < p.top {
		p.top = p.cursor
	}
	if p.cursor >= p.top+p.lines {
		p.top = p.cursor - p.lines + 1
	}
	if max := p.treeLen() - p.lines; p.top > max {
		p.top = max
	}
	if p.top < 0 {
		p.top = 0
	}
}

func (p *schemaPane) moveDown() bool {
	if p.cursor >= p.treeLen()-1 {
		return false
	}
	p.cursor++
	p.clampTop()
	return true
}

func (p *schemaPane) moveUp() bool {
	if p.cursor <= 0 {
		return false
	}
	p.cursor--
	p.clampTop()
	return true
}

// selectCurrent returns a browseRequestMsg when the cursor is on a table or
// column, or toggles the database when it is on a header. Returns nil on a db
// header when collapsed.
func (p *schemaPane) selectCurrent() any {
	if p.treeLen() == 0 {
		return nil
	}
	e, ok := p.entryAt(p.cursor)
	if !ok {
		return nil
	}
	switch e.kind {
	case treeDB:
		p.toggleDB(e.db)
		return nil
	case treeTable, treeCol:
		d := p.dbs[e.db]
		t := d.Tables[e.table]
		var pk []string
		for _, c := range t.Columns {
			if c.Key == "PRI" {
				pk = append(pk, c.Name)
			}
		}
		return browseRequestMsg{Database: d.Name, Table: t.Name, PK: pk}
	}
	return nil
}

// currentDatabase returns the database the cursor is on.
func (p *schemaPane) currentDatabase() string {
	if p.treeLen() == 0 {
		return ""
	}
	e, _ := p.entryAt(p.cursor)
	return p.dbs[e.db].Name
}

func browseQuery(dbName, table string, pk []string) string {
	q := fmt.Sprintf("SELECT * FROM %s.%s", db.QuoteIdentifier(dbName), db.QuoteIdentifier(table))
	if len(pk) > 0 {
		q += " ORDER BY " + quotedList(pk)
	}
	return fmt.Sprintf("%s LIMIT %d", q, browseLimit)
}

func quotedList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = db.QuoteIdentifier(n)
	}
	return strings.Join(q, ", ")
}

// view renders the visible tree window [top, top+lines), marking the cursor
// line, with a filter box line when filtering.
func (p *schemaPane) view() string {
	var b strings.Builder
	if p.filterMode || p.filter != "" {
		fmt.Fprintf(&b, "[filter: %s▌]\n", p.filter)
	}
	end := p.treeLen()
	if p.lines > 0 && p.top+p.lines < end {
		end = p.top + p.lines
	}
	for line := p.top; line < end; line++ {
		e, ok := p.entryAt(line)
		if !ok {
			break
		}
		switch e.kind {
		case treeDB:
			head := "▸ "
			if line == p.cursor {
				head = "◉ "
			}
			fmt.Fprintf(&b, "%s%s (%d)\n", head, p.dbs[e.db].Name, len(p.dbs[e.db].Tables))
		case treeTable:
			head := "  "
			if line == p.cursor {
				head = "◉ "
			}
			fmt.Fprintf(&b, "%s%s\n", head, p.dbs[e.db].Tables[e.table].Name)
		case treeCol:
			head := "    "
			if line == p.cursor {
				head = "  ◉ "
			}
			fmt.Fprintf(&b, "%s%s\n", head, p.dbs[e.db].Tables[e.table].Columns[e.col].Name)
		}
	}
	return b.String()
}
