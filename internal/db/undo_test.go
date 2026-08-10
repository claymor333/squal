package db

import "testing"

func TestClassifyUpdateFeasible(t *testing.T) {
	v, err := Classify("UPDATE users SET name='x' WHERE id=7")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Feasible {
		t.Fatalf("want feasible, got: %s", v.Reason)
	}
	if v.Table != "users" {
		t.Fatalf("table = %q", v.Table)
	}
}

func TestClassifyJoinNotFeasible(t *testing.T) {
	v, err := Classify("UPDATE users u JOIN orders o ON o.user_id=u.id SET u.x=1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Feasible {
		t.Fatalf("multi-table update must not be feasible")
	}
	if v.Table != "" {
		t.Fatalf("table should be empty")
	}
}

func TestClassifyDDLNotFeasible(t *testing.T) {
	v, err := Classify("DROP TABLE users")
	if err != nil {
		t.Fatal(err)
	}
	if v.Feasible {
		t.Fatal("DDL must not be feasible")
	}
}

func TestClassifyDeleteFeasible(t *testing.T) {
	v, err := Classify("DELETE FROM orders WHERE id < 100")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Feasible || v.Kind != "delete" {
		t.Fatalf("want feasible delete, got feasible=%v kind=%q", v.Feasible, v.Kind)
	}
}
