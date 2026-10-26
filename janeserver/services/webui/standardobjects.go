package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"a10/logging"
	"a10/operations"
	"a10/structures"
)

const STDOBJECTSURL = "https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardobjects/standardobjects.json"

// loadstandardobjects downloads the standard opaque objects (#102) and adds
// or updates them by value: a downloaded object overwrites an existing one
// with the same value, and objects not in the download are never touched or
// removed. A failure (network or TLS error, HTTP status, bad JSON) is written
// to Jane's log and reported on the opaque objects page; an empty list is not
// an error. A copy matching the installed version is in
// /etc/opt/jane/standardobjects.json.
func loadstandardobjects(c echo.Context) error {
	var objs []structures.OpaqueObject

	b, err := getfile(STDOBJECTSURL)
	if err == nil {
		if err = json.Unmarshal(b, &objs); err != nil {
			err = fmt.Errorf("parsing JSON: %w", err)
		}
	}
	if err != nil {
		logging.MakeLogEntry("IM", "loadstd", "", "object", "loading standard opaque objects from "+STDOBJECTSURL+" failed: "+err.Error())
		return c.Redirect(http.StatusSeeOther, "/opaqueobjects?stdobjects=failed")
	}
	if len(objs) == 0 {
		logging.MakeLogEntry("IM", "loadstd", "", "object", "no standard opaque objects to load from "+STDOBJECTSURL)
		return c.Redirect(http.StatusSeeOther, "/opaqueobjects?stdobjects=empty")
	}

	n := 0
	for _, o := range objs {
		if o.Value == "" {
			continue
		}
		if _, err := operations.AddOpaqueObject(o); err != nil {
			logging.MakeLogEntry("IM", "loadstd", o.Value, "object", "loading a standard opaque object failed: "+err.Error())
			continue
		}
		n++
	}
	logging.MakeLogEntry("IM", "loadstd", "", "object", fmt.Sprintf("loaded %d standard opaque objects from %s", n, STDOBJECTSURL))
	return c.Redirect(http.StatusSeeOther, "/opaqueobjects?stdobjects="+strconv.Itoa(n))
}
