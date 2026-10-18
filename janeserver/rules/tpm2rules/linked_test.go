package tpm2rules

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"a10/operations"
	"a10/structures"
)

// fakeClaims replaces getClaim with a lookup in the given claims.
func fakeClaims(t *testing.T, claims ...structures.Claim) {
	t.Helper()
	old := getClaim
	t.Cleanup(func() { getClaim = old })
	getClaim = func(id string) (structures.Claim, error) {
		for _, c := range claims {
			if c.ItemID == id {
				return c, nil
			}
		}
		return structures.Claim{}, operations.ErrorItemNotFound
	}
}

func makeClaim(id, function, element, session string, body map[string]interface{}) structures.Claim {
	var c structures.Claim
	c.ItemID = id
	c.BodyType = function
	c.Header.Intent.Function = function
	c.Header.Element.ItemID = element
	c.Header.Session.ItemID = session
	c.Body = body
	return c
}

// PCR values of the COS 85 test log, from go-eventlog's replay tests.
var cos85SHA256 = map[string]interface{}{
	"0": "0f35c214608d93c7a6e68ae7359b4a8be5a0e99eea9107ece427c4dea4e439cf",
	"1": "6eb40f5b6bfafcb9914d486ce59404acd24bc13a6a3c45cda3b44c9d7053d638",
	"2": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
	"3": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
	"4": "d690bdac2aa8b73a1d718cb91990df07d0747b07ea57b3b2d0f0d511f0d90491",
	"5": "e9e0b32564b6f8215b1bd43954d9f910682d39c3b18abd4737ac3b797cf269e0",
	"6": "3d458cfe55cc03ea1f443f1562beec8df51c75e14a9fcf9a7234a13f198e7969",
	"7": "3365d7fa2b024c852913c06e04ffbfa6ea5289f743bbf1a76f7ffdf21ed84793",
	"8": "9e9b6511ae6ad443aae4c7bf998ffffbcd271c874f1efab9d692f129eb6e6c18",
	"9": "f4f2d92d6d54f6c41f2706fd98091317642e0680a7902c72893d41e3464a93b7",
}

var cos85SHA1 = map[string]interface{}{
	"0": "c032c3b51dbb6f96b047421512fd4b4dfde496f3",
	"1": "e3e9e1d9deacd95b289bbbd3a1717a57af7d211b",
	"2": "b2a83b0ebf2f8374299a5b2bdfc31ea955ad7236",
	"3": "b2a83b0ebf2f8374299a5b2bdfc31ea955ad7236",
	"4": "6168c9ce88a8658920f2cf2f9012d3c6bbfab79b",
	"5": "fb6b3a15b220a74b0c4f73416919476702e930e2",
	"6": "b2a83b0ebf2f8374299a5b2bdfc31ea955ad7236",
	"7": "42e669233f0e826df5093abfd6998c020df2de88",
	"8": "72778b0ba3c491db25eb7c8368cb1fb51f0ce458",
	"9": "08bd04f0dbadf591510340d94a0019c0ddcb779f",
}

func copyBank(b map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func TestUEFILogReplay(t *testing.T) {
	raw, err := os.ReadFile("../../utilities/testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Fatal(err)
	}
	logClaim := makeClaim("log", "uefi/eventlog", "e1", "s1", map[string]interface{}{"eventlog": base64.StdEncoding.EncodeToString(raw)})
	otherElement := makeClaim("log2", "uefi/eventlog", "e2", "s1", logClaim.Body)
	notALog := makeClaim("q", "tpm2/quote", "e1", "s1", map[string]interface{}{})
	fakeClaims(t, logClaim, otherElement, notALog)

	good := map[string]interface{}{"sha1": copyBank(cos85SHA1), "sha256": copyBank(cos85SHA256)}
	bad := map[string]interface{}{"sha256": copyBank(cos85SHA256)}
	bad["sha256"].(map[string]interface{})["4"] = strings.Repeat("00", 32)

	cases := []struct {
		name  string
		body  map[string]interface{}
		param map[string]interface{}
		want  structures.ResultValue
		msg   string
	}{
		{"matches", good, map[string]interface{}{"uefilogcid": "log"}, structures.Success, "sha256: PCRs [0"},
		{"PCR 4 changed", bad, map[string]interface{}{"uefilogcid": "log"}, structures.Fail, "PCRs [4] do NOT match"},
		{"sha1 only in log", bad, map[string]interface{}{"uefilogcid": "log"}, structures.Fail, "sha1: not in the PCRs claim, skipped"},
		{"no parameter", good, map[string]interface{}{}, structures.Fail, "uefilogcid"},
		{"unknown claim", good, map[string]interface{}{"uefilogcid": "nope"}, structures.Fail, "could not be read"},
		{"wrong claim type", good, map[string]interface{}{"uefilogcid": "q"}, structures.Fail, "is a tpm2/quote claim"},
		{"other element", good, map[string]interface{}{"uefilogcid": "log2"}, structures.Fail, "another element"},
		{"no bank in common", map[string]interface{}{"sha384": map[string]interface{}{}}, map[string]interface{}{"uefilogcid": "log"}, structures.Fail, "No bank to compare"},
	}
	for _, c := range cases {
		pcrs := makeClaim("pcrs", "tpm2/pcrs", "e1", "s1", c.body)
		got, msg, _ := UEFILogReplay(pcrs, "tpm2_uefiLogReplay", structures.ExpectedValue{}, structures.Session{}, c.param)
		if got != c.want || !strings.Contains(msg, c.msg) {
			t.Errorf("%s: got %v %q, want %v containing %q", c.name, got, msg, c.want, c.msg)
		}
	}

	quote := makeClaim("q", "tpm2/quote", "e1", "s1", nil)
	if got, _, _ := UEFILogReplay(quote, "", structures.ExpectedValue{}, structures.Session{}, map[string]interface{}{"uefilogcid": "log"}); got != structures.Fail {
		t.Error("ran on a quote claim")
	}
}

func quoteClaim(session, sel, digest string) structures.Claim {
	q := makeClaim("quote", "tpm2/quote", "e1", session, map[string]interface{}{
		"quote": map[string]interface{}{"attested": map[string]interface{}{"pcrdigest": digest}},
	})
	q.Header.CallParameters = map[string]interface{}{"bank": "sha256", "pcrSelection": sel}
	return q
}

func TestPCRDigest(t *testing.T) {
	pcrs := makeClaim("pcrs", "tpm2/pcrs", "e1", "s1", map[string]interface{}{"sha256": copyBank(cos85SHA256)})
	pcrs.Header.Timing.Requested = 0
	fakeClaims(t, pcrs)

	// ascending index order, whatever order the selection is written in
	h := sha256.New()
	for _, i := range []string{"0", "4", "5"} {
		d, _ := hex.DecodeString(cos85SHA256[i].(string))
		h.Write(d)
	}
	digest := base64.StdEncoding.EncodeToString(h.Sum(nil))
	param := map[string]interface{}{"pcrscid": "pcrs"}

	cases := []struct {
		name  string
		quote structures.Claim
		param map[string]interface{}
		want  structures.ResultValue
		msg   string
	}{
		{"matches", quoteClaim("s1", "4,5,0", digest), param, structures.Success, "PCRs [0 4 5] of bank sha256"},
		{"duplicate PCR", quoteClaim("s1", "0, 4,5,5", digest), param, structures.Success, "give the quote's attested value."},
		{"other selection", quoteClaim("s1", "0,4", digest), param, structures.Fail, "typically PCR 10"},
		{"other session", quoteClaim("s2", "0,4,5", digest), param, structures.Success, "different sessions"},
		{"bad selection", quoteClaim("s1", "0,x", digest), param, structures.Fail, "not a PCR index"},
		{"PCR missing", quoteClaim("s1", "0,17", digest), param, structures.Fail, "PCR 17 of bank sha256 is not in the PCRs claim"},
		{"no parameter", quoteClaim("s1", "0,4,5", digest), map[string]interface{}{}, structures.Fail, "pcrscid"},
	}
	for _, c := range cases {
		got, msg, _ := PCRDigest(c.quote, "tpm2_pcrDigest", structures.ExpectedValue{}, structures.Session{}, c.param)
		if got != c.want || !strings.Contains(msg, c.msg) {
			t.Errorf("%s: got %v %q, want %v containing %q", c.name, got, msg, c.want, c.msg)
		}
	}
}

func TestLinkedClaimErrorClaim(t *testing.T) {
	e := makeClaim("err", "tpm2/pcrs", "e1", "s1", nil)
	e.BodyType = structures.CLAIMERROR
	fakeClaims(t, e)
	if _, msg := linkedClaim(makeClaim("x", "tpm2/quote", "e1", "s1", nil), map[string]interface{}{"pcrscid": "err"}, "pcrscid", "tpm2/pcrs"); !strings.Contains(msg, "error claim") {
		t.Errorf("got %q", msg)
	}
}
