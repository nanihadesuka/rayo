package report

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanihadesuka/rayo/internal/flow"
)

const hostile = "\x1b[31mred\x1b[0m \x00nul ￾￿ bad\xff utf8 & <tag> \"quoted\""

func hostileSummary() *Summary {
	failed := flow.Result{Err: errors.New(hostile), Failure: &flow.Failure{Blame: flow.Test, Screen: hostile, Crash: hostile}}
	passed := flow.Result{}
	s := &Summary{Title: hostile, Device: hostile, Began: time.Now()}
	for i, name := range []string{"a", "b", "c"} {
		o := Outcome{Flow: &flow.Flow{Name: name + hostile, Path: filepath.Join("e2e", name, "flow.yaml")}}
		switch i {
		case 0:
			o.Judge(t0(), failed)
		case 1:
			o.Judge(t0(), failed)
			o.Judge(t0(), passed)
		case 2:
			o.Status, o.Why = NotRun, hostile
		}
		s.Outcomes = append(s.Outcomes, o)
	}
	return s
}

func TestReportsTakeHostileText(t *testing.T) {
	s := hostileSummary()
	data, err := s.junit()
	if err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() != "EOF" {
				t.Fatalf("JUnit doesn't parse: %v\n%s", err, data)
			}
			break
		}
	}
	data, err = s.json()
	if err != nil || !json.Valid(data) {
		t.Fatalf("JSON doesn't parse: %v", err)
	}
	dir := t.TempDir()
	for _, name := range []string{"r.txt", "r.md", "r.json", "r.xml"} {
		if err := s.Write(filepath.Join(dir, name)); err != nil {
			t.Fatal(name, err)
		}
	}
}

func TestPlainBoundsText(t *testing.T) {
	long := strings.Repeat("a", mostText) + "é" + strings.Repeat("z", mostText)
	got := plain(long)
	if len(got) > mostText+100 || !strings.HasPrefix(got, "aaa") || !strings.HasSuffix(got, "zzz") || !strings.Contains(got, "bytes left out") {
		t.Errorf("not bounded: %d bytes", len(got))
	}
	if plain("a\x1bb\tc\n") != "ab\tc\n" {
		t.Errorf("%q", plain("a\x1bb\tc\n"))
	}
}

func TestJUnitHeadlines(t *testing.T) {
	step := &flow.Step{Path: filepath.Join("e2e", "login.yaml"), Line: 12}
	fail := flow.Result{Err: &flow.StepError{Step: step, Err: errors.New("it is false\nmore")}, Failure: &flow.Failure{Cause: flow.StepFailed}}
	o := Outcome{Flow: &flow.Flow{Name: "login", Path: filepath.Join("e2e", "auth", "login.yaml")}}
	o.Judge(t0(), fail)
	other := Outcome{Flow: &flow.Flow{Name: "home", Path: filepath.Join("e2e", "home.yaml")}}
	other.Judge(t0(), flow.Result{})
	s := &Summary{Outcomes: []Outcome{o, other}}
	data, err := s.junit()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`classname="auth" file="e2e/auth/login.yaml"`, `classname="e2e" file="e2e/home.yaml"`,
		`type="step failed"`, `: step failed: it is false (login.yaml:12)"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("no %s in\n%s", want, data)
		}
	}
}

func TestVerdictOutsideAStep(t *testing.T) {
	o := Outcome{Flow: &flow.Flow{Name: "f"}}
	before := o.Verdict(flow.Result{Err: errors.New("boom")})
	after := o.Verdict(flow.Result{Err: errors.New("boom"), Failure: &flow.Failure{Before: []string{"back"}, Cause: "unmocked call"}})
	if before != "f failed before its first step: step failed: boom" || after != "f failed outside a step: unmocked call" {
		t.Errorf("%q, %q", before, after)
	}
}

func TestNotesWithoutBlame(t *testing.T) {
	o := Outcome{Flow: &flow.Flow{Name: "f"}}
	o.Judge(t0(), flow.Result{Err: errors.New("boom"), Failure: &flow.Failure{}})
	o.Judge(t0(), flow.Result{})
	if strings.Contains(o.Notes(), "()") {
		t.Errorf("%q", o.Notes())
	}
}

func TestExitInJSONAndErrors(t *testing.T) {
	s := &Summary{Exit: ExitInterrupted}
	if got := s.summary().Exit; got.Code != 130 || got.Name != "interrupted" {
		t.Errorf("%+v", got)
	}
	if e := ErrorsOf(errors.New("bad"), ExitConfig); e.Exit != ExitConfig || len(e.Errors) == 0 {
		t.Errorf("exit 2 without errors: %+v", e)
	}
}

func t0() context.Context { return context.Background() }
