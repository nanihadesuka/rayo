package flow

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Blame is whose fault a failure is.
type Blame string

// Test is all that isn't known to come from elsewhere.
const (
	Test Blame = "test"
	App  Blame = "app"    // it crashed, hung or isn't there
	Dev  Blame = "device" // it was lost, restarted or hung
)

// diagnose says where a failure comes from. It only blames what it has evidence of.
func (r *Runner) diagnose(err error, log string) {
	f := r.failure
	f.Blame = Test
	trouble, unusable := r.health()
	switch {
	case trouble != "":
		f.Blame, f.Because, f.Unusable = Dev, trouble, unusable
	case r.agent.Dead() != nil:
		f.Blame, f.Because = Dev, "the connection to the device was lost"
	case f.Crash != "":
		f.Blame, f.Because = App, signature(f.Crash)
	case r.systemTrouble(log) != "":
		f.Blame, f.Because = Dev, r.systemTrouble(log)
	case strings.Contains(err.Error(), "is not installed"):
		f.Blame, f.Because = App, "the app is not installed"
	case strings.Contains(err.Error(), NotOnScreen) && blank(f.Screen, r.flow.App):
		f.Blame, f.Because, f.blank = App, "the app was in front but showed nothing that can be read", true
	}
}

// blank is whether the app is in front, on top of the screen, and its content is empty: still
// starting, stuck, or not readable by accessibility. Not the test's to fix. Only the content: the
// window around it, such as an action bar with the app's name, is Android's.
func blank(screen, app string) bool {
	lines := strings.Split(screen, "\n")
	if app == "" || lines[0] != "["+app+"]" {
		return false
	}
	for i, line := range lines[1:] {
		if strings.HasPrefix(line, "[") {
			return false // the next window, and no content seen
		}
		if bounds.ReplaceAllString(strings.TrimSpace(line), "") != "id: android:id/content" {
			continue
		}
		next := lines[min(i+2, len(lines)-1)]
		return i+2 >= len(lines) || indent(next) <= indent(line)
	}
	return false
}

func indent(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

// health is what is wrong with the device, and whether nothing can run on it.
func (r *Runner) health() (trouble string, unusable bool) {
	out, err := r.device.ExecWithin("getprop sys.boot_completed; ps -o ETIME= -p $(pidof system_server)", 10*time.Second)
	if err != nil {
		return fmt.Sprintf("the device doesn't answer: %v", err), true
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 || lines[0] != "1" {
		return "the device is starting", true
	}
	if len(lines) < 2 {
		return "Android isn't running", true
	}
	if up, ok := elapsed(lines[1]); ok && up < time.Since(r.began) {
		return "Android restarted during the flow", false
	}
	return "", false
}

// elapsed reads [[days-]hours:]minutes:seconds.
func elapsed(text string) (time.Duration, bool) {
	days, rest, ok := strings.Cut(text, "-")
	if !ok {
		days, rest = "0", text
	}
	d, err := strconv.Atoi(days)
	if err != nil {
		return 0, false
	}
	total := 0
	for part := range strings.SplitSeq(rest, ":") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		total = total*60 + n
	}
	return time.Duration(d)*24*time.Hour + time.Duration(total)*time.Second, true
}

// systemTrouble is Android's own, from the device log since the flow began.
func (r *Runner) systemTrouble(log string) string {
	const ui = "ANR in com.android.systemui"
	const dog = "WATCHDOG KILLING SYSTEM PROCESS"
	if !strings.Contains(log, ui) && !strings.Contains(log, dog) {
		return ""
	}
	for line := range r.sinceBegan(log) {
		switch {
		case strings.Contains(line, ui):
			return "Android's interface hung"
		case strings.Contains(line, dog):
			return "Android hung"
		}
	}
	return ""
}

// signature is what is the same in two crashes that are the same: not the thread or the process.
func signature(crash string) string {
	lines := strings.Split(crash, "\n")
	if len(lines) > 2 && strings.HasPrefix(lines[0], "FATAL EXCEPTION") {
		lines = lines[2:]
	}
	return "the app crashed: " + strings.Join(lines[:min(2, len(lines))], " ")
}
