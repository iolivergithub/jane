package x3270

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/racingmars/go3270"

	"a10/structures"
)

// ---------------------------------------------------------------- fakes

// emptyStore is a store with nothing in it.
type emptyStore struct{}

func (emptyStore) Elements() ([]structures.Element, error)             { return nil, nil }
func (emptyStore) Element(string) (structures.Element, error)          { return structures.Element{}, nil }
func (emptyStore) Intents() ([]structures.Intent, error)               { return nil, nil }
func (emptyStore) Intent(string) (structures.Intent, error)            { return structures.Intent{}, nil }
func (emptyStore) ExpectedValues() ([]structures.ExpectedValue, error) { return nil, nil }
func (emptyStore) ExpectedValue(string) (structures.ExpectedValue, error) {
	return structures.ExpectedValue{}, nil
}
func (emptyStore) CountSessions() int64                                    { return 0 }
func (emptyStore) SessionsPage(int64, int64) ([]structures.Session, error) { return nil, nil }
func (emptyStore) Session(string) (structures.Session, error)              { return structures.Session{}, nil }
func (emptyStore) CountClaims() int64                                      { return 0 }
func (emptyStore) ClaimsPage(int64, int64) ([]structures.Claim, error)     { return nil, nil }
func (emptyStore) Claim(string) (structures.Claim, error)                  { return structures.Claim{}, nil }
func (emptyStore) ClaimsBySession(string) ([]structures.Claim, error)      { return nil, nil }
func (emptyStore) CountResults() int64                                     { return 0 }
func (emptyStore) ResultsPage(int64, int64) ([]structures.Result, error)   { return nil, nil }
func (emptyStore) Result(string) (structures.Result, error)                { return structures.Result{}, nil }
func (emptyStore) ResultsBySession(string) ([]structures.Result, error)    { return nil, nil }
func (emptyStore) ResultsByElement(string, int64) ([]structures.Result, error) {
	return nil, nil
}
func (emptyStore) CountLog() int64                                 { return 0 }
func (emptyStore) LogEntries(int64) ([]structures.LogEntry, error) { return nil, nil }
func (emptyStore) System() systemInfo                              { return systemInfo{LogSize: -1} }
func (emptyStore) Rules() []structures.Rule                        { return nil }
func (emptyStore) Attest(attestRequest) (string, error)            { return "", nil }
func (emptyStore) CloseSession(string, string) error               { return nil }

// fakeAuth accepts ian/secret and records what would be logged.
type fakeAuth struct {
	enabled bool
	idle    time.Duration
	mu      sync.Mutex
	log     []string
}

func (f *fakeAuth) Enabled() bool              { return f.enabled }
func (f *fakeAuth) IdleTimeout() time.Duration { return f.idle }
func (f *fakeAuth) Authenticate(user, password, remote string) bool {
	ok := user == "ian" && password == "secret"
	f.record(fmt.Sprintf("login %s %v", user, ok))
	return ok
}
func (f *fakeAuth) LogLogoff(user, remote, reason string) {
	f.record(fmt.Sprintf("logoff %s %s", user, reason))
}
func (f *fakeAuth) record(s string) { f.mu.Lock(); f.log = append(f.log, s); f.mu.Unlock() }
func (f *fakeAuth) entries() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.log, "; ")
}

// ---------------------------------------------------------------- unit tests

func TestMenuCommand(t *testing.T) {
	cases := []struct {
		aid    go3270.AID
		cmd    string
		logons bool
		want   menuAction
	}{
		{go3270.AIDPF3, "", true, menuStay},  // F3 never leaves the top menu
		{go3270.AIDPF12, "", true, menuStay}, // nor does F12
		{go3270.AIDPF3, "", false, menuStay}, // with or without logons
		{go3270.AIDPF3, "X", true, menuStay}, // F3 wins over a typed command
		{go3270.AIDEnter, "x", true, menuExit},
		{go3270.AIDEnter, " exit ", false, menuExit},
		{go3270.AIDEnter, "L", true, menuLogoff},
		{go3270.AIDEnter, "logoff", true, menuLogoff},
		{go3270.AIDEnter, "L", false, menuOpen}, // no logoff without logons: falls through to "not valid"
		{go3270.AIDEnter, "1", true, menuOpen},
		{go3270.AIDEnter, "", true, menuStay},
		{go3270.AIDPF8, "1", true, menuStay},
	}
	for _, c := range cases {
		if got, _ := menuCommand(c.aid, c.cmd, c.logons); got != c.want {
			t.Errorf("menuCommand(%v, %q, logons=%v) = %v, want %v", c.aid, c.cmd, c.logons, got, c.want)
		}
	}
	if _, msg := menuCommand(go3270.AIDPF3, "", true); !strings.Contains(msg, "L to log off") {
		t.Errorf("F3 message %q does not explain how to log off", msg)
	}
}

func TestStatusLine(t *testing.T) {
	start := time.Date(2026, 10, 14, 18, 30, 0, 0, time.UTC)
	now := start.Add(75 * time.Minute)
	s := &session{auth: &fakeAuth{enabled: true}, system: "ASVR", remote: "10.0.0.5", terminal: "IBM-3278-2-E",
		since: start, now: func() time.Time { return now }}

	if got := s.statusLine(); !strings.HasPrefix(got, "Not logged on  Since 18:30Z (1:15)") {
		t.Errorf("before logon: %q", got)
	}
	s.user = "ian"
	got := s.statusLine()
	for _, want := range []string{"User ian", "Since 18:30Z (1:15)", "IBM-3278-2-E", "From 10.0.0.5", "Sys ASVR"} {
		if !strings.Contains(got, want) {
			t.Errorf("status line %q lacks %q", got, want)
		}
	}
	if len(got) > lineWidth {
		t.Errorf("status line is %d characters, more than a screen row", len(got))
	}
	s.auth = &fakeAuth{enabled: false}
	if got := s.statusLine(); !strings.HasPrefix(got, "No logon required") {
		t.Errorf("auth off: %q", got)
	}
}

// ---------------------------------------------------------------- end to end with s3270

// terminal drives serve over a real TCP connection with s3270, the scriptable
// x3270 client. Tests using it are skipped when s3270 is not installed.
type terminal struct {
	t   *testing.T
	in  io.WriteCloser
	out *bufio.Reader
	cmd *exec.Cmd
}

func startTerminal(t *testing.T, a authenticator) (*terminal, chan struct{}) {
	t.Helper()
	path, err := exec.LookPath("s3270")
	if err != nil {
		t.Skip("s3270 not installed (apt install s3270)")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		ln.Close()
		if err != nil {
			return
		}
		defer conn.Close()
		serve(conn, emptyStore{}, a)
	}()

	cmd := exec.Command(path, "-model", "3279-2-E")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	term := &terminal{t: t, in: in, out: bufio.NewReader(out), cmd: cmd}
	t.Cleanup(func() { in.Close(); cmd.Process.Kill(); cmd.Wait() })
	term.do("Connect(" + ln.Addr().String() + ")")
	term.do("Wait(10,InputField)")
	return term, done
}

// do runs one s3270 action and returns its data lines.
func (term *terminal) do(action string) (string, bool) {
	term.t.Helper()
	fmt.Fprintln(term.in, action)
	var data []string
	for {
		line, err := term.out.ReadString('\n')
		if err != nil {
			term.t.Fatalf("%s: s3270 output ended: %v", action, err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "data: "):
			data = append(data, strings.TrimPrefix(line, "data: "))
		case line == "data:":
			data = append(data, "")
		case line == "ok":
			return strings.Join(data, "\n"), true
		case line == "error":
			return strings.Join(data, "\n"), false
		}
	}
}

// screen returns the screen text after the host has answered.
func (term *terminal) screen() string {
	term.t.Helper()
	term.do("Wait(10,Unlock)")
	s, _ := term.do("Ascii()")
	return s
}

// key sends an AID key (e.g. "Enter", "PF(3)") and returns the new screen.
func (term *terminal) key(k string) string {
	term.t.Helper()
	term.do(k)
	return term.screen()
}

func (term *terminal) typeText(s string) { term.t.Helper(); term.do(`String("` + s + `")`) }

func expect(t *testing.T, screen string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(screen, w) {
			t.Fatalf("screen lacks %q:\n%s", w, screen)
		}
	}
}

func TestX3270LogonFlow(t *testing.T) {
	a := &fakeAuth{enabled: true, idle: time.Minute}
	term, done := startTerminal(t, a)

	s := term.screen()
	expect(t, s, `| |_| / ___ \| |\  || |___`, "User ID   ===>", "Password  ===>", "Not logged on", "Enter=Logon")

	// F3 does not leave the logon screen
	expect(t, term.key("PF(3)"), "F3 does not leave the logon screen")

	// wrong password
	term.typeText("ian")
	term.do("Tab")
	term.typeText("wrong")
	s = term.key("Enter")
	expect(t, s, "Logon failed")
	if strings.Contains(s, "wrong") {
		t.Fatalf("password shown on screen:\n%s", s)
	}

	// right password
	term.typeText("ian")
	term.do("Tab")
	term.typeText("secret")
	s = term.key("Enter")
	expect(t, s, "Primary Option Menu", "User ian", "Since ", "L  Logoff", "X  Exit", "L=Logoff  X=Exit")
	if strings.Contains(s, "F3=Exit") {
		t.Fatalf("menu still offers F3=Exit:\n%s", s)
	}

	// F3 and F12 on the menu stay on the menu
	expect(t, term.key("PF(3)"), "Primary Option Menu", "type L to log off or X to exit")
	expect(t, term.key("PF(12)"), "Primary Option Menu")

	// into a list and back with F3; the status line is on every screen
	term.typeText("1")
	expect(t, term.key("Enter"), "JANE  Elements", "User ian")
	expect(t, term.key("PF(3)"), "Primary Option Menu")

	// L logs off, back to the logon screen
	term.typeText("L")
	expect(t, term.key("Enter"), "User ID   ===>", "Not logged on")

	// log on again and exit with X
	term.typeText("ian")
	term.do("Tab")
	term.typeText("secret")
	expect(t, term.key("Enter"), "Primary Option Menu")
	term.typeText("X")
	term.do("Enter")

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("X did not end the session")
	}
	want := "login ian false; login ian true; logoff ian logoff; login ian true; logoff ian exit"
	if got := a.entries(); got != want {
		t.Errorf("auth log:\n %s\nwant\n %s", got, want)
	}
}

func TestX3270LogonFailuresDisconnect(t *testing.T) {
	a := &fakeAuth{enabled: true}
	term, done := startTerminal(t, a)
	term.screen()
	for i := 0; i < maxLogonFailures; i++ {
		term.typeText("ian")
		term.do("Tab")
		term.typeText("nope")
		term.do("Enter")
		if i < maxLogonFailures-1 {
			expect(t, term.screen(), "Logon failed")
		}
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("not disconnected after repeated failures")
	}
	if got := strings.Count(a.entries(), "login ian false"); got != maxLogonFailures {
		t.Errorf("%d failed logons recorded, want %d", got, maxLogonFailures)
	}
}

func TestX3270WithoutLogon(t *testing.T) {
	a := &fakeAuth{enabled: false}
	term, done := startTerminal(t, a)
	s := term.screen()
	expect(t, s, "Primary Option Menu", "No logon required", "X  Exit")
	if strings.Contains(s, "Logoff") {
		t.Fatalf("logoff offered without logons:\n%s", s)
	}
	expect(t, term.key("PF(3)"), "Primary Option Menu", "type X to exit")
	term.typeText("L")
	expect(t, term.key("Enter"), `Option "L" is not valid`)
	term.typeText("X")
	term.do("Enter")
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("X did not end the session")
	}
	if a.entries() != "" {
		t.Errorf("logged %q with logons off", a.entries())
	}
}

func TestX3270IdleTimeout(t *testing.T) {
	a := &fakeAuth{enabled: true, idle: 500 * time.Millisecond}
	term, done := startTerminal(t, a)
	term.screen()
	term.typeText("ian")
	term.do("Tab")
	term.typeText("secret")
	expect(t, term.key("Enter"), "Primary Option Menu")
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("idle logon was not timed out")
	}
	if !strings.HasSuffix(a.entries(), "logoff ian timed out") {
		t.Errorf("auth log %q does not end with a timeout", a.entries())
	}
}
