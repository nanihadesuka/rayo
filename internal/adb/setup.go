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
	Size      string   `yaml:"size"`
	DPI       int      `yaml:"dpi"`
	FontScale *float64 `yaml:"fontScale"`
	DarkMode  *bool    `yaml:"darkMode"`
}

var screenSize = regexp.MustCompile(`^[1-9]\d{2,3}x[1-9]\d{2,3}$`)

// Check says what is wrong with it.
func (s *Setup) Check() error {
	switch {
	case s.Size != "" && !screenSize.MatchString(s.Size):
		return fmt.Errorf("size is in pixels, such as 1080x2400, not %q", s.Size)
	case s.DPI != 0 && (s.DPI < 72 || s.DPI > 1000):
		return fmt.Errorf("dpi goes from 72 to 1000, not %d", s.DPI)
	case s.FontScale != nil && (*s.FontScale < 0.5 || *s.FontScale > 3):
		return fmt.Errorf("fontScale goes from 0.5 to 3, not %v", *s.FontScale)
	}
	return nil
}

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
	script := append([]string{"trap '' HUP PIPE"}, was...)
	for _, c := range set {
		script = append(script, "{ "+c+"; } 2>&1")
	}
	script = append(script, "sleep 1", "echo rayo:ready", "read -r line", "{ "+strings.Join(back, "; ")+"; } >/dev/null 2>&1", "echo rayo:done")
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
		return nil, fmt.Errorf("couldn't set up the device: %w %s", err, said)
	}
	if said != "" {
		return nil, errors.Join(fmt.Errorf("couldn't set up the device: %s", said), undo())
	}
	return undo, nil
}
