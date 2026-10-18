//go:build !windows

package main

func defaultIDFile() string { return "/etc/opt/jane/janeelementid" }

// defaultLegacyIDFile is where jp kept the element ID before 1.5.3; update
// and attest migrate it (to be removed, #81).
func defaultLegacyIDFile() string { return "/etc/janeelementid" }
