package main

import (
	"errors"
	"fmt"
	"slices"
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
	// and --option it names must exist: TestAgentsNamesExist checks.
	{"agents", "start here: the fast, cheap way to use rayo", `For anyone driving rayo, a person or a program. The cheapest step that answers the question first.

Know before you run
  rayo check <flows>, no device and instant: the plan, and every mistake. Fix all it says, then run.

Run the smallest unit
  One flow, not the suite: rayo test --only <name> --retries 0 --fail-fast <suite>.

Read the cause, not the logs
  A failure leads with its verdict, <flow> failed at <step>: <cause>, and a blame of test, app or
  device. --detail brief is that alone; rayo explain says it again for the last run, without one.
  Open the recordings, logcat or calls in --output only if the verdict isn't enough.

A failure is information, not a reason to rerun (by its blame)
  device: fix the environment; the same flow on a broken device only costs time.
  app: a crash or a hang of the app, a finding to report.
  test: the flow or the mocks are wrong; fix them. Retry only what a retry can change.

Batch edits, then verify once
  Make all the changes, rayo check --changed-only, run the narrowest unit, then
  rayo test --rerun-failed.
  Don't rerun flows that passed to be sure.

Fix as it fails, not run by run
  rayo test --on-fail hold <suite> stops at a failure with the app there. rayo do --format json
  .why, fix the flow's own file, rayo do .retry: seconds, not a run. .probe 5 tells a flake from a
  bug before an edit; .mark app-bug <why> when it isn't the flow's, and .skip. A fixed flow is
  PARTIAL until rayo test --rerun-failed runs it whole: only that is a pass. Never make it pass
  by checking less: the results say WEAKENED.

Author forward
  rayo live <flow>, then rayo do --screen --find <text> <command>: one command, see only the
  elements that matter, go on from it. rayo do --screen=changes gives only the lines that
  appeared (+) or went (-) since the last screen: after - waitForAnimationToEnd, as --screen
  doesn't wait for a screen still moving; --ignore-screen <regex> leaves out a clock.
  rayo do --timeout 2m never waits for ever. rayo do .save says the lines it wrote and checks them.
  rayo do .quit ends the session and frees the device; with steps not saved, only with --discard.

Stop after a pass

Prefer structured output
  When a program reads: --format json or ndjson, --detail brief, --show failed, --keep failed.

What it costs, roughly
  rayo check        instant, no device
  one flow          seconds to minutes
  a whole suite     many minutes
  a capture         slow; it makes new ids, so diffs are noisy
  rayo live         holds the device until it ends

I want to...
  write a flow                         rayo help write-a-flow
  find why a flow failed               rayo explain (it finds the suite's output), then rayo help debug-a-flow
  fix failing flows as they fail       rayo test --on-fail hold <suite>; rayo help hold
  go on from a failure, after a fix    rayo test --resume <suite>; rayo help resume
  record a video                       record: true in the header; rayo help recipes
  check the device log, reset the app  rayo help recipes
  mock the backend                     rayo help record-mocks, rayo help connect-mocks
  run in a CI                          rayo help run-in-ci
  find any word in the help            rayo help search <word>
`},
	{"write-a-flow", "write a flow", `A flow is a header, ---, and a list of commands. Each command waits for what it needs.

  rayo hierarchy --emulator                          what is on screen, and how to refer to it
  rayo live --emulator --app com.example.app f.yaml  type commands, keep those that work; .save writes them
  rayo check f.yaml                                  mistakes, without a device
  rayo test --emulator f.yaml                        run it

Prefer an id to a text: texts change with the language. rayo help flow has the header and the
commands, rayo help <command> what one takes.
`},
	{"debug-a-flow", "debug a failing flow", `A failure begins with a verdict, <flow> failed at <step>: <cause>, then why, where in the file, the
elements most like what wasn't on screen, and the calls. --output (default .rayo) has failure.jpg,
screen.txt, logcat.txt and requests.txt, cut to the failure window.

  rayo explain                                       only that, for each flow of the last run that failed
  rayo hierarchy --find Pay                          only the elements most like Pay
  rayo test --changed-only all.suite.yaml            only the flows whose files changed since the last commit
  rayo test --results report.md all.suite.yaml       a short report, for a chat or a mail
  rayo test --emulator --timings f.yaml              how long each step took
  rayo test --emulator --rerun-failed all.suite.yaml  only what failed last time
  rayo test --emulator --resume all.suite.yaml       after a fix, go on from the step that failed
  rayo live --emulator f.yaml                        run its steps, then try commands where it stopped

The blame is app if the app crashed or hung, device if the device was lost, and else test.
`},
	{"resume", "go on from where a flow failed, after a fix, without running it all again", `A long flow failed near its end, and the fix is in the mocks or the flow, not the app. Fix the files,
then go on from the step that failed. The app must still be where the failure left it: run nothing
else on the device in between.

  rayo explain                                    where it failed, and the mocks' scenarios then
  rayo test --resume all.suite.yaml               each flow that failed, from the step that failed
  rayo test --from book.yaml:42 --only book all.suite.yaml        one flow, from a step
  rayo test --until book.yaml:42 --only book all.suite.yaml       stop after a step, leave the app there
  rayo test --resume --mock-state booking=booked all.suite.yaml       the scenarios set by hand

What happens
  - Files are read again, so a fix to a mock or a flow counts. The step that failed runs again first.
  - onFlowStart and the steps before the step don't run: the app is as the last run left it. Their
    mock: steps and the mocks of the flows they runFlow are still loaded, and so are the
    startRecording steps of onFlowStart, so that a stopRecording finds its recording.
  - A step inside a runFlow with when, or a repeat with while, is reached as if the condition held.
  - The mocks' scenarios are set as they were when that step began in the last run (failed.json keeps
    them), and --mock-state changes some. A step the last run didn't reach has none: say them.
  - --until stops after the step; onFlowComplete doesn't run.
  - No retries. A pass from the middle is PARTIAL, not PASS, so it stays in failed.json:
    run the flow whole once it passes, rayo test --only <name> <suite>.
  - --resume leaves out, and says so, a flow that failed in onFlowStart or onFlowComplete, and goes
    on with the others; with --only, only that flow.
  - Fixing as it fails, without a run for each try: rayo help hold.

--mock-state needs --resume or --from. A wrong scenario or state is an error before the device is used.
A step is file:line, as the at: of rayo explain writes it; the file may be only its name, a line
alone is of the flow's own file, and a text such as pickDate is the one step that has it. Steps of onFlowStart and onFlowComplete can't be begun from.

Gotchas
  - A step that isn't idempotent, such as a picker left half set, may need --from an earlier step
    that puts the screen back, or a rayo live session to fix the app by hand first.
  - --from goes back as well as forward: the scenarios of the last run are used if it reached that
    step; the screen isn't moved back, so begin where the app is.
  - In rayo live, .states lists the scenarios, and mockState: { booking: booked } sets one.
`},
	{"hold", "fix failing flows as they fail: hold the run at a failure, fix the files, run again from there, go on", `The suite and the flows are the ones in the repository: nothing is copied, and no temporary flow
is needed. A run that holds stops at the first failure with the app as it is, until it is told how to
go on; meanwhile the flow's files are fixed where they are.

  rayo test --on-fail hold all.suite.yaml        run; at a failure, hold the flow there
  rayo do .why                                   why, where (and from where), the elements most like it
  rayo do .screen          rayo do .find Pay      what is on screen now, or what is most like a text
  rayo do .diff                                  what of the screen changed since it failed
  (fix the flow, a flow it runs, or a mock, in its file)
  rayo do .retry                                 read the files again, run from the step that failed
  rayo do .from book.yaml:42                     or from another step, also one inside a when or a repeat
  rayo do .rewind 2                              or from 2 steps before the one that failed
  rayo do .probe 5                               the step that failed, 5 times: a flake, or always?
  rayo do .mark app-bug search answers 500       what it is, if it isn't the flow's to fix
  rayo do .continue                              once .retry passed: onFlowComplete, then the next flows
  rayo do .skip            rayo do .abort         leave it failed and go on; or run no more
  rayo test --rerun-failed all.suite.yaml        at the end: the fixed flows whole, so that a pass is honest

What happens
  - No retries: the first failure holds, before onFlowComplete, with the device, the app and the mocks
    as they are, and says so: HELD <flow>, the verdict, why, at, from, nearest, next.
  - rayo do finds the session in the suite's output folder by itself; --output says another. Typed
    commands work too. rayo do --format json answers with held: { flow, at, fixed }.
  - .retry and .from read the flow and the flows it runs again. If lines were added above the step
    that failed, it is found again by its text. Steps before it don't run; mocks are still loaded,
    and the env and variables are the run's.
  - A step inside a runFlow with when, or a repeat with while, is reached as if the condition held.
  - A flow fixed while held is PARTIAL, not PASS: it stays in failed.json, the summary says NOT WHOLE,
    and rayo test --rerun-failed runs it whole. A pass is only a whole flow's.
  - If the flow checks less after the fix than when it failed (fewer asserts, more optional), the
    results say WEAKENED: a fix that checks less isn't one.
  - .mark keeps app-bug, test-bug or flaky in the results and in failed.json, with why.
  - With no command for 15 minutes (--hold-timeout), the flow is left failed and the run goes on.
  - --format ndjson has a held event, with the session's folder.

Gotchas
  - Steps run again from the middle start from the app as it is: if the steps before changed it,
    move it back first, with a command such as rayo do '- back', or .from an earlier step.
  - What is typed with rayo do '- tapOn: OK' only moves the app; the flow has what its file has.
`},
	{"record-mocks", "record mocks from the real backend", `With mock: in the header, rayo answers as the backend. To make the mocks, capture what the real
one answers, once, then keep them up to date:

  rayo test --capture mock --capture-from https://api.example.com f.yaml  write the calls as mocks
  rayo test --capture-update mock f.yaml                                  say what the backend changed
  rayo mocks apply mock                                                   take the changes, or --only some
  rayo mocks import capture.har mock                                      or begin from a HAR file

With no code in the app, --network proxy and rayo ca export do it through the device's proxy;
rayo help connect-mocks says which to use.
A suite's capture: says how to capture, so the commands need no options.
`},
	{"connect-mocks", "connect the app to the mocks: interceptor or proxy", `The app's calls have to reach rayo to be mocked or captured. Two ways:

  interceptor (default)  the app sends its calls to rayo: a few lines of code, only in its debug build;
                         sees the calls of the app's own HTTP client
  proxy                  rayo sets the device's proxy while the flow runs: no code, the debug build
                         trusts rayo's certificate; sees every call, also of SDKs

  rayo test f.yaml                                       the interceptor, or the flow's network:
  rayo ca new                                            once, for the proxy: .rayo/ca, to commit
  rayo ca export app/src/debug/res/raw/rayo_ca.pem       and the certificate the app trusts
  rayo test --network proxy f.yaml                       the proxy

Can change the app, one HTTP client: interceptor. Can't change it, or an SDK calls on its own: proxy.
A host the app pins, or doesn't trust rayo's CA for, fails the flow at once: its calls never reach the real one.
Both use the same mock folders. Code and details: MOCKING.md, "Two ways to connect the app".
`},
	{"run-in-ci", "run in a CI", `Any CI with an emulator. The exit code isn't 0 if a flow fails, and .xml results are JUnit.

One suite holds the settings; the CI's command changes only what differs, with the option of the same name:

  rayo check --keep failed all.suite.yaml                  fail early, without a device; it says what the run would write
  rayo test --emulator --keep failed all.suite.yaml        only the files, and videos, of the tries that failed
  rayo test --emulator --results junit.xml all.suite.yaml  run, and write the results
  rayo test --emulator --retries 2 all.suite.yaml          try a failing flow again

Keep the --output folder as an artifact: it has the failures.
`},
	{"recipes", "short answers: video, logcat, reset, log in, deeplink, what a run leaves", `Record a video, and keep it when the flow passes
  record: true in the header (or startRecording and stopRecording around some steps). Every
  try's files are kept; --keep failed keeps only those of the tries that failed, --keep none none.

Check the device log
  - shell: logcat -d | grep -q "payment done"
  The command runs on the device; an exit code other than 0 fails the step. logcat -c clears it.

Start from a clean app
  - clearState      the app's data, as after an install
  - launchApp       then open it

Open a deeplink
  - openLink: myapp://orders/42
  Whichever app handles it opens.

Log in once, use it everywhere
  A flow of its own, run from the others:
  - runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }

What a try leaves in --output (default .rayo)
  <flow>_failure.jpg    the screen when it failed, if it did
  <flow>_screen.txt     the elements on it
  <flow>_logcat.txt     the device log, cut to the failure
  <flow>_requests.txt   the calls to the backend
  <flow>.mp4            the video, with record: true
  failed.json           what failed, for rayo explain and --rerun-failed; always written
  --name-template says how they are named, --artifacts which kinds are written.

The device was lost
  rayo devices; if it isn't listed, restart the emulator or reconnect it, then rayo test --rerun-failed.
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

// helpPart is a part of rayo help all: what rayo help takes to show it alone, its title and text.
type helpPart struct{ name, title, text string }

// helpParts are all the help, in the order of rayo help all.
func helpParts() ([]helpPart, error) {
	parts := []helpPart{{topics[0].name, topics[0].about, topics[0].text}, {"", "commands and options", help()}}
	for _, name := range append([]string{"flow", "suite"}, flow.Commands()...) {
		text, err := helpOn(name)
		if err != nil {
			return nil, err
		}
		parts = append(parts, helpPart{name, name, text})
	}
	for _, t := range topics[1:] {
		parts = append(parts, helpPart{t.name, t.about, t.text})
	}
	return parts, nil
}

// helpAll is all the help, on one page: each part begins with "## ".
func helpAll() (string, error) {
	parts, err := helpParts()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range parts {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", p.title, strings.TrimRight(p.text, "\n"))
	}
	return b.String(), nil
}

// helpSearch is the lines of all the help that have every word, ignoring case, under the
// rayo help that shows each whole.
func helpSearch(words []string) (string, error) {
	if len(words) == 0 {
		return "", errors.New("search for what? such as rayo help search video")
	}
	parts, err := helpParts()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range parts {
		var found []string
		for _, line := range strings.Split(p.text, "\n") {
			lower := strings.ToLower(line)
			if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(lower, strings.ToLower(w)) }) {
				found = append(found, "  "+strings.TrimSpace(line))
			}
		}
		if len(found) == 0 {
			continue
		}
		where := "rayo help"
		if p.name != "" {
			where += " " + p.name
		}
		fmt.Fprintf(&b, "%s (%s)\n%s\n\n", p.title, where, strings.Join(found, "\n"))
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("no help has %q; rayo help all is all of it", strings.Join(words, " "))
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
