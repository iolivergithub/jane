package x3270

// One terminal connection: who is logged on (when the x3270 logon is enabled,
// auth: x3270: true), the logon screen, and the status line shown on every
// screen (#45).
//
// The session travels with the connection: serve wraps the net.Conn in a
// sessionConn, and send (which every screen uses) adds the status line and
// the idle timeout. There is no package-level session state.

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/racingmars/go3270"

	"a10/auth"
	"a10/configuration"
)

// authenticator is what the 3270 interface needs from Jane's auth package;
// tests supply their own.
type authenticator interface {
	Enabled() bool // does the x3270 interface require a logon?
	Authenticate(user, password, remote string) bool
	LogLogoff(user, remote, reason string)
	IdleTimeout() time.Duration // a logged-on terminal left this long is disconnected
}

// janeAuth is the authenticator backed by the auth package (users collection,
// Argon2id hashes, Jane's log), as used by the Web UI.
type janeAuth struct{}

func (janeAuth) Enabled() bool { return auth.X3270Enabled() }
func (janeAuth) Authenticate(user, password, remote string) bool {
	return auth.Authenticate(user, password, "x3270", remote)
}
func (janeAuth) LogLogoff(user, remote, reason string) { auth.LogLogoff(user, "x3270", remote, reason) }
func (janeAuth) IdleTimeout() time.Duration            { return auth.SessionTimeout() }

const (
	maxLogonFailures = 3           // the connection is closed after this many failed logons
	failureDelay     = time.Second // pause after each failed logon
)

// session is the state of one terminal connection.
type session struct {
	auth     authenticator
	system   string // system name from config.yaml
	remote   string // client host
	terminal string // e.g. IBM-3278-2-E
	user     string // logged-on user; empty when not logged on (or auth is off)
	since    time.Time
	ending   string // why the session is ending, when the user chose to: "exit"
	failures int
	now      func() time.Time
}

func newSession(a authenticator, remote, terminal string) *session {
	s := &session{auth: a, remote: remote, terminal: terminal, now: time.Now}
	if configuration.ConfigData != nil {
		s.system = configuration.ConfigData.System.Name
	}
	if host, _, err := net.SplitHostPort(remote); err == nil {
		s.remote = host
	}
	s.since = s.now()
	return s
}

// statusLine is row 21 of every screen: who is logged on, since when (UTC)
// and for how long, the terminal type, where from and the system name.
func (s *session) statusLine() string {
	user := "No logon required"
	if s.auth.Enabled() {
		user = "Not logged on"
		if s.user != "" {
			user = "User " + s.user
		}
	}
	d := s.now().Sub(s.since)
	parts := []string{
		user,
		fmt.Sprintf("Since %sZ (%d:%02d)", s.since.UTC().Format("15:04"), int(d.Hours()), int(d.Minutes())%60),
		or(s.terminal, "?"),
		"From " + or(s.remote, "?"),
	}
	if s.system != "" {
		parts = append(parts, "Sys "+s.system)
	}
	return strings.Join(parts, "  ")
}

// sessionConn carries a session with its connection through go3270.
type sessionConn struct {
	net.Conn
	sess *session
}

// sessionOf returns the session travelling with conn, if any.
func sessionOf(conn net.Conn) *session {
	if sc, ok := conn.(*sessionConn); ok {
		return sc.sess
	}
	return nil
}

// loggedOn reports whether conn's terminal may use the screens beyond the logon.
func loggedOn(conn net.Conn) bool {
	s := sessionOf(conn)
	return s == nil || !s.auth.Enabled() || s.user != ""
}

// send shows a screen, adding the status line and, once logged on, the idle
// timeout. Every screen goes through here.
func send(conn net.Conn, dev go3270.DevInfo, screen go3270.Screen, opts go3270.ScreenOpts) (go3270.Response, error) {
	if s := sessionOf(conn); s != nil {
		screen = append(screen, go3270.Field{Row: statusRow, Col: 0, Color: go3270.Turquoise, Content: cut(s.statusLine(), lineWidth)})
		if !opts.NoResponse && s.user != "" {
			if t := s.auth.IdleTimeout(); t > 0 {
				conn.SetReadDeadline(time.Now().Add(t))
			}
		}
	}
	return go3270.ShowScreenOpts(screen, nil, conn, opts)
}

// logo is drawn on the logon screen.
var logo = []string{
	`     _   _    _   _  _____ `,
	`    | | / \  | \ | || ____|`,
	` _  | |/ _ \ |  \| ||  _|  `,
	`| |_| / ___ \| |\  || |___ `,
	` \___/_/   \_\_| \_||_____|`,
}

// logonTx is the logon screen, shown first when auth: x3270: true. A good
// user ID and password lead to the primary option menu; after
// maxLogonFailures failures the connection is closed. F3 does nothing here.
func logonTx(st store) go3270.Tx {
	message := ""
	var self go3270.Tx
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		s := sessionOf(conn)
		const labelCol, fieldCol, fieldLen = 22, 39, 24
		logoCol := (80 - len(logo[0])) / 2
		screen := go3270.Screen{}
		for i, l := range logo {
			screen = append(screen, go3270.Field{Row: 2 + i, Col: logoCol, Intense: true, Color: go3270.Green, Content: l})
		}
		title := "Jane Attestation Engine"
		if s != nil && s.system != "" {
			title += "  -  " + s.system
		}
		title = cut(title, 76)
		screen = append(screen,
			go3270.Field{Row: 8, Col: (80 - len(title)) / 2, Color: go3270.White, Content: title},

			go3270.Field{Row: 11, Col: labelCol, Intense: true, Content: "User ID   ===>"},
			go3270.Field{Row: 11, Col: fieldCol, Write: true, Name: "userid", Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 11, Col: fieldCol + fieldLen + 1, Autoskip: true},

			go3270.Field{Row: 13, Col: labelCol, Intense: true, Content: "Password  ===>"},
			go3270.Field{Row: 13, Col: fieldCol, Write: true, Hidden: true, Name: "password", Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 13, Col: fieldCol + fieldLen + 1, Autoskip: true},

			go3270.Field{Row: 16, Col: labelCol - 6, Color: go3270.Blue, Content: "Type your Jane user ID and password, then press Enter."},
			go3270.Field{Row: 17, Col: labelCol - 6, Color: go3270.Blue, Content: "This connection is not encrypted."},
			go3270.Field{Row: msgRow, Col: 0, Intense: true, Color: go3270.Red, Content: cut(message, lineWidth)},
			go3270.Field{Row: keysRow, Col: 0, Color: go3270.Turquoise, Content: "Enter=Logon"},
		)
		message = ""

		resp, err := send(conn, dev, screen, go3270.ScreenOpts{CursorRow: 11, CursorCol: fieldCol + 1, Codepage: dev.Codepage()})
		if err != nil {
			return nil, nil, err
		}
		switch resp.AID {
		case go3270.AIDEnter:
		case go3270.AIDPF3, go3270.AIDPF12:
			message = "F3 does not leave the logon screen; close the terminal session to disconnect"
			return self, nil, nil
		default:
			return self, nil, nil
		}

		user := strings.TrimSpace(resp.Values["userid"])
		password := resp.Values["password"]
		if user == "" {
			message = "Type a user ID"
			return self, nil, nil
		}
		if s == nil {
			return menuTx(st), nil, nil
		}
		if !s.auth.Authenticate(user, password, s.remote) {
			s.failures++
			if s.failures >= maxLogonFailures {
				return nil, nil, nil // disconnect
			}
			time.Sleep(failureDelay)
			message = "Logon failed: user ID or password not recognised"
			return self, nil, nil
		}
		s.user, s.since, s.failures = user, s.now(), 0
		return menuTx(st), nil, nil
	}
	return self
}

// logoff ends the logged-on user's session and returns to the logon screen.
func logoff(conn net.Conn, st store) go3270.Tx {
	if s := sessionOf(conn); s != nil && s.user != "" {
		s.auth.LogLogoff(s.user, s.remote, "logoff")
		s.user, s.since = "", s.now()
	}
	conn.SetReadDeadline(time.Time{})
	return logonTx(st)
}

// endReason says how a session ended, for the log.
func endReason(s *session, err error) string {
	switch {
	case s.ending != "":
		return s.ending
	case err != nil && errors.Is(err, os.ErrDeadlineExceeded):
		return "timed out"
	default:
		return "disconnected"
	}
}
