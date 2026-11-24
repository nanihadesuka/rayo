package main

import (
	"regexp"
	"testing"
)

// Every rayo <command> and --option the agents page names is one rayo has.
func TestAgentsNamesExist(t *testing.T) {
	text := helpTopic("agents")
	if text == "" || topics[0].name != "agents" {
		t.Fatal("agents isn't the first topic")
	}
	commands := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^  rayo ([a-z]+)`).FindAllStringSubmatch(usage, -1) {
		commands[m[1]] = true
	}
	options := map[string]bool{}
	for _, s := range sections {
		for _, o := range s.options {
			options[o.name] = true
		}
	}
	for _, m := range regexp.MustCompile(`\brayo ([a-z][a-z-]*)`).FindAllStringSubmatch(text, -1) {
		if !commands[m[1]] {
			t.Errorf("rayo help agents names rayo %s, a command rayo doesn't have", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`(?:^|[^\w-])--([a-z][a-z-]*)`).FindAllStringSubmatch(text, -1) {
		if !options[m[1]] {
			t.Errorf("rayo help agents names --%s, an option rayo doesn't have", m[1])
		}
	}
}
