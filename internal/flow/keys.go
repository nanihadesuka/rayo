package flow

import (
	"fmt"
	"strings"

	"github.com/nanihadesuka/rayo/internal/schema"
	"gopkg.in/yaml.v3"
)

// Key is one that a flow may use; its value is read as a T, see kinds.go.
type Key[T any] struct {
	name  string
	kind  kind[T]
	about string
}

func newKey[T any](name string, k kind[T], about string) *Key[T] { return &Key[T]{name, k, about} }

// anyKey is a Key whose T doesn't matter.
type anyKey interface {
	title() string
	takes() string
	describe() string
	check(p *reader, n *yaml.Node) error
	value(n *yaml.Node) any
	schema() schema.S
}

func (k *Key[T]) title() string    { return k.name }
func (k *Key[T]) takes() string    { return k.kind.name }
func (k *Key[T]) describe() string { return k.about }

func (k *Key[T]) schema() schema.S { return schema.Dynamic(k.kind.shape()) }

func (k *Key[T]) value(n *yaml.Node) any {
	v, err := k.kind.read(n)
	if err != nil {
		panic(fmt.Sprintf("%s was not checked: %v", k.name, err))
	}
	return v
}

func (k *Key[T]) check(p *reader, n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && dynamic(n) && !p.filled {
		return nil // checked when it is filled in
	}
	if k.kind.check != nil {
		return k.kind.check(p, k.name, n)
	}
	if _, err := k.kind.read(n); err != nil {
		return p.fail(n, "%s: %v", k.name, err)
	}
	return nil
}

// or is the value in a mapping that has been checked, or otherwise if it isn't there.
func (k *Key[T]) or(v *yaml.Node, otherwise T) T {
	if n := get(v, k); n != nil {
		return k.value(n).(T)
	}
	return otherwise
}

// of is like or, with nothing (zero) otherwise.
func (k *Key[T]) of(v *yaml.Node) T {
	var zero T
	return k.or(v, zero)
}

// Options of every command.
var (
	keyLabel    = newKey("label", aText, "What to show instead of the command")
	keyOptional = newKey("optional", aFlag, "A failure doesn't fail the flow")
	keyTimeout  = newKey("timeout", aTime, "How long it may take")

	commonKeys = []anyKey{keyLabel, keyOptional, keyTimeout}
)

// Selectors.
var (
	keyID    = newKey("id", aText, "The resource id, whole or after :id/; in Compose, a testTag with testTagsAsResourceId. A regular expression of the whole")
	keyText  = newKey("text", aText, "The text, description or hint. A regular expression of the whole")
	keyIndex = newKey("index", aNumber, "The n-th match, from 0, top to bottom and left to right; negative counts from the end")
)

// Filled in here because selectors hold selectors.
var selectorKeys []anyKey

func init() {
	selectorKeys = []anyKey{keyID, keyText, keyIndex,
		newKey("enabled", aFlag, "Whether it is enabled"),
		newKey("checked", aFlag, "Whether it is checked"),
		newKey("focused", aFlag, "Whether it has the focus"),
		newKey("selected", aFlag, "Whether it is selected"),
		newKey("below", anElement, "Entirely below another"),
		newKey("above", anElement, "Entirely above another"),
		newKey("leftOf", anElement, "Entirely to the left of another"),
		newKey("rightOf", anElement, "Entirely to the right of another"),
		newKey("containsChild", anElement, "The direct parent of another"),
		newKey("childOf", anElement, "Anywhere inside another"),
		newKey("containsDescendants", elements, "Around all of these"),
	}
}

// Conditions.
var (
	keyVisible    = newKey("visible", anElement, "On screen")
	keyNotVisible = newKey("notVisible", anElement, "Not on screen")
	keyPlatform   = newKey("platform", aPlatform, "Android")
	keyTrue       = newKey("true", aScript, "An expression that holds")

	whenKeys = []anyKey{keyVisible, keyNotVisible, keyPlatform, keyTrue}
)

// Apps.
var (
	keyAppID       = newKey("appId", aText, "The app; by default the header's")
	keyClearState  = newKey("clearState", aFlag, "Erase its data first (false)")
	keyStopApp     = newKey("stopApp", aFlag, "Close it first (true); false brings it back as it was")
	keyPermissions = newKey("permissions", permissions, "Name: allow, deny or unset. With clearState, all are allowed if not said")
	keyArguments   = newKey("arguments", arguments, "Intent extras; booleans, integers and decimals keep their type")
	keyLink        = newKey("link", aText, "What to open")
)

// Touch.
var (
	keyPoint     = newKey("point", aPoint, "Alone, of the screen; with a selector, of the element in %")
	keyRepeat    = newKey("repeat", aNumber, "Taps (1)")
	keyTapDelay  = newKey("delay", aTime, "Between taps (100)")
	keyDirection = newKey("direction", aDirection, "Where the finger or the search goes (DOWN when searching)")
	keyStart     = newKey("start", aPoint, "Where the finger goes down, instead of direction")
	keyEnd       = newKey("end", aPoint, "Where it goes up")
	keyDuration  = newKey("duration", aTime, "Of the swipe (250)")
	keyFrom      = newKey("from", anElement, "Start on this element; with direction")
	keyElement   = newKey("element", anElement, "What to look for")
)

// Text and keys.
var (
	keyValue  = newKey("value", aText, "The new text")
	keyInput  = newKey("text", aText, "The text to add")
	keyLength = newKey("length", aNumber, "How long (8)")
	keyErase  = newKey("charactersToErase", aNumber, "How many (50)")
	keyKey    = newKey("key", aKeyCode, "A name, such as Enter, or an Android key code")
)

// Control and scripts.
var (
	keyFile      = newKey("file", aText, "A path relative to this flow")
	keyEnv       = newKey("env", variables, "Variables for it")
	keyWhen      = newKey("when", conditions, "Only if these hold")
	keyWhile     = newKey("while", conditions, "As long as these hold")
	keyCommands  = newKey("commands", raw("commands", schema.Ref("commands")), "What to run")
	keyTimes     = newKey("times", aNumber, "At most")
	keyRetries   = newKey("retries", aNumber, "How many more times (1)")
	keyCondition = newKey("condition", aScript, "An expression")
	keyScript    = newKey("script", aScript, "A line of it")
)

// Device.
var (
	keyLatitude  = newKey("latitude", aDecimal, "In degrees")
	keyLongitude = newKey("longitude", aDecimal, "In degrees")
	keyPoints    = newKey("points", places, "A list of 'latitude,longitude'")
	keySpeed     = newKey("speed", aSpeed, "Metres per second (7)")
	keyOn        = newKey("value", onOff, "enabled or disabled")
	keyPath      = newKey("path", aText, "The file, in the debug output")
	keyVideo     = newKey("path", aText, "The file, in the debug output; by default the flow's name")
	keyImage     = newKey("image", aText, "What it sees, a path relative to this flow; or none")
	keyCommand   = newKey("command", aText, "Run as the shell user")
)

// Server.
var (
	keyMethod = newKey("method", aText, "Such as POST (GET)")
	keyURL    = newKey("url", aText, "What to call, from the computer")
	keyJSON   = newKey("json", raw("values", schema.S{}), "The body")
	keyExpect = newKey("expect", aNumber, "The status (any below 300)")
)

// Mock.
var (
	keyMockID   = newKey("id", aText, "A name for it")
	keyMockName = newKey("name", aText, "A name for it")
	keyPriority = newKey("priority", aNumber, "The lowest wins (0)")
	keyScenario = newKey("scenarioName", aText, "The scenario it belongs to")
	keyRequired = newKey("requiredScenarioState", aText, "Only in this state")
	keyNewState = newKey("newScenarioState", aText, "Moves to this state")
	keyRequest  = newKey("request", raw("values", schema.S{}), "Which calls")
	keyResponse = newKey("response", raw("values", schema.S{}), "What to answer")
	keyState    = newKey("state", aState, "online or offline")
	keyDelay    = newKey("delay", aTime, "Every answer takes longer")
	keyFail     = newKey("fail", aStatus, "Every answer is this status, 400 to 599")
	keyCallPath = newKey("path", aPattern, "Only these calls, a regular expression of the whole path")
	keyBody     = newKey("body", aText, "A text in the body")
)

// lateKeys hold code, or what is filled in when it runs, so ${...} is left as it is. By name, as they may be nested.
var lateKeys = map[string]bool{}

func init() {
	for _, k := range []anyKey{keyWhen, keyWhile, keyCommands, keyCondition, keyScript} {
		lateKeys[k.title()] = true
	}
}

func find(list []anyKey, name string) anyKey {
	for _, k := range list {
		if k.title() == name {
			return k
		}
	}
	return nil
}

func isOption(name string) bool { return find(commonKeys, name) != nil }

// names joins those of some keys.
func names(keys []anyKey, with string) string {
	var list []string
	for _, k := range keys {
		list = append(list, k.title())
	}
	return strings.Join(list, with)
}

// Help describes a command.
func Help(name string) (string, error) {
	c, ok := commands[name]
	if !ok {
		return "", fmt.Errorf("unknown command %q%s", name, suggest(name))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", name, c.about)
	list := func(title string, keys []anyKey) {
		if len(keys) > 0 {
			fmt.Fprintf(&b, "\n%s:\n", title)
		}
		for _, k := range keys {
			mark := ""
			if k == c.plain {
				mark = " (short form)"
			}
			fmt.Fprintf(&b, "  %-22s %-12s %s%s\n", k.title(), k.takes(), k.describe(), mark)
		}
	}
	list("keys", c.keys)
	if c.element {
		list("selector", selectorKeys)
	}
	list("options", commonKeys)
	return b.String(), nil
}
