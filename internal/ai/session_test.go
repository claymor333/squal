package ai

import "testing"

func TestSessionBuildsHistory(t *testing.T) {
	s := NewSession(nil)
	s.AddTurn("user", "show users")
	s.AddTurn("assistant", "SELECT * FROM users")
	s.AddTurn("user", "only active")
	if len(s.Messages()) != 3 {
		t.Fatalf("messages = %d", len(s.Messages()))
	}
	s.Reset()
	if len(s.Messages()) != 0 {
		t.Fatalf("after reset = %d", len(s.Messages()))
	}
}
