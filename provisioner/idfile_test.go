package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The element ID file is /etc/opt/jane/janeelementid (since 1.5.3). The
// pre-1.5.3 /etc/janeelementid and its migration were removed in 1.5.6 (#81).

func TestDefaultIDFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("different location on Windows")
	}
	if defaultIDFile() != "/etc/opt/jane/janeelementid" {
		t.Errorf("path %s", defaultIDFile())
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

// A file at the old location is no longer looked at: attest and update need
// the element ID file at IDFile.
func TestOldIDFileLocationIgnored(t *testing.T) {
	for _, op := range []string{"update", "attest"} {
		h := newHarness(t, op)
		old := filepath.Join(filepath.Dir(h.pv.IDFile), "old-janeelementid")
		os.WriteFile(old, []byte("E1\n"), 0o644)
		if code := h.pv.Run(); code != exitError {
			t.Errorf("%s: exit %d, want %d", op, code, exitError)
		}
		if len(h.jane.calls) != 0 {
			t.Errorf("%s: no requests expected, got %v", op, h.jane.calls)
		}
		if _, err := os.Stat(h.pv.IDFile); err == nil {
			t.Errorf("%s: %s should not have been created", op, h.pv.IDFile)
		}
	}
}

func TestCreateWritesReadOnlyIDFile(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	readOnly(t, h.pv.IDFile)
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

func TestRecreateRewritesReadOnlyIDFile(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Recreate = true
	os.WriteFile(h.pv.IDFile, []byte("OLD"), 0o444)
	h.jane.elements = []fakeElement{{"OLD", "Debian13VM", "abc123"}}
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if b, _ := os.ReadFile(h.pv.IDFile); string(b) != "E1" {
		t.Errorf("%s = %q, want E1", h.pv.IDFile, b)
	}
	readOnly(t, h.pv.IDFile)
}
