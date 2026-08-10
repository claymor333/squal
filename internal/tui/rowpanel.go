package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type rowPanel struct {
	names   []string
	vals    map[string]string
	cur     int
	raw     bool
	rawBuf  string
	editing bool
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

func (p *rowPanel) toggleRaw() {
	if !p.raw {
		// enter raw mode with current JSON
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

func (p *rowPanel) view() string {
	if p.raw {
		return p.rawBuf
	}
	var b strings.Builder
	for i, n := range p.names {
		mark := " "
		if i == p.cur {
			mark = "▸"
		}
		fmt.Fprintf(&b, "%s %q: %q\n", mark, n, p.vals[n])
	}
	return b.String()
}
