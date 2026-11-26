// Package agent runs rayo's part on the device and talks to it.
package agent

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/nanihadesuka/rayo/internal/adb"
)

//go:embed agent.jar
var agentJar []byte

// Agent is the part that runs on the device (agent/java). It stops, and undoes what it changed,
// when the connection closes. It does one request at a time.
type Agent struct {
	conn  net.Conn
	lines *bufio.Reader
	log   []string
	dead  error
	spent map[string]int64 // since the last step, in microseconds
}

// Args of a request.
type Args = map[string]any

// Answer to a request.
type Answer struct {
	Error string           `json:"error"`
	Out   string           `json:"out"`
	Text  string           `json:"text"`
	Ready bool             `json:"ready"`
	Spent map[string]int64 `json:"spent"`
}

// Start brings the agent to the device, if it is not there, and starts it.
func Start(d *adb.Device) (*Agent, error) {
	hash := sha256.Sum256(agentJar)
	path := fmt.Sprintf("/data/local/tmp/rayo-%x.jar", hash[:8])
	there, err := awaitGone(d, path)
	if err != nil {
		return nil, err
	}
	if !there {
		// Renamed into place, so an interrupted push is never mistaken for the agent.
		if err := d.Push(agentJar, path+".part"); err != nil {
			return nil, err
		}
		if _, err := d.Exec("mv " + path + ".part " + path); err != nil {
			return nil, err
		}
	}
	// Or Android kills it when the connection closes, before it can undo anything.
	conn, err := d.Open("exec:trap '' HUP; CLASSPATH=" + path + " exec app_process /system/bin rayo.Agent")
	if err != nil {
		return nil, err
	}
	a := &Agent{conn: conn, lines: bufio.NewReaderSize(conn, 64*1024)}
	if err := conn.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil {
		conn.Close()
		return nil, err
	}
	if r, err := a.read(); err != nil || !r.Ready {
		conn.Close()
		return nil, fmt.Errorf("the device agent didn't start: %v\n%s", err, strings.Join(a.log, "\n"))
	}
	return a, nil
}

// Close waits for the agent to undo its changes.
func (a *Agent) Close() {
	if a.dead == nil {
		// It restores the same when the connection closes; this waits until it has.
		_, _ = a.Do(context.Background(), "bye", nil, 5*time.Second)
		a.dead = errors.New("closed")
	}
	a.conn.Close()
}

// awaitGone waits until an agent whose connection was lost has undone its changes and ended.
// Starting another before would take its changes as how the device was. In the same trip, it says
// whether the agent's file is there already. It looks every 200 ms, for up to 10 s.
func awaitGone(d *adb.Device, path string) (there bool, err error) {
	out, err := d.Exec(awaitGoneCommand(path))
	if err != nil {
		return false, err
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 || lines[0] != "gone" {
		return false, errors.New("another rayo is using the device")
	}
	return len(lines) > 1 && lines[1] == path, nil
}

func awaitGoneCommand(path string) string {
	return "i=0; while pgrep -f 'rayo[.]Agent' >/dev/null; do i=$((i+1)); [ $i -gt 50 ] && exit; sleep 0.2; done; echo gone; ls " + path + " 2>/dev/null"
}

// lost gives up: the next answer would belong to this request.
func (a *Agent) lost(err error) (*Answer, error) {
	a.dead = fmt.Errorf("lost the device: %w\n%s", err, strings.Join(a.log, "\n"))
	return &Answer{}, a.dead
}

func (a *Agent) read() (*Answer, error) {
	for {
		line, err := a.lines.ReadString('\n')
		if body, ok := strings.CutPrefix(line, "@@"); ok && err == nil {
			r := &Answer{}
			return r, json.Unmarshal([]byte(body), r)
		}
		if s := strings.TrimSpace(line); s != "" && len(a.log) < 100 {
			a.log = append(a.log, s)
		}
		if err != nil {
			return nil, err
		}
	}
}

// Do sends a request; the device gets timeout to fulfil it. Cancelling loses the agent.
func (a *Agent) Do(ctx context.Context, op string, args Args, timeout time.Duration) (*Answer, error) {
	if a.dead != nil {
		return &Answer{}, a.dead
	}
	r := Args{"op": op, "timeout": timeout.Milliseconds()}
	for k, v := range args {
		r[k] = v
	}
	body, err := json.Marshal(r)
	if err != nil {
		return &Answer{}, err
	}
	now := time.Now()
	if err := errors.Join(a.conn.SetWriteDeadline(now.Add(10*time.Second)), a.conn.SetReadDeadline(now.Add(timeout+30*time.Second))); err != nil {
		return a.lost(err)
	}
	// After the deadlines, so that they can't undo it.
	stop := context.AfterFunc(ctx, func() { _ = a.conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	_, err = a.conn.Write(append(body, '\n'))
	var got *Answer
	if err == nil {
		got, err = a.read()
	}
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return a.lost(err)
	}
	for k, v := range got.Spent {
		if a.spent == nil {
			a.spent = map[string]int64{}
		}
		a.spent[k] += v
	}
	if got.Error != "" {
		return got, errors.New(got.Error)
	}
	return got, nil
}

// Dead is why the agent can't be used any more; nil while it can.
func (a *Agent) Dead() error { return a.dead }

// ResetSpent forgets the time spent so far.
func (a *Agent) ResetSpent() { a.spent = nil }

// Explain says where the time since ResetSpent went.
func (a *Agent) Explain(total time.Duration) string {
	s := a.spent
	if s == nil {
		return ""
	}
	ms := func(k string) int64 { return s[k] / 1000 }
	text := fmt.Sprintf("  [adb+host %d", (total.Microseconds()-s["device"])/1000)
	for _, k := range []string{"read", "cache", "check", "wait", "still", "inject", "shell"} {
		if ms(k) > 0 {
			text += fmt.Sprintf(", %s %d", k, ms(k))
		}
	}
	if s["reads"] > 0 {
		text += fmt.Sprintf(" (%d reads, %d elements)", s["reads"], s["nodes"])
	}
	return text + "]"
}
