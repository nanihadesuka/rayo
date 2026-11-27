package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/nanihadesuka/rayo/internal/schema"
	"github.com/nanihadesuka/rayo/internal/suite"
)

func names[T any](prefix string, fields []schema.Field[T]) []string {
	var list []string
	for _, f := range fields {
		list = append(list, prefix+f.Name)
	}
	return list
}

// suiteOnly are the keys of a suite without an option, and why.
var suiteOnly = map[string]string{
	"name":     "the suite's own name, shown in the results",
	"flows":    "the flows are the arguments",
	"app":      "an install, which has no option",
	"device":   "how the device is set up, which has no option",
	"unmocked": "--ignore-unmocked sets unmocked.ignore",
	"capture":  "each of its keys has an option",
	"output":   "each of its keys has an option",
}

// optionOnly are options of test and check that a suite can't say, and why.
var optionOnly = map[string]string{
	"ignore-lock": "of one run", "device": "which device is the machine's", "emulator": "which device is the machine's",
	"changed-only": "of one run", "since": "of one run", "only": "of one run", "include-tags": "of one run",
	"exclude-tags": "of one run", "rerun-failed": "of one run", "shard": "of one machine of several",
	"mock-port": "the app's own, as built", "animations": "of one run", "profile": "of one run",
	"fail-fast": "of one run", "keep-going": "of one run", "shuffle": "of one run", "continuous": "of one run",
	"resume": "of one run", "from": "of one run", "until": "of one run", "mock-state": "of one run",
	"timings": "of one run", "capture": "of one run", "capture-update": "of one run", "check": "of one run",
	"capture-append": "of one run", "on-fail": "someone is there to fix it: of one run", "hold-timeout": "of one run",
}

func TestOptionsAndSuiteKeys(t *testing.T) {
	keys := slices.Concat(names("", suite.Fields), names("output.", suite.OutputFields), names("capture.", suite.CaptureFields),
		names("unmocked.", suite.UnmockedFields))
	given := map[string]bool{}
	for _, s := range sections {
		if !slices.Contains(s.commands, cmdTest) && !slices.Contains(s.commands, cmdCheck) {
			continue
		}
		for _, p := range s.options {
			switch {
			case p.key != "":
				if !slices.Contains(keys, p.key) {
					t.Errorf("--%s overrides %s, which a suite hasn't", p.name, p.key)
				}
				given[p.key] = true
			case optionOnly[p.name] == "":
				t.Errorf("--%s has no key of a suite: give it one in suiteKeys, or a reason in optionOnly", p.name)
			}
		}
	}
	for _, k := range keys {
		if given[k] || suiteOnly[k] != "" || strings.HasPrefix(k, "unmocked.") && k != "unmocked.ignore" {
			continue
		}
		t.Errorf("the suite's %s has no option: add it to suiteKeys, or a reason to suiteOnly", k)
	}
}
