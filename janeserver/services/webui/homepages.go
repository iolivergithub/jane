package webui

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"a10/configuration"
	"a10/operations"
)

type homepagestructure struct {
	Nes           int
	Nps           int
	Nevs          int
	Ncs           int
	Nrs           int
	Nprs          int
	Nhs           int
	Nses          int64
	Nrus          int
	Nlog          int64
	Nmsg          int64
	Szlog         int64
	Cfg           *configuration.ConfigurationStruct
	CmdLineLength int
	CmdLine       []string
}

func homepage(c echo.Context) error {
	var hps homepagestructure

	es, _ := operations.GetElements()
	ps, _ := operations.GetIntents()
	evs, _ := operations.GetExpectedValues()
	cs, _ := operations.GetClaims()
	rs, _ := operations.GetResults()
	nprs := operations.GetProtocols()
	nhs, _ := operations.GetOpaqueObjects()
	nrus := operations.GetRules()

	hps.Nes = len(es)
	hps.Nps = len(ps)
	hps.Nevs = len(evs)
	hps.Ncs = len(cs)
	hps.Nrs = len(rs)

	hps.Nprs = len(nprs)
	hps.Nhs = len(nhs)
	hps.Nrus = len(nrus)

	hps.Nlog = operations.CountLogEntries()
	hps.Nses = operations.CountSessions()
	hps.Nmsg = operations.CountMessages()

	lsz, lerr := os.Stat(configuration.ConfigData.Logging.LogFileLocation)
	if lerr != nil {
		hps.Szlog = -1
	} else {
		hps.Szlog = lsz.Size()
	}

	hps.Cfg = configuration.ConfigData

	hps.CmdLineLength = len(os.Args)
	hps.CmdLine = os.Args

	fmt.Printf("hps is %v\n", hps)

	return c.Render(http.StatusOK, "home.html", hps)
}

func helppage(c echo.Context) error {
	return c.Render(http.StatusOK, "help.html", nil)
}

func aboutpage(c echo.Context) error {
	return c.Render(http.StatusOK, "about.html", nil)
}
