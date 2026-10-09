package webui

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

// showResults lists results a page at a time, newest first. Expected values,
// elements and intents are looked up once per page rather than once per row.
func showResults(c echo.Context) error {
	p := newPager(c.QueryParam("page"), operations.CountResults(), listPageSize, "/results")
	rs, _ := operations.GetResultsPage(p.Skip, p.Size)

	return c.Render(http.StatusOK, "results.html", buildResultsPage(p, rs, dbLookups()))
}

func showResult(c echo.Context) error {
	r, _ := operations.GetResultByItemID(c.Param("itemid"))

	siblings := []structures.Result{}
	if r.ClaimID != "" {
		siblings, _ = operations.GetResultsByClaimID(r.ClaimID)
	}

	return c.Render(http.StatusOK, "result.html", buildResultPage(r, siblings, dbLookups()))
}
