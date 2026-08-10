package mutate

import "testing"

func TestKeysetCondition(t *testing.T) {
	got := keysetCondition([]string{"id"}, []string{"42"})
	want := "(`id` > '42')"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestKeysetConditionComposite(t *testing.T) {
	got := keysetCondition([]string{"a", "b"}, []string{"1", "x"})
	want := "((`a` > '1') OR (`a` = '1' AND `b` > 'x'))"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
