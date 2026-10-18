package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
)

// The element ID file moved to /etc/opt/jane/janeelementid in 1.5.3;
// update and attest migrate the legacy /etc/janeelementid (#81).

func TestDefaultIDFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unchanged on Windows")
	}
	if defaultIDFile() != "/etc/opt/jane/janeelementid" || defaultLegacyIDFile() != "/etc/janeelementid" {
		t.Errorf("paths %s, %s", defaultIDFile(), defaultLegacyIDFile())
	}
}

func readOnly(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o444 {
		t.Errorf("%s mode %v, want -r--r--r--", path, fi.Mode().Perm())
	}
}

func TestAttestMigratesLegacyIDFile(t *testing.T) {
	h := newHarness(t, "attest")
	os.WriteFile(h.pv.LegacyIDFile, []byte("E1\n"), 0o644)
	var out bytes.Buffer
	h.pv.Out = &out
	if code := h.pv.Run(); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if b, _ := os.ReadFile(h.pv.IDFile); string(b) != "E1" {
		t.Errorf("new ID file = %q", b)
	}
	readOnly(t, h.pv.IDFile)
	if b, _ := os.ReadFile(h.pv.LegacyIDFile); string(b) != "E1\n" {
		t.Errorf("legacy file changed: %q", b)
	}
	if !strings.Contains(out.String(), "element ID file copied from") {
		t.Errorf("output:\n%s", out.String())
	}
	if h.jane.bodies["POST /attest"][0].(map[string]any)["eid"] != "E1" {
		t.Error("attest should use the migrated element ID")
	}
}

func TestIDFilesDiffer(t *testing.T) {
	for _, op := range []string{"update", "attest"} {
		h := newHarness(t, op)
		os.WriteFile(h.pv.IDFile, []byte("E1"), 0o444)
		os.WriteFile(h.pv.LegacyIDFile, []byte("E2\n"), 0o644)
		if code := h.pv.Run(); code != exitError {
			t.Errorf("%s: exit %d, want %d", op, code, exitError)
		}
		if len(h.jane.calls) != 0 {
			t.Errorf("%s: no requests expected, got %v", op, h.jane.calls)
		}
	}
}

func TestIDFilesSame(t *testing.T) {
	h := newHarness(t, "attest")
	os.WriteFile(h.pv.IDFile, []byte("E1"), 0o444)
	os.WriteFile(h.pv.LegacyIDFile, []byte("E1\n"), 0o644)
	if code := h.pv.Run(); code != exitOK {
		t.Errorf("exit %d", code)
	}
}

func TestCreateRefusesWithLegacyIDFile(t *testing.T) {
	h := newHarness(t, "create")
	os.WriteFile(h.pv.LegacyIDFile, []byte("OLD\n"), 0o644)
	code, out := runCreate(t, h)
	if code == exitOK || h.jane.mutations() != 0 {
		t.Errorf("exit %d, calls %v", code, h.jane.calls)
	}
	if !strings.Contains(out, h.pv.LegacyIDFile) || !strings.Contains(out, "OLD") {
		t.Errorf("output:\n%s", out)
	}
}

func TestCreateWritesReadOnlyIDFile(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	readOnly(t, h.pv.IDFile)
	if _, err := os.Stat(h.pv.LegacyIDFile); err == nil {
		t.Error("create should not write a legacy file")
	}
}

func TestUpdateDoesNotRewriteIDFile(t *testing.T) {
	h := newHarness(t, "update")
	os.WriteFile(h.pv.IDFile, []byte("E1\n"), 0o444)
	h.jane.elements = []fakeElement{{"E1", "Debian13VM", "abc123"}}
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if b, _ := os.ReadFile(h.pv.IDFile); string(b) != "E1\n" {
		t.Errorf("ID file rewritten: %q", b)
	}
}

func TestRecreateRewritesReadOnlyFiles(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Recreate = true
	os.WriteFile(h.pv.IDFile, []byte("OLD"), 0o444)
	os.WriteFile(h.pv.LegacyIDFile, []byte("OLD\n"), 0o644)
	h.jane.elements = []fakeElement{{"OLD", "Debian13VM", "abc123"}}
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, f := range []string{h.pv.IDFile, h.pv.LegacyIDFile} {
		if b, _ := os.ReadFile(f); string(b) != "E1" {
			t.Errorf("%s = %q, want E1", f, b)
		}
		readOnly(t, f)
	}
}
