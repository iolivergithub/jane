package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"a10/logging"
	"a10/operations"
	"a10/structures"
)

const STDINTENTSURL = "https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.json"

// stdIntentsTimeout bounds the download so the button cannot hang.
const stdIntentsTimeout = 15 * time.Second

// loadstandardintents downloads the standard intents and adds or updates
// them. A failure (network or TLS error, HTTP status, bad JSON, empty list)
// is written to Jane's log and reported on the intents page. A copy matching
// the installed version is in /etc/opt/jane/standardintents.json for loading
// by hand (#67).
func loadstandardintents(c echo.Context) error {
	var intentfilestruct []structures.Intent

	stdintents, err := getfile(STDINTENTSURL)
	if err == nil {
		err = json.Unmarshal(stdintents, &intentfilestruct)
		if err != nil {
			err = fmt.Errorf("parsing JSON: %w", err)
		} else if len(intentfilestruct) == 0 {
			err = fmt.Errorf("the file contains no intents")
		}
	}
	if err != nil {
		logging.MakeLogEntry("IM", "loadstd", "", "stdintent", "loading standard intents from "+STDINTENTSURL+" failed: "+err.Error())
		return c.Redirect(http.StatusSeeOther, "/intents?stdintents=failed")
	}

	for _, e := range intentfilestruct {
		operations.AddStandardIntent(e)
	}
	logging.MakeLogEntry("IM", "loadstd", "", "stdintent", fmt.Sprintf("loaded %d standard intents from %s", len(intentfilestruct), STDINTENTSURL))

	return c.Redirect(http.StatusSeeOther, "/intents")
}

// getfile fetches url with normal TLS certificate verification and returns
// the body of a 200 response.
func getfile(url string) ([]byte, error) {
	client := &http.Client{Timeout: stdIntentsTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
