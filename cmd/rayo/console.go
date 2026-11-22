package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
)

// console prints what happens: each step if verbose, or else a line for each flow.
type console struct {
	verbose  bool
	terminal bool // someone is looking, and not a file
	pending  int  // length of a line to write over
}

func newConsole() *console {
	info, err := os.Stdout.Stat()
	return &console{terminal: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

// Step prints one line.
func (c *console) Step(s flow.StepReport) {
	if c.verbose {
		fmt.Print(s.Line())
	}
}

// Note prints something else worth knowing.
func (c *console) Note(depth int, text string) {
	if c.verbose {
		fmt.Println(strings.Repeat("  ", depth) + text)
	}
}

func (c *console) begin(i, of int, name string) {
	line := fmt.Sprintf("[%d/%d] %s", i, of, name)
	switch {
	case c.verbose:
		fmt.Printf("\n%s\n", line)
	case c.terminal:
		c.pending = len(line)
		fmt.Print(line)
	}
}

func (c *console) end(i, of int, o *report.Outcome) {
	line := fmt.Sprintf("[%d/%d] %s", i, of, o.Line())
	if c.pending > 0 {
		fmt.Print("\r" + strings.Repeat(" ", c.pending) + "\r")
		c.pending = 0
	}
	fmt.Println(line)
	if o.Status == report.Passed {
		fmt.Print(o.Notes() + o.WarningLines())
		return
	}
	fmt.Print(o.Details(!c.verbose)) // verbose has shown the calls, and the screen is long
}
