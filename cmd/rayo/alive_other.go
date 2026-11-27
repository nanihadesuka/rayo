//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// alive says whether a process is running.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
