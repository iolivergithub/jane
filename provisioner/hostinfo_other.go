//go:build !linux

package main

import "errors"

func collectHostInfo() (HostInfo, error) {
	return HostInfo{}, errors.New("collecthostinfo is only supported on Linux")
}
