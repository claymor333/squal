package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// rowPanel is the right-side detail editor for a highlighted row (SPEC §5).
// Each field is shown as name: value with a cursor; enter starts inline editing
// of the current field, r toggles the raw-JSON textarea.
type rowPanel struct {
	names   []string
	vals    map[string]string
	cur     int
	raw     bool
	rawBuf  string
	editing bool
	editBuf []rune
}

func newRowPanel() *rowPanel {
	return &rowPanel{vals: map[string]string{}}
}

func (p *rowPanel) SetRow(row map[string]string) {
	p.names = p.names[:0]
	for n := range row {
		p.names = append(p.names, n)
	}
	sort.Strings(p.names)
	p.vals = row
	p.cur = 0
	p.raw = false
	p.editing = false
	p.editBuf = p.editBuf[:0]
}

func (p *rowPanel) moveDown() {
	if p.cur < len(p.names)-1 {
		p.cur++
	}
}

func (p *rowPanel) moveUp() {
	if p.cur > 0 {
		p.cur--
	}
}

func (p *rowPanel) current() (name, val string) {
	if p.cur >= len(p.names) {
		return "", ""
	}
	return p.names[p.cur], p.vals[p.names[p.cur]]
}

func (p *rowPanel) editValue(v string) {
	if p.cur < len(p.names) {
		p.vals[p.names[p.cur]] = v
	}
}

// startEdit begins inline editing of the current field, seeded with its value.
func (p *rowPanel) startEdit() {
	if p.cur >= len(p.names) {
		return
	}
	p.editBuf = []rune(p.vals[p.names[p.cur]])
	p.editing = true
}

func (p *rowPanel) appendEdit(r rune) {
	p.editBuf = append(p.editBuf, r)
}

func (p *rowPanel) backspaceEdit() {
	if len(p.editBuf) > 0 {
		p.editBuf = p.editBuf[:len(p.editBuf)-1]
	}
}

// commitEdit writes the edited buffer back to the current field.
func (p *rowPanel) commitEdit() {
	p.editValue(string(p.editBuf))
	p.editing = false
}

func (p *rowPanel) cancelEdit() {
	p.editing = false
	p.editBuf = p.editBuf[:0]
}

func (p *rowPanel) toggleRaw() {
	if !p.raw {
		// enter raw mode with the current JSON
		b, _ := json.Marshal(p.vals)
		p.rawBuf = string(b)
		p.raw = true
		return
	}
	// parse and leave raw mode
	var m map[string]string
	if err := json.Unmarshal([]byte(p.rawBuf), &m); err != nil {
		// keep rawBuf so the user can fix; flag via no-op
		return
	}
	p.vals = m
	p.SetRow(m)
}

func (p *rowPanel) SetRawJSON(s string) {
	p.rawBuf = s
}

func (p *rowPanel) Values() (map[string]string, error) {
	if p.raw {
		var m map[string]string
		if err := json.Unmarshal([]byte(p.rawBuf), &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return p.vals, nil
}

// view renders the current edit mode: raw JSON, an inline field edit, or the
// field list.
func (p *rowPanel) view() string {
	switch {
	case p.raw:
		return p.rawBuf
	case p.editing:
		name, _ := p.current()
		mark := "▸"
		return fmt.Sprintf("%s %s: %s█\n%s", mark, name, string(p.editBuf),
			styleDim.Render("enter commit · esc cancel"))
	default:
		var b strings.Builder
		for i, n := range p.names {
			mark := " "
			if i == p.cur {
				mark = "▸"
			}
			fmt.Fprintf(&b, "%s %s: %s\n", mark, n, p.vals[n])
		}
		b.WriteString(styleDim.Render("enter edit · r raw · s save · esc close"))
		return b.String()
	}
}
