package report

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

func (sum *Summary) junit() ([]byte, error) {
	s := junitSuite{Name: "rayo", Tests: len(sum.Outcomes), Failures: sum.count(Failed), Errors: sum.count(Interrupted), Skipped: sum.count(NotRun)}
	var total time.Duration
	for _, o := range sum.Outcomes {
		took := o.took()
		total += took
		c := junitCase{Name: o.Flow.Name, File: filepath.ToSlash(o.Flow.Path), Time: fmt.Sprintf("%.3f", took.Seconds())}
		text := plain(o.Details(true))
		switch o.Status {
		case Failed:
			c.Failure = &text
		case Interrupted:
			text = "interrupted"
			c.Error = &text
		case NotRun:
			c.Skipped = &junitSkipped{o.Why}
		case Passed:
			if o.TimesFailed() > 0 {
				text = plain(o.Line() + "\n" + o.Details(true))
				c.Out = &text
			}
		}
		s.Cases = append(s.Cases, c)
	}
	s.Time = fmt.Sprintf("%.3f", total.Seconds())
	data, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
