package uefirules

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"a10/structures"
	"a10/utilities"
)

func efivar(name, guid string, data []byte) map[string]interface{} {
	return map[string]interface{}{
		"name": name, "guid": guid, "attributes": 0x27, "somevalue": len(data),
		"value": string(data), "errorvalue": "", "rawvalue": base64.StdEncoding.EncodeToString(data),
	}
}

// claimOf builds a claim body the way Jane receives it: through JSON.
func claimOf(t *testing.T, vars ...map[string]interface{}) structures.Claim {
	j, err := json.Marshal(map[string]interface{}{"efivars": vars, "count": len(vars)})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(j, &body); err != nil {
		t.Fatal(err)
	}
	return structures.Claim{BodyType: "uefi/efivars", Body: body}
}

func run(t *testing.T, name string, claim structures.Claim, evs map[string]interface{}) (structures.ResultValue, string) {
	t.Helper()
	for _, r := range Registration() {
		if r.Name == "UEFI_"+name {
			if !r.NeedsEV {
				t.Fatalf("%v should need an EV", r.Name)
			}
			rv, msg, err := r.CallFunction(claim, r.Name, structures.ExpectedValue{Name: "test", EVS: evs}, structures.Session{}, nil)
			if err != nil {
				t.Fatalf("%v returned error %v (%v)", r.Name, err, msg)
			}
			return rv, msg
		}
	}
	t.Fatalf("no rule UEFI_%v", name)
	return 0, ""
}

func TestRegistration(t *testing.T) {
	rs := Registration()
	if len(rs) != 17 {
		t.Fatalf("expected 17 rules, got %d", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.Name] {
			t.Errorf("duplicate rule %v", r.Name)
		}
		seen[r.Name] = true
	}
	for _, n := range []string{"UEFI_SecureBoot", "UEFI_PK", "UEFI_db", "UEFI_dbx", "UEFI_MokSBStateRT", "UEFI_SbatLevelRT", "UEFI_dbxDefault"} {
		if !seen[n] {
			t.Errorf("missing rule %v", n)
		}
	}
}

func TestRules(t *testing.T) {
	g, isdb, shim := utilities.EfiGlobalVariableGUID, utilities.EfiImageSecurityDatabaseGUID, utilities.ShimLockGUID
	pk := make([]byte, 900)
	for i := range pk {
		pk[i] = byte(i * 7)
	}
	pkHash := sha256.Sum256(pk)
	sbat := []byte("sbat,1,2024010900\nshim,4\ngrub,3\n")

	claim := claimOf(t,
		efivar("SecureBoot", g, []byte{1}),
		efivar("SetupMode", g, []byte{0}),
		efivar("PK", g, pk),
		efivar("db", isdb, []byte{0xa1, 0x59}),
		efivar("SbatLevelRT", shim, sbat),
		efivar("dbx", "00000000-0000-0000-0000-000000000000", []byte{1}), // wrong GUID: not the real dbx
	)

	cases := []struct {
		name string
		ev   interface{} // nil means not mentioned
		want structures.ResultValue
	}{
		{"SecureBoot", "01", structures.Success},
		{"SecureBoot", "0x01", structures.Success},
		{"SecureBoot", "00", structures.Fail},
		{"SecureBoot", nil, structures.Fail},
		{"SetupMode", "00", structures.Success},
		{"PK", "sha256:" + hex.EncodeToString(pkHash[:]), structures.Success},
		{"PK", "SHA256:" + hex.EncodeToString(pkHash[:]), structures.Success},
		{"PK", "sha256:00", structures.Fail},
		{"PK", hex.EncodeToString(pk), structures.Success},
		{"db", "A1:59", structures.Success},
		{"SbatLevelRT", hex.EncodeToString(sbat), structures.Success},
		{"MokSBStateRT", "absent", structures.Success},
		{"MokSBStateRT", "01", structures.Fail},
		{"SecureBoot", "absent", structures.Fail},
		{"dbx", "01", structures.Fail}, // only present under another GUID
		{"KEK", "sha256:00", structures.Fail},
		{"KEK", nil, structures.Fail},
	}
	for _, c := range cases {
		evs := map[string]interface{}{}
		if c.ev != nil {
			evs[c.name] = c.ev
		}
		got, msg := run(t, c.name, claim, evs)
		if got != c.want {
			t.Errorf("%v with %v: got %v want %v (%v)", c.name, c.ev, got, c.want, msg)
		}
	}
}

func TestLegacyAgent(t *testing.T) {
	g := utilities.EfiGlobalVariableGUID
	// older tarzan: no rawvalue, the buffer is longer than the value, and PK has invalid UTF-8
	claim := claimOf(t,
		map[string]interface{}{"name": "SecureBoot", "guid": g, "attributes": 6, "somevalue": 1, "value": string([]byte{1, 0, 0, 0, 0, 0, 0, 0}), "errorvalue": ""},
		map[string]interface{}{"name": "PK", "guid": g, "attributes": 0x27, "somevalue": 3, "value": string([]byte{0xa1, 0x59, 0xc0}), "errorvalue": ""},
	)
	if got, msg := run(t, "SecureBoot", claim, map[string]interface{}{"SecureBoot": "01"}); got != structures.Success {
		t.Errorf("legacy SecureBoot: %v %v", got, msg)
	}
	if got, msg := run(t, "PK", claim, map[string]interface{}{"PK": "a159c0"}); got != structures.VerifyCallFailure {
		t.Errorf("legacy lossy PK: %v %v", got, msg)
	}
}

func TestReadError(t *testing.T) {
	claim := claimOf(t, map[string]interface{}{"name": "KEK", "guid": utilities.EfiGlobalVariableGUID, "attributes": 0, "somevalue": 0, "value": "", "errorvalue": "permission denied", "rawvalue": ""})
	if got, msg := run(t, "KEK", claim, map[string]interface{}{"KEK": "00"}); got != structures.Fail {
		t.Errorf("read error: %v %v", got, msg)
	}
}

func TestNotAnEfivarsClaim(t *testing.T) {
	r := Registration()[0]
	_, _, err := r.CallFunction(structures.Claim{Body: map[string]interface{}{"quote": "x"}}, r.Name, structures.ExpectedValue{EVS: map[string]interface{}{"SecureBoot": "01"}}, structures.Session{}, nil)
	if err == nil {
		t.Errorf("expected an error for a claim without efivars")
	}
}

type kv struct {
	Key   string
	Value interface{}
}

// As read back from Mongo: bson.D-like documents and int32/int64 numbers.
func TestDatabaseShapedBody(t *testing.T) {
	d := []kv{{"name", "SecureBoot"}, {"guid", utilities.EfiGlobalVariableGUID}, {"attributes", int32(6)}, {"somevalue", int64(1)}, {"value", "\x01"}, {"errorvalue", ""}, {"rawvalue", "AQ=="}}
	claim := structures.Claim{Body: map[string]interface{}{"efivars": []interface{}{d}, "count": int32(1)}}
	if got, msg := run(t, "SecureBoot", claim, map[string]interface{}{"SecureBoot": "01"}); got != structures.Success {
		t.Errorf("db-shaped: %v %v", got, msg)
	}
}
