package webui

import (
	"a10/configuration"
	"net/http"

	"github.com/labstack/echo/v4"
)

func showLive(c echo.Context) error {

	return c.Render(http.StatusOK, "live.html", configuration.ConfigData)
}
