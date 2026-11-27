package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/schema"
)

// A flow of rayo test --on-fail hold, held where it failed: it is fixed in its own files, run again from
// where it failed or another step, and goes on with the rest of the run.

// HoldHelp is what a held flow takes, as .help says it.
const HoldHelp = `  .why                    the failure again: why, where, and the elements most like the one looked for
  .screen                 what is on screen now
  .find <text>            the elements of the screen most like a text, with how to refer to them
  .diff                   what of the screen changed since it failed: + appeared, - went
  .retry                  read the files again, and run the flow from the step that failed to the end of its steps
  .from <step>            the same from another step: file:line, a text of one, or n of .steps
  .rewind [n]             the same from the n-th step before the one that failed (1); move the app back first
                          if those steps changed it
  .steps                  the last steps that passed, numbered as .from and .rewind count them
  .probe [n]              run the step that failed n times (5) as it is now, and say how often it passes
  .states                 the mocks' scenarios and the state of each
  - tapOn: OK             any command, or a list, run as it is: to move the app, it isn't added to the flow
  .mark <as> [why]        say what it is: app-bug, test-bug or flaky; the results keep it
  .continue               once .retry passed: onFlowComplete, and the rest of the run
  .skip                   leave the flow failed, and go on with the rest
  .abort                  leave it failed, and run no more flows
  .help
`

// DefaultHold is how long a flow is held with no command before it is left failed and the run goes on.
const DefaultHold = 5 * time.Minute

// HoldParams are of every flow a run holds.
type HoldParams struct {
	Output  string        // folder of the session, where rayo do finds it
	Say     io.Writer     // for people; io.Discard if a program reads
	Typed   bool          // also take commands typed in the terminal
	Timeout time.Duration // with no command for this long, the flow is left failed; 0 for an hour
	Held    func()        // as the hold begins, after it says so
}

// HoldEnd is how a hold ended, besides the error the flow fails with.
type HoldEnd struct {
	Mark  *report.Mark
	Abort bool // no more flows
}

// held is a flow held where it failed.
type held struct {
	p       HoldParams
	s       *flow.Session
	f       *flow.Flow // as last read
	err     error      // the failure now; nil once its steps passed
	at      string     // the step it is of, file:line
	text    string     // and how that step shows, to find it again if its line changes
	ordinal int        // and its place among the steps of its file, if its text changed too
	out     *liveOutput
	failed  []string // the screen as it failed
	seen    []string // the screen it last gave, for --screen=changes
	asserts int      // what the flow checked as it failed
	maybe   int      // its optional steps
	end     HoldEnd
}

// typedOnce reads the terminal once for all the holds of a run: each takes what is typed while it holds.
var (
	typedOnce sync.Once
	typedIn   chan string
)

func typedCommands() chan string {
	typedOnce.Do(func() {
		typedIn = make(chan string)
		go func() {
			readCommands(os.Stdin, func(text string) { typedIn <- text })
			close(typedIn)
		}()
	})
	return typedIn
}

// Hold holds a flow that failed until it is told how to go on, and returns nil if its steps passed
// once fixed, else the error it fails with.
func Hold(ctx context.Context, p HoldParams, s *flow.Session, f *flow.Flow, failed error) (HoldEnd, error) {
	h := &held{p: p, s: s, f: f, out: &liveOutput{}}
	h.failedAt(failed)
	h.asserts, h.maybe = f.Checks()
	if x := s.Failure(); x != nil {
		h.failed = report.Lines(x.Screen)
	}
	h.seen = h.failed
	requests := make(chan Request)
	closeServer, err := listen(p.Output, requests)
	if err != nil {
		return h.end, errors.Join(failed, fmt.Errorf("couldn't hold the flow: %w", err))
	}
	defer closeServer()
	fmt.Fprintf(p.Say, "\nHELD %s\n%s", f.Name, h.why())
	fmt.Fprintf(p.Say, "  held: fix the files, then rayo do .retry%s; .help says the rest\n", h.where())
	if p.Held != nil {
		p.Held()
	}
	var typed chan string
	if p.Typed {
		typed = typedCommands()
	}
	idle := p.Timeout
	if idle <= 0 {
		idle = DefaultHold
	}
	for {
		var q Request
		select {
		case text, more := <-typed:
			if !more {
				typed = nil
				continue
			}
			q.Text = text
		case q = <-requests:
		case <-ctx.Done():
			return h.end, h.failure()
		case <-time.After(idle):
			fmt.Fprintf(p.Say, "held for %s with no command: %s is left failed\n", idle, f.Name)
			return h.end, h.failure()
		}
		a, done := h.handle(ctx, q)
		fmt.Fprint(p.Say, a.Text())
		if q.answer != nil {
			q.answer <- a
		}
		if done {
			if h.err != nil {
				return h.end, h.failure()
			}
			return h.end, nil
		}
		if err := s.Dead(); err != nil {
			return h.end, errors.Join(h.failure(), err)
		}
	}
}

// where is the folder rayo do needs to be told, if not the default.
func (h *held) where() string {
	if filepath.Clean(h.p.Output) == ".rayo" {
		return ""
	}
	return " (or rayo do --output " + filepath.ToSlash(h.p.Output) + " .retry)"
}

// failure is the error the flow fails with, if the hold ends now.
func (h *held) failure() error {
	if h.err == nil {
		return errors.New("held, its steps passed, and the hold ended without .continue")
	}
	return h.err
}

// failedAt takes a failure as the one now.
func (h *held) failedAt(err error) {
	h.err, h.at, h.text, h.ordinal = err, "", "", -1
	if s := flow.FailedStep(err); s != nil {
		h.at, h.text, h.ordinal = s.Place(), s.Text(), h.f.Ordinal(s.Place())
	}
}

// why is the failure in a few lines, as rayo explain says it.
func (h *held) why() string {
	if h.err == nil {
		return "  its steps passed from " + h.at + " to their end: .continue runs onFlowComplete and the rest\n"
	}
	o := report.Outcome{Flow: h.f}
	t := flow.Result{Err: h.err, Failure: h.s.Failure()}
	x := report.FailureOf(h.err, t.Failure)
	x.Verdict = o.Verdict(t)
	u := report.Unpassed{Status: report.Failed, Name: h.f.Name, File: filepath.ToSlash(h.f.Path), Failure: &x}
	text := u.Explain()
	// what is next while held is in .help, not --resume
	var kept []string
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.Contains(line, "rayo test --resume") || strings.Contains(line, "--on-fail hold") {
			continue
		}
		kept = append(kept, line)
	}
	if n := len(kept); n > 0 && strings.TrimSpace(kept[n-1]) == "next:" {
		kept = kept[:n-1]
	}
	return strings.Join(kept, "")
}

func (h *held) reply() Reply {
	a := Reply{SchemaVersion: report.SchemaVersion, OK: true, Warnings: []string{}, Calls: nil, Screen: []string{}}
	a.Calls = report.List(a.Calls)
	a.Held = &Held{Flow: h.f.Name, File: filepath.ToSlash(h.f.Path), At: h.at, Fixed: h.err == nil}
	return a
}

func (h *held) fail(err error) Reply {
	a := h.reply()
	a.OK, a.Error = false, err.Error()
	return a
}

// handle answers a request, and says whether the hold is over.
func (h *held) handle(ctx context.Context, q Request) (Reply, bool) {
	text := strings.TrimSpace(q.Text)
	if !strings.HasPrefix(text, ".") {
		return h.poke(ctx, q), false
	}
	name, arg, _ := strings.Cut(text, " ")
	arg = strings.TrimSpace(arg)
	switch name {
	case ".retry", ".from", ".rewind":
		return h.again(ctx, name, arg, q), false
	case ".probe":
		return h.probe(ctx, arg), false
	case ".mark":
		return h.mark(arg), false
	case ".find":
		if arg == "" {
			return h.fail(errors.New(".find takes a text, such as .find Pay")), false
		}
		screen, err := h.s.Screen(ctx)
		if err != nil {
			return h.fail(err), false
		}
		a := h.reply()
		a.Screen = report.Lines(strings.Join(flow.Nearest(screen, arg, 20), "\n"))
		if len(a.Screen) == 0 {
			a.Output = "nothing on screen is like " + strconv.Quote(arg) + "\n"
		}
		return a, false
	case ".continue", ".skip", ".abort":
		if arg != "" {
			return h.fail(fmt.Errorf("%s takes nothing", name)), false
		}
		return h.finish(name)
	}
	if arg != "" {
		return h.fail(fmt.Errorf("%s takes nothing", name)), false
	}
	a := h.reply()
	var err error
	switch name {
	case ".help":
		a.Output = HoldHelp
	case ".why":
		a.Output = h.why()
	case ".screen":
		a.Output, err = h.s.Screen(ctx)
	case ".diff":
		screen, e := h.s.Screen(ctx)
		if err = e; err == nil {
			a.Screen = changes(h.failed, report.Lines(screen))
			if len(a.Screen) == 0 {
				a.Output = "the screen is as it was when the flow failed\n"
			}
		}
	case ".steps":
		a.Output = h.steps()
	case ".states":
		a.Output, err = h.s.States()
	default:
		names := []string{".help", ".why", ".screen", ".find", ".diff", ".retry", ".from", ".rewind", ".steps", ".probe", ".states", ".mark", ".continue", ".skip", ".abort"}
		if near := schema.Nearest(name, names); near != "" {
			err = fmt.Errorf("unknown %s, did you mean %s? See .help", name, near)
		} else {
			err = fmt.Errorf("unknown %s while held, see .help", name)
		}
	}
	if err != nil {
		out := a.Output
		a = h.fail(err)
		a.Output = out
	}
	return a, false
}

// steps lists the last steps that passed, the newest last, numbered as .from and .rewind count them.
func (h *held) steps() string {
	ran := h.s.Ran()
	if len(ran) == 0 {
		return "no step passed before it failed\n"
	}
	var b strings.Builder
	for i, place := range ran[max(0, len(ran)-30):] {
		n := len(ran) - max(0, len(ran)-30) - i
		text := ""
		if s := h.f.At(place); s != nil {
			text = "  " + s.Text()
		}
		fmt.Fprintf(&b, "%3d. %s%s\n", n, place, text)
	}
	fmt.Fprintf(&b, "  failed at %s; .rewind n runs from the n-th above\n", h.at)
	return b.String()
}

// again reads the flow's files again and runs its steps from a step to their end.
func (h *held) again(ctx context.Context, name, arg string, q Request) Reply {
	f, err := flow.Load(h.f.Path)
	if err != nil {
		return h.fail(fmt.Errorf("the flow doesn't read: %w; fix it and .retry", err))
	}
	moved := ""
	from := h.at
	switch name {
	case ".retry":
		if arg != "" {
			return h.fail(errors.New(".retry takes nothing; .from <step> runs from another"))
		}
		if h.err == nil {
			return h.fail(errors.New("its steps passed already: .continue, or .from <step> to run some again"))
		}
		if from == "" {
			return h.fail(errors.New("the failure isn't of a step, so there is none to go on from: .skip, then run it whole"))
		}
		if now := f.Moved(from, h.text, h.ordinal); now != from {
			moved, from = fmt.Sprintf("the step that failed, %s, is at %s now\n", h.text, now), now
		}
	case ".rewind":
		n := 1
		if arg != "" {
			if n, err = strconv.Atoi(arg); err != nil || n < 1 {
				return h.fail(fmt.Errorf(".rewind takes how many steps back, not %q", arg))
			}
		}
		if from, err = h.back(n); err != nil {
			return h.fail(err)
		}
	case ".from":
		if arg == "" {
			return h.fail(errors.New(".from takes a step: file:line, a text of one, or n of .steps"))
		}
		if n, e := strconv.Atoi(arg); e == nil {
			if from, err = h.back(n); err != nil {
				return h.fail(err)
			}
		} else if from, err = f.Resolve(arg); err != nil {
			return h.fail(err)
		}
	}
	h.f = f
	defer h.s.Telling(h.out)()
	d := h.s.From(ctx, f, from)
	a := h.reply()
	a.Output = moved + h.out.take()
	a.Warnings, a.Calls = report.List(d.Warnings), report.List(d.Calls)
	if d.Err != nil {
		h.failedAt(d.Err)
		a.OK, a.Error = false, d.Err.Error()
		x := report.FailureOf(d.Err, d.Failure)
		a.Failure = &x
		h.failed = x.Screen
	} else {
		h.err, h.at = nil, from
		a.Output += fmt.Sprintf("passed from %s to the end of its steps: .continue runs onFlowComplete and the rest of the run\n", from)
	}
	a.Held = h.reply().Held
	h.screen(ctx, &a, q)
	return a
}

// back is the place of the n-th step that passed before the failure, the last being 1.
func (h *held) back(n int) (string, error) {
	ran := h.s.Ran()
	if n > len(ran) {
		return "", fmt.Errorf("only %d steps passed before it: .steps lists them", len(ran))
	}
	return ran[len(ran)-n], nil
}

// screen adds the screen to a reply, as --screen asks.
func (h *held) screen(ctx context.Context, a *Reply, q Request) {
	if !q.Screen || h.s.Dead() != nil {
		return
	}
	out, err := h.s.Screen(ctx)
	if err != nil {
		a.Warnings = append(a.Warnings, "couldn't read the screen: "+err.Error())
		return
	}
	lines := report.Lines(out)
	if q.Ignore != "" {
		lines, a.Ignored = ignoring(lines, q.Ignore)
	}
	a.Screen = lines
	if q.Changes {
		a.Screen = changes(h.seen, lines)
	}
	h.seen = lines
}

// poke runs commands as they are, to move the app; they aren't added to the flow.
func (h *held) poke(ctx context.Context, q Request) Reply {
	written, err := flow.ParseSteps(h.f.Path, q.Text)
	if err != nil {
		return h.fail(err)
	}
	var steps []*flow.Step
	for _, w := range written {
		steps = append(steps, w.Step)
	}
	defer h.s.Telling(h.out)()
	d := h.s.Do(ctx, steps)
	a := h.reply()
	a.Output, a.Warnings, a.Calls = h.out.take(), report.List(d.Warnings), report.List(d.Calls)
	if d.Err != nil {
		a.OK, a.Error = false, d.Err.Error()
		x := report.FailureOf(d.Err, d.Failure)
		a.Failure = &x
	}
	a.Warnings = append(a.Warnings, "what is typed only moves the app: write it in the flow's file for the flow to do it")
	h.screen(ctx, &a, q)
	return a
}

// probe runs the step that failed again and again, as it is now in its file.
func (h *held) probe(ctx context.Context, arg string) Reply {
	n := 5
	if arg != "" {
		var err error
		if n, err = strconv.Atoi(arg); err != nil || n < 1 || n > 50 {
			return h.fail(fmt.Errorf(".probe takes how many times, 1 to 50, not %q", arg))
		}
	}
	if h.at == "" {
		return h.fail(errors.New("no step failed, so there is none to probe"))
	}
	f, err := flow.Load(h.f.Path)
	if err != nil {
		return h.fail(err)
	}
	place := f.Moved(h.at, h.text, h.ordinal)
	step := f.At(place)
	if step == nil {
		return h.fail(fmt.Errorf("there is no step at %s now", place))
	}
	defer h.s.Telling(h.out)()
	tries := h.s.Probe(ctx, step, n)
	passed := 0
	var b strings.Builder
	for i, t := range tries {
		status := "pass"
		if t.Err != nil {
			status = "fail: " + firstLine(t.Err)
		} else {
			passed++
		}
		fmt.Fprintf(&b, "  %d. %d ms  %s\n", i+1, t.Took.Milliseconds(), status)
	}
	h.out.take() // each step's own lines: the list says it
	a := h.reply()
	verdict := "it always fails as it is: the step, or what is on screen, is wrong; not a flake"
	switch {
	case passed == len(tries):
		verdict = "it always passes now: the screen it waits for has come, so it is a wait too short, or the app was slow then; .retry"
	case passed > 0:
		verdict = "it passes at times: a flake; a timeout: on it, or a wait for what it needs before it, holds it still"
	}
	a.Output = fmt.Sprintf("%s, %d of %d passed:\n%s%s\n", step.Text(), passed, len(tries), b.String(), verdict)
	return a
}

func firstLine(err error) string {
	first, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	return first
}

// mark says what the failure is, for the results.
func (h *held) mark(arg string) Reply {
	as, note, _ := strings.Cut(arg, " ")
	if !slices.Contains(report.Marks, as) {
		return h.fail(fmt.Errorf(".mark takes %s, and why, such as .mark app-bug search answers 500", strings.Join(report.Marks, ", ")))
	}
	h.end.Mark = &report.Mark{As: as, Note: strings.TrimSpace(note)}
	a := h.reply()
	a.Output = "marked " + as + "; .skip leaves it failed, so marked, and goes on\n"
	if as == "app-bug" && h.err != nil {
		a.Output = "marked app-bug: the flow stays as it is, failing until the app is fixed; .skip goes on\n"
	}
	return a
}

// finish ends the hold, if it may.
func (h *held) finish(name string) (Reply, bool) {
	a := h.reply()
	switch name {
	case ".continue":
		if h.err != nil {
			return h.fail(errors.New(".continue goes on once the flow's steps pass: fix it, then .retry; .skip leaves it failed")), false
		}
		if f, err := flow.Load(h.f.Path); err == nil {
			if asserts, maybe := f.Checks(); asserts < h.asserts || maybe > h.maybe {
				w := fmt.Sprintf("WEAKENED: as it failed the flow checked %d things, %d of them optional; now %d, %d optional: a fix that checks less makes it pass without fixing it", h.asserts, h.maybe, asserts, maybe)
				h.s.Warn(w)
				a.Warnings = append(a.Warnings, w)
			}
		}
		a.Output = "going on: onFlowComplete, then the rest of the run\n"
	case ".skip":
		a.Output = "left failed; going on with the rest of the run\n"
	case ".abort":
		h.end.Abort = true
		a.Output = "left failed; no more flows run\n"
	}
	return a, true
}
