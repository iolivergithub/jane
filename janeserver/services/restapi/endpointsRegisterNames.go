package restapi

import (
	"fmt"
	"net/http"

	"a10/registernames"

	"github.com/labstack/echo/v4"
)

// Register naming schemes (#101), built into Jane:
//
//	GET /pcrnames       every scheme with its registers, and the default key
//	GET /pcrname/:key   one scheme

type returnPCRNames struct {
	Default string                 `json:"default"`
	Schemes []registernames.Scheme `json:"schemes"`
	Length  int                    `json:"length"`
}

func getPCRNames(c echo.Context) error {
	s := registernames.Schemes()
	return FormattedResponse(c, http.StatusOK, returnPCRNames{registernames.Default, s, len(s)})
}

func getPCRName(c echo.Context) error {
	s, ok := registernames.Get(c.Param("key"))
	if !ok {
		return FormattedResponse(c, http.StatusNotFound, MakeRESTErrorMessage(fmt.Errorf("no register naming scheme %q", c.Param("key"))))
	}
	return FormattedResponse(c, http.StatusOK, s)
}
