//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// defaultIDFile is %ProgramData%\jane\janeelementid.
func defaultIDFile() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "jane", "janeelementid")
}
