package main

import "os"

// alive says whether a process is running.
func alive(pid int) bool {
	p, err := os.FindProcess(pid) // opens it, which fails if there is none
	if err != nil {
		return false
	}
	p.Release()
	return true
}
