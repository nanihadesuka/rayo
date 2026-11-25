package flow

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
)

// shell runs on the device and returns what it prints.
func (c *call) shell(command string) (string, error) {
	a, err := c.r.agent.Do(c.ctx, "shell", agent.Args{"command": command}, 60*time.Second)
	return a.Out, err
}

// command is like shell, but fails if the exit code isn't 0 or Android complains.
func (c *call) command(command string) error {
	// The exit code comes last.
	out, err := c.shell("(" + command + "\n); echo \"rayo:$?\"")
	if err != nil {
		return err
	}
	i := strings.LastIndex(out, "rayo:")
	if i < 0 {
		return errors.New(strings.TrimSpace(out))
	}
	if code := strings.TrimSpace(out[i+len("rayo:"):]); code != "0" {
		return errors.New(strings.TrimSpace("exit code " + code + "\n" + out[:i]))
	}
	return complaint(out[:i])
}

// setAirplane notes how airplane mode was, the first time, so that it can be put back.
func setAirplane(c *call) error {
	if c.r.airplane == "" {
		was, err := c.r.device.Exec("cmd connectivity airplane-mode")
		if err != nil {
			return err
		}
		state := strings.TrimSpace(string(was))
		if state != "enabled" && state != "disabled" {
			return fmt.Errorf("can't tell whether airplane mode is on: %q", state)
		}
		c.r.airplane = state
	}
	return c.command("cmd connectivity airplane-mode " + map[bool]string{true: "enable", false: "disable"}[arg(c, keyOn)])
}

// resetAirplane doesn't need the agent, which may be lost.
func (r *Runner) resetAirplane() error {
	if r.airplane == "" {
		return nil
	}
	out, err := r.device.Exec("cmd connectivity airplane-mode " + strings.TrimSuffix(r.airplane, "d"))
	if err == nil {
		err = complaint(string(out))
	}
	return err
}

// setOrientation notes how rotation was set, the first time, so that it can be put back.
func setOrientation(c *call) error {
	if c.r.turned == "" {
		was, err := c.r.device.Exec("settings get system accelerometer_rotation; settings get system user_rotation")
		if err != nil {
			return err
		}
		c.r.turned = strings.Join(strings.Fields(string(was)), " ")
		if len(strings.Fields(c.r.turned)) != 2 {
			c.r.turned = ""
			return fmt.Errorf("can't tell how the screen is turned: %q", strings.TrimSpace(string(was)))
		}
	}
	return c.command(fmt.Sprintf("settings put system accelerometer_rotation 0; settings put system user_rotation %d", arg(c, keyTurned)))
}

// resetOrientation doesn't need the agent, which may be lost. A setting that was unset ("null") is left as it is now.
func (r *Runner) resetOrientation() error {
	auto, user, _ := strings.Cut(r.turned, " ")
	if r.turned == "" {
		return nil
	}
	var put []string
	if user != "null" {
		put = append(put, "settings put system user_rotation "+user)
	}
	if auto != "null" {
		put = append(put, "settings put system accelerometer_rotation "+auto)
	}
	if len(put) == 0 {
		return nil
	}
	out, err := r.device.Exec(strings.Join(put, "; "))
	if err == nil {
		err = complaint(string(out))
	}
	return err
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func (r *Runner) screenshot() ([]byte, error) {
	png, err := r.device.Exec("screencap -p")
	if err != nil {
		return nil, err
	}
	// A warning may come first on devices with several displays.
	i := bytes.Index(png, []byte("\x89PNG"))
	if i < 0 {
		return nil, errors.New("the device gave no screenshot")
	}
	return png[i:], nil
}

func (r *Runner) saveScreenshot(path string) error {
	if !r.wants(Screenshot) {
		return nil
	}
	png, err := r.screenshot()
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, png, 0o644)
	}
	if err == nil {
		r.wrote(path)
	}
	return err
}

// Place is a location in degrees.
type Place struct{ lat, lng float64 }

func (p Place) String() string { return fmt.Sprintf("%.7f,%.7f", p.lat, p.lng) }

func (p Place) check() error {
	if math.Abs(p.lat) > 90 || math.Abs(p.lng) > 180 {
		return fmt.Errorf("not a location: %s", p)
	}
	return nil
}

func place(text string) (p Place, err error) {
	parts := strings.Split(text, ",")
	if len(parts) == 2 {
		var e1, e2 error
		p.lat, e1 = decimal(parts[0])
		p.lng, e2 = decimal(parts[1])
		if e1 == nil && e2 == nil && p.check() == nil {
			return p, nil
		}
	}
	return Place{}, fmt.Errorf("not a location: %q", text)
}

func (a Place) meters(b Place) float64 {
	const rad = math.Pi / 180
	dLat, dLng := (b.lat-a.lat)*rad, (b.lng-a.lng)*rad
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(a.lat*rad)*math.Cos(b.lat*rad)*math.Pow(math.Sin(dLng/2), 2)
	return 2 * 6371000 * math.Asin(math.Min(1, math.Sqrt(h)))
}

// travel moves at a real speed, in metres per second.
func travel(c *call) error {
	c.r.located = true
	speed := argOr(c, keySpeed, 7)
	points := arg(c, keyPoints)
	const tick = 500 * time.Millisecond
	for i, to := range points {
		if i > 0 {
			from := points[i-1]
			seconds := from.meters(to) / speed
			for start := time.Now(); ; {
				f := time.Since(start).Seconds() / seconds
				if seconds == 0 || f >= 1 {
					break
				}
				at := Place{from.lat + (to.lat-from.lat)*f, from.lng + (to.lng-from.lng)*f}
				if err := c.do("location", agent.Args{"place": at.String()}); err != nil {
					return err
				}
				select {
				case <-time.After(min(tick, time.Duration((1-f)*seconds*float64(time.Second)))):
				case <-c.ctx.Done():
					return c.ctx.Err()
				}
			}
		}
		if err := c.do("location", agent.Args{"place": to.String()}); err != nil {
			return err
		}
	}
	return nil
}
