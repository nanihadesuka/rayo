package test

import (
	"flag"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var real = flag.String("emulator", "", "also test on this emulator, such as emulator-5554")

// What a pretend device can't tell: that the device really is put back, also when rayo is killed.
//
//	go test ./test -run Real -emulator emulator-5554
func TestRealDevicePutBack(t *testing.T) {
	if *real == "" {
		t.Skip("no -emulator")
	}
	if !strings.HasPrefix(*real, "emulator-") {
		t.Fatal("only on an emulator")
	}
	shell := func(command string) string {
		t.Helper()
		out, err := exec.Command("adb", "-s", *real, "shell", command).Output()
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		return strings.Join(strings.Fields(string(out)), " ")
	}
	const scales = "for s in window_animation_scale transition_animation_scale animator_duration_scale; do settings get global $s; done"
	const state = "wm size; wm density; settings get system font_scale; cmd uimode night; " + scales
	before := strings.Fields(shell(scales))
	for i, name := range []string{"window_animation_scale", "transition_animation_scale", "animator_duration_scale"} {
		shell("settings put global " + name + " 1") // so that a change shows
		defer func() {
			if before[i] == "null" {
				shell("settings delete global " + name)
			} else {
				shell("settings put global " + name + " " + before[i])
			}
		}()
	}
	want := shell(state)
	dir := t.TempDir()
	put := func(name, text string) { write(t, filepath.Join(dir, name), text) }
	put("s.rayo.yaml", "device: { size: 720x1600, dpi: 280, fontScale: 1.15, darkMode: true }\nrecord: true\nflows: [a.yaml]\n")
	steps := "appId: com.android.settings\n---\n- setLocation: { latitude: 41.4, longitude: 2.2 }\n- launchApp\n- extendedWaitUntil: { visible: { id: not_there }, timeout: "
	for name, c := range map[string]struct {
		wait string
		kill bool
	}{"fails": {"2s", false}, "killed": {"60s", true}} {
		t.Run(name, func(t *testing.T) {
			put("a.yaml", steps+c.wait+" }\n")
			cmd := exec.Command(rayo, "test", "--device", *real, "--emulator", "--retries", "0", "s.rayo.yaml")
			cmd.Dir = dir
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if c.kill {
				time.Sleep(12 * time.Second)
				if got := shell("wm size"); !strings.Contains(got, "720x1600") {
					t.Errorf("not set up: %s", got)
				}
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait() // it fails, as it should
			got := ""
			for range 15 {
				if got = shell(state); got == want {
					break
				}
				time.Sleep(time.Second)
			}
			if got != want {
				t.Errorf("left as\n%s\nwas\n%s", got, want)
			}
			for what, command := range map[string]string{
				"the agent":    "pgrep -f 'rayo[.]Agent' || true",
				"the recorder": "pgrep screenrecord || true",
			} {
				if left := shell(command); left != "" {
					t.Errorf("%s still runs: %s", what, left)
				}
			}
		})
	}
}
