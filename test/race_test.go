//go:build race

package test

// rayo itself is built with the race detector when the tests are.
func init() { buildFlags = []string{"-race"} }
