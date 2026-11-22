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

// Console runs commands in an emulator's console.
func (d *Device) Console(commands ...string) error {
	port, ok := strings.CutPrefix(d.Serial, "emulator-")
	if !ok {
		return errors.New("not an emulator")
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err != nil {
		return errors.New("no answer from the emulator's console")
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
				return errors.New(strings.TrimSpace(strings.TrimLeft(line, "KO: ")))
			}
		}
	}
	if err := answer(); err != nil { // the greeting
		return err
	}
	if home, err := os.UserHomeDir(); err == nil {
		if token, err := os.ReadFile(filepath.Join(home, ".emulator_console_auth_token")); err == nil {
			commands = append([]string{"auth " + strings.TrimSpace(string(token))}, commands...)
		}
	}
	for _, command := range commands {
		if _, err := fmt.Fprintf(c, "%s\n", command); err != nil {
			return err
		}
		err := answer()
		if err != nil && !strings.HasPrefix(command, "automation stop-play") { // nothing playing is fine
			return err
		}
	}
	_, err = fmt.Fprint(c, "quit\n")
	return err
}
