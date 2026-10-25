package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// REST API authentication by API key (#89).
//
// A client sends its key on every request:
//
//	Authorization: Bearer jane_<43 base64url characters>
//
// Keys are 32 random bytes made by addjaneuser, which stores only
// "sha256:" + the hex SHA-256 of the whole key in the user's `apikey` field.
// Jane checks the format first (so only keys made by addjaneuser can ever
// work, and the prefix marks Jane's own keys apart from any external tokens
// in future, #48), then looks the hash up. Nothing is hashed slowly and no
// state is kept per client.
//
// The REST API and the Web UI are kept separate: this file is the REST side;
// they share only the user store and the failed-attempt limiter.

const (
	apiKeyPrefix  = "jane_"
	apiKeyEncoded = 43 // base64url of 32 bytes, unpadded
	apiKeyHashTag = "sha256:"
)

// validKeyFormat reports whether k looks like a key made by addjaneuser.
func validKeyFormat(k string) bool {
	if len(k) != len(apiKeyPrefix)+apiKeyEncoded || !strings.HasPrefix(k, apiKeyPrefix) {
		return false
	}
	for _, ch := range k[len(apiKeyPrefix):] {
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

// hashAPIKey is the form stored in the users collection (addjaneuser computes the same).
func hashAPIKey(k string) string {
	sum := sha256.Sum256([]byte(k))
	return apiKeyHashTag + hex.EncodeToString(sum[:])
}

// RESTMiddleware protects the REST API with API keys.
//
// CORS preflight (OPTIONS) and /health are left open so browsers and health checks keep working.
// If REST authentication is disabled this middleware does nothing (a key, if sent, is ignored).
func RESTMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !cfg.Rest {
				return next(c)
			}
			req := c.Request()
			if req.Method == http.MethodOptions || req.URL.Path == "/health" {
				return next(c)
			}
			remote := c.RealIP()

			if isBlocked, left := failures.blocked(remote); isBlocked {
				if hourly.first("blocked|" + hostOnly(remote)) {
					logEntry("AUTH", "login/BLOCKED", "", "user", fmt.Sprintf("rest requests from %s refused: too many failed attempts", remote))
				}
				c.Response().Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(left.Seconds()))))
				return echo.NewHTTPError(http.StatusTooManyRequests, "too many failed attempts; try again later")
			}

			header := req.Header.Get(echo.HeaderAuthorization)
			scheme, value, _ := strings.Cut(header, " ")

			switch {
			case strings.EqualFold(scheme, "Basic"):
				// Old clients (jp before 1.5.5, curl -u). Not counted as a guess, logged once an hour per address.
				if hourly.first("basic|" + hostOnly(remote)) {
					logEntry("AUTH", "login/FAIL", "", "user", fmt.Sprintf("rest request with HTTP Basic from %s: no longer supported, an API key is needed", remote))
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "HTTP Basic is no longer supported; use an API key (Authorization: Bearer jane_...) and jp 1.5.5 or later - see docs/authentication.md")

			case !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(value) == "":
				if hourly.first("nokey|" + hostOnly(remote)) {
					logEntry("AUTH", "login/FAIL", "", "user", fmt.Sprintf("rest request without an API key from %s", remote))
				}
				c.Response().Header().Set(echo.HeaderWWWAuthenticate, `Bearer realm="Jane"`)
				return echo.NewHTTPError(http.StatusUnauthorized, "API key required (Authorization: Bearer jane_...)")
			}

			key := strings.TrimSpace(value)
			username := ""
			ok := false
			if validKeyFormat(key) {
				username, ok = users.UserForAPIKey(hashAPIKey(key))
			}
			if !ok {
				failures.fail(remote)
				logEntry("AUTH", "login/FAIL", "", "user", fmt.Sprintf("rest request with an unknown or malformed API key from %s", remote))
				c.Response().Header().Set(echo.HeaderWWWAuthenticate, `Bearer realm="Jane", error="invalid_token"`)
				return echo.NewHTTPError(http.StatusUnauthorized, "unknown or malformed API key")
			}

			c.Set(contextUserKey, username)
			if !authoriser.Allowed(username, req.Method, c.Path()) {
				return echo.NewHTTPError(http.StatusForbidden, "not permitted")
			}
			if hourly.first("ok|" + username + "|" + hostOnly(remote)) {
				logEntry("AUTH", "login/OK", username, "user", fmt.Sprintf("rest API key used from %s (logged once an hour per address)", remote))
			}
			return next(c)
		}
	}
}
