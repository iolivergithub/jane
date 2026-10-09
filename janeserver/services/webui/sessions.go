package webui

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

// showSessions lists sessions a page at a time, newest first, with the
// claims and results recorded for each.
func showSessions(c echo.Context) error {
	p := newPager(c.QueryParam("page"), operations.CountSessions(), listPageSize, "/sessions")
	ss, _ := operations.GetSessionsPage(p.Skip, p.Size)

	ids := make([]string, 0, len(ss))
	for _, s := range ss {
		ids = append(ids, s.ItemID)
	}
	cs, _ := operations.GetClaimsBySessionIDs(ids)
	rs, _ := operations.GetResultsBySessionIDs(ids)

	return c.Render(http.StatusOK, "sessions.html", buildSessionsPage(p, ss, cs, rs))
}

func showSession(c echo.Context) error {
	s, _ := operations.GetSessionByItemID(c.Param("itemid"))

	cs, _ := operations.GetClaimsBySessionIDs([]string{s.ItemID})
	rs, _ := operations.GetResultsBySessionIDs([]string{s.ItemID})

	return c.Render(http.StatusOK, "session.html", buildSessionPage(s, cs, rs, dbLookups()))
}

// dbLookups resolves related items from the database.
func dbLookups() *lookups {
	return &lookups{
		Element: func(id string) (structures.Element, bool) {
			e, err := operations.GetElementByItemID(id)
			return e, err == nil
		},
		Intent: func(id string) (structures.Intent, bool) {
			i, err := operations.GetIntentByItemID(id)
			return i, err == nil
		},
		ExpectedValue: func(id string) (structures.ExpectedValue, bool) {
			ev, err := operations.GetExpectedValueByItemID(id)
			return ev, err == nil
		},
	}
}
