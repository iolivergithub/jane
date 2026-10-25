package auth

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"a10/configuration"
)

//go:embed templates/login.html
var templateFS embed.FS

var loginTemplate = template.Must(template.ParseFS(templateFS, "templates/login.html"))

type loginPageData struct {
	SystemName string
	Next       string
	Username   string
	Error      string
}

// RegisterEndpoints adds /login and /logout to the router.
//
// The login page is rendered from this package's own template, independently of the
// Web UI's template registry, so the Web UI needs no knowledge of it.
// When Web UI authentication is disabled both routes simply redirect to the home page.
func RegisterEndpoints(router *echo.Echo) {
	router.GET(loginPath, showLogin)
	router.POST(loginPath, processLogin)
	router.POST(logoutPath, processLogout)
}

func showLogin(c echo.Context) error {
	if !cfg.Web {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	next := safeNext(c.QueryParam("next"))
	if _, ok := sessionUser(c); ok {
		return c.Redirect(http.StatusSeeOther, next)
	}
	return renderLogin(c, http.StatusOK, loginPageData{Next: next})
}

func processLogin(c echo.Context) error {
	if !cfg.Web {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	next := safeNext(c.FormValue("next"))

	// Too many recent failures from this address, or a wrong password, both give
	// the usual message; the log records which (#89).
	allowed := loginAllowed(username, "web", c.RealIP())
	if !allowed || !users.Authenticate(username, password) {
		if allowed {
			failures.fail(c.RealIP())
			logEntry("AUTH", "login/FAIL", username, "user", "failed login from "+c.RealIP())
		}
		return renderLogin(c, http.StatusUnauthorized, loginPageData{
			Next:     next,
			Username: username,
			Error:    "Incorrect username or password.",
		})
	}

	// Always issue a fresh token on login, and drop any previous one.
	if old, err := c.Cookie(CookieName); err == nil {
		sessions.Delete(old.Value)
	}
	token, err := sessions.Create(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to create session")
	}
	setSessionCookie(c, token)
	logEntry("AUTH", "login/OK", username, "user", "login from "+c.RealIP())

	return c.Redirect(http.StatusSeeOther, next)
}

func processLogout(c echo.Context) error {
	if cookie, err := c.Cookie(CookieName); err == nil {
		if user, ok := sessions.Lookup(cookie.Value); ok {
			logEntry("AUTH", "logout", user, "user", "logout from "+c.RealIP())
		}
		sessions.Delete(cookie.Value)
	}
	clearSessionCookie(c)
	if !cfg.Web {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	return c.Redirect(http.StatusSeeOther, loginPath)
}

func renderLogin(c echo.Context, status int, data loginPageData) error {
	if configuration.ConfigData != nil {
		data.SystemName = configuration.ConfigData.System.Name
	}
	var sb strings.Builder
	if err := loginTemplate.Execute(&sb, data); err != nil {
		return err
	}
	return c.HTML(status, sb.String())
}
