package sys

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"ta10/common"

	"github.com/iolivergithub/jane/lib/machineid"
	"github.com/labstack/echo/v4"
)

type sysinfoReturn struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	NCPU      int    `json:"ncpu"`
	Hostname  string `json:"hostname"`
	Unsafe    bool   `json:"unsafe"`
	Pid       int    `json:"pid"`
	Ppid      int    `json:"ppid"`
	Uid       int    `json:"uid"`
	SessionID string `json:"sessionid"`
	MachineID string `json:"machineid"`
}

func getHostname() string {
	hostname := "?"

	h, err := os.Hostname()
	if err == nil {
		hostname = h
	}

	return hostname
}

func Sysinfo(c echo.Context) error {
	fmt.Println("sysinfo called")

	// the same machine ID as jp records on the element; "" if there is none
	mid, _ := machineid.Get()
	if mid != "" {
		fmt.Printf("Machine id: %v %v\n", len(mid), mid)
	}

	ncpus := runtime.NumCPU()
	s := sysinfoReturn{runtime.GOOS, runtime.GOARCH, ncpus, getHostname(), utilities.IsUnsafe(), os.Getpid(), os.Getppid(), os.Getuid(), utilities.RUNSESSION, mid}

	return c.JSON(http.StatusOK, s)
}
