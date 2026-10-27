package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
}

func TestAddZero(t *testing.T) {
	if got := Add(0, 0); got != 0 {
		t.Errorf("Add(0, 0) = %d, want 0", got)
	}
}
