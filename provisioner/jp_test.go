package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeJane records requests and answers like a minimal janeserver.
type fakeJane struct {
	mu       sync.Mutex
	calls    []string         // "METHOD path"
	bodies   map[string][]any // "METHOD path-prefix" -> decoded JSON bodies
	raw      map[string][]string
	evExists bool // whether GET /expectedValue/... finds one
}

func (f *fakeJane) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		key := r.Method + " " + r.URL.Path
		f.calls = append(f.calls, key)
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				t.Errorf("%s: body is not JSON: %s", key, b)
			}
			k := r.Method + " " + firstSeg(r.URL.Path)
			f.bodies[k] = append(f.bodies[k], v)
			f.raw[k] = append(f.raw[k], string(b))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/session":
			io.WriteString(w, `{"itemid":"S1"}`)
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/session/"):
			io.WriteString(w, `{}`)
		case r.URL.Path == "/element":
			io.WriteString(w, `{"itemid":"E1"}`)
		case r.Method == "POST" && r.URL.Path == "/attest":
			io.WriteString(w, `{"itemid":"C1"}`)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/claim/"):
			io.WriteString(w, `{"itemid":"C1","body":{"machineid":"abc123",
				"quote":{"attested":{"pcrdigest":"deadbeef"},"firmwareVersion":9007199254740993}}}`)
		case r.Method == "POST" && r.URL.Path == "/verify":
			io.WriteString(w, `{"itemid":"R1"}`)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/expectedValue/"):
			if f.evExists {
				io.WriteString(w, `{"itemid":"EV9"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `{"error":"not found"}`)
			}
		case r.URL.Path == "/expectedValue":
			io.WriteString(w, `{"itemid":"EV1"}`)
		default:
			t.Errorf("unexpected request %s", key)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func firstSeg(p string) string {
	s := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)
	return "/" + s[0]
}

func (f *fakeJane) count(key string) int {
	n := 0
	for _, c := range f.calls {
		if c == key || strings.HasPrefix(c, key+"/") {
			n++
		}
	}
	return n
}

type harness struct {
	pv   *Provisioner
	jane *fakeJane
	cmds [][]string
}

func newHarness(t *testing.T, op string) *harness {
	t.Helper()
	p, err := loadProvisioningFile("testdata/examplecreateprovisioningfile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{jane: &fakeJane{bodies: map[string][]any{}, raw: map[string][]string{}}}
	srv := httptest.NewServer(h.jane.handler(t))
	t.Cleanup(srv.Close)
	p.AttestationServer = srv.URL

	pv := NewProvisioner(p, op, true)
	pv.IDFile = filepath.Join(t.TempDir(), "janeelementid")
	pv.Out = io.Discard
	pv.Now = func() time.Time { return time.Date(2026, 9, 29, 19, 53, 12, 123456000, time.UTC) }
	pv.RunCmd = func(name string, args ...string) { h.cmds = append(h.cmds, append([]string{name}, args...)) }
	pv.HostInfo = func() (HostInfo, error) {
		return HostInfo{OS: "Linux-6.1-x86_64", Arch: "x86_64", Hostname: "vm", MachineID: "abc123"}, nil
	}
	h.pv = pv
	return h
}

func TestLoadExampleFile(t *testing.T) {
	p, err := loadProvisioningFile("testdata/examplecreateprovisioningfile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.TPM2.EK.Handle != "0x810100EE" || p.TPM2.AK.Handle != "0x810100AA" {
		t.Errorf("handles = %q, %q", p.TPM2.EK.Handle, p.TPM2.AK.Handle)
	}
	if len(p.EVS) != 7 {
		t.Fatalf("got %d evs, want 7", len(p.EVS))
	}
	first := p.EVS[0]
	if first.IntentID != "std::intent::sys::info" || first.Spec.Type != "sysmachineid" || len(first.Spec.Rules) != 3 {
		t.Errorf("first ev = %+v", first)
	}
	if p.EVS[1].IntentID != "std::intent::sha256::crtm::pcr0" {
		t.Errorf("ev order not kept: %s", p.EVS[1].IntentID)
	}
	if got := p.ProvisionWorklist[len(p.ProvisionWorklist)-1]; got != "processevs_withrules" {
		t.Errorf("last worklist item = %s (commented-out processevs should be skipped)", got)
	}
	if p.Endpoints["tarzan"].Protocol != "A10HTTPRESTv2" {
		t.Errorf("endpoints = %+v", p.Endpoints)
	}
}

func TestUnquotedHexHandleStaysLiteral(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.yaml")
	os.WriteFile(f, []byte("attestationserver: http://x\ntpm2:\n  ek:\n    handle: 0x810100EE\n"), 0o644)
	p, err := loadProvisioningFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if p.TPM2.EK.Handle != "0x810100EE" {
		t.Errorf("handle = %q", p.TPM2.EK.Handle)
	}
}

func TestEVEntryRejectsMultipleIntents(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.yaml")
	os.WriteFile(f, []byte("attestationserver: http://x\nevs:\n  - a:\n      protocol: t\n    b:\n      protocol: t\n"), 0o644)
	if _, err := loadProvisioningFile(f); err == nil {
		t.Error("expected an error for an evs entry with two intents")
	}
}

func TestCreateRunsWholeWorklist(t *testing.T) {
	h := newHarness(t, "create")
	e, err := h.pv.ProcessWorklist()
	if err != nil {
		t.Fatal(err)
	}

	// TPM: 2 evictions + 4 provisioning commands, with the configured handles.
	if len(h.cmds) != 6 {
		t.Fatalf("got %d tpm commands: %v", len(h.cmds), h.cmds)
	}
	if strings.Join(h.cmds[0], " ") != "/usr/bin/tpm2_evictcontrol -c 0x810100EE" {
		t.Errorf("cmd 0 = %v", h.cmds[0])
	}
	if strings.Join(h.cmds[4], " ") != "/usr/bin/tpm2_evictcontrol -c /tmp/ak.ctx 0x810100AA" {
		t.Errorf("cmd 4 = %v", h.cmds[4])
	}

	// Element: created and ID written.
	if e.Host == nil || e.UEFI == nil || e.IMA == nil || e.TPM2 == nil {
		t.Errorf("element missing sections: %+v", e)
	}
	want := "Debian 13 VMElement name: (Debian13VM) Entry added at 2026-09-29 19:53:12.123456+00:00 UTC"
	if e.Description != want {
		t.Errorf("description = %q", e.Description)
	}
	if id, _ := os.ReadFile(h.pv.IDFile); string(id) != "E1" {
		t.Errorf("id file = %q", id)
	}
	el := h.jane.bodies["POST /element"][0].(map[string]any)
	if _, ok := el["itemid"]; ok {
		t.Error("create should not send itemid")
	}
	if el["tpm2"].(map[string]any)["ak"].(map[string]any)["handle"] != "0x810100AA" {
		t.Errorf("element tpm2 = %v", el["tpm2"])
	}
	if el["uefi"].(map[string]any)["eventlog"] != uefiEventLog {
		t.Errorf("element uefi = %v", el["uefi"])
	}

	// EVs: one session, 7 attestations, 3 typed EVs created, 18 rules run.
	j := h.jane
	for key, n := range map[string]int{
		"POST /session": 1, "DELETE /session": 1, "POST /attest": 7, "GET /claim": 7,
		"GET /expectedValue": 4, "POST /expectedValue": 4, "PUT /expectedValue": 0, "POST /verify": 18,
	} {
		if got := j.count(key); got != n {
			t.Errorf("%s called %d times, want %d", key, got, n)
		}
	}
	if last := j.calls[len(j.calls)-1]; last != "DELETE /session/S1" {
		t.Errorf("last call = %s, want the session closed", last)
	}

	at := j.bodies["POST /attest"][0].(map[string]any)
	if at["eid"] != "E1" || at["pid"] != "std::intent::sys::info" || at["epn"] != "tarzan" || at["sid"] != "S1" {
		t.Errorf("attest body = %v", at)
	}

	evs := j.bodies["POST /expectedValue"]
	sys := evs[0].(map[string]any)
	if sys["name"] != "Debian13VM---std::intent::sys::info" ||
		sys["evs"].(map[string]any)["machineid"] != "abc123" {
		t.Errorf("sysmachineid ev = %v", sys)
	}
	q := evs[1].(map[string]any)
	if q["name"] != "Debian13VM---std::intent::sha256::crtm::pcr0-std::intent::sha256::crtm::pcr0" {
		t.Errorf("quote ev name = %v", q["name"])
	}
	qe := q["evs"].(map[string]any)
	if qe["attestedValue"] != "deadbeef" {
		t.Errorf("quote evs = %v", qe)
	}
	// 2^53+1 must survive exactly (it would round through float64).
	if !strings.Contains(j.raw["POST /expectedValue"][1], `"firmwareVersion":9007199254740993`) {
		t.Errorf("firmwareVersion not passed through exactly: %s", j.raw["POST /expectedValue"][1])
	}
	if q["description"] != "Debian13VM---tpm2quote at 2026-09-29 19:53:12.123456+00:00 UTC" {
		t.Errorf("quote description = %v", q["description"])
	}

	v := j.bodies["POST /verify"][0].(map[string]any)
	if v["cid"] != "C1" || v["sid"] != "S1" || v["rule"] != "sys_taRunningSafely" {
		t.Errorf("verify body = %v", v)
	}
}

func TestUpdateUsesIDFileAndUpdatesEVs(t *testing.T) {
	h := newHarness(t, "update")
	h.jane.evExists = true
	os.WriteFile(h.pv.IDFile, []byte("E1\n"), 0o644)

	if code := h.pv.Run(); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	el := h.jane.bodies["PUT /element"]
	if len(el) != 1 || el[0].(map[string]any)["itemid"] != "E1" {
		t.Errorf("PUT /element bodies = %v", el)
	}
	puts := h.jane.bodies["PUT /expectedValue"]
	if len(puts) != 4 || puts[0].(map[string]any)["itemid"] != "EV9" {
		t.Errorf("PUT /expectedValue bodies = %v", puts)
	}
	if h.jane.count("POST /expectedValue") != 0 {
		t.Error("existing EVs should be updated, not created")
	}
}

func TestUpdateWithoutIDFileFails(t *testing.T) {
	h := newHarness(t, "update")
	if code := h.pv.Run(); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if len(h.jane.calls) != 0 {
		t.Errorf("no requests expected, got %v", h.jane.calls)
	}
}

func TestProcessEVsWithoutElementFails(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.P.ProvisionWorklist = []string{"processevs"}
	if _, err := h.pv.ProcessWorklist(); err == nil {
		t.Error("expected an error without processelement first")
	}
}

func TestPlainProcessEVsRunsNoRules(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	if n := h.jane.count("POST /verify"); n != 0 {
		t.Errorf("verify called %d times", n)
	}
}

func TestSafetyPrompts(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Unsafe = false
	h.pv.P.ProvisionWorklist = []string{"tpmclear", "tpmprovision"}
	h.pv.In = bufio.NewReader(strings.NewReader("maybe\nn\ny\n"))
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatal(err)
	}
	// clear declined (after an invalid answer), provision accepted
	if len(h.cmds) != 4 || !strings.HasSuffix(h.cmds[0][0], "tpm2_createek") {
		t.Errorf("cmds = %v", h.cmds)
	}

	h.pv.In = bufio.NewReader(strings.NewReader("q\n"))
	if _, err := h.pv.ProcessWorklist(); !errors.Is(err, errQuit) {
		t.Errorf("err = %v, want errQuit", err)
	}

	h.pv.In = bufio.NewReader(strings.NewReader(""))
	if _, err := h.pv.ProcessWorklist(); err == nil || errors.Is(err, errQuit) {
		t.Errorf("EOF: err = %v, want a read error", err)
	}
}

func TestCreateWithExistingIDFileAsks(t *testing.T) {
	h := newHarness(t, "create")
	h.pv.Unsafe = false
	os.WriteFile(h.pv.IDFile, []byte("OLD"), 0o644)
	h.pv.In = bufio.NewReader(strings.NewReader("n\n"))
	if code := h.pv.Run(); code != 0 {
		t.Errorf("exit code %d", code)
	}
	if len(h.jane.calls) != 0 || len(h.cmds) != 0 {
		t.Error("declining should stop before any work")
	}
}

func TestServerErrorWithoutItemID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":"boom"}`)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).CreateElement(&Element{Name: "x"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}

func TestRunCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"create"},
		{"delete", "testdata/examplecreateprovisioningfile.yaml"},
		{"create", "testdata/nope.yaml"},
		{"create", "x.yaml", "--bogus"},
	} {
		if code := run(args); code == 0 {
			t.Errorf("run(%v) = 0, want non-zero", args)
		}
	}
}

func TestCollectHostInfo(t *testing.T) {
	if _, err := os.Stat("/etc/machine-id"); err != nil {
		t.Skip("no /etc/machine-id")
	}
	h, err := collectHostInfo()
	if err != nil {
		t.Fatal(err)
	}
	if h.Arch == "" || h.Hostname == "" || h.MachineID == "" || !strings.Contains(h.OS, h.Arch) {
		t.Errorf("host info = %+v", h)
	}
}
