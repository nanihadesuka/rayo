// Package live runs a flow while it is written, command by command.
package live

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
)

// Help is what a session takes, as .help says it.
const Help = liveHelp

const liveHelp = `  - tapOn: { id: save }   any command, or a list; what passes is kept as a step
  .screen                 what is on screen, and how to refer to it
  .steps                  the steps kept
  .states                 the mocks' scenarios and the state of each; mockState: { booking: booked } sets one
  .undo                   forget the last step; the device stays as it is
  .save                   add the steps to the flow's file
  .retry                  read the flow's file again, and run its steps from the one that failed
  .from <n>               the same, from its n-th step
  .rewind [n]             forget the last n steps done (1), and the calls they made to a capture;
                          move the app back to where it was by hand
  .help
  .quit                   end, without saving
`

// Reply is how a request went. Every key is always there.
type Reply struct {
	SchemaVersion int             `json:"schemaVersion" about:"Changes if the meaning of a key does"`
	OK            bool            `json:"ok" about:"The steps passed"`
	Output        string          `json:"output" about:"What rayo do prints, without --format"`
	Error         string          `json:"error" about:"Of steps that failed, or of a command"`
	Failure       *report.Failure `json:"failure" about:"Of a step that failed, as in the results"`
	Warnings      []string        `json:"warnings" about:"Such as calls without a mock"`
	Calls         []mock.Call     `json:"calls" about:"To the mock meanwhile"`
	Screen        []string        `json:"screen" about:"With --screen, or after a failure. With --screen=changes, only the lines that appeared, as '+ line', and those that went, as '- line', since the screen the session last gave; none if it is the same"`
	Unsaved       int             `json:"unsaved" about:"Steps not saved to the flow yet"`
}

// Request is what is asked of a session.
type Request struct {
	Text    string `json:"text"`
	Screen  bool   `json:"screen"`
	Changes bool   `json:"changes"` // of the screen, only what changed
	answer  chan Reply
}

// live is a flow written as it runs.
type live struct {
	path    string
	isNew   bool
	header  string // of a new one
	unsaved []string
	session *flow.Session
	out     *liveOutput
	done    []done   // the steps that passed, in order
	next    int      // the first of the file's steps not done
	seen    []string // the screen it last gave, for --screen=changes; nil before the first
}

// done is a step that passed.
type done struct {
	text string // to show
	file int    // its place among the file's steps; -1 if typed
	mark int    // what Captured was before it
}

// liveOutput keeps what a request prints.
type liveOutput struct{ b strings.Builder }

func (o *liveOutput) Step(s flow.StepReport) { o.b.WriteString(s.Line()) }
func (o *liveOutput) Note(depth int, text string) {
	fmt.Fprintln(&o.b, strings.Repeat("  ", depth)+text)
}
func (o *liveOutput) take() string { s := o.b.String(); o.b.Reset(); return s }

// Params of a session.
type Params struct {
	Path   string // of the flow
	App    string // of a new one
	Mocks  []string
	Output string // folder for failures, and to be found by Ask
	NoRun  bool   // not the steps it has
	Serve  bool   // only for Ask
}

// Start prepares the device; what it returns last ends it.
type Start func(reporter flow.Reporter) (*flow.Runner, func(), error)

// Run is a whole session.
func Run(ctx context.Context, p Params, start Start) error {
	l := &live{path: p.Path, out: &liveOutput{}}
	data, err := os.ReadFile(l.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if p.App == "" {
			return fmt.Errorf("%s is new: say which app with --app", filepath.ToSlash(l.path))
		}
		l.isNew, l.header = true, flow.NewFlow(p.App, p.Mocks)
		data = []byte(l.header + "[]\n")
	case err != nil:
		return err
	case p.App != "" || p.Mocks != nil:
		return fmt.Errorf("%s says its app and mocks", filepath.ToSlash(l.path))
	}
	f, err := flow.Parse(l.path, data)
	if err != nil {
		return err
	}
	if _, err := Ask(p.Output, Request{Text: ".help"}); err == nil {
		return errors.New("a session is live already, see rayo do .help; or use another --output")
	}
	runner, end, err := start(l.out)
	if err != nil {
		return err
	}
	defer end()
	err = runner.Live(ctx, f, flow.Attempt{Out: p.Output, Flow: "live"}, func(session *flow.Session) error {
		l.session = session
		return l.talk(ctx, p, f)
	})
	fmt.Print(l.out.take()) // what was said as it ended, such as of a capture
	return err
}

const idle = time.Hour // until a session no one uses ends

func (l *live) talk(ctx context.Context, p Params, f *flow.Flow) error {
	requests := make(chan Request)
	closeServer, err := listen(p.Output, requests)
	if err != nil {
		return err
	}
	defer closeServer()
	if p.NoRun {
		l.next = len(f.Steps)
	}
	if len(f.Steps) > 0 && !p.NoRun {
		fmt.Print(l.do(ctx, f.Steps, nil, Request{}).Text())
		if err := l.session.Dead(); err != nil {
			return err
		}
	}
	fmt.Println("live: type commands, or send them with rayo do; .help")
	typed := make(chan string)
	if !p.Serve {
		go func() {
			defer close(typed)
			readCommands(os.Stdin, func(text string) { typed <- text })
		}()
	} else {
		typed = nil
	}
	for {
		var q Request
		select {
		case text, more := <-typed:
			if !more { // the end of what is typed
				text = ".quit"
			}
			q.Text = text
		case q = <-requests:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(idle):
			fmt.Println("no one has used it for an hour")
			return nil
		}
		quit := strings.TrimSpace(q.Text) == ".quit"
		a := l.handle(ctx, q)
		fmt.Print(a.Text())
		if q.answer != nil {
			q.answer <- a
		}
		if quit {
			return nil
		}
		if err := l.session.Dead(); err != nil {
			return err
		}
	}
}

// readCommands gives each command: a line, or if it ends in ":" all until an empty one.
func readCommands(in io.Reader, give func(text string)) {
	var block []string
	end := func() {
		if len(block) > 0 {
			give(strings.Join(block, "\n"))
		}
		block = nil
	}
	lines := bufio.NewScanner(in)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		line := strings.TrimRight(lines.Text(), " \t\r")
		switch {
		case line == "":
			end()
		case len(block) > 0 || strings.HasSuffix(line, ":"):
			block = append(block, line)
		default:
			give(line)
		}
	}
	end()
}

func (l *live) handle(ctx context.Context, q Request) Reply {
	if text := strings.TrimSpace(q.Text); strings.HasPrefix(text, ".") {
		return l.command(ctx, text, q)
	}
	written, err := flow.ParseSteps(l.path, q.Text)
	if err != nil {
		a := l.reply()
		a.OK, a.Error = false, err.Error()
		return a
	}
	var steps []*flow.Step
	var texts []string
	for _, w := range written {
		steps, texts = append(steps, w.Step), append(texts, w.Text)
	}
	return l.do(ctx, steps, texts, q)
}

func (l *live) reply() Reply {
	return Reply{SchemaVersion: report.SchemaVersion, OK: true, Warnings: []string{}, Calls: []mock.Call{}, Screen: []string{}, Unsaved: len(l.unsaved)}
}

// do runs steps, and keeps those that pass if they have a text.
func (l *live) do(ctx context.Context, steps []*flow.Step, texts []string, q Request) Reply {
	d := l.session.Do(ctx, steps)
	for i := range d.Passed {
		k := done{text: "- " + steps[i].Text() + "\n", file: -1, mark: d.Marks[i]}
		if texts != nil {
			k.text = texts[i]
		} else {
			k.file = l.next
			l.next++
		}
		l.done = append(l.done, k)
	}
	if texts != nil {
		l.unsaved = append(l.unsaved, texts[:d.Passed]...)
	}
	a := l.reply()
	a.Output, a.Warnings, a.Calls = l.out.take(), report.List(d.Warnings), report.List(d.Calls)
	if d.Err != nil {
		a.OK, a.Error = false, d.Err.Error()
		x := report.FailureOf(d.Err, d.Failure)
		a.Failure = &x
	}
	if q.Screen && l.session.Dead() == nil {
		out, err := l.session.Screen(ctx)
		if err != nil {
			a.Warnings = append(a.Warnings, "couldn't read the screen: "+err.Error())
		}
		a.Screen = report.Lines(out)
		if err == nil {
			if q.Changes {
				a.Screen = changes(l.seen, a.Screen)
			}
			l.seen = report.Lines(out)
		}
	}
	return a
}

func (l *live) failed(err error) Reply {
	a := l.reply()
	a.OK, a.Error = false, err.Error()
	return a
}

// command is one of those that begin with a dot.
func (l *live) command(ctx context.Context, text string, q Request) Reply {
	name, arg, _ := strings.Cut(text, " ")
	arg = strings.TrimSpace(arg)
	if name == ".retry" || name == ".from" { // they run steps, and answer as steps do
		steps, err := l.fileSteps(name, arg)
		if err != nil {
			return l.failed(err)
		}
		return l.do(ctx, steps, nil, q)
	}
	out, err := l.meta(ctx, name, arg)
	if err != nil {
		a := l.failed(err)
		a.Output = out
		return a
	}
	a := l.reply()
	a.Output = out
	return a
}

// meta is a command that only answers with text.
func (l *live) meta(ctx context.Context, name, arg string) (string, error) {
	if name == ".rewind" {
		return l.rewind(arg)
	}
	if arg != "" {
		return "", fmt.Errorf("%s takes nothing", name)
	}
	switch name {
	case ".help":
		return liveHelp, nil
	case ".quit":
		if n := len(l.unsaved); n > 0 {
			return fmt.Sprintf("%d steps were not saved:\n%s", n, strings.Join(l.unsaved, "")), nil
		}
		return "", nil
	case ".screen":
		return l.session.Screen(ctx)
	case ".steps":
		return strings.Join(l.unsaved, ""), nil
	case ".states":
		return l.session.States()
	case ".undo":
		return l.undo()
	case ".save":
		return l.save()
	}
	return "", fmt.Errorf("unknown %s, see .help", name)
}

// undo forgets the last step typed and not saved, and its calls if none were made after them.
func (l *live) undo() (string, error) {
	if len(l.unsaved) == 0 {
		return "", errors.New("no steps to forget")
	}
	i := l.lastTyped()
	if i == len(l.done)-1 { // its calls are the last ones
		return l.forget(1), nil
	}
	l.done = slices.Delete(l.done, i, i+1) // steps of the file came after it: their calls stay
	text := l.unsaved[len(l.unsaved)-1]
	l.unsaved = l.unsaved[:len(l.unsaved)-1]
	return "forgot " + text, nil
}

// lastTyped is where the newest typed step is in done; while some aren't saved, it is the last of them.
func (l *live) lastTyped() int {
	for i := len(l.done) - 1; i >= 0; i-- {
		if l.done[i].file < 0 {
			return i
		}
	}
	return -1
}

// fileSteps are the file's steps, read again, from the one that failed or the n-th.
func (l *live) fileSteps(name, arg string) ([]*flow.Step, error) {
	if l.isNew {
		return nil, errors.New("the flow isn't in its file yet: .save first")
	}
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, err
	}
	f, err := flow.Parse(l.path, data)
	if err != nil {
		return nil, err
	}
	from := l.next
	if name == ".from" {
		if from, err = strconv.Atoi(arg); err != nil || from < 1 || from > len(f.Steps) {
			return nil, fmt.Errorf(".from takes the number of one of the file's %d steps, not %q", len(f.Steps), arg)
		}
		from--
	} else if arg != "" {
		return nil, errors.New(".retry takes nothing; .from <n> runs from the n-th step")
	}
	if from >= len(f.Steps) {
		return nil, fmt.Errorf("all the %d steps of the file are done; .from <n> runs some again", len(f.Steps))
	}
	l.next = from
	return f.Steps[from:], nil
}

// rewind forgets the last n steps done, and the calls they made.
func (l *live) rewind(arg string) (string, error) {
	n := 1
	if arg != "" {
		var err error
		if n, err = strconv.Atoi(arg); err != nil || n < 1 {
			return "", fmt.Errorf(".rewind takes how many steps, not %q", arg)
		}
	}
	if n > len(l.done) {
		return "", fmt.Errorf("only %d steps have been done", len(l.done))
	}
	return l.forget(n) + "the app stays as it is: move it back by hand; .retry runs the file's steps from there\n", nil
}

// forget forgets the last n steps done, and the calls made since the first of them.
func (l *live) forget(n int) string {
	gone := l.done[len(l.done)-n:]
	l.done = l.done[:len(l.done)-n]
	dropped := l.session.Drop(gone[0].mark)
	var b strings.Builder
	for i := len(gone) - 1; i >= 0; i-- {
		g := gone[i]
		if g.file >= 0 {
			l.next = min(l.next, g.file)
		} else if len(l.unsaved) > 0 {
			l.unsaved = l.unsaved[:len(l.unsaved)-1]
		}
		b.WriteString("forgot ")
		b.WriteString(g.text)
	}
	if l.session.Capturing() {
		fmt.Fprintf(&b, "dropped %d calls from the capture\n", dropped)
	}
	return b.String()
}

// save adds the steps to the file, which stays as it was if the result isn't a flow.
func (l *live) save() (string, error) {
	if len(l.unsaved) == 0 {
		return "", errors.New("no steps to save")
	}
	data := []byte(l.header)
	if !l.isNew {
		var err error
		if data, err = os.ReadFile(l.path); err != nil {
			return "", err
		}
		if !bytes.HasSuffix(data, []byte("\n")) {
			data = append(data, '\n')
		}
	} else if _, err := os.Stat(l.path); err == nil {
		return "", fmt.Errorf("%s is there by now", filepath.ToSlash(l.path))
	}
	data = append(data, strings.Join(l.unsaved, "")...)
	f, err := flow.Parse(l.path, data)
	if err != nil {
		return "", fmt.Errorf("can't add to the file: %w", err)
	}
	if err := report.WriteFile(l.path, data); err != nil {
		return "", err
	}
	n := len(l.unsaved)
	if l.next == len(f.Steps)-n { // all the file's steps before them are done
		l.nowInFile(n, len(f.Steps))
	}
	l.isNew, l.unsaved = false, nil
	return fmt.Sprintf("saved %d steps in %s\n", n, filepath.ToSlash(l.path)), nil
}

// nowInFile makes the newest n typed steps done the file's last ones, which end at end, so that
// .retry and .rewind treat them as such.
func (l *live) nowInFile(n, end int) {
	at := end
	for i := len(l.done) - 1; i >= 0 && n > 0; i-- {
		if l.done[i].file < 0 {
			at--
			l.done[i].file = at
			n--
		}
	}
	l.next = end
}

// Text is how it is printed.
func (a Reply) Text() string {
	var b strings.Builder
	b.WriteString(a.Output)
	if a.Error != "" {
		fmt.Fprintf(&b, "error: %s\n", a.Error)
	}
	screen := a.Screen
	if a.Failure != nil {
		if a.Failure.Crash != "" {
			fmt.Fprintf(&b, "crash: %s\n", strings.ReplaceAll(a.Failure.Crash, "\n", "\n  "))
		}
		screen = a.Failure.Screen
	}
	for _, w := range a.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", w)
	}
	if len(screen) > 0 {
		b.WriteString("screen:\n")
	}
	for _, line := range screen {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	return b.String()
}

// changes is the lines of a screen that appeared, as "+ line", in the order they are now, then those
// that went, as "- line", in the order they were; a line there twice counts twice. All appeared if
// there was no screen before.
func changes(before, now []string) []string {
	left := map[string]int{}
	for _, line := range before {
		left[line]++
	}
	list := []string{}
	for _, line := range now {
		if left[line] > 0 {
			left[line]--
		} else {
			list = append(list, "+ "+line)
		}
	}
	for _, line := range before {
		if left[line] > 0 {
			left[line]--
			list = append(list, "- "+line)
		}
	}
	return list
}
