//go:build !linux && !windows

package uefi

// github.com/0x5a17ed/uefi reads UEFI variables only on Linux (efivarfs) and
// Windows, and does not build elsewhere (its efivario package has no
// openSafeguard there). On other systems uefi/efivars and uefi/bootconfig
// answer "not supported", as ima/asciilog does on Windows; uefi/eventlog and
// the other APIs work as usual.

import (
	"net/http"
	"runtime"

	"github.com/labstack/echo/v4"
)

// EfivarsSupported reports whether uefi/efivars and uefi/bootconfig work here.
const EfivarsSupported = false

func notSupported(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]interface{}{
		"error": "UEFI variables are not supported by tarzan on " + runtime.GOOS + " (Linux and Windows only)",
	})
}

func Efivars(c echo.Context) error    { return notSupported(c) }
func BootConfig(c echo.Context) error { return notSupported(c) }
