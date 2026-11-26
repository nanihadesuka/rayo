package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
)

// The room needs a moment to show the picture before the walk.
const beforeWalk = 3 * time.Second

const noImage = "none"

func checkCamera(p *reader, s *Step) error {
	n := get(s.Value, keyImage)
	if n == nil || dynamic(n) || n.Value == noImage {
		return needs(keyImage)(p, s)
	}
	if _, err := os.Stat(relative(p.path, n.Value)); err != nil {
		return p.fail(n, "no such file: %s%s", n.Value, nearFile(relative(p.path, n.Value), n.Value))
	}
	return nil
}

// setCamera doesn't fail where there is no such room: a real device's camera sees what it sees.
func setCamera(c *call) error {
	image := arg(c, keyImage)
	if image != noImage {
		var err error
		if image, err = filepath.Abs(relative(c.s.Path, image)); err != nil {
			return err
		}
		if _, err := os.Stat(image); err != nil {
			return fmt.Errorf("no such file: %s", arg(c, keyImage))
		}
	}
	err := c.r.room.reset()
	if err == nil && image != noImage {
		err = c.r.room.show(c.ctx, image)
	}
	if err != nil {
		c.r.warn(2, fmt.Sprintf("camera left as it is: %v", err))
	}
	return nil
}

// room is what an emulator's camera sees: a picture on a wall, and a walk up to it.
type room struct {
	device *adb.Device
	stop   context.CancelFunc // of the walk; nil if nothing is shown
	walked chan error         // how the walk went, once it ends
}

func (r *room) macro(name string) (string, error) {
	folder := adb.SDKFile("emulator", "resources", "macros")
	if folder == "" {
		return "", errors.New("no emulator in the Android SDK")
	}
	return "automation play " + filepath.ToSlash(filepath.Join(folder, name)), nil
}

func (r *room) show(ctx context.Context, image string) error {
	if !r.device.Emulator() {
		return fmt.Errorf("%w, %s is not one", adb.ErrNotEmulator, r.device.Serial)
	}
	begin, err := r.macro("Reset_position")
	if err != nil {
		return err
	}
	walk, err := r.macro("Walk_to_image_room")
	if err != nil {
		return err
	}
	if err := r.device.Console("automation stop-play", "virtualscene-image wall "+filepath.ToSlash(image), begin); err != nil {
		return err
	}
	ctx, r.stop = context.WithCancel(ctx)
	r.walked = make(chan error, 1)
	go func(walked chan<- error) { // the flow goes on meanwhile
		select {
		case <-time.After(beforeWalk):
			walked <- r.device.Console(walk)
		case <-ctx.Done():
			walked <- nil
		}
	}(r.walked)
	return nil
}

// reset puts the room back, if it was changed, after the walk has ended or been called off.
func (r *room) reset() error {
	if r.stop == nil {
		return nil
	}
	r.stop()
	r.stop = nil
	walked := <-r.walked
	begin, err := r.macro("Reset_position")
	if err == nil {
		err = r.device.Console("automation stop-play", "virtualscene-image wall", begin)
	}
	return errors.Join(walked, err)
}
