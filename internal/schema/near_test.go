package schema

import "testing"

func TestDistance(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"test", "test", 0},
		{"tset", "test", 1}, // swapped
		{"tets", "test", 1},
		{"tes", "test", 1},
		{"tests", "test", 1},
		{"best", "test", 1},
		{"kitten", "sitting", 3},
		{"ab", "ba", 1},
		{"abc", "ca", 3}, // a swap is never undone by a later change
		{"", "abc", 3},
	} {
		if got := Distance([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Distance([]rune(c.b), []rune(c.a)); got != c.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", c.b, c.a, got, c.want)
		}
	}
}

func TestNearestCommand(t *testing.T) {
	names := []string{"test", "check", "explain", "live", "do", "hierarchy", "devices", "ca", "mocks", "schema", "help", "version"}
	for written, want := range map[string]string{
		"tset":       "test",
		"chekc":      "check",
		"devcies":    "devices",
		"mock":       "mocks",
		"frobnicate": "",
		"run":        "",
	} {
		if got := Nearest(written, names); got != want {
			t.Errorf("Nearest(%q) = %q, want %q", written, got, want)
		}
	}
}
