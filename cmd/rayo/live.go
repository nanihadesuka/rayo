package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/mock"
)

const liveHelp = `  - tapOn: { id: save }   any command, or a list; what passes is kept as a step
  .screen                 what is on screen, and how to refer to it
  .steps                  the steps kept
  .undo                   forget the last step; the device stays as it is
  .save                   add the steps to the flow's file
  .help
  .quit                   end, without saving
`

// reply is how a request went. Every key is always there.
type reply struct {
	OK       bool         `json:"ok"`
	Output   string       `json:"output"`
	Error    string       `json:"error"`
	Failure  *jsonFailure `json:"failure"`
	Warnings []string     `json:"warnings"`
	Calls    []mock.Call  `json:"calls"`
	Screen   []string     `json:"screen"` // if asked for
	Unsaved  int          `json:"unsaved"`
}

type request struct {
	Text   string `json:"text"`
	Screen bool   `json:"screen"`
	answer chan reply
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

func (o *liveOutput) Step(s flow.StepReport) { o.b.WriteString(stepLine(s)) }
func (o *liveOutput) Note(depth int, text string) {
	fmt.Fprintln(&o.b, strings.Repeat("  ", depth)+text)
}
func (o *liveOutput) take() string { s := o.b.String(); o.b.Reset(); return s }

func runLive(ctx context.Context, args []string) error {
	o, err := parse(args)
	if err != nil {
		return err
	}
	if o.runner.Capture != "" {
		return errors.New("a live session can't capture: save the flow, then rayo test --capture")
	}
	if len(o.paths) != 1 {
		return errors.New("which flow? give a file, there already or not")
	}
	if o.runner.Out == "" {
		o.runner.Out = ".rayo"
	}
	l := &live{path: o.paths[0], out: &liveOutput{}}
	data, err := os.ReadFile(l.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if o.app == "" {
			return fmt.Errorf("%s is new: say which app with --app", filepath.ToSlash(l.path))
		}
		l.isNew, l.header = true, flow.NewFlow(o.app, o.mocks)
		data = []byte(l.header + "[]\n")
	case err != nil:
		return err
	case o.app != "" || o.mocks != nil:
		return fmt.Errorf("%s says its app and mocks", filepath.ToSlash(l.path))
	}
	f, err := flow.Parse(l.path, data)
	if err != nil {
		return err
	}
	if _, err := ask(o, request{Text: ".help"}); err == nil {
		return errors.New("a session is live already, see rayo do .help; or use another --debug-output")
	}
	d, err := adb.Pick(o.device, o.emulator)
	if err != nil {
		return err
	}
	s := &session{o: o, device: d, reporter: l.out}
	if err := s.start(); err != nil {
		return err
	}
	defer func() { s.agent.Close() }()
	return s.runner.Live(ctx, f, filepath.Join(o.runner.Out, "live"), func(session *flow.Session) error {
		l.session = session
		return l.talk(ctx, o, f)
	})
}

const idle = time.Hour // until a session no one uses ends

func (l *live) talk(ctx context.Context, o *options, f *flow.Flow) error {
	requests := make(chan request)
	closeServer, err := listen(o, requests)
	if err != nil {
		return err
	}
	defer closeServer()
	if len(f.Steps) > 0 && !o.noRun {
		fmt.Print(l.do(ctx, f.Steps, nil, false).text())
		if err := l.session.Dead(); err != nil {
			return err
		}
	}
	fmt.Println("live: type commands, or send them with rayo do; .help")
	typed := make(chan string)
	if !o.serve {
		go func() {
			defer close(typed)
			readCommands(os.Stdin, func(text string) { typed <- text })
		}()
	} else {
		typed = nil
	}
	for {
		var q request
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
		fmt.Print(a.text())
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

func (l *live) handle(ctx context.Context, q request) reply {
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

func (l *live) reply() reply {
	return reply{OK: true, Warnings: []string{}, Calls: []mock.Call{}, Screen: []string{}, Unsaved: len(l.unsaved)}
}

// do runs steps, and keeps those that pass if they have a text.
func (l *live) do(ctx context.Context, steps []*flow.Step, texts []string, screen bool) reply {
	d := l.session.Do(ctx, steps)
	if texts != nil {
		l.unsaved = append(l.unsaved, texts[:d.Passed]...)
	}
	a := l.reply()
	a.Output, a.Warnings, a.Calls = l.out.take(), list(d.Warnings), list(d.Calls)
	if d.Err != nil {
		a.OK, a.Error = false, d.Err.Error()
		x := failureOf(d.Err, d.Failure)
		a.Failure = &x
	}
	if screen && l.session.Dead() == nil {
		out, err := l.session.Screen(ctx)
		if err != nil {
			a.Warnings = append(a.Warnings, "couldn't read the screen: "+err.Error())
		}
		a.Screen = linesOf(out)
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
	if err := writeFile(l.path, data); err != nil {
		return "", err
	}
	n := len(l.unsaved)
	l.isNew, l.unsaved = false, nil
	return fmt.Sprintf("saved %d steps in %s\n", n, filepath.ToSlash(l.path)), nil
}

func (a reply) text() string {
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

// --- From another process ---

// door is where a session is, and what proves that one may use it.
type door struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

func doorFile(o *options) string { return filepath.Join(o.runner.Out, "live.json") }

const tokenHeader = "X-Rayo-Token"

// listen takes requests of this computer that have the token, which only this user can read.
func listen(o *options, requests chan<- request) (func(), error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, errors.Join(err, l.Close())
	}
	here := door{l.Addr().(*net.TCPAddr).Port, hex.EncodeToString(secret)}
	data, err := json.Marshal(here)
	if err == nil {
		err = os.MkdirAll(o.runner.Out, 0o755)
	}
	if err == nil {
		_ = os.Remove(doorFile(o)) // of a session that didn't end well; it may have other permissions
		err = os.WriteFile(doorFile(o), data, 0o600)
	}
	if err != nil {
		return nil, errors.Join(err, l.Close())
	}
	done := make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.Method != http.MethodPost || q.Header.Get(tokenHeader) != here.Token {
			http.Error(w, "not allowed", http.StatusForbidden)
			return
		}
		r := request{answer: make(chan reply, 1)}
		if err := json.NewDecoder(io.LimitReader(q.Body, 10<<20)).Decode(&r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case requests <- r:
		case <-done:
			http.Error(w, "the session is over", http.StatusGone)
			return
		}
		select {
		case a := <-r.answer:
			_ = json.NewEncoder(w).Encode(a) // whoever asked may have gone
		case <-done:
			http.Error(w, "the session is over", http.StatusGone)
		}
	})}
	go func() { _ = server.Serve(l) }()
	return func() {
		_ = os.Remove(doorFile(o))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		time.AfterFunc(200*time.Millisecond, func() { close(done) }) // the last answer goes out first
		_ = server.Shutdown(ctx)
	}, nil
}

func ask(o *options, q request) (*reply, error) {
	none := errors.New("no session is live here: start one with rayo live")
	data, err := os.ReadFile(doorFile(o))
	if err != nil {
		return nil, none
	}
	var there door
	if err := json.Unmarshal(data, &there); err != nil {
		return nil, none
	}
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	out, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/", there.Port), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	out.Header.Set(tokenHeader, there.Token)
	res, err := http.DefaultClient.Do(out)
	if err != nil {
		return nil, none
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(io.LimitReader(res.Body, 1000))
		return nil, errors.New(strings.TrimSpace(string(text)))
	}
	a := &reply{}
	return a, json.NewDecoder(res.Body).Decode(a)
}

// errShown is a failure that has been printed.
var errShown = errors.New("")

// runDo sends commands to the live session.
func runDo(args []string) error {
	o := &options{}
	o.runner.Out = ".rayo"
	var parts []string
	q, asJSON := request{}, false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--screen":
			q.Screen = true
		case "--json":
			asJSON = true
		case "--debug-output":
			if i++; i >= len(args) {
				return fmt.Errorf("%s needs a value", a)
			}
			o.runner.Out = args[i]
		case "-":
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			parts = append(parts, string(data))
		default:
			if strings.HasPrefix(a, "--") {
				return fmt.Errorf("unknown option %s", a)
			}
			parts = append(parts, a)
		}
	}
	if len(parts) == 0 {
		return errors.New("do what? such as rayo do '- tapOn: Save', or rayo do .help")
	}
	q.Text = strings.Join(parts, "\n")
	a, err := ask(o, q)
	if err != nil {
		return err
	}
	if asJSON {
		e := json.NewEncoder(os.Stdout)
		e.SetEscapeHTML(false)
		e.SetIndent("", "  ")
		err = e.Encode(a)
	} else {
		fmt.Print(a.text())
	}
	if err == nil && !a.OK {
		err = errShown
	}
	return err
}
