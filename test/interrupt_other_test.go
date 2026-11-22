//go:build !windows

package test

import (
	"os"
	"os/exec"
)

func interruptible(*exec.Cmd) {}

func interrupt(cmd *exec.Cmd) error { return cmd.Process.Signal(os.Interrupt) }
