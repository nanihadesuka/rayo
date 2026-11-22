package main

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type junitCase struct {
	Name    string        `xml:"name,attr"`
	File    string        `xml:"file,attr"`
	Time    string        `xml:"time,attr"`
	Failure *string       `xml:"failure,omitempty"`
	Error   *string       `xml:"error,omitempty"`
	Skipped *junitSkipped `xml:"skipped,omitempty"`
	Out     *string       `xml:"system-out,omitempty"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

// plain drops what XML can't hold.
func plain(text string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
}

func writeJUnit(path string, sum *summary) error {
	s := junitSuite{Name: "rayo", Tests: len(sum.outcomes), Failures: sum.count(failed), Errors: sum.count(interrupted), Skipped: sum.count(notRun)}
	var total time.Duration
	for _, o := range sum.outcomes {
		took := o.took()
		total += took
		c := junitCase{Name: o.flow.Name, File: filepath.ToSlash(o.flow.Path), Time: fmt.Sprintf("%.3f", took.Seconds())}
		text := plain(o.details(true))
		switch o.status {
		case failed:
			c.Failure = &text
		case interrupted:
			text = "interrupted"
			c.Error = &text
		case notRun:
			c.Skipped = &junitSkipped{o.why}
		case flaky:
			text = "flaky\n" + text
			c.Out = &text
		}
		s.Cases = append(s.Cases, c)
	}
	s.Time = fmt.Sprintf("%.3f", total.Seconds())
	data, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append([]byte(xml.Header), data...))
}
