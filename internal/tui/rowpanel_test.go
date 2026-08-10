package tui

import "testing"

func TestRowPanelApplyField(t *testing.T) {
	p := newRowPanel()
	p.SetRow(map[string]string{"id": "1", "name": "alice"})
	if p.cur != 0 {
		t.Fatalf("cursor = %d", p.cur)
	}
	p.moveDown()
	if p.cur != 1 {
		t.Fatalf("cursor after moveDown = %d", p.cur)
	}
	// focus the name field and replace it
	p.editValue("bob")
	got, _ := p.Values()
	if got["name"] != "bob" {
		t.Fatalf("name = %q, want bob", got["name"])
	}
}

func TestRowPanelRawJSONToggle(t *testing.T) {
	p := newRowPanel()
	p.SetRow(map[string]string{"id": "1", "name": "alice"})
	p.toggleRaw()
	if !p.raw {
		t.Fatal("raw not toggled on")
	}
	p.SetRawJSON(`{"id":"1","name":"carol"}`)
	p.toggleRaw()
	got, err := p.Values()
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "carol" {
		t.Fatalf("name = %q, want carol", got["name"])
	}
}
