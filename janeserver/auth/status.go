package auth

import (
	"strings"
	"time"
)

// Status summarises the authentication set-up, for display (e.g. on the Web UI home page).
type Status struct {
	Web            bool   // Web UI requires a login
	Rest           bool   // REST API requires HTTP Basic authentication
	SessionTimeout string // e.g. "8h", "30m"
	Users          int64  // number of users; -1 if the users collection could not be read
	ActiveSessions int    // Web UI logins that have not expired
	PasswordHash   string // how passwords are stored
	UserTool       string // how users are added or their passwords reset
}

// CurrentStatus returns the current authentication status.
func CurrentStatus() Status {
	st := Status{
		Web:            cfg.Web,
		Rest:           cfg.Rest,
		SessionTimeout: shortDuration(cfg.SessionTimeout),
		Users:          -1,
		PasswordHash:   "Argon2id",
		UserTool:       "/opt/jane/addjaneuser (deb package) or etc/addjaneuser/addjaneuser (source)",
	}
	if users != nil {
		if n, err := users.Count(); err == nil {
			st.Users = n
		}
	}
	if sessions != nil {
		st.ActiveSessions = sessions.Active()
	}
	return st
}

// shortDuration formats 8h0m0s as "8h" and 1h30m0s as "1h30m".
func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
