package flow

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Recording is what a flow or a suite says of it: on or off, and where to.
type Recording struct {
	On   bool
	Path string // empty for the default
}

// readRecording reads true, false or a path.
func readRecording(n *yaml.Node) (*Recording, error) {
	switch {
	case n.Kind != yaml.ScalarNode || n.Value == "":
		return nil, errors.New("takes true, false or a path")
	case n.ShortTag() == "!!bool":
		var on bool
		err := n.Decode(&on)
		return &Recording{On: on}, err
	}
	return &Recording{true, n.Value}, nil
}

// video is the one being recorded.
type video struct {
	conn net.Conn
	out  *bufio.Reader
	pid  int
	on   string // file on the device
	to   string // and here
}

// recordingPath is where a video goes: the default has the attempt's name, which no other has.
func (r *Runner) recordingPath(path string) string {
	name := strings.TrimSuffix(filepath.FromSlash(path), ".mp4")
	if filepath.IsAbs(name) {
		return name + ".mp4"
	}
	return r.named(filepath.ToSlash(name), "mp4")
}

// smallSize is the shell that makes the short side of the video 480, as it is only to see what happened:
// the device encodes it, so nothing has to be compressed later. Encoders want multiples of 16.
const smallSize = "r=$(wm size | sed -n '$s/.*: //p'); w=${r%x*}; h=${r#*x}; n=$w; [ $h -lt $n ] && n=$h; s=; " +
	"[ $n -gt 480 ] && s=\"--size $((w*480/n/16*16))x$((h*480/n/16*16))\"; "

// startRecording uses Android's recorder. It stops by itself if the connection closes.
func (r *Runner) startRecording(to string) error {
	if r.video != nil {
		// The same file again, as when a live session replays a flow from its start: it goes on.
		if filepath.Clean(r.video.to) == filepath.Clean(to) {
			return nil
		}
		return fmt.Errorf("a recording to %s is going on", filepath.ToSlash(r.video.to))
	}
	v := &video{on: fmt.Sprintf("/data/local/tmp/rayo-%d.mp4", os.Getpid()), to: to}
	var err error
	// Before Android 11 or so it can't go on for over three minutes.
	if v.conn, err = r.device.Open("exec:l=180; screenrecord --help 2>&1 | grep -q 'remove the time limit' && l=1800; " +
		smallSize +
		"echo $$; exec screenrecord $s --bit-rate 600000 --time-limit $l " + v.on + " 2>&1"); err != nil {
		return err
	}
	v.out = bufio.NewReader(v.conn)
	v.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, err := v.out.ReadString('\n')
	if err == nil {
		v.pid, err = strconv.Atoi(strings.TrimSpace(line))
	}
	if err != nil {
		v.conn.Close()
		return fmt.Errorf("the recording didn't start: %w", err)
	}
	r.video = v
	return nil
}

// stopRecording saves the video. It doesn't need the agent, which may be lost.
func (r *Runner) stopRecording() error {
	v := r.video
	if v == nil {
		return errors.New("nothing is being recorded")
	}
	r.video = nil
	defer v.conn.Close()
	_, err := r.device.Exec(fmt.Sprintf("kill -INT %d 2>/dev/null", v.pid))
	if err != nil {
		return err
	}
	v.conn.SetReadDeadline(time.Now().Add(20 * time.Second)) // it ends the file first
	said, err := io.ReadAll(v.out)
	if err != nil {
		return fmt.Errorf("the recorder doesn't end: %w", err)
	}
	data, err := r.device.Exec("cat " + v.on + "; rm -f " + v.on)
	if err != nil {
		return err
	}
	if len(data) < 100 { // not a video, but what went wrong
		return fmt.Errorf("the device recorded nothing: %s", strings.TrimSpace(string(said)+string(data)))
	}
	if !r.wants(Video) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(v.to), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(v.to, data, 0o644); err != nil {
		return err
	}
	r.recordings = append(r.recordings, filepath.ToSlash(v.to))
	r.wrote(v.to)
	return nil
}

// begin does what the header says, before any step. The flow's word wins over the suite's.
func (r *Runner) begin(f *Flow) error {
	if what := f.Recording; what != nil {
		if what.On {
			if err := r.startRecording(r.recordingPath(what.Path)); err != nil {
				return err
			}
		}
	} else if all := r.Recording; all != nil && all.On {
		to := r.recordingPath("")
		if all.Path != "" { // a folder
			to = filepath.Join(all.Path, filepath.Base(to))
		}
		if err := r.startRecording(to); err != nil {
			return err
		}
	}
	if r.Network == "" && strings.Contains(f.Network, "${") {
		way, err := r.script.fill(f.Network)
		if err != nil {
			return problem(f.Path, f.NetworkLine, "network: %v", err)
		}
		if way != "" && way != Interceptor && way != Proxy {
			return problem(f.Path, f.NetworkLine, "network: takes interceptor or proxy, not %q", way)
		}
		r.backend.proxy = way == Proxy
	}
	return r.loadMocks(f)
}
