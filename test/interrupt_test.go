package test

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Ctrl-C stops a flow wherever it is, and what was set up on the device is undone.
func TestInterrupt(t *testing.T) {
	steps := map[string]string{
		"a wait on the device":  `tapOn: { id: hangs }`,
		"a wait for a request":  `waitForRequest: /never`,
		"a trip":                `travel: { points: ["0,0", "1,1"], speed: 1 }`,
		"a script":              `evalScript: "${while (true) {}}"`,
		"an endless repetition": `repeat: { while: { true: "${true}" }, commands: [ back ] }`,
	}
	for name, step := range steps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			write(t, filepath.Join(dir, "flow.yaml"), "appId: a.b\nmock: mock.json\n---\n- back\n- "+step+"\n")
			write(t, filepath.Join(dir, "mock.json"), `{"request":{"urlPath":"/a"},"response":{}}`)
			d := startDevice(t, "emulator-5554\tdevice\n", false)
			cmd := d.command(dir, "test", "flow.yaml")
			var errs bytes.Buffer
			cmd.Stderr = &errs
			interruptible(cmd)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			lines := bufio.NewScanner(out)
			for lines.Scan() && !strings.Contains(lines.Text(), "✓ back") {
			}
			read := make(chan struct{})
			go func() { // or rayo would stop at a full pipe
				defer close(read)
				for lines.Scan() {
				}
			}()
			time.Sleep(200 * time.Millisecond) // into the step
			if err := interrupt(cmd); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				<-read
				done <- cmd.Wait()
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatal("it didn't stop")
			}
			if code := cmd.ProcessState.ExitCode(); code != 130 || strings.TrimSpace(errs.String()) != "interrupted" {
				t.Errorf("exit %d: %s", code, errs.String())
			}
			d.mu.Lock()
			defer d.mu.Unlock()
			if len(d.forwards) > 0 {
				t.Error("left the port forwarded")
			}
		})
	}
}
