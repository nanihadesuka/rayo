package flow

import (
	"strings"
	"testing"
)

func TestCapScreen(t *testing.T) {
	if got := CapScreen("a\nb\n", 2); got != "a\nb\n" {
		t.Errorf("small: %q", got)
	}
	if got := CapScreen("", 2); got != "" {
		t.Errorf("empty: %q", got)
	}
	got := CapScreen(strings.Repeat("x\n", 5), 2)
	if want := "x\nx\n... 3 more elements: --find, or a selector with an id, narrows it\n"; got != want {
		t.Errorf("capped: %q", got)
	}
}
