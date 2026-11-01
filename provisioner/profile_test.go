package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, text string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestBuildProfileFromFixture(t *testing.T) {
	p, err := loadProvisioningFile("testdata/examplecreateprovisioningfile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	prof, ok := p.buildProfile()
	if !ok || len(prof.Steps) != len(p.EVS) {
		t.Fatalf("ok %v, %d steps for %d evs", ok, len(prof.Steps), len(p.EVS))
	}
	for i, ev := range p.EVS {
		st := prof.Steps[i]
		if st.Intent != ev.IntentID || st.Endpoint != ev.Spec.Protocol || st.CID != ev.Spec.CID || len(st.Rules) != len(ev.Spec.Rules) {
			t.Errorf("step %d = %+v from %+v", i, st, ev)
		}
	}
	// type and issuerurl concern expected values only: not in the profile
	b, _ := json.Marshal(prof)
	if strings.Contains(string(b), "tpm2quote") || strings.Contains(string(b), "issuerurl") {
		t.Errorf("profile carries expected-value fields: %s", b)
	}
	if p.profileName() != "default" {
		t.Errorf("name %q, want default", p.profileName())
	}
}

const profileFileHead = `attestationserver: http://127.0.0.1:8520
element:
  name: x
`

func TestProfileKeys(t *testing.T) {
	f := writeFile(t, profileFileHead+`defaultattestationprofile: nightly
attestationprofiledescription: Nightly check
evs:
  - std::intent::sys::info :
      protocol: tarzan
      type: sysmachineid
      rules: [ sys_taRunningSafely ]
follow:
  - relationship: hosted-on
    profile: hostcheck
  - relationship: runs-on
`)
	p, err := loadProvisioningFile(f)
	if err != nil {
		t.Fatal(err)
	}
	prof, ok := p.buildProfile()
	want := AttestationProfile{
		Description: "Nightly check",
		Steps:       []ProfileStep{{Intent: "std::intent::sys::info", Endpoint: "tarzan", Rules: []ProfileRule{{Name: "sys_taRunningSafely"}}}},
		Follow:      []FollowSpec{{Relationship: "hosted-on", Profile: "hostcheck"}, {Relationship: "runs-on"}},
	}
	if !ok || p.profileName() != "nightly" || !reflect.DeepEqual(prof, want) {
		t.Errorf("got %q %+v, want nightly %+v", p.profileName(), prof, want)
	}

	// only follow: a profile without steps (decision 37)
	f = writeFile(t, profileFileHead+"follow:\n  - relationship: hosted-on\n")
	p, _ = loadProvisioningFile(f)
	if prof, ok := p.buildProfile(); !ok || len(prof.Steps) != 0 {
		t.Errorf("follow only: %v %+v", ok, prof)
	}
	// neither: nothing to store
	f = writeFile(t, profileFileHead)
	p, _ = loadProvisioningFile(f)
	if _, ok := p.buildProfile(); ok {
		t.Error("an empty profile would be stored")
	}
}

func TestProfileKeyErrors(t *testing.T) {
	for _, text := range []string{
		"defaultattestationprofile: night.ly\n",
		"follow:\n  - profile: x\n",
		"follow:\n  - relationship: hosted-on\n    profile: a/b\n",
	} {
		_, err := loadProvisioningFile(writeFile(t, profileFileHead+text))
		var verr *VarError
		if !errors.As(err, &verr) {
			t.Errorf("%q: got %v, want a VarError (exit 2)", text, err)
		}
	}
}

func TestCreateStoresProfile(t *testing.T) {
	h := newHarness(t, "create")
	code, out := runCreate(t, h)
	if code != exitOK {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if h.jane.count("PUT /profile/element/E1/attestationprofile/default") != 1 {
		t.Errorf("calls %v", h.jane.calls)
	}
	if !strings.Contains(out, "WARNING: no defaultattestationprofile") || !strings.Contains(out, "attestation profile default stored") {
		t.Errorf("output:\n%s", out)
	}
	bodies := h.jane.bodies["PUT /profile"]
	if len(bodies) != 1 {
		t.Fatalf("profile bodies %v", bodies)
	}
	steps, _ := bodies[0].(map[string]any)["steps"].([]any)
	if len(steps) != len(h.pv.P.EVS) {
		t.Errorf("%d steps uploaded, want %d", len(steps), len(h.pv.P.EVS))
	}
}

func TestOldJaneProfileWarning(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.profileStatus = 404 // a Jane without attestation profiles
	code, out := runCreate(t, h)
	if code != exitOK || !strings.Contains(out, "does not support attestation profiles") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

// The substitution rules are shared with Jane (janeserver/operations): the
// same fixture is used by both test suites.
func TestSubstituteSharedFixture(t *testing.T) {
	var fx struct {
		Cases []struct {
			Name   string            `json:"name"`
			Params map[string]any    `json:"params"`
			Vars   map[string]string `json:"vars"`
			Want   map[string]any    `json:"want"`
			Unset  string            `json:"unset"`
		} `json:"cases"`
	}
	b, err := os.ReadFile("../janeserver/operations/testdata/profilevariables.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	for _, c := range fx.Cases {
		got, unset := substitute(c.Params, c.Vars)
		if unset != c.Unset || !reflect.DeepEqual(got, c.Want) {
			t.Errorf("%s: got %v %q, want %v %q", c.Name, got, unset, c.Want, c.Unset)
		}
	}
}

func TestParseRemoteArgs(t *testing.T) {
	isFile := func(f string) bool { return f == "p.yaml" }
	env := func(k string) string { return map[string]string{"JANE_SERVER": "http://env:8520"}[k] }
	noenv := func(string) string { return "" }
	five := "5"
	cases := []struct {
		args []string
		ro   remoteOptions
		env  func(string) string
		want remoteRequest
		err  bool
	}{
		{nil, remoteOptions{}, env, remoteRequest{server: "http://env:8520", followLimit: -1}, false},
		{[]string{"nightly"}, remoteOptions{server: "http://s"}, noenv, remoteRequest{profile: "nightly", server: "http://s", followLimit: -1}, false},
		{[]string{"p.yaml"}, remoteOptions{}, noenv, remoteRequest{pfile: "p.yaml", followLimit: -1}, false},
		{[]string{"p.yaml", "nightly"}, remoteOptions{noFollow: true, parallel: true}, noenv, remoteRequest{pfile: "p.yaml", profile: "nightly", followLimit: 0, parallel: true}, false},
		{nil, remoteOptions{followLimit: &five, parent: "S0"}, env, remoteRequest{server: "http://env:8520", parent: "S0", followLimit: 5}, false},
		{nil, remoteOptions{}, noenv, remoteRequest{}, true},                                    // no server at all
		{[]string{"bad.name"}, remoteOptions{server: "http://s"}, noenv, remoteRequest{}, true}, // not a file, not a profile name
		{nil, remoteOptions{noFollow: true, followLimit: &five}, env, remoteRequest{}, true},
		{[]string{"a", "b", "c"}, remoteOptions{}, env, remoteRequest{}, true},
	}
	for i, c := range cases {
		got, err := parseRemoteArgs(c.args, c.ro, c.env, isFile)
		if (err != nil) != c.err || (!c.err && got != c.want) {
			t.Errorf("case %d: got %+v %v", i, got, err)
		}
	}
}

const reportPass = `{"elementid":"E1","elementname":"vm","profile":"nightly","sessionid":"S1","parent":"",
 "steps":[{"intent":"i1","endpoint":"tarzan","claimid":"C1","error":"","rules":[{"rule":"r1","resultid":"R1","result":0,"message":"ok","notrun":"","error":""}]}],
 "vars":[{"name":"q","kind":"cid","value":"C1"}],
 "follow":[
  {"relationship":"hosted-on","target":"H1","profile":"","kind":"run","message":"","report":
    {"elementid":"H1","elementname":"host","profile":"hostcheck","sessionid":"S2","parent":"S1","steps":[],"vars":[],"follow":[]}},
  {"relationship":"hosted-on","target":"H2","profile":"","kind":"skipped","message":"already in the plan"},
  {"relationship":"hosted-on","target":"H3","profile":"","kind":"limit","message":"follow limit 1 reached"}]}`

func remoteHarness(t *testing.T, status int, body string) *harness {
	h := newHarness(t, "attest")
	os.WriteFile(h.pv.IDFile, []byte("E1\n"), 0o644)
	h.jane.runStatus, h.jane.runBody = status, body
	return h
}

func TestRemoteAttest(t *testing.T) {
	var out strings.Builder

	h := remoteHarness(t, 0, reportPass)
	h.pv.Out = &out
	if code := h.pv.remoteAttest("E1", remoteRequest{profile: "nightly", followLimit: 1, parallel: true, parent: "S0"}); code != exitOK {
		t.Errorf("all passed, skips and the limit don't count: exit %d\n%s", code, out.String())
	}
	if h.jane.count("POST /profile/element/E1/runprov/nightly") != 1 || len(h.jane.queries) != 1 || h.jane.queries[0] != "followlimit=1&parallel=true&parent=S0" {
		t.Errorf("calls %v queries %v", h.jane.calls, h.jane.queries)
	}
	for _, want := range []string{"this may take a while", "Jane's Web UI", "PASS   r1", "follow hosted-on -> H1", "Profile hostcheck on host", "SKIPPED", "LIMIT", "2 sessions"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	// a follow error fails the run (decision 25)
	h = remoteHarness(t, 0, strings.Replace(reportPass, `"kind":"limit"`, `"kind":"error"`, 1))
	h.pv.Out = &out
	if code := h.pv.remoteAttest("E1", remoteRequest{followLimit: -1}); code != exitNotTrust {
		t.Errorf("follow error: exit %d", code)
	}
	if h.jane.count("POST /profile/element/E1/runprov") != 1 {
		t.Errorf("default profile: calls %v", h.jane.calls)
	}

	// a failed rule deep in the tree
	h = remoteHarness(t, 0, strings.Replace(reportPass, `"steps":[],"vars":[]`, `"steps":[{"intent":"i","endpoint":"tarzan","claimid":"C2","rules":[{"rule":"r","result":9001}]}],"vars":[]`, 1))
	h.pv.Out = &out
	if code := h.pv.remoteAttest("E1", remoteRequest{followLimit: -1}); code != exitNotTrust {
		t.Errorf("failed rule in a followed run: exit %d", code)
	}

	// refused: no such profile; and a Jane without profiles
	h = remoteHarness(t, 404, `{"error":"no such attestation profile: nope"}`)
	out.Reset()
	h.pv.Out = &out
	if code := h.pv.remoteAttest("E1", remoteRequest{profile: "nope", followLimit: -1}); code != exitError || !strings.Contains(out.String(), "no such attestation profile") {
		t.Errorf("refused: exit %d\n%s", code, out.String())
	}
	h = remoteHarness(t, 404, `{"message":"Not Found"}`)
	out.Reset()
	h.pv.Out = &out
	if code := h.pv.remoteAttest("E1", remoteRequest{followLimit: -1}); code != exitError || !strings.Contains(out.String(), "does not support attestation profiles") {
		t.Errorf("old Jane: exit %d\n%s", code, out.String())
	}
}
