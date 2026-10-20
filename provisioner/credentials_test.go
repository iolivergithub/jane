package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #46: jp sends HTTP Basic credentials when it has a user.
func TestNoCredentialsSendsNoHeader(t *testing.T) {
	h := newHarness(t, "create")
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	for _, a := range h.jane.authSeen {
		if a != "" {
			t.Fatalf("Authorization header sent without credentials: %q", a)
		}
	}
}

func TestCredentialsAreSent(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.wantUser, h.jane.wantPass = "ian", "secret"
	h.pv.Client.User, h.pv.Client.Pass = "ian", "secret"
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	if len(h.jane.authSeen) == 0 || !strings.HasPrefix(h.jane.authSeen[0], "Basic ") {
		t.Fatalf("no Basic credentials sent: %q", h.jane.authSeen)
	}
}

func TestWrongCredentialsGiveClearError(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.wantUser, h.jane.wantPass = "ian", "secret"
	h.pv.Client.User, h.pv.Client.Pass = "ian", "wrong"
	_, err := h.pv.ProcessWorklist()
	if !errors.Is(err, errUnauthorized) {
		t.Fatalf("err = %v, want errUnauthorized", err)
	}
	if !strings.Contains(err.Error(), "JANE_USERNAME") {
		t.Errorf("message does not say how to give credentials: %v", err)
	}
}

func TestCredentialsPrecedence(t *testing.T) {
	env := map[string]string{"JANE_USERNAME": "envuser", "JANE_PASSWORD": "envpass"}
	getenv := func(k string) string { return env[k] }
	if u, p := credentials(nil, nil, getenv); u != "envuser" || p != "envpass" {
		t.Errorf("from env: %q %q", u, p)
	}
	fu, fp := "flaguser", "flagpass"
	if u, p := credentials(&fu, &fp, getenv); u != "flaguser" || p != "flagpass" {
		t.Errorf("flags should override env: %q %q", u, p)
	}
	if u, _ := credentials(nil, nil, func(string) string { return "" }); u != "" {
		t.Errorf("no credentials expected, got %q", u)
	}
}

func TestUserAndPassMustBeGivenTogether(t *testing.T) {
	f := "testdata/examplecreateprovisioningfile.yaml"
	for _, args := range [][]string{
		{"attest", f, "--user", "ian"},
		{"attest", f, "--pass", "secret"},
		{"attest", f, "--user=ian"},
		{"attest", f, "--user"},
	} {
		if code := run(args); code != exitUsage {
			t.Errorf("run(%v) = %d, want %d", args, code, exitUsage)
		}
	}
}

func TestInsecureRemote(t *testing.T) {
	for url, want := range map[string]bool{
		"http://127.0.0.1:8520":     false,
		"http://localhost:8520":     false,
		"http://[::1]:8520":         false,
		"https://jane.example:8520": false,
		"http://jane.example:8520":  true,
	} {
		if got := insecureRemote(url); got != want {
			t.Errorf("insecureRemote(%q) = %v, want %v", url, got, want)
		}
	}
}

// Packaging review: create/update without root stop before contacting Jane,
// and an unreadable ID file is not reported as missing. Root may write
// anywhere, so these only run as a normal user.
func TestCreateWithoutWriteAccessStopsEarly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	h := newHarness(t, "create")
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	h.pv.IDFile = filepath.Join(dir, "janeelementid")
	if code := h.pv.Run(); code != exitError {
		t.Fatalf("Run = %d, want %d", code, exitError)
	}
	if len(h.jane.calls) != 0 {
		t.Errorf("Jane was contacted: %v", h.jane.calls)
	}
}

func TestUnreadableIDFileIsNotMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	h := newHarness(t, "attest")
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	h.pv.IDFile = filepath.Join(dir, "janeelementid")
	if err := os.WriteFile(h.pv.IDFile, []byte("E1"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700&^0o100); err != nil { // no search permission
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := h.pv.idFileUnreadable()
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("idFileUnreadable = %v, want permission denied", err)
	}
}
