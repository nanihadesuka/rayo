package adb

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Setup is how a device should be while flows run. What is nil or empty stays as it is.
type Setup struct {
	Size      string
	DPI       int
	FontScale *float64
	DarkMode  *bool
}

// ScreenSize is how a size is written.
var ScreenSize = regexp.MustCompile(`^[1-9]\d{2,3}x[1-9]\d{2,3}$`)

// Apply changes the device, and returns what undoes it. The device itself undoes it if the connection
// closes, however this process died.
func (d *Device) Apply(s *Setup) (undo func() error, err error) {
	var was, set, back []string
	if s.Size != "" {
		was = append(was, "size=$(wm size | sed -n 's/^Override size: //p')")
		set = append(set, "wm size "+s.Size)
		back = append(back, "wm size ${size:-reset}")
	}
	if s.DPI != 0 {
		was = append(was, "dpi=$(wm density | sed -n 's/^Override density: //p')")
		set = append(set, fmt.Sprintf("wm density %d", s.DPI))
		back = append(back, "wm density ${dpi:-reset}")
	}
	if s.FontScale != nil {
		was = append(was, "font=$(settings get system font_scale)")
		set = append(set, fmt.Sprintf("settings put system font_scale %v", *s.FontScale))
		back = append(back, "if [ \"$font\" = null ]; then settings delete system font_scale; else settings put system font_scale $font; fi")
	}
	if s.DarkMode != nil {
		was = append(was, "night=$(cmd uimode night | sed 's/.*: //')")
		set = append(set, "cmd uimode night "+map[bool]string{true: "yes", false: "no"}[*s.DarkMode]+" >/dev/null")
		back = append(back, "cmd uimode night $night")
	}
	if len(set) == 0 {
		return func() error { return nil }, nil
	}
	return d.hold(was, set, back, "set up the device", true)
}

// SetProxy makes the device's HTTP proxy an address of it, and returns what puts the one before back. The
// device puts it back by itself if the connection closes, however this process died: a proxy left with
// nothing behind it would leave the device without a network.
func (d *Device) SetProxy(address string) (undo func() error, err error) {
	return d.hold(
		[]string{"proxy=$(settings get global http_proxy)"},
		[]string{"settings put global http_proxy " + address},
		[]string{`if [ "$proxy" = null ] || [ -z "$proxy" ]; then proxy=:0; fi; settings put global http_proxy "$proxy"`},
		"set the device's proxy", false)
}

// hold runs set in a shell that stays open, and back when it is told to or the connection closes.
func (d *Device) hold(was, set, back []string, what string, settle bool) (undo func() error, err error) {
	script := append([]string{"trap '' HUP PIPE"}, was...)
	for _, c := range set {
		script = append(script, "{ "+c+"; } 2>&1")
	}
	if settle { // for the screen to take it
		script = append(script, "sleep 1")
	}
	script = append(script, "echo rayo:ready", "read -r line", "{ "+strings.Join(back, "; ")+"; } >/dev/null 2>&1", "echo rayo:done")
	c, err := d.Open("exec:" + strings.Join(script, "\n"))
	if err != nil {
		return nil, err
	}
	out := bufio.NewReader(c)
	await := func(mark string) (said string, err error) {
		c.SetDeadline(time.Now().Add(20 * time.Second))
		for {
			line, err := out.ReadString('\n')
			if strings.TrimSpace(line) == mark {
				return strings.TrimSpace(said), nil
			}
			said += line
			if err != nil {
				return said, err
			}
		}
	}
	undo = func() error {
		defer c.Close()
		c.SetDeadline(time.Now().Add(20 * time.Second))
		if _, err := io.WriteString(c, "\n"); err != nil {
			return err
		}
		_, err := await("rayo:done")
		return err
	}
	said, err := await("rayo:ready")
	if err != nil {
		c.Close() // which undoes it
		return nil, fmt.Errorf("couldn't %s: %w %s", what, err, said)
	}
	if said != "" {
		return nil, errors.Join(fmt.Errorf("couldn't %s: %s", what, said), undo())
	}
	return undo, nil
}
