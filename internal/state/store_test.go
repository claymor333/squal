package state

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "actions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	act := &Action{
		Verdict:    Undoable,
		Kind:       "update",
		Connection: "dev",
		Database:   "app",
		Table:      "users",
		PK:         map[string]string{"id": "7"},
		Before:     map[string]string{"id": "7", "name": "old"},
		After:      map[string]string{"id": "7", "name": "new"},
		SQL:        "UPDATE users SET name='new' WHERE id=7",
	}
	if err := s.Record(act); err != nil {
		t.Fatal(err)
	}
	acts, err := s.List(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].ID != act.ID {
		t.Fatalf("List = %d items, want 1 matching id", len(acts))
	}
	// flip to undone
	if err := s.SetStatus(act.ID, Undone); err != nil {
		t.Fatal(err)
	}
	a, err := s.Find(act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != Undone {
		t.Fatalf("status = %v, want undone", a.Status)
	}
}
