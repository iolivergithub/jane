package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testAPIKey = "jane_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ"

// #89: jp sends its API key as a Bearer token when it has one, and nothing otherwise.
func TestNoKeySendsNoHeader(t *testing.T) {
	h := newHarness(t, "create")
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	for _, a := range h.jane.authSeen {
		if a != "" {
			t.Fatalf("Authorization header sent without a key: %q", a)
		}
	}
}

func TestKeyIsSent(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.wantKey = testAPIKey
	h.pv.Client.APIKey = testAPIKey
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	if len(h.jane.authSeen) == 0 || h.jane.authSeen[0] != "Bearer "+testAPIKey {
		t.Fatalf("no Bearer key sent: %q", h.jane.authSeen)
	}
}

func TestWrongKeyGivesClearError(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.wantKey = testAPIKey
	h.pv.Client.APIKey = "jane_" + strings.Repeat("x", 43)
	_, err := h.pv.ProcessWorklist()
	if !errors.Is(err, errUnauthorized) {
		t.Fatalf("err = %v, want errUnauthorized", err)
	}
	if !strings.Contains(err.Error(), "JANE_API_KEY") || !strings.Contains(err.Error(), "1.5.5") {
		t.Errorf("message does not say how to give a key: %v", err)
	}
}

func TestAPIKeySources(t *testing.T) {
	env := map[string]string{"JANE_API_KEY": testAPIKey}
	getenv := func(k string) string { return env[k] }
	other := "jane_" + strings.Repeat("Q", 43)
	files := map[string]string{"/k": other + "\n", "/bad": "cluster-a-2026\n"}
	readFile := func(p string) ([]byte, error) {
		if s, ok := files[p]; ok {
			return []byte(s), nil
		}
		return nil, os.ErrNotExist
	}
	if k, err := apiKey(nil, getenv, readFile); err != nil || k != testAPIKey {
		t.Errorf("from env: %q %v", k, err)
	}
	f := "/k"
	if k, err := apiKey(&f, getenv, readFile); err != nil || k != other {
		t.Errorf("file should override env (and be trimmed): %q %v", k, err)
	}
	if k, err := apiKey(nil, func(string) string { return "" }, readFile); err != nil || k != "" {
		t.Errorf("no key expected: %q %v", k, err)
	}
	bad := "/bad"
	if _, err := apiKey(&bad, getenv, readFile); err == nil {
		t.Error("a malformed key was accepted")
	}
	missing := "/missing"
	if _, err := apiKey(&missing, getenv, readFile); err == nil {
		t.Error("an unreadable key file was accepted")
	}
}

func TestOldCredentialOptionsRefused(t *testing.T) {
	f := "testdata/examplecreateprovisioningfile.yaml"
	for _, args := range [][]string{
		{"attest", f, "--user", "ian", "--pass", "secret"},
		{"attest", f, "--user=ian"},
		{"attest", f, "--pass", "secret"},
		{"attest", f, "--apikeyfile"},
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
