package main

import (
	"fmt"
	"strings"

	"github.com/nanihadesuka/rayo/internal/flow"
	"github.com/nanihadesuka/rayo/internal/report"
)

// topic is a part of the help that isn't a command: small, and whole on its own.
type topic struct {
	name, about, text string
}

// topics are the tasks, then what is good to know. rayo help all has them after the tables.
var topics = []topic{
	// agents is the page to start from, so it is first, here and in rayo help all. Every rayo <command>
	// and --option it names must exist: TestAgentsNamesExist checks. Once they land, add:
	// rayo explain <run folder|flow> under "Read the cause"; --find <text> next to --screen under
	// "Author forward"; --changed-only (or --since) under "Batch edits".
	{"agents", "start here: the fast, cheap way to use rayo", `For anyone driving rayo, a person or a program. The cheapest step that answers the question first.

Know before you run
  rayo check <flows>, no device and instant: the plan, and every mistake. Fix all it says, then run.

Run the smallest unit
  One flow, not the suite: rayo test --only <name> --retries 0 --fail-fast <suite>.

Read the cause, not the logs
  A failure leads with its verdict: the step, and a cause of test, app or device. --detail brief
  is that alone. Open the recordings, logcat or calls in --output only if the verdict isn't enough.

A failure is information, not a reason to rerun
  device: fix the environment; the same flow on a broken device only costs time.
  app: a crash or a hang of the app, a finding to report.
  test: the flow or the mocks are wrong; fix them. Retry only what a retry can change.

Batch edits, then verify once
  Make all the changes, rayo check, run the narrowest unit, then rayo test --rerun-failed.
  Don't rerun flows that passed to be sure.

Author forward
  rayo live <flow>, then rayo do --screen <command>: one command, see the result, go on from it.
  rayo do .quit ends the session and frees the device.

Stop after a pass

Prefer structured output
  When a program reads: --format json or ndjson, --detail brief, --show failed, --keep failed.

What it costs, roughly
  rayo check        instant, no device
  one flow          seconds to minutes
  a whole suite     many minutes
  a capture         slow; it makes new ids, so diffs are noisy
  rayo live         holds the device until it ends
`},
	{"write-a-flow", "write a flow", `A flow is a header, ---, and a list of commands. Each command waits for what it needs.

  rayo hierarchy --emulator                          what is on screen, and how to refer to it
  rayo live --emulator --app com.example.app f.yaml  type commands, keep those that work; .save writes them
  rayo check f.yaml                                  mistakes, without a device
  rayo test --emulator f.yaml                        run it

Prefer an id to a text: texts change with the language. rayo help flow has the header and the
commands, rayo help <command> what one takes.
`},
	{"debug-a-flow", "debug a failing flow", `A failure says why, the step, where in the file, and what was on screen. Its folder in --output
(default .rayo) has failure.jpg, screen.txt, logcat.txt and requests.txt.

  rayo test --results report.md all.suite.yaml       a short report, for a chat or a mail
  rayo test --emulator --timings f.yaml              how long each step took
  rayo test --emulator --rerun-failed all.suite.yaml  only what failed last time
  rayo live --emulator f.yaml                        run its steps, then try commands where it stopped

The cause is app if the app crashed or hung, device if the device was lost, and else test.
`},
	{"record-mocks", "record mocks from the real backend", `With mock: in the header, rayo answers as the backend. To make the mocks, capture what the real
one answers, once, then keep them up to date:

  rayo test --capture mock --capture-from https://api.example.com f.yaml  write the calls as mocks
  rayo test --capture-update mock f.yaml                                  say what the backend changed
  rayo mocks apply mock                                                   take the changes, or --only some
  rayo mocks import capture.har mock                                      or begin from a HAR file

With no code in the app, --network proxy and rayo ca export do it through the device's proxy.
A suite's capture: says how to capture, so the commands need no options.
`},
	{"run-in-ci", "run in a CI", `Any CI with an emulator. The exit code isn't 0 if a flow fails, and .xml results are JUnit.

  rayo check all.suite.yaml                                fail early, without a device
  rayo test --emulator --results junit.xml all.suite.yaml  run, and write the results
  rayo test --emulator --retries 2 all.suite.yaml          try a failing flow again

Keep the --output folder as an artifact: it has the failures.
`},
	{"limits", "limitations and gotchas", `- Android only, one device per run; it needs adb, and nothing installed on the device.
- Mocks reach only the calls of the app's own client, through its interceptor, or those through
  the device's proxy with --network proxy. A third-party SDK with its own client isn't mocked.
- With the proxy, the app must trust rayo's CA in its debug build; a release build ignores it.
  A Custom Tab or the browser trusts only the system's CAs: its hosts are passed untouched.
- A call that isn't mocked fails the flow, unless it is unmocked: in the suite or --ignore-unmocked.
- What WireMock can do and rayo can't, such as webhooks or proxying, is an error when the mocks
  load, not a mock that does less.
- Flows from Maestro mostly run as they are. A command rayo doesn't know is an error, said by
  rayo check; flows that expect MAESTRO_FILENAME need it set, such as in a suite's env.
- A failure is app or device only with proof of it; something slow is test.
`},
	{"exit-codes", "exit codes", exitCodes()},
}

// helpTopic is a topic's text, with its title; empty if there is none of that name.
func helpTopic(name string) string {
	for _, t := range topics {
		if t.name == name {
			return t.about + ":\n\n" + t.text
		}
	}
	return ""
}

// helpAll is all the help, on one page: each part begins with "## ".
func helpAll() (string, error) {
	var b strings.Builder
	part := func(title, text string) {
		fmt.Fprintf(&b, "## %s\n\n%s\n", title, strings.TrimRight(text, "\n"))
		b.WriteString("\n")
	}
	part(topics[0].about, topics[0].text)
	part("commands and options", help())
	for _, name := range []string{"flow", "suite"} {
		text, err := helpOn(name)
		if err != nil {
			return "", err
		}
		part(name, text)
	}
	for _, name := range flow.Commands() {
		text, err := flow.Help(name)
		if err != nil {
			return "", err
		}
		part(name, text)
	}
	for _, t := range topics[1:] {
		part(t.about, t.text)
	}
	return b.String(), nil
}

// exitCodes is the topic of the codes, from report.Exits, the one place they are said.
func exitCodes() string {
	var b strings.Builder
	for _, e := range report.Exits {
		fmt.Fprintf(&b, "  %-4d %-14s %s\n", e.Code, e.Name, strings.ReplaceAll(e.What, "`", ""))
	}
	return b.String()
}
