package specimen

import "testing"

func TestAdd(t *testing.T) {
	for _, c := range [][3]int{{2, 3, 5}, {-2, 3, 1}, {0, 0, 0}} {
		if got := Add(c[0], c[1]); got != c[2] {
			t.Errorf("Add(%d,%d)=%d, want %d", c[0], c[1], got, c[2])
		}
	}
}
func TestMultiply(t *testing.T) {
	for _, c := range [][3]int{{2, 3, 6}, {-2, 3, -6}, {0, 4, 0}} {
		if got := Multiply(c[0], c[1]); got != c[2] {
			t.Errorf("Multiply(%d,%d)=%d, want %d", c[0], c[1], got, c[2])
		}
	}
}
func TestReverse(t *testing.T) {
	for _, c := range [][2]string{{"abc", "cba"}, {"a界é", "é界a"}, {"", ""}} {
		if got := Reverse(c[0]); got != c[1] {
			t.Errorf("Reverse(%q)=%q, want %q", c[0], got, c[1])
		}
	}
}
func TestClamp(t *testing.T) {
	for _, c := range [][4]int{{-3, 0, 10, 0}, {12, 0, 10, 10}, {5, 0, 10, 5}, {0, 0, 10, 0}, {10, 0, 10, 10}} {
		if got := Clamp(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("Clamp(%d,%d,%d)=%d, want %d", c[0], c[1], c[2], got, c[3])
		}
	}
}
