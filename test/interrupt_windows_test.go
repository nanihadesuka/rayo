package test

import (
	"os/exec"
	"syscall"
)

// A group of its own, so that the interruption doesn't reach the test too.
func interruptible(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// interrupt sends Ctrl-Break, which Go treats as Ctrl-C: Windows can't send Ctrl-C to a group.
func interrupt(cmd *exec.Cmd) error {
	ok, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent").Call(syscall.CTRL_BREAK_EVENT, uintptr(cmd.Process.Pid))
	if ok == 0 {
		return err
	}
	return nil
}
