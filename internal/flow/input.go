package flow

import (
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	"github.com/nanihadesuka/rayo/internal/agent"
	"github.com/nanihadesuka/rayo/internal/schema"
)

func copyText(c *call) error {
	a, err := c.r.agent.Do(c.ctx, "wait", agent.Args{"sel": selector(c.v)}, c.limit)
	if err != nil {
		return err
	}
	vm := c.r.script.vm
	if name := arg(c, keyKeptAs); name != "" {
		if err := vm.GlobalObject().Get("output").ToObject(vm).Set(name, a.Text); err != nil {
			return err
		}
	}
	return vm.GlobalObject().Get("rayo").ToObject(vm).Set("copiedText", a.Text)
}

func pasteText(c *call) error {
	copied, err := c.r.script.fill("${rayo.copiedText}")
	if err != nil {
		return err
	}
	return c.do("input", agent.Args{"value": copied})
}

const letters = "abcdefghijklmnopqrstuvwxyz"

func random(n int, from string) string {
	b := make([]byte, max(0, n))
	for i := range b {
		b[i] = from[rand.Intn(len(from))]
	}
	return string(b)
}

func randomName(length int) string {
	return strings.ToUpper(random(1, letters)) + random(length-1, letters)
}

var keyCodes = map[string]int{
	"home": 3, "back": 4, "enter": 66, "tab": 61, "backspace": 67, "delete": 67, "escape": 111, "space": 62, "lock": 223, "power": 26,
	"volume up": 24, "volume down": 25, "recents": 187, "menu": 82, "search": 84,
	"remote dpad up": 19, "remote dpad down": 20, "remote dpad left": 21, "remote dpad right": 22, "remote dpad center": 23,
	"remote media play pause": 85, "remote media stop": 86, "remote media next": 87, "remote media previous": 88,
}

func keyCode(name string) (int, error) {
	if c, ok := keyCodes[strings.ToLower(strings.TrimSpace(name))]; ok {
		return c, nil
	}
	if c, err := strconv.Atoi(name); err == nil {
		return c, nil
	}
	if near := schema.Nearest(name, slices.Sorted(maps.Keys(keyCodes))); near != "" {
		return 0, fmt.Errorf("unknown key %q, did you mean %s?", name, near)
	}
	return 0, fmt.Errorf("unknown key %q", name)
}
