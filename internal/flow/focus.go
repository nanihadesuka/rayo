package flow

import (
	"cmp"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/schema"
)

// What went wrong, in a word or two: the cause of a failure's verdict.
const (
	NotOnScreen  = "not on screen"
	Timeout      = "timeout"
	AppCrashed   = "app crashed"
	UnmockedCall = "unmocked call"
	DeviceError  = "device error"
	StepFailed   = "step failed" // anything else, such as a script or a command that failed
)

// Causes are all of them.
var Causes = []string{NotOnScreen, Timeout, AppCrashed, UnmockedCall, DeviceError, StepFailed}

// DefaultWindow is how long before the failing step began the files of a failure begin.
const DefaultWindow = 10 * time.Second

// What detail takes: how much is printed.
const (
	Brief  = "brief"
	Normal = "normal"
	Full   = "full"
)

// Details are all of them.
var Details = []string{Brief, Normal, Full}

// causeOf is what went wrong, from the error and what diagnose found.
func causeOf(err error, f *Failure) string {
	text := err.Error()
	switch {
	case f.Blame == Dev:
		return DeviceError
	case f.Blame == App && !f.blank:
		return AppCrashed
	case strings.Contains(text, "isn't mocked"):
		return UnmockedCall
	case strings.Contains(text, "not on screen"):
		return NotOnScreen
	case strings.Contains(text, "timeout"), strings.Contains(text, "timed out"), strings.Contains(text, "still on screen"),
		strings.Contains(text, "deadline exceeded"):
		return Timeout
	}
	return StepFailed
}

// wanted is what a step that failed looked for, from its error: "not on screen: Pay" is Pay.
func wanted(err error) string {
	text := err.Error()
	i := strings.LastIndex(text, NotOnScreen+": ")
	if i < 0 {
		return ""
	}
	want := strings.TrimSpace(strings.SplitN(text[i+len(NotOnScreen)+2:], "\n", 2)[0])
	var sel map[string]any // a selector, as the agent says it: what it has of text
	if strings.HasPrefix(want, "{") && json.Unmarshal([]byte(want), &sel) == nil {
		var parts []string
		for _, k := range []string{"text", "id"} {
			if v, ok := sel[k].(string); ok {
				parts = append(parts, v)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}
	return want
}

var bounds = regexp.MustCompile(`\s+\[-?\d+,-?\d+\]\[-?\d+,-?\d+\]\s*$`)

// Nearest are the elements of a screen, as rayo hierarchy prints it, most like what is looked for:
// at most n, the closest first. None is close enough if nothing of it is alike.
func Nearest(screen, want string, n int) []string {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return nil
	}
	type scored struct {
		line  string
		score float64
		at    int
	}
	var all []scored
	for i, line := range strings.Split(strings.ReplaceAll(screen, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		best := 0.0
		for _, part := range strings.Split(bounds.ReplaceAllString(line, ""), " | ") {
			_, value, ok := strings.Cut(part, ": ")
			if !ok {
				value = part // as it is, if it says no key
			}
			value = strings.ToLower(value)
			if _, id, ok := strings.Cut(value, ":id/"); ok { // com.app:id/pay is pay
				value = id
			}
			best = max(best, likeness(want, value))
		}
		if best >= 0.4 {
			all = append(all, scored{line, best, i})
		}
	}
	slices.SortStableFunc(all, func(a, b scored) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.at, b.at)) })
	var lines []string
	for _, s := range all[:min(n, len(all))] {
		lines = append(lines, s.line)
	}
	return lines
}

// likeness is from 0, nothing alike, to 1, the same.
func likeness(a, b string) float64 {
	switch {
	case a == b:
		return 1
	case a == "" || b == "":
		return 0
	case strings.Contains(b, a) || strings.Contains(a, b):
		return 0.6 + 0.3*float64(min(len(a), len(b)))/float64(max(len(a), len(b)))
	}
	if re, err := regexp.Compile("(?i)^(?:" + a + ")$"); err == nil && re.MatchString(b) { // a selector may be a regex
		return 0.95
	}
	d := schema.Distance([]rune(a), []rune(b))
	return 1 - float64(d)/float64(max(len([]rune(a)), len([]rune(b))))
}

// errored says whether a call went wrong: not answered 2xx or 3xx, or without a mock.
func errored(c mock.Call) bool {
	return c.Unmocked || !(strings.HasPrefix(c.Outcome, "2") || strings.HasPrefix(c.Outcome, "3"))
}

// relevant are the last few calls, and before them those since from that went wrong.
func relevant(calls []mock.Call, from time.Time) []mock.Call {
	last := max(0, len(calls)-fewCalls)
	var list []mock.Call
	for i, c := range calls {
		if i >= last || errored(c) && !c.At.Before(from) {
			list = append(list, c)
		}
	}
	return list
}

// logLayout is how a line of the device log begins: its clock, which isn't the computer's.
const logLayout = "01-02 15:04:05"

// window is the lines of the device log from a while before the failing step to the end, of the app
// and of the crashes; all of it if the times can't be read. The device's clock is told by the last line.
func window(log, app string, since time.Duration) string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(log, "\r\n", "\n"), "\n"), "\n")
	var end time.Time
	for i := len(lines) - 1; i >= 0 && end.IsZero(); i-- {
		end = stamped(lines[i])
	}
	if end.IsZero() {
		return log
	}
	start := end.Add(-since)
	pids := map[string]bool{}
	if app != "" {
		started := regexp.MustCompile(`(?:Start proc |pid=|PID: )(\d+)[:, ]*` + regexp.QuoteMeta(app))
		said := regexp.MustCompile(regexp.QuoteMeta(app) + `[, ]+(?:PID|pid)[:=] *(\d+)`)
		for _, l := range lines {
			if !strings.Contains(l, app) {
				continue
			}
			if f := strings.Fields(l); len(f) > 2 {
				pids[f[2]] = true
			}
			for _, re := range []*regexp.Regexp{started, said} {
				if m := re.FindStringSubmatch(l); m != nil {
					pids[m[1]] = true
				}
			}
		}
	}
	var b strings.Builder
	for _, l := range lines {
		t := stamped(l)
		if t.IsZero() || t.Before(start) {
			continue
		}
		f := strings.Fields(l)
		if app == "" || strings.Contains(l, app) || len(f) > 2 && pids[f[2]] || strings.Contains(l, "AndroidRuntime") || strings.Contains(l, "ANR in") {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// stamped is when a line of the device log was written; zero if it doesn't say.
func stamped(line string) time.Time {
	if len(line) < len(logLayout) {
		return time.Time{}
	}
	t, err := time.Parse(logLayout, line[:len(logLayout)])
	if err != nil {
		return time.Time{}
	}
	return t
}
