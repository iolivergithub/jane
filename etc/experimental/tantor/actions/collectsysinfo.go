package actions

import (
	"os"
	"runtime"

	"tantor/structures"
)

func CollectSystemInfo() (structures.HostMachine, error) {
	return structures.HostMachine{runtime.GOOS, runtime.GOARCH, gethostname()}, nil
}

func gethostname() string {
	hostname := "?"

	h, err := os.Hostname()
	if err == nil {
		hostname = h
	}

	return hostname
}
