package adb

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefusal(t *testing.T) {
	for line, want := range map[string]string{
		"KO: Out of range": "Out of range",
		"KO:  OK then":     "OK then",
		"KO":               "refused",
		"KO: ":             "refused",
		"KOala":            "ala",
	} {
		if got := Refusal(line); got != want {
			t.Errorf("Refusal(%q) = %q, want %q", line, got, want)
		}
	}
}

// console is a fake emulator console that answers each line with what answer says.
func console(t *testing.T, answer func(string) string) string {
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
					io.WriteString(c, answer(s.Text())+"\r\n")
				}
			}()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func TestConsole(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := (&Device{Serial: "R5CX"}).Console("geo fix 1 2"); !errors.Is(err, NotEmulator) {
		t.Errorf("real device: %v", err)
	}
	port := console(t, func(line string) string {
		switch {
		case strings.HasPrefix(line, "auth "):
			return "KO: authentication token does not match"
		case strings.HasPrefix(line, "geo"):
			return "KO: Out of range"
		}
		return "OK"
	})
	d := &Device{Serial: "emulator-" + port}
	err := d.Console("geo fix 1 2")
	if err == nil || err.Error() != `the emulator's console refused "geo": Out of range` {
		t.Errorf("refused command: %v", err)
	}
	if err := d.Console("automation stop-play"); err != nil {
		t.Errorf("stop-play may be refused: %v", err)
	}
	os.WriteFile(filepath.Join(home, ".emulator_console_auth_token"), []byte("secret\n"), 0o600)
	err = d.Console("geo fix 1 2")
	if err == nil || !strings.Contains(err.Error(), "refused the token in") || !strings.HasSuffix(err.Error(), "authentication token does not match") {
		t.Errorf("refused token: %v", err)
	}
}
