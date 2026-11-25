package flow

import (
	"context"
	"errors"
	"fmt"
	"github.com/nanihadesuka/rayo/internal/mock"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/agent"
	"gopkg.in/yaml.v3"
)

const (
	findTimeout     = 15 * time.Second
	goneTimeout     = 5 * time.Second
	optionalTimeout = 3 * time.Second
	peekTimeout     = 700 * time.Millisecond // conditions
)

// Options of a Runner.
type Options struct {
	Env           map[string]string // wins over the flow's
	Profile       bool
	Timings       bool           // a group's total, and a note while a step takes long
	MockPort      int            // on the device
	Recording     *Recording     // of flows that don't say; its path is a folder
	JSON          bool           // the calls of a failure are saved so, and not as text
	Capture       string         // folder for what the real backend answers; empty to use the mocks
	Backend       string         // what a capture calls; empty when the path begins with the host
	CaptureAppend bool           // the capture goes on from the one in its folder
	CaptureUpdate string         // the mocks the capture is compared with; it is in their .update
	CaptureCheck  bool           // a difference from them is a failure
	Scale         float64        // of the default waits; 0 is 1. A step's own timeout isn't scaled
	Network       string         // how calls reach the mock: interceptor or proxy; empty for the flow's
	Hosts         []string       // with a proxy, those to open; empty for the default
	Matching      mock.Settings  // how a capture becomes mocks; what isn't set is the saved capture's, or the default
	Unmocked      *mock.Unmocked // calls without a mock that don't fail a flow, besides its own; nil for none
	NameTemplate  string         // of the files of an attempt; empty for DefaultTemplate
	Artifacts     []string       // the kinds of file written; nil for all
	Window        time.Duration  // of a failure's files, before the failing step began; 0 for DefaultWindow
	Detail        string         // Full keeps a failure's files whole
}

// Networks are the ways calls reach the mock.
const (
	Interceptor = "interceptor"
	Proxy       = "proxy"
)

// scaled is a default wait, times Scale.
func (o *Options) scaled(d time.Duration) time.Duration {
	if o.Scale <= 0 {
		return d
	}
	return time.Duration(float64(d) * o.Scale)
}

// Runner runs flows on a device, one at a time.
type Runner struct {
	Options
	device   *adb.Device
	agent    *agent.Agent
	reporter Reporter
	backend  *backend
	room     *room
	Start    *Start // of the next flows to run; nil to run them whole

	// Of the flow running now.
	flow       *Flow
	began      time.Time
	attempt    Attempt  // where its files go
	files      []string // it wrote
	script     *script
	recent     []string // the last steps that passed
	failure    *Failure
	log        string // of the device, when it failed
	warnings   []string
	located    bool      // a pretend location was set
	video      *video    // being recorded
	recordings []string  // saved
	airplane   string    // how airplane mode was before the flow changed it
	skipped    bool      // the step running now said it was skipped
	stepBegan  time.Time // of the last step that isn't a group's
	from       string    // the step to begin from, until it is reached
	passed     int       // steps gone by before it
	trail      []StepStates
}

// NewRunner makes a Runner; what happens goes to the reporter.
func NewRunner(d *adb.Device, a *agent.Agent, reporter Reporter, o Options) *Runner {
	return &Runner{Options: o, device: d, agent: a, reporter: reporter,
		backend: newBackend(d, o), room: &room{device: d}}
}

// Run runs a flow and puts back what it changed, also when the context is cancelled. Failures,
// screenshots and videos are named after the attempt.
func (r *Runner) Run(ctx context.Context, f *Flow, a Attempt) Result {
	start := time.Now()
	r.prepare(ctx, f, a)
	err := r.withEnv(f, func() error { return r.run(ctx, f) })
	if e := r.saveCapture(); e != nil {
		err = errors.Join(err, e)
	}
	r.noteProxy()
	r.noteIgnored()
	r.reset()
	if err != nil && ctx.Err() == nil {
		if r.failure == nil {
			r.failure = &Failure{}
		}
		r.diagnose(err, r.log)
		r.failure.Cause = causeOf(err, r.failure)
		if r.failure.Cause == NotOnScreen {
			r.failure.Nearest = Nearest(r.failure.Screen, wanted(err), 5)
		}
	}
	return Result{Flow: f, Err: err, Elapsed: time.Since(start), Failure: r.failure, Warnings: r.warnings, Recordings: r.recordings, Files: r.files}
}

// withEnv sets -e, and then the flow's header for what -e doesn't set.
func (r *Runner) withEnv(f *Flow, fn func() error) error {
	env := map[string]string{} // -e and a suite's env may use ${...}, as a header does
	for k, v := range r.Env {
		filled, err := r.script.fill(v)
		if err != nil {
			return fmt.Errorf("variable %s: %w", k, err)
		}
		env[k] = filled
	}
	return r.script.with(env, func() error {
		header, err := r.script.header(f.Env)
		if err != nil {
			return err
		}
		return r.script.with(header, fn)
	})
}

// prepare forgets the flow before.
func (r *Runner) prepare(ctx context.Context, f *Flow, a Attempt) {
	r.flow, r.began, r.attempt = f, time.Now(), a
	r.recent, r.failure, r.warnings, r.recordings, r.files = nil, nil, nil, nil, nil
	r.log, r.located, r.airplane = "", false, ""
	r.from, r.passed, r.trail = "", 0, nil
	if r.Start != nil {
		r.from = r.Start.From
	}
	for _, e := range evidence { // of an earlier run
		_ = os.Remove(r.named(e.name, e.ext))
	}
	r.backend.unmocked = f.Unmocked.Merge(r.Unmocked)
	r.backend.proxy = r.Network == Proxy || r.Network == "" && f.Network == Proxy // again in begin, if ${...}
	r.script = newScript(ctx, func(text string) { r.reporter.Note(2, text) })
	// rayo.file is the flow's file name without its extension: -e or a suite's env may give it
	// another name, such as MAESTRO_FILENAME: ${rayo.file}.
	_ = r.script.vm.Get("rayo").ToObject(r.script.vm).Set("file", strings.TrimSuffix(filepath.Base(f.Path), filepath.Ext(f.Path)))
}

// reset undoes what the flow changed, so the next one starts the same whatever this one did. One that
// fails must not keep the others from it.
func (r *Runner) reset() {
	try := func(what string, err error) {
		if err != nil {
			r.warn(1, fmt.Sprintf("couldn't %s, which may affect the next flows: %v", what, err))
		}
	}
	if r.video != nil {
		if err := r.stopRecording(); err != nil {
			r.warn(1, fmt.Sprintf("couldn't save the recording: %v", err))
		}
	}
	try("stop the mock", r.backend.close())
	try("reset the camera", r.room.reset())
	if r.located && r.agent.Dead() == nil { // an agent that ends does this by itself
		_, err := r.agent.Do(context.Background(), "resetLocation", nil, 10*time.Second)
		try("reset the location", err)
	}
	try("reset airplane mode", r.resetAirplane())
}

// warn tells something that doesn't fail the flow, but may explain it or the next.
func (r *Runner) warn(depth int, text string) {
	r.warnings = append(r.warnings, text)
	r.reporter.Note(depth, text)
}

func (r *Runner) run(ctx context.Context, f *Flow) error {
	err := r.begin(f)
	if err == nil && r.from == "" {
		err = r.steps(ctx, f.OnStart, 0)
	}
	if err == nil {
		err = r.steps(ctx, f.Steps, 0)
	}
	if err == nil && r.from != "" {
		err = fmt.Errorf("never reached the step %s to begin from: a runFlow's when or a repeat's while may have left it out", r.from)
	}
	until := errors.Is(err, errUntil)
	if until {
		err = nil
	}
	if err == nil {
		err = r.backend.check()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		r.evidence(ctx)
	}
	if err == nil && r.Start != nil {
		defer r.reporter.Note(0, "ran only part of the flow, so its pass is not the whole flow's: it stays in failed.json; once the fix is right, run it whole with --only "+f.Name)
	}
	if until {
		r.reporter.Note(0, "stopped after "+r.Start.Until+" as --until says: onFlowComplete didn't run, and the app is left as it is")
		return err
	}
	if e := r.steps(ctx, f.OnComplete, 0); err == nil {
		err = e
	}
	if err == nil { // calls made since
		if err = r.backend.check(); err != nil {
			r.evidence(ctx)
		}
	}
	return err
}

func (r *Runner) steps(ctx context.Context, steps []*Step, depth int) error {
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.from != "" {
			if !s.Is(r.from, r.flow.Path) && !r.leadsTo(s, 0) {
				if err := r.pass(ctx, s, depth); err != nil {
					return &StepError{Step: s, Err: err}
				}
				continue
			}
			if s.Is(r.from, r.flow.Path) {
				if err := r.reached(s); err != nil {
					return &StepError{Step: s, Err: err}
				}
			}
		}
		if err := r.step(ctx, s, depth); err != nil {
			return err
		}
		if r.stopsAfter(s) {
			return errUntil
		}
	}
	return nil
}

// step runs a command and reports it. A group's own commands have been reported by then.
func (r *Runner) step(ctx context.Context, s *Step, depth int) error {
	start := time.Now()
	r.agent.ResetSpent()
	k := commands[s.Kind]
	if !k.group {
		r.backend.during(s)
		r.stepBegan = start
		if states := r.backend.scenarios(); len(states) > 0 {
			r.trail = append(r.trail, StepStates{At: s.Place(), States: states})
			r.trail = r.trail[max(0, len(r.trail)-trailMost):]
		}
	}
	r.skipped = false
	slow := func() {}
	if r.Timings && !k.group {
		slow = r.whileSlow(s.describe(), depth, start)
	}
	optional, err := r.optional(s)
	if err == nil {
		err = r.call(ctx, s, k, optional, depth)
	}
	stopping := errors.Is(err, errUntil) // after a step inside this one
	if stopping {
		err = nil
	}
	if err == nil && !k.group && r.backend.mock != nil {
		err = r.backend.mock.Err() // such as a call that can't be mocked: at once, not after the flow
	}
	slow()
	report := StepReport{Depth: depth, Text: s.describe(), Took: time.Since(start)}
	if r.Profile && !k.group {
		report.Profile = r.agent.Explain(report.Took)
	}
	switch {
	case err == nil:
		report.Status = Passed
	case optional && r.agent.Dead() == nil && ctx.Err() == nil:
		report.Status = Optional
		err = nil
	default:
		report.Status = Failed
	}
	if report.Status == Passed && !k.group {
		r.recent = append(r.recent, report.Text)
		r.recent = r.recent[max(0, len(r.recent)-3):]
	}
	if !k.group || report.Status == Optional {
		r.reporter.Step(report)
	} else if r.Timings && !r.skipped {
		total := report
		total.Status = Finished
		if report.Status == Failed {
			total.Text += " (failed)"
		}
		r.reporter.Step(total)
	}
	r.skipped = false
	if stopping && err == nil {
		return errUntil
	}
	var known *StepError
	if errors.As(err, &known) {
		known.From = append(known.From, s)
	} else if err != nil {
		err = &StepError{Step: s, Err: err}
	}
	return err
}

// slowNote is how often a step that takes long is told of.
const slowNote = 5 * time.Second

// whileSlow notes a step that is still running, every few seconds, so that a hang shows where it
// is, until stop is called.
func (r *Runner) whileSlow(text string, depth int, start time.Time) (stop func()) {
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(slowNote)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				r.reporter.Note(depth+1, fmt.Sprintf("… still on %s after %d s", text, int(time.Since(start).Seconds())))
			}
		}
	}()
	return func() { close(done) }
}

// StepError says which step an error is of.
type StepError struct {
	Step *Step
	From []*Step // the steps it is inside of, innermost first
	Err  error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Step.Place(), e.Step.Text(), e.Err)
}

// Place is the file and line of a step.
func (s *Step) Place() string { return fmt.Sprintf("%s:%d", filepath.ToSlash(s.Path), s.Line) }

// Text is how a step shows in the output.
func (s *Step) Text() string { return s.describe() }

func (e *StepError) Unwrap() error { return e.Err }

func (r *Runner) optional(s *Step) (bool, error) {
	n := get(s.Value, keyOptional)
	if n == nil {
		return false, nil
	}
	n, err := r.script.resolve(n)
	if err == nil {
		err = filledIn(n, keyOptional.check)
	}
	if err != nil {
		return false, err
	}
	return keyOptional.kind.read(n)
}

func (r *Runner) call(ctx context.Context, s *Step, k command, optional bool, depth int) error {
	v, err := r.script.resolve(s.Value)
	if err != nil {
		return err
	}
	if err := s.recheck(v); err != nil {
		return err
	}
	c := &call{ctx: ctx, r: r, s: s, k: k, v: v, depth: depth, limit: findTimeout}
	if optional {
		c.limit = optionalTimeout
	}
	if k.limit != 0 {
		c.limit = k.limit
	}
	c.limit = keyTimeout.or(v, r.scaled(c.limit))
	return k.run(c)
}

// recheck checks a value again once each ${...} in it is known.
func (s *Step) recheck(v *yaml.Node) error {
	k := commands[s.Kind]
	if k.free || !hasExpression(s.Value) {
		return nil
	}
	return filledIn(v, func(p *reader, n *yaml.Node) error { return p.mapping(n, s.Kind, k.accepts(), commonKeys) })
}

// filledIn checks a value without saying where, as the step's line is already in the message.
func filledIn(n *yaml.Node, check func(p *reader, n *yaml.Node) error) error {
	err := check(&reader{filled: true}, n)
	var f *Problem
	if errors.As(err, &f) {
		return errors.New(f.What)
	}
	return err
}

// evidence saves what can be known of a failure. What can't be saved is reported, as it isn't the failure.
func (r *Runner) evidence(ctx context.Context) {
	r.failure = &Failure{Before: r.recent, Scenarios: r.backend.scenarios(), Trail: r.trail}
	from := r.windowStart()
	if r.backend.mock != nil {
		r.failure.Calls = relevant(r.backend.mock.Calls(), from)
		for i := range r.failure.Calls {
			r.failure.Calls[i].Body = "" // long, or private; those of a mock are in the requests file
		}
	}
	var saved []string
	save := func(kind, name, ext string, data []byte, err error) {
		if !r.wants(kind) {
			return
		}
		path := r.named(name, ext)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(path), 0o755)
		}
		if err == nil {
			err = os.WriteFile(path, data, 0o644)
		}
		if err != nil {
			r.reporter.Note(1, fmt.Sprintf("couldn't save %s: %v", filepath.ToSlash(path), err))
			return
		}
		r.wrote(path)
		saved = append(saved, filepath.ToSlash(path))
	}
	png, err := r.screenshot()
	if small, e := shrink(png); err == nil && e == nil {
		save(Screenshot, "failure", "jpg", small, nil)
	} else { // as it is, if it can't be read
		save(Screenshot, "failure", "png", png, err)
	}
	if r.agent.Dead() == nil {
		a, err := r.agent.Do(ctx, "tree", nil, 5*time.Second)
		save(Screen, "screen", "txt", []byte(a.Out), err)
		r.failure.Screen = a.Out
	}
	log, err := r.device.Exec("logcat -d -t 2000")
	cut := log
	if r.Detail != Full && err == nil {
		cut = []byte(window(string(log), r.flow.App, time.Since(from)))
	}
	save(Logcat, "logcat", "txt", cut, err)
	r.log = string(log)
	r.failure.Crash = r.crash(string(log))
	if r.backend.mock != nil {
		ext := "txt"
		if r.JSON {
			ext = "json"
		}
		since := from
		if r.Detail == Full {
			since = time.Time{}
		}
		data, err := r.backend.calls(r.reporter, r.JSON, since)
		save(Requests, "requests", ext, data, err)
	}
	if len(saved) > 0 {
		r.failure.Saved = saved[0]
		r.reporter.Note(1, "saved "+strings.Join(saved, ", "))
	}
}

// windowStart is when the files of a failure begin: a while before the failing step began.
func (r *Runner) windowStart() time.Time {
	w := r.Window
	if w <= 0 {
		w = DefaultWindow
	}
	began := r.stepBegan
	if began.IsZero() || began.Before(r.began) {
		began = r.began
	}
	return began.Add(-w)
}

// evidence is what a failure may save, by the name and the extension of each.
var evidence = []struct{ name, ext string }{{"failure", "jpg"}, {"failure", "png"}, {"screen", "txt"}, {"logcat", "txt"}, {"requests", "txt"}, {"requests", "json"}}

// FolderName is a flow's, for what is saved of it.
func FolderName(name string) string { return unsafe.ReplaceAllString(name, "_") }

var unsafe = regexp.MustCompile(`[^\w.-]+`)
