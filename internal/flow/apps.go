package flow

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
	"gopkg.in/yaml.v3"
)

var packageName = regexp.MustCompile(`^[A-Za-z]\w*(\.\w+)+$`)

func (c *call) app() (string, error) {
	app := arg(c, keyAppID)
	if app == "" {
		var err error
		if app, err = c.r.script.fill(c.r.flow.App); err != nil {
			return "", err
		}
	}
	if !packageName.MatchString(app) {
		return "", fmt.Errorf("appId isn't a package name: %q", app)
	}
	return app, nil
}

// onApp runs a shell command on the app.
func onApp(command string) func(c *call) error {
	return func(c *call) error {
		app, err := c.app()
		if err != nil {
			return err
		}
		return c.command(command + app)
	}
}

var permissionGroups = map[string][]string{
	"location":      {"ACCESS_FINE_LOCATION", "ACCESS_COARSE_LOCATION"},
	"camera":        {"CAMERA"},
	"notifications": {"POST_NOTIFICATIONS"},
	"contacts":      {"READ_CONTACTS", "WRITE_CONTACTS"},
	"microphone":    {"RECORD_AUDIO"},
	"bluetooth":     {"BLUETOOTH_CONNECT", "BLUETOOTH_SCAN"},
	"storage":       {"READ_EXTERNAL_STORAGE", "WRITE_EXTERNAL_STORAGE"},
	"phone":         {"CALL_PHONE", "ANSWER_PHONE_CALLS"},
	"calendar":      {"READ_CALENDAR", "WRITE_CALENDAR"},
	"sms":           {"READ_SMS", "RECEIVE_SMS", "SEND_SMS"},
	"medialibrary":  {"READ_MEDIA_IMAGES", "READ_MEDIA_VIDEO", "READ_MEDIA_AUDIO"},
}

// permissionsSince are the groups an older Android doesn't have, with the API level and the version that
// brought them: there, an app is never asked, so setting them is skipped with a note.
var permissionsSince = map[string]struct{ api, version int }{
	"notifications": {33, 13},
	"medialibrary":  {33, 13},
	"bluetooth":     {31, 12},
}

// permissionNames are Android's for one written in a flow.
func permissionNames(name string) []string {
	group := permissionGroups[strings.ToLower(name)]
	if group == nil {
		group = []string{name}
	}
	var names []string
	for _, n := range group {
		if !strings.Contains(n, ".") {
			n = "android.permission." + n
		}
		names = append(names, n)
	}
	return names
}

// runtimePermissions is the shell for those of an app that can be changed.
func runtimePermissions(app string) string {
	return "$(dumpsys package " + app + " | sed -n '/runtime permissions:/,$p' | " +
		"sed -n 's/^ *\\([A-Za-z0-9_.]*\\): granted=.*/\\1/p' | sort -u)"
}

// changePermissions is the shell for it, in order, so a later one wins. They are listed while whatever
// is running in the background ends, and changed after.
func changePermissions(app string, permissions *yaml.Node) ([]string, error) {
	lines := []string{"ps=" + runtimePermissions(app), "sdk=$(getprop ro.build.version.sdk)", "wait"}
	err := pairs(permissions, func(k, p *yaml.Node) error {
		allow, err := aPermission.read(p)
		if err != nil {
			return err
		}
		change := "pm " + map[bool]string{true: "grant", false: "revoke"}[allow] + " " + app + " "
		if k.Value == "all" {
			lines = append(lines, "for p in $ps; do "+change+"$p >/dev/null 2>&1 & done")
			return nil
		}
		// Of a group, an app may have only some. Asking pm wouldn't tell: it accepts any.
		lines = append(lines, "wait", "ok=")
		since, newer := permissionsSince[strings.ToLower(k.Value)]
		if newer {
			lines = append(lines, fmt.Sprintf("if [ \"${sdk:-99}\" -lt %d ]; then echo %s; else", since.api,
				quote(fmt.Sprintf("Note: %s: not asked before Android %d, ignored", k.Value, since.version))))
		}
		for _, name := range permissionNames(k.Value) {
			lines = append(lines, "echo \"$ps\" | grep -qxF "+quote(name)+" && "+change+quote(name)+" && ok=1")
		}
		lines = append(lines, "[ $ok ] || echo "+quote("Error: "+app+" has no permission "+k.Value+" that can be changed"))
		if newer {
			lines = append(lines, "fi")
		}
		return nil
	})
	return lines, err
}

// setPermissions takes one trip to the device.
func setPermissions(c *call) error {
	app, err := c.app()
	if err != nil {
		return err
	}
	lines, err := changePermissions(app, arg(c, keyPermissions))
	if err != nil {
		return err
	}
	installed := "pm path " + app + " >/dev/null || { echo 'Error: " + app + " is not installed'; exit; }"
	out, err := c.shell(strings.Join(slices.Concat([]string{installed}, lines, []string{"wait"}), "\n"))
	if err != nil {
		return err
	}
	c.notePermissions(out)
	return complaint(out)
}

// notePermissions says which permissions were skipped, as that Android doesn't have them.
func (c *call) notePermissions(out string) {
	for line := range strings.Lines(out) {
		if note, ok := strings.CutPrefix(strings.TrimSpace(line), "Note: "); ok {
			c.r.reporter.Note(2, note)
		}
	}
}

// complaint is the first line of an answer that says something went wrong: Android's tools don't always fail.
func complaint(out string) error {
	for line := range strings.Lines(out) {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "Error") || strings.HasPrefix(line, "Exception occurred") || line == "Failed" {
			return errors.New(line)
		}
	}
	return nil
}

func launch(c *call) error {
	app, err := c.app()
	if err != nil {
		return err
	}
	// Everything in one shell, so there is a single trip to the device.
	var lines []string
	clear := arg(c, keyClearState)
	if clear {
		lines = append(lines, "pm clear "+app+" &")
	} else if argOr(c, keyStopApp, true) {
		lines = append(lines, "am force-stop "+app)
	}
	permissions := arg(c, keyPermissions)
	if permissions == nil && clear { // the data went, so what was allowed did too
		permissions = allowAll
	}
	if permissions != nil {
		change, err := changePermissions(app, permissions)
		if err != nil {
			return err
		}
		lines = append(lines, change...)
	}
	lines = append(lines, startApp(app, arg(c, keyArguments))...)
	a, err := c.r.agent.Do(c.ctx, "shell", agent.Args{"command": strings.Join(lines, "\n"), "quiet": true}, 60*time.Second)
	if err != nil {
		return err
	}
	c.notePermissions(a.Out)
	if err := complaint(a.Out); err != nil {
		return err
	}
	if clear && !strings.Contains(a.Out, "Success") {
		return errors.New("the data couldn't be cleared: " + strings.TrimSpace(a.Out))
	}
	return c.do("front", agent.Args{"app": app})
}

var allowAll = &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "all"}, {Kind: yaml.ScalarNode, Value: "allow"}}}

// startApp is the shell that opens an app's launcher screen.
func startApp(app string, arguments *yaml.Node) []string {
	start := "am start -n \"$a\" -a android.intent.action.MAIN -c android.intent.category.LAUNCHER"
	_ = pairs(arguments, func(k, a *yaml.Node) error { // never fails
		flag := "--es"
		switch a.ShortTag() {
		case "!!bool":
			flag = "--ez"
		case "!!int":
			flag = "--ei"
		case "!!float":
			flag = "--ef"
		}
		start += " " + flag + " " + quote(k.Value) + " " + quote(a.Value)
		return nil
	})
	return []string{
		"a=$(cmd package resolve-activity --brief -a android.intent.action.MAIN -c android.intent.category.LAUNCHER " + app + " | tail -n 1)",
		"wait",
		"case $a in " + app + "/*) " + start + " ;; *) echo 'Error: " + app + " is not installed or has no launcher screen' ;; esac",
	}
}
