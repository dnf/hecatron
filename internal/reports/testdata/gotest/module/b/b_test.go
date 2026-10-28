package b

import "testing"

func TestPanics(t *testing.T) {
	t.Log("before")
	var m map[string]int
	m["x"] = 1
}
