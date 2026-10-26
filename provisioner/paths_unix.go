//go:build !windows

package main

func defaultIDFile() string { return "/etc/opt/jane/janeelementid" }
