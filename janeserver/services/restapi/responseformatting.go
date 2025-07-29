package restapi

import (
	"github.com/labstack/echo/v4"
)

// Context, int = http.Status and whatever data
func FormattedResponse(c echo.Context, s int, v interface{}) error {
	asform := c.QueryParam("form")

	switch asform {
	case "xml":
		return c.XML(s, v)
	case "json":
		return c.JSON(s, v)
	case "prettyxml":
		return c.XMLPretty(s, v, "   ")
	case "prettyjson":
		return c.JSONPretty(s, v, "   ")
	default:
		return c.JSON(s, v)
	}

}
