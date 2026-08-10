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

type schemaPane struct {
	dbs        []db.Database
	expanded   map[int]bool
	dbIndex    int
	tableIndex int
	onTable    bool // cursor on a table row (vs the db header)
}

func newSchemaPane(dbs []db.Database) *schemaPane {
	return &schemaPane{dbs: dbs, expanded: map[int]bool{}}
}

func (p *schemaPane) toggleDB(i int) {
	p.expanded[i] = !p.expanded[i]
}

// moveDown moves the cursor one logical row down; returns false when at bottom.
func (p *schemaPane) moveDown() bool {
	if p.dbIndex >= len(p.dbs) {
		return false
	}
	if p.onTable {
		if p.tableIndex < len(p.dbs[p.dbIndex].Tables)-1 {
			p.tableIndex++
			return true
		}
		p.onTable = false
		if p.dbIndex < len(p.dbs)-1 {
			p.dbIndex++
		}
		return true
	}
	if p.expanded[p.dbIndex] && len(p.dbs[p.dbIndex].Tables) > 0 {
		p.onTable = true
		p.tableIndex = 0
		return true
	}
	if p.dbIndex < len(p.dbs)-1 {
		p.dbIndex++
		return true
	}
	return false
}

func (p *schemaPane) moveUp() bool {
	if p.dbIndex < 0 {
		return false
	}
	if p.onTable {
		if p.tableIndex > 0 {
			p.tableIndex--
			return true
		}
		p.onTable = false
		return true
	}
	if p.dbIndex > 0 {
		p.dbIndex--
		if p.expanded[p.dbIndex] && len(p.dbs[p.dbIndex].Tables) > 0 {
			p.onTable = true
			p.tableIndex = len(p.dbs[p.dbIndex].Tables) - 1
		}
		return true
	}
	return false
}

func (p *schemaPane) selectCurrent() any {
	if p.onTable {
		d := p.dbs[p.dbIndex]
		t := d.Tables[p.tableIndex]
		var pk []string
		for _, c := range t.Columns {
			if c.Key == "PRI" {
				pk = append(pk, c.Name)
			}
		}
		return browseRequestMsg{Database: d.Name, Table: t.Name, PK: pk}
	}
	if p.dbIndex < len(p.dbs) {
		p.toggleDB(p.dbIndex)
	}
	return nil
}

// currentDatabase returns the database the cursor is on (or the last selected).
func (p *schemaPane) currentDatabase() string {
	if p.dbIndex < len(p.dbs) {
		return p.dbs[p.dbIndex].Name
	}
	return ""
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

func (p *schemaPane) view() string {
	var b strings.Builder
	for i, d := range p.dbs {
		head := "▸"
		if p.dbIndex == i && !p.onTable {
			head = "◉"
		}
		fmt.Fprintf(&b, "%s %s (%d)\n", head, d.Name, len(d.Tables))
		if !p.expanded[i] {
			continue
		}
		for j, t := range d.Tables {
			mark := "  "
			if p.onTable && p.dbIndex == i && p.tableIndex == j {
				mark = "◉ "
			}
			fmt.Fprintf(&b, "%s%s\n", mark, t.Name)
		}
	}
	return b.String()
}
