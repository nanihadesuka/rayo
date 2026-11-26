// Package test runs the real rayo against a pretend device. Each file in cases/ holds the input and
// everything that must come out: what rayo prints, what it asks the device, the files it writes.
//
//	go test ./test            check
//	go test ./test -update    write what comes out now; read the diff before committing it
package test

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

var (
	update = flag.Bool("update", false, "write the expected output")
	given  = flag.String("rayo", "", "test this rayo instead of building it")
	rayo   string
)

var buildFlags []string

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(func() int {
		if rayo = *given; rayo != "" {
			return m.Run()
		}
		dir, err := os.MkdirTemp("", "rayo")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)
		rayo = filepath.Join(dir, "rayo")
		if runtime.GOOS == "windows" {
			rayo += ".exe"
		}
		if out, err := exec.Command("go", append(append([]string{"build"}, buildFlags...), "-o", rayo, "../cmd/rayo")...).CombinedOutput(); err != nil {
			fmt.Printf("%s\n", out)
			return 1
		}
		return m.Run()
	}())
}

var section = regexp.MustCompile(`(?m)^-- (.+) --\n`)

type file struct{ name, text string }

// split reads "-- name --" sections.
func split(text string) (list []file) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	at := section.FindAllStringSubmatchIndex(text, -1)
	for i, m := range at {
		end := len(text)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		list = append(list, file{text[m[2]:m[3]], text[m[1]:end]})
	}
	return list
}

var masks = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`\d+ ms`), "# ms"},
	{regexp.MustCompile(`\d+\.\d+ s\b`), "# s"},
	{regexp.MustCompile(`\[adb\+host -?\d+`), "[adb+host #"},
	{regexp.MustCompile(`time="[\d.]+"`), `time="#"`},
	{regexp.MustCompile(`rayo-\d+\.mp4`), "rayo-<pid>.mp4"},
	{regexp.MustCompile(`"seconds": ?[\d.]+`), `"seconds": #`},
	{regexp.MustCompile(`20\d\d-\d\d-\d\dT\d\d:\d\d:\d\d\.\d\d\dZ`), "<now>"},
	{regexp.MustCompile(`\d\d:\d\d:\d\d\.\d\d\d`), "<time>"},
	{regexp.MustCompile(`20\d\d-\d\d-\d\dT\d\d:\d\d:\d\dZ`), "<now>"},
	{regexp.MustCompile(`;tcp:\d+`), ";tcp:<free>"},
	{regexp.MustCompile(`emulator-\d+ \(`), "emulator-5554 ("},
	{regexp.MustCompile(`rayo-[0-9a-f]{16}`), "rayo-<hash>"},
	{regexp.MustCompile(`http://127\.0\.0\.1:\d+`), "http://backend"},
	{regexp.MustCompile(`"value":"[1-9]{8}"`), `"value":"<8 digits>"`},
	{regexp.MustCompile(`"value":"[a-z]{10}@example.com"`), `"value":"<email>"`},
	{regexp.MustCompile(`"value":"[a-z]{5}"`), `"value":"<5 letters>"`},
	{regexp.MustCompile(`"value":"[A-Z][a-z]{5} [A-Z][a-z]{7}"`), `"value":"<name>"`},
	{regexp.MustCompile(`(open|l?stat|CreateFile|GetFileAttributesEx|readlink) ([^:\n]+): (The system cannot find the (file|path) specified\.|no such file or directory)`), "no file $2"},
}

// slashes makes paths look the same on every system.
func slashes(text string) string { return strings.ReplaceAll(text, `\`, "/") }

func masked(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, m := range masks {
		text = m.pattern.ReplaceAllString(text, m.with)
	}
	return text
}

func TestCases(t *testing.T) {
	paths, _ := filepath.Glob("cases/*.txt")
	if len(paths) == 0 {
		t.Fatal("no cases")
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".txt"), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			input, want, _ := strings.Cut(strings.ReplaceAll(string(data), "\r\n", "\n"), "==== out ====\n")
			got := run(t, input)
			if *update {
				write(t, path, input+"==== out ====\n"+got)
			} else if got != want {
				t.Errorf("came out different:\n%s", diff(want, got))
			}
		})
	}
}

// backend answers the calls rayo itself makes: the last part of the path is the status.
func backend(t *testing.T) string {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		body, _ := io.ReadAll(q.Body)
		status := 200
		fmt.Sscanf(q.URL.Path, "/status/%d", &status)
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"method":%q,"body":%q,"type":%q}`, q.Method, body, q.Header.Get("Content-Type"))
	}))
	t.Cleanup(s.Close)
	return s.URL
}

func run(t *testing.T, input string) string {
	dir := filepath.Join(t.TempDir(), "case")
	os.Mkdir(dir, 0o755)
	args, devices, empty, silent, retries, console, health := "test flow.yaml", "emulator-5554\tdevice\n", false, false, false, "", ""
	inputs := map[string]string{}
	var env []string
	for _, f := range split(input) {
		switch f.name {
		case "args":
			args = strings.TrimSpace(f.text)
		case "devices":
			devices = f.text
		case "console": // how the emulator's console answers: fine, or the command it refuses
			console = strings.TrimSpace(f.text)
		case "device without recorder":
			silent = true
		case "with retries":
			retries = true
		case "env": // NAME=value lines for rayo
			env = strings.Fields(f.text)
		case "health":
			health = f.text
		case "device without agent":
			empty = true
		default:
			p := filepath.Join(dir, f.name)
			write(t, p, strings.ReplaceAll(f.text, "$DIR", filepath.ToSlash(dir)))
			inputs[f.name] = f.text
		}
	}
	d := startDevice(t, devices, empty)
	if console != "" {
		devices = "emulator-" + d.startConsole(t, console) + "\tdevice\n"
	}
	d.mu.Lock()
	d.silent, d.devices = silent, devices
	if health != "" {
		d.health = health
	}
	d.mu.Unlock()
	args = strings.ReplaceAll(args, "$BACKEND", backend(t))
	words := strings.Fields(args)
	if words[0] == "test" && !strings.Contains(args, "--retries") && !retries { // most cases aren't about them
		words = append(words, "--retries", "0")
	}
	cmd := d.command(dir, words...)
	cmd.Env = append(cmd.Env, env...)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	var b strings.Builder
	part := func(name, text string) {
		if text != "" {
			text = strings.ReplaceAll(text, filepath.ToSlash(dir), "<dir>")
			fmt.Fprintf(&b, "-- %s --\n%s", name, masked(text))
			if !strings.HasSuffix(text, "\n") {
				b.WriteString("\n")
			}
		}
	}
	part("exit", fmt.Sprintln(cmd.ProcessState.ExitCode()))
	part("stdout", slashes(out.String()))
	part("stderr", slashes(errs.String()))
	d.mu.Lock()
	part("device", strings.Join(d.log, "\n"))
	if len(d.forwards) > 0 {
		part("left behind", fmt.Sprintln(len(d.forwards), "forwards"))
	}
	d.mu.Unlock()
	filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		name, _ := filepath.Rel(dir, p)
		name = filepath.ToSlash(name)
		if _, ok := inputs[name]; ok {
			return nil
		}
		data, err := os.ReadFile(p)
		part("file "+name, string(bytes.TrimPrefix(data, []byte("\x89"))))
		return err
	})
	return b.String()
}

// diff shows the first different line of each, with what comes before.
func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	i := 0
	for i < len(w) && i < len(g) && w[i] == g[i] {
		i++
	}
	cut := func(l []string, to int) string {
		return strings.Join(l[max(0, i-4):max(max(0, i-4), min(len(l), to))], "\n")
	}
	return fmt.Sprintf("line %d\n--- want\n%s\n--- got\n%s", i+1, cut(w, i+8), cut(g, i+8))
}

// Continuous mode runs again when a flow, an included one or a mock changes.
func TestContinuous(t *testing.T) {
	dir := t.TempDir()
	put := func(name, text string) { write(t, filepath.Join(dir, name), text) }
	mapping := `{"request":{"urlPath":"/a"},"response":{"body":"%s"}}`
	put("flow.yaml", "appId: a.b\nmock: mock.json\n---\n- runFlow: sub.yaml\n")
	put("sub.yaml", "appId: a.b\n---\n- tapOn: one\n")
	put("mock.json", fmt.Sprintf(mapping, "1"))
	d := startDevice(t, "emulator-5554\tdevice\n", false)
	cmd := d.command(dir, "test", "-c", "flow.yaml")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	lines := make(chan string, 1000)
	go func() {
		for s := bufio.NewScanner(pipe); s.Scan(); {
			lines <- s.Text()
		}
	}()
	until := func(text string) {
		t.Helper()
		for limit := time.After(10 * time.Second); ; {
			select {
			case l := <-lines:
				if strings.Contains(l, text) {
					return
				}
			case <-limit:
				t.Fatalf("never printed %q", text)
			}
		}
	}
	edit := func(name, text string) {
		time.Sleep(300 * time.Millisecond) // it looks for changes from here on
		put(name, text)
	}
	until("waiting for changes")
	edit("sub.yaml", "appId: a.b\n---\n- tapOn: two\n")
	until("tapOn two")
	until("waiting for changes")
	edit("mock.json", fmt.Sprintf(mapping, "2"))
	until("TOTAL 1: 1 passed")
	until("waiting for changes")
	edit("sub.yaml", "appId: a.b\n---\n- tapOm: two\n")
	until("unknown command")
	edit("sub.yaml", "appId: a.b\n---\n- tapOn: three\n")
	until("tapOn three")
}

// command is rayo in a folder, with only this device and nothing of this computer's.
func (d *fakeDevice) command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(rayo, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ANDROID_ADB_SERVER_PORT="+d.port, "ANDROID_SERIAL=",
		"ANDROID_HOME="+filepath.Join(dir, "sdk"), "HOME="+dir, "USERPROFILE="+dir)
	return cmd
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
