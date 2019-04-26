package util

import "testing"

func TestFindRule(t *testing.T) {
	a := []string{"1", "2", "3"}
	b := []string{"1", "2", "4"}
	c := RemoveCommonElements(a, b)
	d := RemoveCommonElements(b, a)
	if c[0] != "3" {
		t.Errorf("Unexpected result: %s (wanted %s)", c[0], "3")
	}
	if d[0] != "4" {
		t.Errorf("Unexpected result: %s (wanted %s)", c[0], "3")
	}
}

func TestInArray(t *testing.T) {
	a := []string{"1", "2", "3"}
	c, i := InArray(a, "2")
	d, y := InArray(a, "4")
	if c == false || i != 1 {
		t.Errorf("Unexpected result: %v (wanted %v)", c, true)
	}
	if d == true || y != -1 {
		t.Errorf("Unexpected result: %v (wanted %v)", d, false)
	}
}
