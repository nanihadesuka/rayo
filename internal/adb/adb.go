// Package adb speaks to the adb server directly: starting adb for every command would be slow.
package adb

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Device is one connected to the adb server.
type Device struct{ Serial string }

func adbPath() string {
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}
	name := "adb"
	if runtime.GOOS == "windows" {
		name = "adb.exe"
	}
	if p := SDKFile("platform-tools", name); p != "" {
		return p
	}
	return name
}

func dialServer() (net.Conn, error) {
	port := os.Getenv("ANDROID_ADB_SERVER_PORT")
	if port == "" {
		port = "5037"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	if c, err := net.DialTimeout("tcp", address, 2*time.Second); err == nil {
		return c, nil
	}
	adb := adbPath()
	if out, err := exec.Command(adb, "start-server").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("can't start adb (%s): %v %s", adb, err, out)
	}
	return net.DialTimeout("tcp", address, 5*time.Second)
}

// ask sends one service and checks the answer; the connection then belongs to the service.
func ask(c net.Conn, service string) error {
	if _, err := fmt.Fprintf(c, "%04x%s", len(service), service); err != nil {
		return err
	}
	status := make([]byte, 4)
	if _, err := io.ReadFull(c, status); err != nil {
		return err
	}
	if string(status) == "OKAY" {
		return nil
	}
	message, _ := readBlock(c)
	return fmt.Errorf("adb: %s", message)
}

func readBlock(c net.Conn) (string, error) {
	size := make([]byte, 4)
	if _, err := io.ReadFull(c, size); err != nil {
		return "", err
	}
	n, err := strconv.ParseUint(string(size), 16, 32)
	if err != nil {
		return "", err
	}
	data := make([]byte, n)
	_, err = io.ReadFull(c, data)
	return string(data), err
}

// Devices lists the serials of those ready to use.
func Devices() ([]string, error) {
	c, err := dialServer()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if err := ask(c, "host:devices"); err != nil {
		return nil, err
	}
	list, err := readBlock(c)
	var serials []string
	for _, line := range strings.Split(list, "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == "device" {
			serials = append(serials, f[0])
		}
	}
	return serials, err
}

// Pick is the device with a serial, or the only one; or only among emulators.
func Pick(serial string, emulator bool) (*Device, error) {
	all, err := Devices()
	if err != nil {
		return nil, err
	}
	if serial == "" {
		serial = os.Getenv("ANDROID_SERIAL")
	}
	if emulator {
		real := func(s string) bool { return !(&Device{Serial: s}).Emulator() }
		if serial != "" && real(serial) {
			return nil, fmt.Errorf("%s is not an emulator", serial)
		}
		if all = slices.DeleteFunc(all, real); len(all) == 0 {
			return nil, fmt.Errorf("no emulator running")
		}
	}
	if serial == "" {
		switch len(all) {
		case 0:
			return nil, fmt.Errorf("no device connected")
		case 1:
			return &Device{Serial: all[0]}, nil
		}
		return nil, fmt.Errorf("several devices, choose one with --device: %s", strings.Join(all, ", "))
	}
	for _, s := range all {
		if s == serial {
			return &Device{Serial: s}, nil
		}
	}
	return nil, fmt.Errorf("device %s is not connected", serial)
}

// Open starts a service on the device, such as "exec:ls".
func (d *Device) Open(service string) (net.Conn, error) {
	c, err := dialServer()
	if err != nil {
		return nil, err
	}
	if err = ask(c, "host:transport:"+d.Serial); err == nil {
		err = ask(c, service)
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Reverse makes a port of the device lead to one of the computer.
func (d *Device) Reverse(device, computer int) error {
	return d.reverse(fmt.Sprintf("reverse:forward:tcp:%d;tcp:%d", device, computer))
}

// Unreverse undoes Reverse.
func (d *Device) Unreverse(device int) error {
	return d.reverse(fmt.Sprintf("reverse:killforward:tcp:%d", device))
}

func (d *Device) reverse(service string) error {
	c, err := d.Open(service)
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	// adb has accepted the service; now the device says whether it worked.
	status := make([]byte, 4)
	if _, err := io.ReadFull(c, status); err != nil {
		return fmt.Errorf("adb reverse: %w", err)
	}
	if string(status) != "OKAY" {
		message, _ := readBlock(c)
		return fmt.Errorf("adb reverse: %s", message)
	}
	return nil
}

// Exec runs a shell command and returns its raw output.
func (d *Device) Exec(command string) ([]byte, error) {
	return d.ExecWithin(command, 60*time.Second)
}

// ExecWithin is Exec for what should be quick.
func (d *Device) ExecWithin(command string, limit time.Duration) ([]byte, error) {
	c, err := d.Open("exec:" + command)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(limit))
	return io.ReadAll(c)
}

// Install installs an app, unless that same file is there already. It says whether it did.
func (d *Device) Install(app string, apk []byte) (bool, error) {
	there, err := d.Exec("p=$(pm path " + app + " | sed -n '1s/^package://p'); [ -n \"$p\" ] && sha256sum \"$p\"")
	if err != nil {
		return false, err
	}
	if f := strings.Fields(string(there)); len(f) > 0 && f[0] == fmt.Sprintf("%x", sha256.Sum256(apk)) {
		return false, nil
	}
	const path = "/data/local/tmp/rayo-install.apk"
	if err := d.Push(apk, path); err != nil {
		return false, err
	}
	c, err := d.Open("exec:pm install -r -t " + path + " 2>&1; rm -f " + path)
	if err != nil {
		return false, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Minute))
	out, err := io.ReadAll(c)
	if err == nil && !strings.Contains(string(out), "Success") {
		err = fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return true, err
}

// Push writes a file with adb's sync protocol.
func (d *Device) Push(data []byte, path string) error {
	c, err := d.Open("sync:")
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(60*time.Second + time.Duration(len(data)>>20)*time.Second)) // and 1 s per MB
	var b bytes.Buffer
	part := func(id string, n int) {
		b.WriteString(id)
		binary.Write(&b, binary.LittleEndian, uint32(n))
	}
	target := path + ",33188" // 0644
	part("SEND", len(target))
	b.WriteString(target)
	for rest := data; len(rest) > 0; {
		n := min(len(rest), 64*1024)
		part("DATA", n)
		b.Write(rest[:n])
		rest = rest[n:]
	}
	part("DONE", int(time.Now().Unix()))
	if _, err := c.Write(b.Bytes()); err != nil {
		return err
	}
	head := make([]byte, 8)
	if _, err := io.ReadFull(c, head); err != nil {
		return err
	}
	if string(head[:4]) != "OKAY" {
		message := make([]byte, min(binary.LittleEndian.Uint32(head[4:]), 4096))
		io.ReadFull(c, message)
		return fmt.Errorf("push %s: %s", path, message)
	}
	return nil
}
