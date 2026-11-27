package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The skill is made from the help topics, so it says what rayo help says and never drifts from it.
// Only SKILL.md is read when an agent picks the skill; the other files only when a task needs them.
// TestSkillBudget keeps each small.
const (
	skillName      = "rayo"
	skillMaxLines  = 110 // SKILL.md
	skillFileLines = 150 // each other file
	skillDesc      = "Write, run, debug and fix Android end-to-end tests with rayo (YAML flows), " +
		"and mock or capture the app's backend (WireMock format, interceptor or proxy). Use for any rayo flow, suite, mock or command."
)

// skillAgents are where each agent finds its skills, under the home folder or a project.
var skillAgents = []struct{ name, dir string }{
	{"claude", ".claude"},
	{"codex", ".codex"},
}

// skillParts are the files besides SKILL.md: what each is for, and the topics it is made of.
var skillParts = []struct {
	file, about string
	topics      []string
}{
	{"writing-tests.md", "write a flow: hierarchy, live, check", []string{"write-a-flow"}},
	{"mocking-in-flows.md", "mock steps, scenarios, network, checking calls", []string{"mock-in-flows"}},
	{"capture.md", "record mocks from the real backend, update them", []string{"record-mocks"}},
	{"connecting.md", "interceptor or proxy, and the interceptor's code for the app", []string{"connect-mocks", "interceptor"}},
	{"debugging.md", "why a flow failed", []string{"debug-a-flow"}},
	{"fixing-tests.md", "fix failing flows: hold, retry, resume", []string{"hold", "resume"}},
	{"suites-and-ci.md", "suites, CI, exit codes", []string{"run-in-ci", "exit-codes"}},
	{"device.md", "location, trips, camera, airplane mode, rotation", []string{"device"}},
	{"recipes.md", "video, logcat, reset, deeplink, what a run leaves; limits", []string{"recipes", "limits"}},
}

// skillFiles are the skill's files, by name.
func skillFiles() map[string]string {
	agents := topics[0].text // the agents topic, without its title
	if i := strings.Index(agents, "\nI want to..."); i >= 0 {
		agents = agents[:i]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n<!-- rayo %s: rayo skills add writes it again -->\n\n# rayo\n\n", skillName, skillDesc, version)
	b.WriteString(strings.TrimRight(agents, "\n") + "\n\nRead for a task, from the files next to this one\n")
	files := map[string]string{}
	for _, p := range skillParts {
		fmt.Fprintf(&b, "  %-22s %s\n", p.file, p.about)
		var f strings.Builder
		for _, name := range p.topics {
			fmt.Fprintf(&f, "# %s\n\n%s\n", name, strings.TrimRight(helpTopic(name), "\n"))
		}
		files[p.file] = strings.TrimRight(f.String(), "\n") + "\n"
	}
	b.WriteString("\nAnything else: rayo help <command or topic>; rayo help search <word>.\n")
	files["SKILL.md"] = b.String()
	return files
}

// skillTargets are the agents to work on, with their skills folder.
func skillTargets(o *options, onlyFound bool) ([][2]string, error) {
	base := o.project
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = home
	}
	var names []string
	for _, a := range skillAgents {
		names = append(names, a.name)
	}
	for _, n := range o.agents {
		if !slices.Contains(names, n) {
			return nil, fmt.Errorf("--agent takes %s, not %q", strings.Join(names, ", "), n)
		}
	}
	var targets [][2]string
	for _, a := range skillAgents {
		dir := filepath.Join(base, a.dir)
		if len(o.agents) > 0 && !slices.Contains(o.agents, a.name) {
			continue
		}
		if _, err := os.Stat(dir); len(o.agents) == 0 && onlyFound && err != nil {
			continue
		}
		targets = append(targets, [2]string{a.name, filepath.Join(dir, "skills", skillName)})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("found no agent in %s: say which with --agent %s", base, strings.Join(names, ","))
	}
	return targets, nil
}

// ours is whether a folder holds the rayo skill, so that only it is ever removed.
func ours(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	return err == nil && strings.Contains(string(data), "\nname: "+skillName+"\n")
}

const skillsUsage = "rayo skills add, remove or list [--agent claude,codex] [--project <folder>]"

func commandSkills(args []string) error {
	o, err := parse(cmdSkills, args)
	if err != nil {
		return err
	}
	if len(o.args) != 1 {
		if near := nearSub(o.args, "add", "remove", "list"); near != "" {
			return fmt.Errorf("unknown rayo skills %s, did you mean %s?", o.args[0], near)
		}
		return errors.New(skillsUsage)
	}
	switch o.args[0] {
	case "add":
		targets, err := skillTargets(o, true)
		if err != nil {
			return err
		}
		files := skillFiles()
		for _, t := range targets {
			if _, err := os.Stat(t[1]); err == nil && !ours(t[1]) {
				return fmt.Errorf("%s has a skill that isn't rayo's: left as it is", t[1])
			}
			if err := os.RemoveAll(t[1]); err != nil { // no file of an older version stays
				return err
			}
			if err := os.MkdirAll(t[1], 0o755); err != nil {
				return err
			}
			for name, text := range files {
				if err := os.WriteFile(filepath.Join(t[1], name), []byte(text), 0o644); err != nil {
					return err
				}
			}
			fmt.Printf("%s: wrote the rayo %s skill in %s\n", t[0], version, t[1])
		}
		return nil
	case "remove":
		targets, err := skillTargets(o, false)
		if err != nil {
			return err
		}
		for _, t := range targets {
			switch {
			case !ours(t[1]):
				fmt.Printf("%s: no rayo skill in %s\n", t[0], t[1])
			default:
				if err := os.RemoveAll(t[1]); err != nil {
					return err
				}
				fmt.Printf("%s: removed %s\n", t[0], t[1])
			}
		}
		return nil
	case "list":
		targets, err := skillTargets(o, false)
		if err != nil {
			return err
		}
		want := skillFiles()["SKILL.md"]
		for _, t := range targets {
			data, err := os.ReadFile(filepath.Join(t[1], "SKILL.md"))
			switch {
			case err != nil || !ours(t[1]):
				fmt.Printf("%s: not there (%s); rayo skills add --agent %s\n", t[0], t[1], t[0])
			case string(data) != want:
				fmt.Printf("%s: out of date (%s); rayo skills add --agent %s\n", t[0], t[1], t[0])
			default:
				fmt.Printf("%s: up to date (%s)\n", t[0], t[1])
			}
		}
		return nil
	}
	if near := nearSub(o.args, "add", "remove", "list"); near != "" {
		return fmt.Errorf("unknown rayo skills %s, did you mean %s?", o.args[0], near)
	}
	return errors.New(skillsUsage)
}
