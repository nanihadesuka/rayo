package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The skill costs context each time an agent reads it: SKILL.md always, the others when a task needs them.
func TestSkillBudget(t *testing.T) {
	for name, text := range skillFiles() {
		limit := skillFileLines
		if name == "SKILL.md" {
			limit = skillMaxLines
		}
		if n := strings.Count(text, "\n"); n > limit {
			t.Errorf("%s has %d lines, more than %d: make it shorter, or point to rayo help", name, n, limit)
		}
	}
}

func TestSkillTopicsExist(t *testing.T) {
	for _, p := range skillParts {
		for _, name := range p.topics {
			if helpTopic(name) == "" {
				t.Errorf("%s is made of topic %s, which rayo help doesn't have", p.file, name)
			}
		}
		if !strings.Contains(skillFiles()["SKILL.md"], p.file) {
			t.Errorf("SKILL.md doesn't say when to read %s", p.file)
		}
	}
}

func TestSkillsAddListRemove(t *testing.T) {
	dir := t.TempDir()
	if err := commandSkills([]string{"add", "--project", dir, "--agent", "claude,codex"}); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{".claude", ".codex"} {
		if !ours(filepath.Join(dir, a, "skills", "rayo")) {
			t.Errorf("no skill in %s", a)
		}
	}
	other := filepath.Join(dir, ".claude", "skills", "rayo", "SKILL.md")
	if err := os.WriteFile(other, []byte("---\nname: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commandSkills([]string{"add", "--project", dir, "--agent", "claude"}); err == nil {
		t.Error("overwrote a skill that isn't rayo's")
	}
	if err := commandSkills([]string{"remove", "--project", dir, "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "skills", "rayo")); err == nil {
		t.Error("remove left the skill")
	}
	if err := commandSkills([]string{"add", "--project", dir, "--agent", "gemini"}); err == nil {
		t.Error("took an agent it doesn't know")
	}
}
