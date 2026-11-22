package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
	"gopkg.in/yaml.v3"
)

// command is all there is to know about one: to add one, add it to the table below.
type command struct {
	about   string
	keys    []anyKey // accepted in a mapping
	plain   anyKey   // what a bare value stands for; nil if a value isn't accepted
	alone   bool     // works without a value
	element bool     // takes a selector too, and needs one
	group   bool     // made of other commands, which report themselves
	free    bool     // any key is accepted
	limit   time.Duration

	check func(p *reader, s *Step) error // what the keys alone don't tell; may be nil
	say   func(v *yaml.Node) string      // its value in the output; nil for the usual
	run   func(c *call) error
}

// accepts lists its keys, without the options of every command.
func (k command) accepts() []anyKey {
	if k.element {
		return append(slices.Clip(k.keys), selectorKeys...)
	}
	return k.keys
}

// call is a command being run.
type call struct {
	ctx   context.Context
	r     *Runner
	s     *Step
	k     command
	v     *yaml.Node // the value, with each ${...} replaced
	depth int
	limit time.Duration
}

// arg is the value of a key, or nothing (zero).
func arg[T any](c *call, k *Key[T]) T {
	var zero T
	return argOr(c, k, zero)
}

func argOr[T any](c *call, k *Key[T], otherwise T) T {
	if !slices.Contains(c.k.accepts(), anyKey(k)) && !slices.Contains(commonKeys, anyKey(k)) {
		panic(c.s.Kind + " doesn't declare " + k.name)
	}
	return k.or(c.v, otherwise)
}

func (c *call) do(op string, args agent.Args) error {
	_, err := c.r.agent.Do(c.ctx, op, args, c.limit)
	return err
}

var commands map[string]command

// Filled in here because commands run other commands, so they refer to the table.
func init() {
	input := func(value func(c *call) string) func(c *call) error {
		return func(c *call) error { return c.do("input", agent.Args{"value": value(c)}) }
	}
	commands = map[string]command{
		// Apps
		"launchApp": {about: "Opens the app and waits until Android has it in front",
			keys: []anyKey{keyAppID, keyClearState, keyStopApp, keyPermissions, keyArguments}, plain: keyAppID, alone: true,
			run: launch},
		"stopApp": {about: "Closes the app",
			keys: []anyKey{keyAppID}, plain: keyAppID, alone: true, run: onApp("am force-stop ")},
		"killApp": {about: "Closes the app",
			keys: []anyKey{keyAppID}, plain: keyAppID, alone: true, run: onApp("am force-stop ")},
		"clearState": {about: "Erases the app's data",
			keys: []anyKey{keyAppID}, plain: keyAppID, alone: true, run: onApp("pm clear ")},
		"setPermissions": {about: "Allows or denies permissions",
			keys: []anyKey{keyAppID, keyPermissions}, check: needs(keyPermissions), run: setPermissions},
		"openLink": {about: "Opens a link in whichever app handles it",
			keys: []anyKey{keyLink}, plain: keyLink, run: func(c *call) error {
				return c.command("am start -a android.intent.action.VIEW -d " + quote(arg(c, keyLink)))
			}},

		// Touch
		"tapOn": {about: "Waits until the element is on screen, enabled and still, then taps it",
			keys: []anyKey{keyPoint, keyRepeat, keyTapDelay}, plain: keyText, element: true, run: func(c *call) error {
				return tap(c, agent.Args{"repeat": argOr(c, keyRepeat, 1), "delay": argOr(c, keyTapDelay, 100*time.Millisecond).Milliseconds()})
			}},
		"doubleTapOn": {about: "Like tapOn, twice",
			keys: []anyKey{keyPoint, keyTapDelay}, plain: keyText, element: true, run: func(c *call) error {
				return tap(c, agent.Args{"repeat": 2, "delay": argOr(c, keyTapDelay, 100*time.Millisecond).Milliseconds()})
			}},
		"longPressOn": {about: "Like tapOn, held for a second",
			keys: []anyKey{keyPoint}, plain: keyText, element: true, run: func(c *call) error {
				return tap(c, agent.Args{"hold": 1000})
			}},
		"swipe": {about: "Moves a finger across the screen",
			keys: []anyKey{keyDirection, keyStart, keyEnd, keyDuration, keyFrom}, check: checkSwipe, run: swipe},
		"scroll": {about: "Scrolls down once",
			alone: true, run: func(c *call) error {
				return c.do("swipe", agent.Args{"x": 50, "y": 70, "x2": 50, "y2": 30, "percent": true, "ms": 250})
			}},
		"scrollUntilVisible": {about: "Scrolls until the element is on screen",
			keys: []anyKey{keyElement, keyDirection}, limit: 20 * time.Second,
			check: needs(keyElement), run: scrollUntil},

		// Text and keys
		"type": {about: "Replaces the text of a field at once, without the keyboard",
			keys: []anyKey{keyValue}, element: true, check: needs(keyValue), run: func(c *call) error {
				return c.do("set", agent.Args{"sel": selector(c.v), "value": arg(c, keyValue)})
			}},
		"inputText": {about: "Adds text to the focused field",
			keys: []anyKey{keyInput}, plain: keyInput, run: input(func(c *call) string { return arg(c, keyInput) })},
		"inputRandomEmail": {about: "Adds a random email",
			alone: true, run: input(func(*call) string { return random(10, letters) + "@example.com" })},
		"inputRandomPersonName": {about: "Adds a random name",
			alone: true, run: input(func(*call) string { return randomName(6) + " " + randomName(8) })},
		"inputRandomNumber": {about: "Adds a random number",
			keys: []anyKey{keyLength}, alone: true, run: input(func(c *call) string {
				return random(argOr(c, keyLength, 8), "123456789")
			})},
		"inputRandomText": {about: "Adds random letters",
			keys: []anyKey{keyLength}, alone: true, run: input(func(c *call) string {
				return random(argOr(c, keyLength, 8), letters)
			})},
		"eraseText": {about: "Deletes from the end of the focused field",
			keys: []anyKey{keyErase}, plain: keyErase, alone: true, run: func(c *call) error {
				return c.do("erase", agent.Args{"count": argOr(c, keyErase, 50)})
			}},
		"copyTextFrom": {about: "Keeps an element's text in rayo.copiedText",
			plain: keyText, element: true, run: copyText},
		"pasteText": {about: "Adds rayo.copiedText to the focused field",
			alone: true, run: pasteText},
		"hideKeyboard": {about: "Closes the keyboard",
			alone: true, run: func(c *call) error { return c.do("hideKeyboard", nil) }},
		"pressKey": {about: "Presses a key",
			keys: []anyKey{keyKey}, plain: keyKey, check: needs(keyKey), run: func(c *call) error {
				return c.do("key", agent.Args{"code": arg(c, keyKey)})
			}},
		"back": {about: "Presses Back",
			alone: true, run: func(c *call) error { return c.do("key", agent.Args{"code": 4}) }},

		// Checks and waits
		"assertVisible": {about: "The element is on screen, or appears in time",
			plain: keyText, element: true, run: func(c *call) error {
				return c.do("wait", agent.Args{"sel": selector(c.v)})
			}},
		"assertNotVisible": {about: "The element isn't on screen, or goes away in time",
			plain: keyText, element: true, limit: goneTimeout, run: func(c *call) error {
				return c.do("gone", agent.Args{"sel": selector(c.v)})
			}},
		"extendedWaitUntil": {about: "Waits for an element to appear or go away",
			keys: []anyKey{keyVisible, keyNotVisible}, check: func(p *reader, s *Step) error {
				if (get(s.Value, keyVisible) == nil) == (get(s.Value, keyNotVisible) == nil) {
					return p.fail(s.Value, "extendedWaitUntil needs visible or notVisible")
				}
				return nil
			}, run: func(c *call) error {
				if sel := arg(c, keyVisible); sel != nil {
					return c.do("wait", agent.Args{"sel": sel})
				}
				return c.do("gone", agent.Args{"sel": arg(c, keyNotVisible)})
			}},
		"assertTrue": {about: "An expression is true",
			keys: []anyKey{keyCondition}, plain: keyCondition, run: func(c *call) error {
				ok, err := c.r.script.truthy(arg(c, keyCondition))
				if err == nil && !ok {
					err = errors.New("it is false")
				}
				return err
			}},
		"waitForAnimationToEnd": {about: "Waits until nothing has changed for a while",
			alone: true, run: func(c *call) error { return c.do("settle", agent.Args{"quiet": 200}) }},

		// Control
		"runFlow": {about: "Runs another flow, or commands right here",
			keys: []anyKey{keyFile, keyEnv, keyWhen, keyCommands}, plain: keyFile, group: true, check: checkFileOrCommands, run: runFlow},
		"retry": {about: "Runs again what fails",
			keys: []anyKey{keyMaxRetries, keyCommands, keyFile, keyEnv}, group: true, check: checkFileOrCommands, run: retry},
		"repeat": {about: "Runs commands several times",
			keys: []anyKey{keyTimes, keyWhile, keyCommands}, group: true, run: repeat, check: func(p *reader, s *Step) error {
				if s.Sub == nil || (get(s.Value, keyTimes) == nil && get(s.Value, keyWhile) == nil) {
					return p.fail(s.Value, "repeat needs commands, and times or while")
				}
				return nil
			}},

		// Scripts
		"runScript": {about: "Runs a JavaScript file",
			keys: []anyKey{keyFile, keyEnv, keyWhen}, plain: keyFile, check: func(p *reader, s *Step) error {
				file := get(s.Value, keyFile)
				if file != nil && !dynamic(file) {
					if _, err := os.Stat(relative(p.path, file.Value)); err != nil {
						return p.fail(file, "no such file: %s", file.Value)
					}
				}
				return needs(keyFile)(p, s)
			}, run: runScript},
		"evalScript": {about: "Runs a line of JavaScript",
			keys: []anyKey{keyScript}, plain: keyScript, run: func(c *call) error {
				_, err := c.r.script.eval("evalScript", code(arg(c, keyScript)))
				return err
			}},

		// Device
		"setLocation": {about: "Stays at a place",
			keys: []anyKey{keyLatitude, keyLongitude}, check: needs(keyLatitude, keyLongitude), run: func(c *call) error {
				p := Place{arg(c, keyLatitude), arg(c, keyLongitude)}
				if err := p.check(); err != nil {
					return err
				}
				c.r.located = true
				return c.do("location", agent.Args{"place": p.String()})
			}},
		"travel": {about: "Moves along points at a real speed",
			keys: []anyKey{keyPoints, keySpeed}, check: needs(keyPoints), run: travel},
		"setAirplaneMode": {about: "Turns airplane mode on or off",
			keys: []anyKey{keyOn}, plain: keyOn, run: setAirplane},
		"takeScreenshot": {about: "Saves the screen",
			keys: []anyKey{keyPath}, plain: keyPath, run: func(c *call) error {
				name := strings.TrimSuffix(arg(c, keyPath), ".png") + ".png"
				return c.r.saveScreenshot(filepath.Join(c.r.folder, filepath.FromSlash(name)))
			}},
		"startRecording": {about: "Records the screen, until stopRecording or the end of the flow",
			keys: []anyKey{keyVideo}, plain: keyVideo, alone: true, run: func(c *call) error {
				return c.r.startRecording(c.r.recordingPath(arg(c, keyVideo)))
			}},
		"stopRecording": {about: "Saves the recording",
			alone: true, run: func(c *call) error { return c.r.stopRecording() }},
		"camera": {about: "What an emulator's camera sees; does nothing on other devices",
			keys: []anyKey{keyImage}, plain: keyImage, check: checkCamera, run: setCamera},
		"shell": {about: "Runs a command on the device",
			keys: []anyKey{keyCommand}, plain: keyCommand, run: func(c *call) error { return c.command(arg(c, keyCommand)) }},

		// Server
		"http": {about: "Calls a server from the computer",
			keys: []anyKey{keyMethod, keyURL, keyJSON, keyExpect}, plain: keyURL, check: needs(keyURL), run: httpStep},

		// Mock
		"mock": {about: "Adds mocks: a path, or one written here",
			keys:  []anyKey{keyFile, keyMockID, keyMockName, keyPriority, keyScenario, keyRequired, keyNewState, keyRequest, keyResponse},
			plain: keyFile, check: checkMock, say: sayMock, run: addMock},
		"mockState": {about: "Moves scenarios to a state, such as { ride: docked }",
			free: true, check: checkMockState, say: listing, run: setMockState},
		"network": {about: "How the connection to the mock behaves",
			keys: []anyKey{keyState, keyDelay, keyFail, keyCallPath}, plain: keyState, check: checkNetwork, run: setNetwork},
		"waitForRequest": {about: "Waits until the app makes a call",
			keys: []anyKey{keyMethod, keyCallPath, keyBody}, plain: keyCallPath, check: needs(keyCallPath), run: awaitRequest},
	}
}

// --- Checks ---

func needs(keys ...anyKey) func(p *reader, s *Step) error {
	return func(p *reader, s *Step) error {
		for _, k := range keys {
			if get(s.Value, k) == nil {
				return p.fail(s.Value, "%s needs %s", s.Kind, names(keys, " and "))
			}
		}
		return nil
	}
}

func checkFileOrCommands(p *reader, s *Step) error {
	file := get(s.Value, keyFile)
	if file != nil && !dynamic(file) {
		_, err := load(relative(p.path, file.Value), p.depth+1)
		if errors.Is(err, errCircle) {
			return err
		}
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filepath.ToSlash(p.path), file.Line, err)
		}
	}
	if (file == nil) == (s.Sub == nil) {
		return p.fail(s.Value, "%s needs either a file or commands", s.Kind)
	}
	return nil
}

func checkSwipe(p *reader, s *Step) error {
	if get(s.Value, keyDirection) == nil && (get(s.Value, keyStart) == nil || get(s.Value, keyEnd) == nil) {
		return p.fail(s.Value, "swipe needs a direction, or start and end")
	}
	return nil
}
