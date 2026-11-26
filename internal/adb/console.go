package adb

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Emulator says whether the device is one.
func (d *Device) Emulator() bool { return strings.HasPrefix(d.Serial, "emulator-") }

// ErrNotEmulator is what Console says on a real device.
var ErrNotEmulator = errors.New("only works on an emulator")

// Refusal is what a console's "KO: ..." line says, without the KO.
func Refusal(line string) string {
	why := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(line), "KO"), ":"))
	if why == "" {
		return "refused"
	}
	return why
}

// Console runs commands in an emulator's console.
func (d *Device) Console(commands ...string) error {
	port, ok := strings.CutPrefix(d.Serial, "emulator-")
	if !ok {
		return ErrNotEmulator
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err != nil {
		return fmt.Errorf("no answer from the emulator's console on port %s", port)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	in := bufio.NewReader(c)
	answer := func() error {
		for {
			line, err := in.ReadString('\n')
			line = strings.TrimSpace(line)
			switch {
			case err != nil:
				return err
			case line == "OK":
				return nil
			case strings.HasPrefix(line, "KO"):
				return errors.New(Refusal(line))
			}
		}
	}
	if err := answer(); err != nil { // the greeting
		return fmt.Errorf("the emulator's console did not greet: %w", err)
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".emulator_console_auth_token")
		if token, err := os.ReadFile(path); err == nil {
			if _, err := fmt.Fprintf(c, "auth %s\n", strings.TrimSpace(string(token))); err != nil {
				return err
			}
			if err := answer(); err != nil {
				return fmt.Errorf("the emulator's console refused the token in %s: %w", path, err)
			}
		}
	}
	for _, command := range commands {
		if _, err := fmt.Fprintf(c, "%s\n", command); err != nil {
			return err
		}
		err := answer()
		if err != nil && !strings.HasPrefix(command, "automation stop-play") { // nothing playing is fine
			return fmt.Errorf("the emulator's console refused %q: %w", strings.Fields(command)[0], err)
		}
	}
	_, err = fmt.Fprint(c, "quit\n")
	return err
}
