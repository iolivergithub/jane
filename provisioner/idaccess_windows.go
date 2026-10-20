//go:build windows

package main

// dirWritable is not checked on Windows: jp runs as Administrator there, and
// access is decided by ACLs that a simple check would not reflect.
func dirWritable(dir string) error { return nil }
