package webui

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"

	"a10/auth"
	"a10/configuration"
	"a10/logging"
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

	page := buildSessionsPage(p, ss, cs, rs)
	now := time.Now()
	for i := range page.Rows {
		page.Rows[i].Long = isLongSession(page.Rows[i].S.Timing, now)
	}
	return c.Render(http.StatusOK, "sessions.html", page)
}

func showSession(c echo.Context) error {
	s, _ := operations.GetSessionByItemID(c.Param("itemid"))

	cs, _ := operations.GetClaimsBySessionIDs([]string{s.ItemID})
	rs, _ := operations.GetResultsBySessionIDs([]string{s.ItemID})

	page := buildSessionPage(s, cs, rs, dbLookups())
	page.Long = isLongSession(s.Timing, time.Now())
	return c.Render(http.StatusOK, "session.html", page)
}

// isLongSession reports whether a session lasted, or if still open has so far
// lasted, longer than sessions: longsession in config.yaml (#98).
func isLongSession(t structures.SessionTiming, now time.Time) bool {
	if t.Opened == 0 {
		return false
	}
	end := time.Unix(0, int64(t.Closed))
	if t.Closed == 0 {
		end = now
	}
	return end.Sub(time.Unix(0, int64(t.Opened))) > configuration.LongSession()
}

// processSessionClose closes an open session from the sessions list or the
// session page (#98), e.g. one left open by a client that crashed. The close
// is the normal one (signed footer, "S close" log entry); one more log entry
// records who forced it. back is "list" to return to the sessions list,
// otherwise the session page.
func processSessionClose(c echo.Context) error {
	id := c.Param("itemid")
	if err := operations.CloseSession(id); err != nil {
		return echo.NewHTTPError(http.StatusConflict, "Session not closed: "+err.Error())
	}
	by := auth.CurrentUser(c)
	if by == "" {
		by = "(auth off)"
	}
	logging.MakeLogEntry("S", "close/FORCED", id, "session", fmt.Sprintf("session forcibly closed by %s from the Web UI (%s)", by, c.RealIP()))

	if c.FormValue("back") == "list" {
		return c.Redirect(http.StatusSeeOther, "/sessions")
	}
	return c.Redirect(http.StatusSeeOther, "/session/"+url.PathEscape(id))
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
