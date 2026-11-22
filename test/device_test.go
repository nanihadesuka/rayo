package test

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDevice is an adb server with a device behind it. It writes down everything rayo asks for, so a
// test also shows how many trips to the device a flow takes.
//
// The screen is pretend. The id of an element says how it behaves:
//
//	missing         never on screen
//	stays           never goes away
//	flaky 2         its first two taps fail
//	after 2 swipes  on screen after two swipes
//	GET /a          tapping it makes the app call that; a body may follow
//	hangs           tapping it never ends
//	app crashes     tapping it does that, and fails
//	crash           tapping it cuts the connection to the agent
//
// Anything else is on screen.
type fakeDevice struct {
	port    string
	devices string
	empty   bool // without the agent yet

	mu       sync.Mutex
	log      []string
	forwards map[int]int
	taps     map[string]int
	swipes   int
	crashed  bool
	killed   chan bool
	silent   bool // its recorder doesn't start
}

func startDevice(t *testing.T, devices string, empty bool) *fakeDevice {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	d := &fakeDevice{devices: devices, empty: empty, forwards: map[int]int{}, taps: map[string]int{}, killed: make(chan bool, 9)}
	_, d.port, _ = net.SplitHostPort(l.Addr().String())
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go d.serve(c)
		}
	}()
	return d
}

// startConsole returns the port of an emulator's console, which refuses commands that start so.
func (d *fakeDevice) startConsole(t *testing.T, refused string) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				io.WriteString(c, "Android Console: hello\r\nOK\r\n")
				for s := bufio.NewScanner(c); s.Scan() && s.Text() != "quit"; {
					d.note("console %s", s.Text())
					if strings.HasPrefix(s.Text(), refused) {
						io.WriteString(c, "KO: on purpose\r\n")
					} else {
						io.WriteString(c, "OK\r\n")
					}
				}
			}()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func (d *fakeDevice) note(format string, a ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.log = append(d.log, fmt.Sprintf(format, a...))
}

func (d *fakeDevice) serve(c net.Conn) {
	defer c.Close()
	in := bufio.NewReader(c)
	service := func() string {
		size := make([]byte, 4)
		if _, err := io.ReadFull(in, size); err != nil {
			return ""
		}
		n, _ := strconv.ParseUint(string(size), 16, 32)
		data := make([]byte, n)
		io.ReadFull(in, data)
		return string(data)
	}
	s := service()
	if s == "host:devices" {
		fmt.Fprintf(c, "OKAY%04x%s", len(d.devices), d.devices)
		return
	}
	if !strings.HasPrefix(s, "host:transport:") {
		return
	}
	io.WriteString(c, "OKAY")
	s = service()
	io.WriteString(c, "OKAY")
	if command, ok := strings.CutPrefix(s, "exec:"); ok {
		d.exec(c, in, command)
		return
	}
	if s == "sync:" {
		d.push(c, in)
		return
	}
	d.note("adb %s", s)
	var device, computer int
	if n, _ := fmt.Sscanf(s, "reverse:forward:tcp:%d;tcp:%d", &device, &computer); n == 2 {
		if device == 9 {
			io.WriteString(c, "FAIL0006denied")
			return
		}
		d.mu.Lock()
		d.forwards[device] = computer
		d.mu.Unlock()
	} else if n, _ := fmt.Sscanf(s, "reverse:killforward:tcp:%d", &device); n == 1 {
		d.mu.Lock()
		delete(d.forwards, device)
		d.mu.Unlock()
	}
	io.WriteString(c, "OKAY")
}

func (d *fakeDevice) exec(c net.Conn, in *bufio.Reader, command string) {
	d.note("adb exec:%s", command)
	switch {
	case strings.HasPrefix(command, "trap '' HUP; CLASSPATH="):
		d.agent(c, in)
	case strings.HasPrefix(command, "ls ") && !d.empty:
		io.WriteString(c, strings.Fields(command)[1]+"\n")
	case strings.HasPrefix(command, "screencap"):
		io.WriteString(c, "a warning\n\x89PNG...")
	case strings.HasPrefix(command, "trap"): // a setup, until it is told to undo it
		if strings.Contains(command, "wm size 999x999") {
			io.WriteString(c, "Error: bad size\n")
		}
		io.WriteString(c, "rayo:ready\n")
		in.ReadString('\n')
		d.note("  the device is put back")
		io.WriteString(c, "rayo:done\n")
	case strings.HasPrefix(command, "l=180"): // the recorder, until it is killed
		if d.silent {
			return
		}
		io.WriteString(c, "4321\n")
		select {
		case <-d.killed:
		case <-time.After(20 * time.Second):
		}
	case strings.HasPrefix(command, "kill -INT 4321"):
		d.killed <- true
	case strings.HasPrefix(command, "cat "):
		io.WriteString(c, "a video "+strings.Repeat(".", 100))
	case strings.HasPrefix(command, "for i in"): // has the lost agent ended?
		io.WriteString(c, "gone\n")
	case command == "cmd connectivity airplane-mode":
		io.WriteString(c, "disabled\n")
	case strings.HasPrefix(command, "pm install"):
		io.WriteString(c, "Success\n")
	case strings.HasPrefix(command, "date"):
		io.WriteString(c, "11-22 10:00:00\n")
	case strings.HasPrefix(command, "logcat") && d.crashed:
		io.WriteString(c, `11-21 09:00:00.000  1  1 E AndroidRuntime: FATAL EXCEPTION: before
11-21 09:00:00.000  1  1 E AndroidRuntime: Process: a.b, PID: 1
11-22 10:00:00.000  1  1 E AndroidRuntime: FATAL EXCEPTION: main
11-22 10:00:00.000  1  1 E AndroidRuntime: Process: a.b, PID: 1
11-22 10:00:00.000  1  1 E AndroidRuntime: java.lang.IllegalStateException: oops
`)
	case strings.HasPrefix(command, "logcat"):
		io.WriteString(c, "the log")
	}
}

func (d *fakeDevice) push(c net.Conn, in *bufio.Reader) {
	head := make([]byte, 8)
	size := 0
	for {
		if _, err := io.ReadFull(in, head); err != nil {
			return
		}
		n := int(binary.LittleEndian.Uint32(head[4:]))
		switch string(head[:4]) {
		case "SEND":
			name := make([]byte, n)
			io.ReadFull(in, name)
			d.note("adb push %s", name)
		case "DATA":
			io.CopyN(io.Discard, in, int64(n))
			size += n
		case "DONE":
			d.note("adb pushed something: %v", size > 1000)
			io.WriteString(c, "OKAY\x00\x00\x00\x00")
			return
		}
	}
}

func (d *fakeDevice) agent(c net.Conn, in *bufio.Reader) {
	io.WriteString(c, "noise before\n@@{\"ready\":true}\n")
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return
		}
		var q map[string]any
		json.Unmarshal([]byte(line), &q)
		op, _ := q["op"].(string)
		delete(q, "op")
		rest, _ := json.Marshal(q)
		d.note("%s %s", op, rest)
		answer := d.act(op, q)
		if answer == nil {
			return
		}
		data, _ := json.Marshal(answer)
		fmt.Fprintf(c, "@@%s\n", data)
	}
}

var appCall = regexp.MustCompile(`^[A-Z]+ [:/]`)

var numbered = regexp.MustCompile(`^(flaky|after) (\d+)`)

// act returns what the agent answers, or nil to cut the connection.
func (d *fakeDevice) act(op string, q map[string]any) map[string]any {
	ok := map[string]any{"spent": map[string]int{"device": 1000, "wait": 2000}}
	fail := func(message string) map[string]any { return map[string]any{"error": message} }
	sel, _ := q["sel"].(map[string]any)
	id, _ := sel["id"].(string)
	m := numbered.FindStringSubmatch(id)
	n := 0
	if m != nil {
		n, _ = strconv.Atoi(m[2])
	}
	switch op {
	case "tree":
		ok["out"] = "a screen\n"
	case "shell":
		command := q["command"].(string)
		switch {
		case strings.Contains(command, "not.installed"):
			ok["out"] = "Error: not.installed is not installed"
		case strings.Contains(command, "NO_SUCH"):
			ok["out"] = "Error: a.b has no permission NO_SUCH that can be changed\n"
		case strings.Contains(command, "boom"):
			ok["out"] = "it broke\nrayo:1\n"
		case strings.Contains(command, "nobody.opens"):
			ok["out"] = "Error: Activity not started, unable to resolve Intent\nrayo:0\n"
		case strings.Contains(command, "rayo:"):
			ok["out"] = "fine\nrayo:0\n"
		case strings.Contains(command, "pm clear"):
			ok["out"] = "Success"
		}
	case "swipe":
		d.swipes++
	case "gone":
		if id == "stays" {
			return fail("still on screen: stays")
		}
	case "bye":
		return ok
	}
	switch {
	case id == "missing" && op != "gone":
		return fail("not on screen: missing")
	case id == "hangs" && op == "tap":
		time.Sleep(time.Minute)
	case id == "app crashes" && op == "tap":
		d.crashed = true
		return fail("not on screen: home")
	case id == "crash" && op == "tap":
		return nil
	case m != nil && m[1] == "after" && d.swipes < n:
		return fail("not on screen: " + id)
	case m != nil && m[1] == "flaky" && op == "tap":
		if d.taps[id]++; d.taps[id] <= n {
			return fail("flaked")
		}
	case appCall.MatchString(id) && op == "tap":
		d.app(id)
	}
	if op == "wait" {
		ok["text"] = "text of " + id
	}
	return ok
}

// app makes a call as the app would: through the device's port 8080, or the one in "GET :9000/a".
func (d *fakeDevice) app(call string) {
	parts := strings.SplitN(call, " ", 3)
	port, path := 8080, parts[1]
	if rest, ok := strings.CutPrefix(path, ":"); ok {
		i := strings.Index(rest, "/")
		port, _ = strconv.Atoi(rest[:i])
		path = rest[i:]
	}
	body := ""
	if len(parts) == 3 {
		body = parts[2]
	}
	d.mu.Lock()
	to, open := d.forwards[port]
	d.mu.Unlock()
	if !open {
		d.note("  app: %s -> refused", call)
		return
	}
	q, err := http.NewRequest(parts[0], fmt.Sprintf("http://127.0.0.1:%d%s", to, path), strings.NewReader(body))
	if err != nil {
		d.note("  app: %s -> %v", call, err)
		return
	}
	q.Header.Set("X-App", "fake")
	client := http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	res, err := client.Do(q)
	if err != nil {
		d.note("  app: %s -> no answer", call)
		return
	}
	defer res.Body.Close()
	got, _ := io.ReadAll(res.Body)
	d.note("  app: %s -> %d %s %s", call, res.StatusCode, res.Header.Get("Content-Type"), got)
}
