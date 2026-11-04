package uefirules

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"a10/structures"

	"github.com/iolivergithub/jane/lib/uefi"
)

func logClaim(t *testing.T) structures.Claim {
	b, err := os.ReadFile("../../../lib/uefi/testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Fatal(err)
	}
	return structures.Claim{ItemID: "CL", BodyType: "uefi/eventlog",
		Header: structures.ClaimHeader{Intent: structures.Intent{Function: "uefi/eventlog"}, Element: structures.Element{ItemID: "E1"}},
		Body:   map[string]interface{}{"eventlog": base64.StdEncoding.EncodeToString(b)}}
}

func ruleNamed(t *testing.T, name string) structures.Rule {
	for _, r := range Registration() {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no rule %s", name)
	return structures.Rule{}
}

func runLog(t *testing.T, name string, claim structures.Claim, evs map[string]interface{}, params map[string]interface{}) (structures.ResultValue, string) {
	t.Helper()
	rv, msg, err := ruleNamed(t, name).CallFunction(claim, name, structures.ExpectedValue{Name: "ev", EVS: evs}, structures.Session{}, params)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return rv, msg
}

// the measured values of the test log (cos-85-amd-sev.bin)
var cosEVS = map[string]interface{}{
	"secureboot":      "enabled",
	"pk":              "sha256:ec70b95b7fce7a97faeaf5c536f5d496406d6846000ab89116185380d5cd29de",
	"kek":             "sha256:7b83a1ae5c2e2049b044f1ece68b6b81a1fabff175d9a6ae15da5dbfe701ec13",
	"db":              "sha256:640ff6e96d1808b9a1ba9dc2f75f80b2d030103757595706aaf7d47f805f3ef9",
	"dbx":             "sha256:d0e6400fb15f4f9bc679de112b3f25b364789f3e5dfb326e37eb886e39bf2631",
	"authorities":     []interface{}{"sha256:a7b8c851deb2acba7c7d0e6ec091c93440f942c65d776fdcb58016f8fd1674b0"},
	"dmaprotection":   "notreported",
	"firmwareversion": "GCE Virtual Firmware v1",
}

func TestLogRulesPass(t *testing.T) {
	c := logClaim(t)
	for _, n := range []string{"uefi_logSecureBoot", "uefi_logPK", "uefi_logKEK", "uefi_logdb", "uefi_logdbx", "uefi_logAuthorities", "uefi_logDMAProtection", "uefi_logFirmwareVersion"} {
		if rv, msg := runLog(t, n, c, cosEVS, nil); rv != structures.Success {
			t.Errorf("%s: %v %s", n, rv, msg)
		}
	}
}

func TestLogRulesFail(t *testing.T) {
	c := logClaim(t)
	off := map[string]interface{}{"secureboot": "disabled", "authorities": []interface{}{"sha256:00"}, "pk": "absent"}
	for n, want := range map[string]string{"uefi_logSecureBoot": "secureboot is enabled, expected disabled", "uefi_logAuthorities": "1 of 1 authorities are not expected", "uefi_logPK": "expected absent"} {
		if rv, msg := runLog(t, n, c, off, nil); rv != structures.Fail || !strings.Contains(msg, want) {
			t.Errorf("%s: %v %q, want Fail %q", n, rv, msg, want)
		}
	}
	if rv, msg := runLog(t, "uefi_logKEK", c, map[string]interface{}{}, nil); rv != structures.Fail || !strings.Contains(msg, "does not mention kek") {
		t.Errorf("missing entry: %v %s", rv, msg)
	}
	other := c
	other.Header.Intent.Function = "uefi/efivars"
	if rv, msg := runLog(t, "uefi_logSecureBoot", other, cosEVS, nil); rv != structures.Fail || !strings.Contains(msg, "uefi/eventlog claim") {
		t.Errorf("wrong claim: %v %s", rv, msg)
	}
}

func TestLogMatchesEfivars(t *testing.T) {
	logc := logClaim(t)
	b, _ := os.ReadFile("../../../lib/uefi/testdata/cos-85-amd-sev.bin")
	el, _ := uefi.DecodeEventLog(base64.StdEncoding.EncodeToString(b))
	st, err := uefi.MeasuredState(el)
	if err != nil {
		t.Fatal(err)
	}
	efivars := func(skip string, change string) structures.Claim {
		var vs []interface{}
		for _, name := range []string{"SecureBoot", "PK", "KEK", "db", "dbx"} {
			if name == skip {
				continue
			}
			d := st.Data[name]
			if name == change {
				d = append([]byte{}, d...)
				d[0] ^= 1
			}
			v := variableNamed(name)
			vs = append(vs, map[string]interface{}{"name": name, "guid": v.GUID, "attributes": 7, "size": len(d), "rawvalue": base64.StdEncoding.EncodeToString(d)})
		}
		return structures.Claim{ItemID: "EV", BodyType: "uefi/efivars",
			Header: structures.ClaimHeader{Intent: structures.Intent{Function: "uefi/efivars"}, Element: structures.Element{ItemID: "E1"}},
			Body:   map[string]interface{}{"efivars": vs, "count": len(vs)}}
	}
	saved := getClaim
	defer func() { getClaim = saved }()
	getClaim = func(string) (structures.Claim, error) { return logc, nil }
	params := map[string]interface{}{"uefilogcid": "CL"}

	if rv, msg := runLog(t, "uefi_logMatchesEfivars", efivars("", ""), nil, params); rv != structures.Success {
		t.Errorf("same: %v %s", rv, msg)
	}
	if rv, msg := runLog(t, "uefi_logMatchesEfivars", efivars("", "dbx"), nil, params); rv != structures.Fail || !strings.Contains(msg, "dbx differs") {
		t.Errorf("changed dbx: %v %s", rv, msg)
	}
	if rv, msg := runLog(t, "uefi_logMatchesEfivars", efivars("KEK", ""), nil, params); rv != structures.Fail || !strings.Contains(msg, "KEK measured but not reported") {
		t.Errorf("missing KEK: %v %s", rv, msg)
	}
	if rv, msg := runLog(t, "uefi_logMatchesEfivars", efivars("", ""), nil, nil); rv != structures.Fail || !strings.Contains(msg, "uefilogcid") {
		t.Errorf("no parameter: %v %s", rv, msg)
	}
	if rv, msg := runLog(t, "uefi_logMatchesEfivars", logc, nil, params); rv != structures.Fail || !strings.Contains(msg, "uefi/efivars claim") {
		t.Errorf("on the log claim: %v %s", rv, msg)
	}
}
