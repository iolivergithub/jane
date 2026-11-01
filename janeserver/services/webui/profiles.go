package webui

// The attestation profile page (#132) and running a profile from it in the
// background (#136). Stored profiles are only shown here: they change only
// through jp create/update. An edit made on the page is used for one run and
// never saved; the run's session records it (#130).

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/labstack/echo/v4"

	"a10/auth"
	"a10/logging"
	"a10/operations"
	"a10/structures"
)

type profilePage struct {
	E         structures.Element
	Name      string
	Found     bool
	Default   bool
	Profile   structures.AttestationProfile
	YAML      string // the text in the box
	Editable  bool   // the box can be edited (Edit, Edit again, or after an error)
	Edited    bool   // the text differs from the stored profile
	Error     string
	MaxFollow int
	Depths    []int // 1..MaxFollow, for the depth selector
}

func newProfilePage(e structures.Element, name string) profilePage {
	page := profilePage{E: e, Name: name, MaxFollow: operations.DefaultFollowLimit}
	for i := 1; i <= page.MaxFollow; i++ {
		page.Depths = append(page.Depths, i)
	}
	if p, ok := e.AttestationProfiles[name]; ok {
		page.Found, page.Profile, page.YAML = true, p, operations.ProfileYAML(p)
		page.Default = e.DefaultAttestationProfile == name
	}
	return page
}

// showProfile shows a stored profile; ?edit=1 makes the box editable.
func showProfile(c echo.Context) error {
	e, _ := operations.GetElementByItemID(c.Param("itemid"))
	page := newProfilePage(e, c.Param("name"))
	page.Editable = c.QueryParam("edit") == "1"
	return c.Render(http.StatusOK, "profile.html", page)
}

// processProfile handles the profile page's form: action "edit" shows the
// posted YAML in an editable box (Edit again, from a session page); action
// "attest" runs it.
func processProfile(c echo.Context) error {
	e, err := operations.GetElementByItemID(c.Param("itemid"))
	page := newProfilePage(e, c.Param("name"))
	if err != nil || !page.Found {
		return c.Render(http.StatusOK, "profile.html", page)
	}
	text := c.FormValue("profile")
	page.YAML, page.Editable = text, true

	p, perr := operations.ParseProfileYAML(text)
	page.Edited = perr != nil || !operations.SameProfile(p, page.Profile)
	if c.FormValue("action") != "attest" {
		return c.Render(http.StatusOK, "profile.html", page)
	}
	if perr != nil {
		page.Error = perr.Error()
		return c.Render(http.StatusOK, "profile.html", page)
	}

	opts := operations.RunOptions{}
	if c.FormValue("follow") == "on" {
		opts.FollowLimit = operations.DefaultFollowLimit
		if n, err := strconv.Atoi(c.FormValue("depth")); err == nil && n >= 1 && n <= operations.DefaultFollowLimit {
			opts.FollowLimit = n
		}
		opts.Parallel = c.FormValue("parallel") == "on"
	}
	user := auth.CurrentUser(c)
	if user == "" {
		user = "anonymous"
	}
	rc := operations.RunContext{User: user, Via: "Web UI", From: c.RealIP()}

	req, err := operations.StartProfileRun(e, page.Name, p, page.Edited, "", opts, rc)
	if err != nil {
		page.Error = "The run could not be started: " + err.Error()
		return c.Render(http.StatusOK, "profile.html", page)
	}
	// The run goes on without the browser: navigating away loses nothing.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.MakeLogEntry("UI", "runprov", req.SessionID, "session", fmt.Sprintf("profile run failed: %v", r))
				operations.CloseSession(req.SessionID) // best effort; an error just means it is closed
			}
		}()
		req.Run()
	}()
	return c.Redirect(http.StatusSeeOther, "/session/"+url.PathEscape(req.SessionID))
}
