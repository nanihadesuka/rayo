package flow

import (
	"errors"
	"slices"
	"testing"
)

func TestNearest(t *testing.T) {
	screen := `[com.shop]
  id: com.shop:id/pay_button | text: Pay now  [0,0][100,50]
  text: Payment methods  [0,60][100,90]
  text: Cancel  [0,100][100,150]
  desc: Pey  [0,200][100,250]
`
	got := Nearest(screen, "Pay", 5)
	want := []string{"id: com.shop:id/pay_button | text: Pay now  [0,0][100,50]", "desc: Pey  [0,200][100,250]", "text: Payment methods  [0,60][100,90]"}
	if !slices.Equal(got, want) {
		t.Errorf("Nearest:\n got %q\nwant %q", got, want)
	}
	if got := Nearest(screen, "Zzzzzz", 5); len(got) != 0 {
		t.Errorf("nothing alike, got %q", got)
	}
}

func TestWanted(t *testing.T) {
	if got := wanted(errors.New("not on screen: Pay now")); got != "Pay now" {
		t.Errorf("wanted: %q", got)
	}
}

func TestWindow(t *testing.T) {
	log := "11-22 10:00:00.000  5  5 I Other: old\n" +
		"11-22 10:00:50.000  7  7 I ActivityManager: Start proc 9:com.shop/u0a1\n" +
		"11-22 10:00:55.000  9  9 D Shop: mine\n" +
		"11-22 10:00:56.000  8  8 D Other: not mine\n" +
		"11-22 10:00:58.000  1  1 E AndroidRuntime: FATAL EXCEPTION: main\n" +
		"11-22 10:01:00.000  9  9 D Shop: last\n"
	got := window(log, "com.shop", 20e9)
	want := "11-22 10:00:50.000  7  7 I ActivityManager: Start proc 9:com.shop/u0a1\n" +
		"11-22 10:00:55.000  9  9 D Shop: mine\n" +
		"11-22 10:00:58.000  1  1 E AndroidRuntime: FATAL EXCEPTION: main\n" +
		"11-22 10:01:00.000  9  9 D Shop: last\n"
	if got != want {
		t.Errorf("window:\n%s\nwant\n%s", got, want)
	}
}

// What the agent says of a selector is looked for by its text, not as JSON.
func TestWantedSelector(t *testing.T) {
	if got := wanted(errors.New(`not on screen: {"text":"Pay now"}`)); got != "Pay now" {
		t.Errorf("%q", got)
	}
	if got := wanted(errors.New("not on screen: missing")); got != "missing" {
		t.Errorf("%q", got)
	}
}
