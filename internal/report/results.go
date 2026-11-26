package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
)

// What show takes: which flows the results list.
const (
	ShowFailed = "failed" // FAIL, INTERRUPTED and NOT_RUN
	ShowFlaky  = "flaky"
	ShowSlow   = "slow" // the slowest, as many as slowest says
	ShowAll    = "all"
)

// Shows are all of them, the default last.
var Shows = []string{ShowFailed, ShowFlaky, ShowSlow, ShowAll}

// DefaultSlowest is how many flows the slow list has, if nothing says.
const DefaultSlowest = 5

// CheckShow says whether each is one of Shows.
func CheckShow(list []string) error {
	for _, v := range list {
		if !slices.Contains(Shows, v) {
			return fmt.Errorf("takes %s, not %q", strings.Join(Shows, ", "), v)
		}
	}
	return nil
}

// slowest are the flows that ran, the slowest first, as many as Slowest says.
func (s *Summary) slowest() []*Outcome {
	var ran []*Outcome
	for i := range s.Outcomes {
		if s.Outcomes[i].Status != NotRun {
			ran = append(ran, &s.Outcomes[i])
		}
	}
	// By tenths of a second, as shown; those alike stay in the order they ran.
	tenths := func(o *Outcome) time.Duration { return o.took().Truncate(100 * time.Millisecond) }
	slices.SortStableFunc(ran, func(a, b *Outcome) int { return int(tenths(b) - tenths(a)) })
	n := s.Slowest
	if n <= 0 {
		n = DefaultSlowest
	}
	return ran[:min(n, len(ran))]
}

// Shown are the flows the results list, in the order they ran.
func (s *Summary) Shown() []Outcome {
	if len(s.Show) == 0 || slices.Contains(s.Show, ShowAll) {
		return s.Outcomes
	}
	slow := s.slowest()
	var list []Outcome
	for i, o := range s.Outcomes {
		switch {
		case slices.Contains(s.Show, ShowFailed) && !o.Passed(),
			slices.Contains(s.Show, ShowFlaky) && o.Status == Flaky,
			slices.Contains(s.Show, ShowSlow) && slices.Contains(slow, &s.Outcomes[i]):
			list = append(list, o)
		}
	}
	return list
}

// Attempts says how it went over its tries, such as "passed on retry 2"; empty for one try.
func (o *Outcome) Attempts() string {
	switch {
	case len(o.Tries) < 2:
		return ""
	case o.Passed():
		return fmt.Sprintf("passed on retry %d", len(o.Tries)-1)
	}
	return fmt.Sprintf("failed %d times", o.TimesFailed())
}

// failing is the step that failed, where it is written and what went wrong, of its last try that failed.
func (o *Outcome) failing() (step, at, why, cause string, blame flow.Blame) {
	for i := len(o.Tries) - 1; i >= 0; i-- {
		t := o.Tries[i]
		if t.Err == nil {
			continue
		}
		why = strings.Join(strings.Fields(t.Err.Error()), " ")
		var e *flow.StepError
		if errors.As(t.Err, &e) {
			step, at, why = e.Step.Text(), e.Step.Place(), strings.Join(strings.Fields(e.Err.Error()), " ")
		}
		if t.Failure != nil {
			cause, blame = t.Failure.Cause, t.Failure.Blame
		}
		return step, at, why, cause, blame
	}
	return "", "", "", "", ""
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// jsonSummary is the run at a glance.
type jsonSummary struct {
	Began       string        `json:"began" about:"In UTC"`
	Seconds     float64       `json:"seconds" about:"All of it"`
	Total       int           `json:"total" about:"Flows"`
	Passed      int           `json:"passed" about:"At the first try"`
	Flaky       int           `json:"flaky" about:"Failed, then passed on a retry"`
	Failed      int           `json:"failed" about:""`
	Interrupted int           `json:"interrupted" about:""`
	NotRun      int           `json:"notRun" about:""`
	Retried     int           `json:"retried" about:"Flows tried more than once, whether they passed or not"`
	FailedFlows []string      `json:"failedFlows" about:"The names of those that failed, were interrupted or didn't run"`
	FlakyFlows  []string      `json:"flakyFlows" about:"The names of those that were flaky"`
	Slowest     []jsonSlowest `json:"slowest" about:"The flows that took longest, as many as slowest says"`
	Exit        jsonExit      `json:"exit" about:"The exit code of the run, see [Exit codes](#exit-codes)"`
}

type jsonExit struct {
	Code int    `json:"code" about:""`
	Name string `json:"name" about:"Such as 'failed', see [Exit codes](#exit-codes)"`
}

// exitOf is a code with its name.
func exitOf(code int) jsonExit {
	for _, e := range Exits {
		if e.Code == code {
			return jsonExit{e.Code, e.Name}
		}
	}
	return jsonExit{Code: code}
}

type jsonSlowest struct {
	Name    string  `json:"name" about:""`
	Seconds float64 `json:"seconds" about:""`
}

func (s *Summary) summary() jsonSummary {
	j := jsonSummary{Began: stamp(s.Began), Seconds: seconds(s.Took), Total: len(s.Outcomes), Passed: s.count(Passed), Flaky: s.count(Flaky),
		Failed: s.count(Failed), Interrupted: s.count(Interrupted), NotRun: s.count(NotRun), Retried: s.retried(),
		FailedFlows: []string{}, FlakyFlows: []string{}, Slowest: []jsonSlowest{}, Exit: exitOf(s.Exit)}
	for _, o := range s.Outcomes {
		switch {
		case o.Status == Flaky:
			j.FlakyFlows = append(j.FlakyFlows, o.Flow.Name)
		case !o.Passed():
			j.FailedFlows = append(j.FailedFlows, o.Flow.Name)
		}
	}
	for _, o := range s.slowest() {
		j.Slowest = append(j.Slowest, jsonSlowest{o.Flow.Name, seconds(o.took())})
	}
	return j
}

// verdict is the first word of the run: how it went.
func (s *Summary) verdict() string {
	var flaky *FlakyError
	switch err := s.Err(); {
	case errors.As(err, &flaky):
		return "FLAKY"
	case err != nil:
		return "FAIL"
	case s.count(Flaky) > 0:
		return "PASS, with flaky flows"
	}
	return "PASS"
}

// counts is "3 flows: 1 passed, 1 flaky, 1 failed".
func (s *Summary) counts() string {
	var parts []string
	for _, c := range []struct {
		status Status
		name   string
	}{{Passed, "passed"}, {Flaky, "flaky"}, {Failed, "failed"}, {Interrupted, "interrupted"}, {NotRun, "not run"}} {
		if n := s.count(c.status); n > 0 || c.status == Passed || c.status == Failed {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.name))
		}
	}
	return fmt.Sprintf("%d flows: %s", len(s.Outcomes), strings.Join(parts, ", "))
}

// markdown is a short report, for a chat or a mail: what failed, what was flaky, what was slow.
func (s *Summary) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n**%s** · %s · %.1f s · %s · %s\n", s.Title, s.verdict(), s.counts(), s.Took.Seconds(), stamp(s.Began), s.Device)
	if s.Shards > 0 {
		fmt.Fprintf(&b, "\nOnly %s, of --shard\n", s.Part())
	}
	if s.Stopped != "" {
		fmt.Fprintf(&b, "\nStopped: %s\n", s.Stopped)
	}
	shown := s.Shown()
	section := func(title string, which func(o *Outcome) bool) {
		first := true
		for i := range shown {
			o := &shown[i]
			if !which(o) {
				continue
			}
			if first {
				fmt.Fprintf(&b, "\n## %s\n\n", title)
				first = false
			}
			b.WriteString("- " + o.mdLine() + "\n")
		}
	}
	section("Failed", func(o *Outcome) bool { return !o.Passed() })
	section("Flaky", func(o *Outcome) bool { return o.Status == Flaky })
	if len(s.Show) == 0 || slices.Contains(s.Show, ShowAll) || slices.Contains(s.Show, ShowSlow) {
		if slow := s.slowest(); len(slow) > 0 {
			b.WriteString("\n## Slowest\n\n")
			for _, o := range slow {
				fmt.Fprintf(&b, "- %s %.1f s\n", o.Flow.Name, o.took().Seconds())
			}
		}
	}
	section("Passed", func(o *Outcome) bool {
		return o.Status == Passed && (len(s.Show) == 0 || slices.Contains(s.Show, ShowAll))
	})
	return b.String()
}

// mdLine is a flow, on a line of markdown.
func (o *Outcome) mdLine() string {
	line := fmt.Sprintf("**%s** %.1f s", o.Flow.Name, o.took().Seconds())
	if o.Status == NotRun {
		return fmt.Sprintf("**%s** not run: %s", o.Flow.Name, o.Why)
	}
	if a := o.Attempts(); a != "" {
		line += ", " + a
	}
	if o.Status == Interrupted {
		return line + ", " + o.Interruption()
	}
	if step, at, why, _, blame := o.failing(); why != "" {
		if step != "" {
			line += fmt.Sprintf(": `%s` at %s", step, at)
		}
		line += ": " + why
		if blame != "" {
			line += " (" + string(blame) + ")"
		}
	}
	return line
}

var flowBlock = regexp.MustCompile(`(?s)\{flows\}(.*?)\{end\}`)

// fromTemplate is the results as the template says.
func (s *Summary) fromTemplate() ([]byte, error) {
	data, err := os.ReadFile(s.Template)
	if err != nil {
		return nil, fmt.Errorf("results-template: %w", err)
	}
	text := flowBlock.ReplaceAllStringFunc(string(data), func(block string) string {
		inner := flowBlock.FindStringSubmatch(block)[1]
		var b strings.Builder
		for _, o := range s.Shown() {
			step, at, why, cause, blame := o.failing()
			b.WriteString(strings.NewReplacer("{name}", o.Flow.Name, "{file}", filepath.ToSlash(o.Flow.Path), "{status}", string(o.Status),
				"{attempts}", o.Attempts(), "{tries}", fmt.Sprint(len(o.Tries)), "{duration}", fmt.Sprintf("%.1f s", o.took().Seconds()),
				"{started}", stamp(o.Began), "{step}", step, "{at}", at, "{error}", why, "{cause}", cause, "{blame}", string(blame)).Replace(inner))
		}
		return b.String()
	})
	return []byte(strings.NewReplacer("{suite}", s.Title, "{device}", s.Device, "{verdict}", s.verdict(), "{total}", fmt.Sprint(len(s.Outcomes)),
		"{passed}", fmt.Sprint(s.count(Passed)), "{flaky}", fmt.Sprint(s.count(Flaky)), "{failed}", fmt.Sprint(s.count(Failed)),
		"{interrupted}", fmt.Sprint(s.count(Interrupted)), "{skipped}", fmt.Sprint(s.count(NotRun)),
		"{duration}", fmt.Sprintf("%.1f s", s.Took.Seconds()), "{started}", stamp(s.Began)).Replace(text)), nil
}

// CheckTemplate says whether a results template can be used: it is there, and its name ends as results do.
func CheckTemplate(path string) error {
	if _, err := FormatOf(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no such file: %s", filepath.ToSlash(path))
	}
	return nil
}
