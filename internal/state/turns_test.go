package state

import (
	"path/filepath"
	"testing"
)

func TestTurnsPersist(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.AddTurn("conn1", "get_schema", `{"table":"users"}`, "8 cols"); err != nil {
		t.Fatal(err)
	}
	turns, err := s.ListTurns("conn1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 || turns[0].Tool != "get_schema" {
		t.Fatalf("turns = %+v", turns)
	}
}
