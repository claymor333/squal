package tui

import (
	"testing"

	"github.com/claymor333/squal/internal/config"
)

func TestModelTabSwitch(t *testing.T) {
	m := newModelForTest(3) // 3 tabs
	if m.active != 0 {
		t.Fatalf("active = %d", m.active)
	}
	m.nextTab()
	if m.active != 1 {
		t.Fatalf("after nextTab active = %d", m.active)
	}
	m.prevTab()
	if m.active != 0 {
		t.Fatalf("after prevTab active = %d", m.active)
	}
}

func TestModelBrowseRequestBuildsRun(t *testing.T) {
	m := newModelForTest(1)
	req := browseRequestMsg{Database: "app", Table: "users", PK: []string{"id"}}
	msg := m.onBrowse(req)
	rm, ok := msg.(runQueryMsg)
	if !ok {
		t.Fatalf("onBrowse = %T, want runQueryMsg", msg)
	}
	if rm.SQL == "" {
		t.Fatal("empty query")
	}
}

func newModelForTest(count int) *model {
	m := &model{focus: focusSchema}
	for i := 0; i < count; i++ {
		m.conns = append(m.conns, &connData{profile: config.Profile{Name: "t"}})
	}
	return m
}
