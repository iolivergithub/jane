//go:build !windows

package main

func defaultIDFile() string { return "/etc/janeelementid" }
