package live

import (
	"slices"
	"testing"
)

func TestChanges(t *testing.T) {
	for _, c := range []struct {
		name        string
		before, now []string
		want        []string
	}{
		{"first screen", nil, []string{"a", "b"}, []string{"+ a", "+ b"}},
		{"the same", []string{"a", "b"}, []string{"a", "b"}, []string{}},
		{"one appeared", []string{"a"}, []string{"a", "b"}, []string{"+ b"}},
		{"one went", []string{"a", "b"}, []string{"b"}, []string{"- a"}},
		{"one changed", []string{"title", "price 5"}, []string{"title", "price 6"}, []string{"+ price 6", "- price 5"}},
		{"moved is the same", []string{"a", "b"}, []string{"b", "a"}, []string{}},
		{"twice counts twice", []string{"row"}, []string{"row", "row"}, []string{"+ row"}},
		{"all went", []string{"a"}, []string{}, []string{"- a"}},
	} {
		if got := changes(c.before, c.now); !slices.Equal(got, c.want) || got == nil {
			t.Errorf("%s: changes(%q, %q) = %q, want %q", c.name, c.before, c.now, got, c.want)
		}
	}
}
