//go:build !windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// dirWritable reports whether this process may create files in dir, or in
// its nearest existing parent if dir does not exist yet (writeIDFile creates
// it). jp create and update check this before contacting Jane, so a run
// without root stops before registering an element whose ID could not be
// saved.
func dirWritable(dir string) error {
	d := dir
	for {
		if _, err := os.Stat(d); err == nil {
			break
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	return unix.Access(d, unix.W_OK)
}
