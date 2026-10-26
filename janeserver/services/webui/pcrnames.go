package webui

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/registernames"
)

// Register naming schemes (#101): the list and one scheme. Built in, read only.

func showPCRNames(c echo.Context) error {
	page := struct {
		Default string
		Schemes []registernames.Scheme
	}{registernames.Default, registernames.Schemes()}
	return c.Render(http.StatusOK, "pcrnames.html", page)
}

func showPCRName(c echo.Context) error {
	s, ok := registernames.Get(c.Param("key"))
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/pcrnames")
	}
	return c.Render(http.StatusOK, "pcrname.html", s)
}
