package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
)

// console prints what happens: each step if verbose, or else a line for each flow.
type console struct {
	mu       sync.Mutex // a note of a slow step comes from elsewhere
	verbose  bool       // every step
	detail   string     // of a failure
	timings  bool
	steps    []flow.StepReport // of the flow running now, with timings
	terminal bool              // someone is looking, and not a file
	pending  int               // length of a line to write over
	out      io.Writer         // nowhere, if a program reads what is printed
}

func newConsole(out io.Writer) *console {
	info, err := os.Stdout.Stat()
	return &console{out: out, terminal: out == os.Stdout && err == nil && info.Mode()&os.ModeCharDevice != 0}
}

// Step prints one line.
func (c *console) Step(s flow.StepReport) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timings && (s.Status == flow.Passed || s.Status == flow.Failed || s.Status == flow.Optional) {
		c.steps = append(c.steps, s)
	}
	if c.verbose {
		fmt.Fprint(c.out, s.Line())
	}
}

// Note prints something else worth knowing.
func (c *console) Note(depth int, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.verbose {
		fmt.Fprintln(c.out, strings.Repeat("  ", depth)+text)
	}
}

func (c *console) begin(i, of int, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps = nil
	line := fmt.Sprintf("[%d/%d] %s", i, of, name)
	switch {
	case c.verbose:
		fmt.Fprintf(c.out, "\n%s\n", line)
	case c.terminal:
		c.pending = len(line)
		fmt.Fprint(c.out, line)
	}
}

func (c *console) end(i, of int, o *report.Outcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.slowestSteps()
	line := fmt.Sprintf("[%d/%d] %s", i, of, o.Line())
	if c.pending > 0 {
		fmt.Fprint(c.out, "\r"+strings.Repeat(" ", c.pending)+"\r")
		c.pending = 0
	}
	fmt.Fprintln(c.out, line)
	if o.Passed() {
		fmt.Fprint(c.out, o.Notes()+o.WarningLines())
		return
	}
	fmt.Fprint(c.out, o.Details(c.detail))
}

// slowestSteps prints, with --timings, the steps of the flow that took longest.
func (c *console) slowestSteps() {
	if !c.timings || len(c.steps) == 0 {
		return
	}
	steps := slices.Clone(c.steps)
	slices.SortStableFunc(steps, func(a, b flow.StepReport) int { return int(b.Took - a.Took) })
	var total time.Duration
	for _, s := range c.steps {
		total += s.Took
	}
	fmt.Fprintf(c.out, "  slowest steps, of %d in %.1f s:\n", len(c.steps), total.Seconds())
	for _, s := range steps[:min(5, len(steps))] {
		fmt.Fprintf(c.out, "    %6d ms  %s\n", s.Took.Milliseconds(), s.Text)
	}
	for _, s := range c.steps {
		// It passed, but close to its limit: a slower day and it fails.
		if s.Status == flow.Passed && s.Limit > time.Second && s.Took*10 > s.Limit*7 {
			fmt.Fprintf(c.out, "  tight: %s took %d ms of its %d ms timeout\n", s.Text, s.Took.Milliseconds(), s.Limit.Milliseconds())
		}
	}
	c.steps = nil
}

// slowestFlows prints, with --timings, the flows that took longest.
func (c *console) slowestFlows(outcomes []report.Outcome) {
	if !c.timings || len(outcomes) < 2 {
		return
	}
	ran := []report.Outcome{}
	for _, o := range outcomes {
		if o.Status != report.NotRun {
			ran = append(ran, o)
		}
	}
	slices.SortStableFunc(ran, func(a, b report.Outcome) int { return int(b.Took() - a.Took()) })
	fmt.Fprintln(c.out, "\nslowest flows:")
	for _, o := range ran[:min(10, len(ran))] {
		fmt.Fprintf(c.out, "  %6.1f s  %s\n", o.Took().Seconds(), o.Flow.Name)
	}
}
