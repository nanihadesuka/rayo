package report

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type junitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	File      string        `xml:"file,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitProblem `xml:"failure,omitempty"`
	Error     *junitProblem `xml:"error,omitempty"`
	Skipped   *junitSkipped `xml:"skipped,omitempty"`
	Flaky     []junitFlaky  `xml:"flakyFailure"`
}

// junitProblem is a failure or an error: the verdict as its message, the cause as its type, so that CI
// pages show them as the headline.
type junitProblem struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// junitFlaky is a try that failed of a flow that passed at last, as Maven Surefire writes it.
type junitFlaky struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:"stackTrace"`
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

// mostText is as much of a text as a report holds: of more, the beginning and the end.
const mostText = 256 << 10

// plain drops what XML 1.0 can't hold, and keeps it under mostText.
func plain(text string) string {
	text = strings.Map(func(r rune) rune {
		switch {
		case r == utf8.RuneError, r < ' ' && r != '\n' && r != '\t', r == 0xFFFE, r == 0xFFFF:
			return -1
		}
		return r
	}, strings.ToValidUTF8(text, ""))
	return bounded(text)
}

// bounded keeps a text under mostText: its beginning and its end, and says how much is left out.
func bounded(text string) string {
	if len(text) <= mostText {
		return text
	}
	head, tail := text[:mostText/2], text[len(text)-mostText/2:]
	head = strings.ToValidUTF8(head, "") // a rune cut in two
	tail = strings.ToValidUTF8(tail, "")
	return fmt.Sprintf("%s\n... %d bytes left out ...\n%s", head, len(text)-len(head)-len(tail), tail)
}

// classes are, for each flow, its folder from the one all flows are in, with dots, as CI pages group them.
func (sum *Summary) classes() map[string]string {
	dirs := map[string]string{}
	root := ""
	for i, o := range sum.Outcomes {
		dir := filepath.Dir(filepath.Clean(o.Flow.Path))
		dirs[o.Flow.Path] = dir
		switch {
		case i == 0:
			root = dir
		default:
			for root != "." && root != dir && !strings.HasPrefix(dir, root+string(filepath.Separator)) {
				parent := filepath.Dir(root)
				if parent == root {
					break
				}
				root = parent
			}
		}
	}
	base := filepath.Base(root)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "rayo"
	}
	classes := map[string]string{}
	for path, dir := range dirs {
		rel, err := filepath.Rel(root, dir)
		switch {
		case err != nil || rel == ".":
			classes[path] = base
		default:
			classes[path] = strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		}
	}
	return classes
}

// relative is a flow's file from where rayo runs, with slashes.
func relative(path string) string {
	if filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
		}
	}
	return filepath.ToSlash(path)
}

func (sum *Summary) junit() ([]byte, error) {
	name := "rayo"
	if sum.Shards > 0 { // each part's file, told apart where CI gathers them
		name = fmt.Sprintf("rayo-shard-%d-of-%d", sum.Shard, sum.Shards)
	}
	s := junitSuite{Name: name, Tests: len(sum.Outcomes), Failures: sum.count(Failed), Errors: sum.count(Interrupted), Skipped: sum.count(NotRun)}
	classes := sum.classes()
	var total time.Duration
	for _, o := range sum.Outcomes {
		took := o.took()
		total += took
		c := junitCase{Name: plain(o.Flow.Name), ClassName: plain(classes[o.Flow.Path]), File: plain(relative(o.Flow.Path)), Time: fmt.Sprintf("%.3f", took.Seconds())}
		switch o.Status {
		case Failed:
			c.Failure = &junitProblem{Text: plain(o.Details(sum.fileDetail()))}
			if t := o.lastFailed(); t != nil {
				c.Failure.Message = plain(o.Verdict(*t))
				c.Failure.Type = string(causeOf(*t))
			}
		case Interrupted:
			where := plain(o.Interruption())
			c.Error = &junitProblem{Message: where, Type: "interrupted", Text: where}
		case NotRun:
			c.Skipped = &junitSkipped{plain(o.Why)}
		case Flaky:
			for _, f := range o.flakyFailures(sum.fileDetail()) {
				c.Flaky = append(c.Flaky, junitFlaky{plain(f.message), f.cause, plain(f.text)})
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
