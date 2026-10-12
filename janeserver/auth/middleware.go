package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"a10/configuration"
)

const (
	// CookieName is the name of the Web UI session cookie.
	CookieName = "jane_session"

	// contextUserKey is where the authenticated username is stored in the echo.Context.
	contextUserKey = "auth.user"

	loginPath  = "/login"
	logoutPath = "/logout"
)

// CurrentUser returns the authenticated username for this request, or "" if there is none
// (including when authentication is disabled).
func CurrentUser(c echo.Context) string {
	if u, ok := c.Get(contextUserKey).(string); ok {
		return u
	}
	return ""
}

// WebUIMiddleware protects every Web UI route except the login and logout pages.
//
// Without a valid session cookie, GET/HEAD requests are redirected to the login page
// (remembering where they were going) and everything else gets 401.
// If Web UI authentication is disabled this middleware does nothing.
func WebUIMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !cfg.Web {
				return next(c)
			}
			p := c.Request().URL.Path
			if p == loginPath || p == logoutPath {
				return next(c)
			}

			if user, ok := sessionUser(c); ok {
				c.Set(contextUserKey, user)
				if !authoriser.Allowed(user, c.Request().Method, c.Path()) {
					return echo.NewHTTPError(http.StatusForbidden, "not permitted")
				}
				return next(c)
			}

			switch c.Request().Method {
			case http.MethodGet, http.MethodHead:
				target := loginPath + "?next=" + url.QueryEscape(c.Request().URL.RequestURI())
				return c.Redirect(http.StatusSeeOther, target)
			default:
				return echo.NewHTTPError(http.StatusUnauthorized, "login required")
			}
		}
	}
}

// RESTMiddleware protects the REST API with HTTP Basic authentication against the same user store.
//
// CORS preflight (OPTIONS) and /health are left open so browsers and health checks keep working.
// If REST authentication is disabled this middleware does nothing.
func RESTMiddleware() echo.MiddlewareFunc {
	basic := middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
		Realm: "Jane",
		Skipper: func(c echo.Context) bool {
			return c.Request().Method == http.MethodOptions || c.Request().URL.Path == "/health"
		},
		Validator: func(username, password string, c echo.Context) (bool, error) {
			if !users.Authenticate(username, password) {
				return false, nil
			}
			c.Set(contextUserKey, username)
			if !authoriser.Allowed(username, c.Request().Method, c.Path()) {
				return false, echo.NewHTTPError(http.StatusForbidden, "not permitted")
			}
			return true, nil
		},
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		protected := basic(next)
		return func(c echo.Context) error {
			if !cfg.Rest {
				return next(c)
			}
			return protected(c)
		}
	}
}

// sessionUser returns the user for the request's session cookie, if valid.
func sessionUser(c echo.Context) (string, bool) {
	cookie, err := c.Cookie(CookieName)
	if err != nil {
		return "", false
	}
	return sessions.Lookup(cookie.Value)
}

func setSessionCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// secureCookies marks the cookie Secure when the Web UI is served over HTTPS.
func secureCookies() bool {
	return configuration.ConfigData != nil && !configuration.ConfigData.Web.UseHTTP
}

// safeNext only allows local redirect targets, preventing open redirects via ?next=.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	if strings.HasPrefix(next, loginPath) || strings.HasPrefix(next, logoutPath) {
		return "/"
	}
	return next
}
