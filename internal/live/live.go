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
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
	"github.com/nanihadesuka/rayo/internal/report"
)

const liveHelp = `  - tapOn: { id: save }   any command, or a list; what passes is kept as a step
  .screen                 what is on screen, and how to refer to it
  .steps                  the steps kept
  .undo                   forget the last step; the device stays as it is
  .save                   add the steps to the flow's file
  .help
  .quit                   end, without saving
`

// Reply is how a request went. Every key is always there.
type Reply struct {
	OK       bool            `json:"ok"`
	Output   string          `json:"output"`
	Error    string          `json:"error"`
	Failure  *report.Failure `json:"failure"`
	Warnings []string        `json:"warnings"`
	Calls    []mock.Call     `json:"calls"`
	Screen   []string        `json:"screen"` // if asked for
	Unsaved  int             `json:"unsaved"`
}

// Request is what is asked of a session.
type Request struct {
	Text   string `json:"text"`
	Screen bool   `json:"screen"`
	answer chan Reply
}

// live is a flow written as it runs.
type live struct {
	path    string
	isNew   bool
	header  string // of a new one
	unsaved []string
	session *flow.Session
	out     *liveOutput
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
	return runner.Live(ctx, f, filepath.Join(p.Output, "live"), func(session *flow.Session) error {
		l.session = session
		return l.talk(ctx, p, f)
	})
}

const idle = time.Hour // until a session no one uses ends

func (l *live) talk(ctx context.Context, p Params, f *flow.Flow) error {
	requests := make(chan Request)
	closeServer, err := listen(p.Output, requests)
	if err != nil {
		return err
	}
	defer closeServer()
	if len(f.Steps) > 0 && !p.NoRun {
		fmt.Print(l.do(ctx, f.Steps, nil, false).Text())
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
	text := strings.TrimSpace(q.Text)
	if strings.HasPrefix(text, ".") {
		out, err := l.meta(ctx, text)
		a := l.reply()
		a.Output = out
		if err != nil {
			a.OK, a.Error = false, err.Error()
		}
		return a
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
	return l.do(ctx, steps, texts, q.Screen)
}

func (l *live) reply() Reply {
	return Reply{OK: true, Warnings: []string{}, Calls: []mock.Call{}, Screen: []string{}, Unsaved: len(l.unsaved)}
}

// do runs steps, and keeps those that pass if they have a text.
func (l *live) do(ctx context.Context, steps []*flow.Step, texts []string, screen bool) Reply {
	d := l.session.Do(ctx, steps)
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
	if screen && l.session.Dead() == nil {
		out, err := l.session.Screen(ctx)
		if err != nil {
			a.Warnings = append(a.Warnings, "couldn't read the screen: "+err.Error())
		}
		a.Screen = report.Lines(out)
	}
	return a
}

func (l *live) meta(ctx context.Context, text string) (string, error) {
	switch text {
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
	case ".undo":
		if len(l.unsaved) == 0 {
			return "", errors.New("no steps to forget")
		}
		last := l.unsaved[len(l.unsaved)-1]
		l.unsaved = l.unsaved[:len(l.unsaved)-1]
		return "forgot " + last, nil
	case ".save":
		return l.save()
	}
	return "", fmt.Errorf("unknown %s, see .help", text)
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
	if _, err := flow.Parse(l.path, data); err != nil {
		return "", fmt.Errorf("can't add to the file: %w", err)
	}
	if err := report.WriteFile(l.path, data); err != nil {
		return "", err
	}
	n := len(l.unsaved)
	l.isNew, l.unsaved = false, nil
	return fmt.Sprintf("saved %d steps in %s\n", n, filepath.ToSlash(l.path)), nil
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
