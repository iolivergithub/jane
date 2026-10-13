package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// fakePK is a value longer than uefiInlineLimit, so it is recorded by hash.
var fakePK = func() []byte {
	b := make([]byte, 40)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}()

// fakeEfivars is the efivars list of the fake claim, as tarzan sends it.
var fakeEfivars = fmt.Sprintf(`[
	{"name":"SecureBoot","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","attributes":6,"somevalue":1,"value":"\u0001","errorvalue":"","rawvalue":"AQ=="},
	{"name":"PK","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","attributes":39,"somevalue":40,"value":"","errorvalue":"","rawvalue":%q},
	{"name":"db","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","attributes":7,"somevalue":1,"value":"x","errorvalue":"","rawvalue":"eA=="},
	{"name":"dbx","guid":"d719b2cb-3d3a-4596-a3bc-dad00e67656f","attributes":39,"somevalue":0,"value":"","errorvalue":"buffer too small","rawvalue":""},
	{"name":"MokListRT","guid":"605DAB50-E046-4300-ABB6-3DD810DD8B23","attributes":6,"somevalue":3,"value":"abc","errorvalue":"","rawvalue":"YWJj"},
	{"name":"BootCurrent","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","attributes":6,"somevalue":2,"value":"","errorvalue":"","rawvalue":"AAA="}
]`, base64.StdEncoding.EncodeToString(fakePK))

//
// uefiefivars expected values
//

func TestUEFIEFIVarsEV(t *testing.T) {
	h := newHarness(t, "create")
	var out bytes.Buffer
	h.pv.Out = &out
	for i, ev := range h.pv.P.EVS {
		if ev.IntentID == "std::intent::uefi::efivars" {
			h.pv.P.EVS[i].Spec.Type = "uefiefivars"
		}
	}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}

	var ev map[string]any
	for _, b := range h.jane.bodies["POST /expectedValue"] {
		if m := b.(map[string]any); m["intentid"] == "std::intent::uefi::efivars" {
			ev = m
		}
	}
	if ev == nil {
		t.Fatal("no expected value created for the efivars intent")
	}
	if ev["name"] != "Debian13VM---std::intent::uefi::efivars" ||
		ev["description"] != "Debian13VM---uefiefivars at 2026-09-29 19:53:12.123456+00:00 UTC" ||
		ev["elementid"] != "E1" || ev["endpointname"] != "tarzan" {
		t.Errorf("ev = %v", ev)
	}

	h256 := sha256.Sum256(fakePK)
	evs := ev["evs"].(map[string]any)
	for name, want := range map[string]any{
		"SecureBoot":   "01",                                    // short: hex
		"PK":           "sha256:" + hex.EncodeToString(h256[:]), // long: hash
		"db":           "absent",                                // only present under the wrong GUID
		"MokListRT":    "616263",                                // GUID matched case-insensitively
		"MokSBStateRT": "absent",
		"dbx":          nil, // unreadable: left out
	} {
		if evs[name] != want {
			t.Errorf("evs[%s] = %v, want %v", name, evs[name], want)
		}
	}
	if len(evs) != len(uefiRuleVariables)-1 {
		t.Errorf("evs has %d entries, want %d (every rule variable but dbx)", len(evs), len(uefiRuleVariables)-1)
	}
	if _, ok := evs["BootCurrent"]; ok {
		t.Error("variables without a UEFI_ rule should not be recorded")
	}
	if !strings.Contains(out.String(), "dbx could not be read by the agent (buffer too small)") {
		t.Errorf("no warning for dbx:\n%s", out.String())
	}
}

func TestUEFIEFIVarsUpdateReplacesEV(t *testing.T) {
	h := newHarness(t, "update")
	h.jane.evExists = true
	os.WriteFile(h.pv.IDFile, []byte("E1\n"), 0o644)
	for i, ev := range h.pv.P.EVS {
		if ev.IntentID == "std::intent::uefi::efivars" {
			h.pv.P.EVS[i].Spec.Type = "uefiefivars"
		}
	}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if code := h.pv.Run(); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	found := false
	for _, b := range h.jane.bodies["PUT /expectedValue"] {
		if m := b.(map[string]any); m["intentid"] == "std::intent::uefi::efivars" {
			found = m["itemid"] == "EV9" && m["evs"].(map[string]any)["SecureBoot"] == "01"
		}
	}
	if !found {
		t.Errorf("update should replace the UEFI expected value: %v", h.jane.bodies["PUT /expectedValue"])
	}
}

func TestUEFIEFIVarsRefusesBadClaims(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"no efivars", `{"machineid":"x"}`, "no efivars"},
		{"empty list", `{"efivars":[],"count":0}`, "lists no UEFI variables"},
		{"null list", `{"efivars":null}`, "no efivars"},
		{"old agent", `{"efivars":[{"name":"SecureBoot","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","value":"\u0001"}]}`, "update tarzan"},
		{"bad base64", `{"efivars":[{"name":"PK","guid":"8be4df61-93ca-11d2-aa0d-00e098032b8c","rawvalue":"!!"}]}`, "not base64"},
	} {
		_, err := uefiEVs([]byte(c.body), io.Discard)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

func TestUEFIEFIVarsBadClaimStopsWithoutEV(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.claimBody = `{"efivars":[],"count":0}`
	h.pv.P.EVS = []EVEntry{{IntentID: "std::intent::uefi::efivars", Spec: EVSpec{Protocol: "tarzan", Type: "uefiefivars"}}}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if _, err := h.pv.ProcessWorklist(); err == nil {
		t.Fatal("expected an error for a claim with no variables")
	}
	if n := h.jane.count("POST /expectedValue") + h.jane.count("PUT /expectedValue"); n != 0 {
		t.Errorf("%d expected values written from an empty claim", n)
	}
	if h.jane.count("DELETE /session") != 1 {
		t.Error("the session should still be closed")
	}
}

//
// create must not touch an already registered machine
//

func runCreate(t *testing.T, h *harness) (int, string) {
	t.Helper()
	var out bytes.Buffer
	h.pv.Out = &out
	return h.pv.Run(), out.String()
}

func TestCreateRefusesWithIDFileEvenUnsafe(t *testing.T) {
	h := newHarness(t, "create") // harness runs with Unsafe set
	os.WriteFile(h.pv.IDFile, []byte("OLD\n"), 0o644)
	code, out := runCreate(t, h)
	if code != exitError {
		t.Errorf("exit %d, want %d", code, exitError)
	}
	if h.jane.mutations() != 0 || len(h.tpm.opened) != 0 {
		t.Errorf("nothing should be changed: jane %v, tpm opened %v", h.jane.calls, h.tpm.opened)
	}
	if id, _ := os.ReadFile(h.pv.IDFile); string(id) != "OLD\n" {
		t.Errorf("ID file overwritten: %q", id)
	}
	for _, want := range []string{"element OLD", "Refusing to create", "jp update"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestCreateRefusesWhenNameInJane(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.elements = []fakeElement{{"E7", "Debian13VM", "someotherid"}}
	code, out := runCreate(t, h)
	if code != exitError || h.jane.mutations() != 0 || len(h.tpm.opened) != 0 {
		t.Errorf("exit %d, calls %v, tpm %v", code, h.jane.calls, h.tpm.opened)
	}
	if !strings.Contains(out, `element E7 with the same name "Debian13VM"`) {
		t.Errorf("output:\n%s", out)
	}
}

func TestCreateRefusesWhenMachineIDInJane(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.elements = []fakeElement{{"E3", "other", "zzz"}, {"E8", "renamed", "ABC123"}}
	code, out := runCreate(t, h)
	if code != exitError || h.jane.mutations() != 0 || len(h.tpm.opened) != 0 {
		t.Errorf("exit %d, calls %v, tpm %v", code, h.jane.calls, h.tpm.opened)
	}
	if !strings.Contains(out, "element E8 with the same machine ID abc123") || strings.Contains(out, "E3") {
		t.Errorf("output:\n%s", out)
	}
}

func TestCreateReportsBothMatchesOnce(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.elements = []fakeElement{{"E7", "Debian13VM", "abc123"}}
	_, out := runCreate(t, h)
	if strings.Count(out, "E7") != 1 || !strings.Contains(out, `same name "Debian13VM" and same machine ID abc123`) {
		t.Errorf("output:\n%s", out)
	}
}

func TestCreateProceedsWhenNotRegistered(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.elements = []fakeElement{{"E3", "other", "zzz"}}
	code, out := runCreate(t, h)
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if h.jane.count("POST /element") != 1 || len(h.tpm.opened) != 1 {
		t.Errorf("create should run: calls %v", h.jane.calls)
	}
	if id, _ := os.ReadFile(h.pv.IDFile); string(id) != "E1" {
		t.Errorf("id file = %q", id)
	}
}

func TestCreateProceedsOnEmptyJane(t *testing.T) {
	h := newHarness(t, "create") // no elements: janeserver answers "elements": null
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestCreateRefusesWhenJaneCannotBeAsked(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.elementLookup = 500
	code, out := runCreate(t, h)
	if code != exitError || h.jane.mutations() != 0 || len(h.tpm.opened) != 0 {
		t.Errorf("exit %d, calls %v, tpm %v", code, h.jane.calls, h.tpm.opened)
	}
	if !strings.Contains(out, "Could not check Jane") {
		t.Errorf("output:\n%s", out)
	}

	h = newHarness(t, "create")
	h.pv.Client = NewClient("http://127.0.0.1:1")
	if code, _ := runCreate(t, h); code != exitError || len(h.tpm.opened) != 0 {
		t.Errorf("unreachable Jane: exit %d, tpm %v", code, h.tpm.opened)
	}
}

func TestCreateWithoutMachineIDChecksNameOnly(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.HostInfo = func() (HostInfo, error) { return HostInfo{}, nil }
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	h.jane.elements = []fakeElement{{"E3", "other", ""}}
	code, out := runCreate(t, h)
	if code != exitOK || !strings.Contains(out, "only checked by element name") {
		t.Errorf("exit %d\n%s", code, out)
	}
	if h.jane.count("GET /elements") != 1 || h.jane.count("GET /element") != 0 {
		t.Errorf("calls %v", h.jane.calls)
	}
}

func TestRecreateAsksAndCanBeDeclined(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Unsafe, h.pv.Recreate = false, true
	os.WriteFile(h.pv.IDFile, []byte("OLD"), 0o644)
	h.pv.In = bufio.NewReader(strings.NewReader("n\n"))
	code, out := runCreate(t, h)
	if code != exitOK || h.jane.mutations() != 0 || len(h.tpm.opened) != 0 {
		t.Errorf("declining: exit %d, calls %v, tpm %v", code, h.jane.calls, h.tpm.opened)
	}
	if !strings.Contains(out, "Recreate this element?") {
		t.Errorf("output:\n%s", out)
	}
}

func TestRecreateGoesAhead(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Recreate = true // with Unsafe from the harness: no prompt
	os.WriteFile(h.pv.IDFile, []byte("OLD"), 0o644)
	h.jane.elements = []fakeElement{{"OLD", "Debian13VM", "abc123"}}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if h.jane.count("POST /element") != 1 {
		t.Errorf("calls %v", h.jane.calls)
	}
	if id, _ := os.ReadFile(h.pv.IDFile); string(id) != "E1" {
		t.Errorf("id file = %q", id)
	}
}

func TestRecreateOnlyWithCreate(t *testing.T) {
	for _, op := range []string{"update", "attest"} {
		if code := run([]string{op, "testdata/examplecreateprovisioningfile.yaml", "--recreate"}); code != exitUsage {
			t.Errorf("%s --recreate: exit %d, want %d", op, code, exitUsage)
		}
	}
}

func TestUpdateIgnoresRegistrationChecks(t *testing.T) {
	h := newHarness(t, "update")
	os.WriteFile(h.pv.IDFile, []byte("E1\n"), 0o644)
	h.jane.elements = []fakeElement{{"E1", "Debian13VM", "abc123"}}
	h.pv.P.ProvisionWorklist = []string{"processelement"}
	if code, out := runCreate(t, h); code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if h.jane.count("GET /elements") != 0 || h.jane.count("PUT /element") != 1 {
		t.Errorf("calls %v", h.jane.calls)
	}
}

// The skeletons list one UEFI_ rule per variable the uefiefivars type records.
func TestSkeletonUEFIRulesMatchVariables(t *testing.T) {
	for _, f := range []string{"skeleton/examplecreateprovisioningfile.yaml", "skeleton/examplecreatetestvm.yaml"} {
		p, err := loadProvisioningFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var rules []string
		for _, ev := range p.EVS {
			if ev.Spec.Type == "uefiefivars" {
				rules = ev.Spec.Rules
			}
		}
		if len(rules) != len(uefiRuleVariables) {
			t.Errorf("%s: %d UEFI rules, want %d", f, len(rules), len(uefiRuleVariables))
			continue
		}
		for i, v := range uefiRuleVariables {
			if rules[i] != "UEFI_"+v.Name {
				t.Errorf("%s: rule %d is %s, want UEFI_%s", f, i, rules[i], v.Name)
			}
		}
	}
}
