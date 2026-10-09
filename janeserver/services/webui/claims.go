package webui

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

// showClaims lists claims a page at a time, newest first (without loading claim bodies).
func showClaims(c echo.Context) error {
	p := newPager(c.QueryParam("page"), operations.CountClaims(), listPageSize, "/claims")
	cs, _ := operations.GetClaimsPage(p.Skip, p.Size)

	return c.Render(http.StatusOK, "claims.html", buildClaimsPage(p, cs))
}

func showClaim(c echo.Context) error {
	x, _ := operations.GetClaimByItemID(c.Param("itemid"))

	rs := []structures.Result{}
	if x.ItemID != "" {
		rs, _ = operations.GetResultsByClaimID(x.ItemID)
	}

	return c.Render(http.StatusOK, "claim.html", buildClaimPage(x, rs))
}
