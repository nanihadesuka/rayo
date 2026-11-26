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

func TestNearestEdges(t *testing.T) {
	for _, c := range []struct {
		written string
		names   []string
		want    string
	}{
		{"", nil, ""},
		{"x", nil, ""},
		{"", []string{"a", "b"}, "a"}, // a name of one letter is one change away
		{"tapOm", []string{"tapOn", "tapOff"}, "tapOn"},                     // the closest wins
		{"ab", []string{"ac", "ad"}, "ac"},                                  // a tie: the first listed
		{"failureWindow", []string{"--failure-window"}, "--failure-window"}, // a dash is one change
		{"--failurewindow", []string{"--failure-window"}, "--failure-window"},
		{"abcdefghij", []string{"abcdefxxxx"}, ""},                // too many changes for its length
		{"retry", []string{"retries", "results"}, "retries"},      // a word of the same meaning
		{"VISIBLE", []string{"visible", "notVisible"}, "visible"}, // case
	} {
		if got := Nearest(c.written, c.names); got != c.want {
			t.Errorf("Nearest(%q, %q) = %q, want %q", c.written, c.names, got, c.want)
		}
	}
}
