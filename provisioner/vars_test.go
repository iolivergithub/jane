package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Variables in the provisioning file: cid, rid, resvar and $name (#72).

const varsEVs = `attestationserver: http://127.0.0.1:1
evs:
  - std::intent::uefi::eventlog :
      protocol: tarzan
      cid: uefilog
  - std::intent::tpm::pcrs :
      protocol: tarzan
      cid: pcrs
      rules:
        - tpm2_uefiLogReplay:
            rid: r_log
            resvar: r_log_code
            parameters: { "uefilogcid": $uefilog }
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      type: tpm2quote
      cid: q_pcr0
      rules:
        - tpm2_attestedValue
        - tpm2_pcrDigest:
            rid: r_quote
            resvar: r_quote_code
            parameters:
              pcrscid: "$pcrs"
`

func writeProvisioningFile(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "p.yaml")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestVarsParse(t *testing.T) {
	p, err := loadProvisioningFile(writeProvisioningFile(t, varsEVs))
	if err != nil {
		t.Fatal(err)
	}
	if p.EVS[1].Spec.CID != "pcrs" {
		t.Errorf("cid = %q", p.EVS[1].Spec.CID)
	}
	q := p.EVS[2].Spec
	if got := strings.Join(q.RuleNames(), ","); got != "tpm2_attestedValue,tpm2_pcrDigest" {
		t.Errorf("rules = %s", got)
	}
	r := q.Rules[1]
	if r.RID != "r_quote" || r.ResVar != "r_quote_code" || r.Parameters["pcrscid"] != "$pcrs" {
		t.Errorf("rule = %+v", r)
	}
	if p.EVS[1].Spec.Rules[0].Parameters["uefilogcid"] != "$uefilog" {
		t.Errorf("box-style parameters = %+v", p.EVS[1].Spec.Rules[0].Parameters)
	}
}

func TestVarsErrors(t *testing.T) {
	for name, c := range map[string]struct{ from, to string }{
		"clash cid/rid":     {"rid: r_log", "rid: uefilog"},
		"clash rid/resvar":  {"resvar: r_quote_code", "resvar: r_quote"},
		"undefined":         {`pcrscid: "$pcrs"`, `pcrscid: "$nosuch"`},
		"forward reference": {`{ "uefilogcid": $uefilog }`, `{ "uefilogcid": $q_pcr0 }`},
		"own rid":           {`{ "uefilogcid": $uefilog }`, `{ "uefilogcid": $r_log }`},
		"bad name":          {"cid: pcrs\n", "cid: 1pcrs\n"},
	} {
		f := writeProvisioningFile(t, strings.Replace(varsEVs, c.from, c.to, 1))
		_, err := loadProvisioningFile(f)
		var verr *VarError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a VarError", name, err)
		}
		if code := run([]string{"attest", f}); code != exitUsage {
			t.Errorf("%s: exit %d, want %d", name, code, exitUsage)
		}
	}

	// parameters given as text rather than a mapping
	f := writeProvisioningFile(t, strings.Replace(varsEVs, `{ "uefilogcid": $uefilog }`, `'{ "uefilogcid": $uefilog }'`, 1))
	if _, err := loadProvisioningFile(f); err == nil || !strings.Contains(err.Error(), "must be a mapping") {
		t.Errorf("text parameters: err = %v", err)
	}
}

func varsHarness(t *testing.T) *harness {
	h := attestHarness(t)
	p, err := loadProvisioningFile(writeProvisioningFile(t, varsEVs))
	if err != nil {
		t.Fatal(err)
	}
	h.pv.P.EVS = p.EVS
	return h
}

func TestVarsSubstitutedAndTabled(t *testing.T) {
	h := varsHarness(t)
	h.jane.ruleResults = map[string]int{"tpm2_pcrDigest": ResultFail}
	var out bytes.Buffer
	h.pv.Out = &out
	report, err := h.pv.Attest()
	if err != nil {
		t.Fatal(err)
	}
	report.Print(h.pv)

	verifies := h.jane.bodies["POST /verify"]
	if len(verifies) != 3 {
		t.Fatalf("%d verifies", len(verifies))
	}
	log := verifies[0].(map[string]any)
	if log["rule"] != "tpm2_uefiLogReplay" || log["parameters"].(map[string]any)["uefilogcid"] != "C1" {
		t.Errorf("first verify = %v", log)
	}
	if _, ok := verifies[1].(map[string]any)["parameters"]; ok {
		t.Errorf("a rule without parameters sent some: %v", verifies[1])
	}
	if verifies[2].(map[string]any)["parameters"].(map[string]any)["pcrscid"] != "C1" {
		t.Errorf("third verify = %v", verifies[2])
	}

	want := []Var{
		{"uefilog", "cid", "C1"}, {"pcrs", "cid", "C1"},
		{"r_log", "rid", "R-tpm2_uefiLogReplay"}, {"r_log_code", "resvar", "0 Success"},
		{"q_pcr0", "cid", "C1"},
		{"r_quote", "rid", "R-tpm2_pcrDigest"}, {"r_quote_code", "resvar", "9001 Fail"},
	}
	if len(report.Vars) != len(want) {
		t.Fatalf("vars = %+v", report.Vars)
	}
	for i := range want {
		if report.Vars[i] != want[i] {
			t.Errorf("var %d = %+v, want %+v", i, report.Vars[i], want[i])
		}
	}
	for _, s := range []string{"var                  | kind   | value", "r_quote_code         | resvar | 9001 Fail"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output lacks %q:\n%s", s, out.String())
		}
	}
}

func TestVarsFromFailedStepNotRun(t *testing.T) {
	h := varsHarness(t)
	h.jane.failIntents = map[string]bool{"std::intent::uefi::eventlog": true}
	var out bytes.Buffer
	h.pv.Out = &out
	report, err := h.pv.Attest()
	if err != nil {
		t.Fatal(err)
	}
	report.Print(h.pv)
	if n := h.jane.count("POST /verify"); n != 2 {
		t.Errorf("%d verifies, want 2 (tpm2_uefiLogReplay not run)", n)
	}
	passed, failed, errs := report.Counts()
	if passed != 2 || failed != 1 || errs != 1 || report.Trusted() {
		t.Errorf("counts = %d passed, %d failed, %d errors", passed, failed, errs)
	}
	if !strings.Contains(out.String(), "NOTRUN tpm2_uefiLogReplay: $uefilog has no value (its step failed)") {
		t.Errorf("output:\n%s", out.String())
	}
}
