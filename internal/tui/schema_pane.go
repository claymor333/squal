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

// schemaPane is the DB/table tree. The cursor is an absolute line index into the
// rendered tree (one line per db header, one per expanded table), so selection
// and viewport scrolling share a single coordinate space.
type schemaPane struct {
	dbs      []db.Database
	expanded map[int]bool
	cursor   int
	lines    int // viewport height; 0 = unbounded (tests)
	top      int // first visible line
}

func newSchemaPane(dbs []db.Database) *schemaPane {
	return &schemaPane{dbs: dbs, expanded: map[int]bool{}}
}

// toggleDB expands or collapses a database's table list.
func (p *schemaPane) toggleDB(i int) {
	if i < 0 || i >= len(p.dbs) {
		return
	}
	p.expanded[i] = !p.expanded[i]
	p.clampCursor()
}

// SetLines bounds the viewport; view() renders only lines [top, top+lines).
func (p *schemaPane) SetLines(n int) {
	p.lines = n
	p.clampTop()
}

// treeLen returns the total rendered line count.
func (p *schemaPane) treeLen() int {
	n := len(p.dbs)
	for i, d := range p.dbs {
		if p.expanded[i] {
			n += len(d.Tables)
		}
	}
	return n
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

// current identifies the tree entry under the cursor.
func (p *schemaPane) current() (dbIdx, tableIdx int, onTable bool) {
	line := 0
	for i, d := range p.dbs {
		if line == p.cursor {
			return i, 0, false
		}
		line++
		if !p.expanded[i] {
			continue
		}
		for j := range d.Tables {
			if line == p.cursor {
				return i, j, true
			}
			line++
		}
	}
	return 0, 0, false
}

// selectCurrent returns a browseRequestMsg when the cursor is on a table, or
// toggles the database when it is on a header. Returns nil for an empty tree.
func (p *schemaPane) selectCurrent() any {
	if p.treeLen() == 0 {
		return nil
	}
	i, j, onTable := p.current()
	if !onTable {
		p.toggleDB(i)
		return nil
	}
	d := p.dbs[i]
	t := d.Tables[j]
	var pk []string
	for _, c := range t.Columns {
		if c.Key == "PRI" {
			pk = append(pk, c.Name)
		}
	}
	return browseRequestMsg{Database: d.Name, Table: t.Name, PK: pk}
}

// currentDatabase returns the database the cursor is on.
func (p *schemaPane) currentDatabase() string {
	if p.treeLen() == 0 {
		return ""
	}
	i, _, _ := p.current()
	return p.dbs[i].Name
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

// view renders the tree window [top, top+lines), marking the cursor line.
func (p *schemaPane) view() string {
	var b strings.Builder
	line := 0
	end := p.treeLen()
	if p.lines > 0 && p.top+p.lines < end {
		end = p.top + p.lines
	}
	for i, d := range p.dbs {
		if line >= end {
			break
		}
		if line >= p.top {
			mark := "▸ "
			if line == p.cursor {
				mark = "◉ "
			}
			fmt.Fprintf(&b, "%s%s (%d)\n", mark, d.Name, len(d.Tables))
		}
		line++
		if !p.expanded[i] {
			continue
		}
		for _, t := range d.Tables {
			if line >= end {
				break
			}
			if line >= p.top {
				mark := "  "
				if line == p.cursor {
					mark = "◉ "
				}
				fmt.Fprintf(&b, "%s%s\n", mark, t.Name)
			}
			line++
		}
	}
	return b.String()
}
