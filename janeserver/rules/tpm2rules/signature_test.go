package tpm2rules

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"a10/structures"

	"github.com/google/go-tpm/tpm2"
)

// testAK is a software RSA key standing in for the TPM's AK.
type testAK struct {
	key    *rsa.PrivateKey
	public string // base64 TPM2B_PUBLIC, as jp stores it in the element
}

func newTestAK(t *testing.T) testAK {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := tpm2.TPMTPublic{
		Type:             tpm2.TPMAlgRSA,
		NameAlg:          tpm2.TPMAlgSHA256,
		ObjectAttributes: tpm2.TPMAObject{FixedTPM: true, FixedParent: true, SensitiveDataOrigin: true, UserWithAuth: true, Restricted: true, SignEncrypt: true},
		Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
			Scheme:    tpm2.TPMTRSAScheme{Scheme: tpm2.TPMAlgNull},
			KeyBits:   2048,
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: k.N.Bytes()}),
	}
	return testAK{k, base64.StdEncoding.EncodeToString(tpm2.Marshal(tpm2.New2B(pub)))}
}

type clockIn struct {
	clock           uint64
	reset, restart  uint32
	safe            bool
	firmwareVersion uint64
}

// attestBytes builds a TPMS_ATTEST of the given type, as a TPM would.
func attestBytes(typ tpm2.TPMISTAttest, nonce []byte, c clockIn) []byte {
	ci := tpm2.TPMSClockInfo{Clock: c.clock, ResetCount: c.reset, RestartCount: c.restart, Safe: tpm2.TPMIYesNo(c.safe)}
	a := tpm2.TPMSAttest{
		Magic:           tpm2.TPMGeneratedValue,
		Type:            typ,
		QualifiedSigner: tpm2.TPM2BName{Buffer: []byte{0, 0x0b, 1, 2, 3}},
		ExtraData:       tpm2.TPM2BData{Buffer: nonce},
		ClockInfo:       ci,
		FirmwareVersion: c.firmwareVersion,
	}
	if typ == tpm2.TPMSTAttestTime {
		a.Attested = tpm2.NewTPMUAttest(tpm2.TPMSTAttestTime, &tpm2.TPMSTimeAttestInfo{
			Time:            tpm2.TPMSTimeInfo{Time: c.clock * 1000, ClockInfo: ci},
			FirmwareVersion: c.firmwareVersion,
		})
	} else {
		a.Attested = tpm2.NewTPMUAttest(tpm2.TPMSTAttestQuote, &tpm2.TPMSQuoteInfo{
			PCRSelect: tpm2.TPMLPCRSelection{PCRSelections: []tpm2.TPMSPCRSelection{{Hash: tpm2.TPMAlgSHA256, PCRSelect: []byte{1, 0, 0}}}},
			PCRDigest: tpm2.TPM2BDigest{Buffer: make([]byte, 32)},
		})
	}
	return tpm2.Marshal(a)
}

// sign makes the RSASSA-SHA256 TPMT_SIGNATURE over attest.
func (k testAK) sign(t *testing.T, attest []byte) []byte {
	t.Helper()
	h := sha256.Sum256(attest)
	sig, err := rsa.SignPKCS1v15(rand.Reader, k.key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	return tpm2.Marshal(tpm2.TPMTSignature{
		SigAlg:    tpm2.TPMAlgRSASSA,
		Signature: tpm2.NewTPMUSignature(tpm2.TPMAlgRSASSA, &tpm2.TPMSSignatureRSA{Hash: tpm2.TPMAlgSHA256, Sig: tpm2.TPM2BPublicKeyRSA{Buffer: sig}}),
	})
}

// signedClaim is a quote or gettime claim for element E1 carrying a signed attest.
func signedClaim(t *testing.T, ak testAK, id, function string, requested int64, c clockIn) structures.Claim {
	t.Helper()
	typ := tpm2.TPMSTAttestQuote
	if function == "tpm2/gettime" {
		typ = tpm2.TPMSTAttestTime
	}
	att := attestBytes(typ, []byte("nonce"), c)
	cl := makeClaim(id, function, "E1", "S1", map[string]interface{}{
		"attest":       base64.StdEncoding.EncodeToString(att),
		"rawsignature": base64.StdEncoding.EncodeToString(ak.sign(t, att)),
	})
	cl.Header.Element.TPM2.AK.Public = ak.public
	cl.Header.Element.TPM2.AK.Handle = "0x810100AA"
	cl.Header.CallParameters = map[string]interface{}{}
	cl.Header.Timing.Requested = structures.Timestamp(requested)
	return cl
}

func TestValidSignature(t *testing.T) {
	ak, other := newTestAK(t), newTestAK(t)
	c := clockIn{clock: 1000, reset: 3, restart: 1, safe: true, firmwareVersion: 0x7003d00000000}

	run := func(cl structures.Claim) (structures.ResultValue, string) {
		r, msg, err := ValidSignature(cl, "tpm2_validSignature", structures.ExpectedValue{}, structures.Session{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r, msg
	}

	for _, f := range []string{"tpm2/quote", "tpm2/gettime"} {
		if r, msg := run(signedClaim(t, ak, "C1", f, 1, c)); r != structures.Success {
			t.Errorf("%s: good signature: %v %s", f, r, msg)
		}
	}

	bad := map[string]func(*structures.Claim){
		"tampered attest": func(cl *structures.Claim) {
			b, _ := base64.StdEncoding.DecodeString(cl.Body["attest"].(string))
			b[len(b)-1] ^= 1
			cl.Body["attest"] = base64.StdEncoding.EncodeToString(b)
		},
		"tampered signature": func(cl *structures.Claim) {
			b, _ := base64.StdEncoding.DecodeString(cl.Body["rawsignature"].(string))
			b[len(b)-1] ^= 1
			cl.Body["rawsignature"] = base64.StdEncoding.EncodeToString(b)
		},
		"other AK in the element":       func(cl *structures.Claim) { cl.Header.Element.TPM2.AK.Public = other.public },
		"other AK as tpm2/akpublic":     func(cl *structures.Claim) { cl.Header.CallParameters["tpm2/akpublic"] = other.public },
		"no attest (old tarzan)":        func(cl *structures.Claim) { delete(cl.Body, "attest") },
		"no AK public":                  func(cl *structures.Claim) { cl.Header.Element.TPM2.AK.Public = "" },
		"gettime attest on quote claim": func(cl *structures.Claim) { cl.Header.Intent.Function = "tpm2/quote" },
		"not a signed function":         func(cl *structures.Claim) { cl.Header.Intent.Function = "tpm2/pcrs" },
	}
	for name, change := range bad {
		cl := signedClaim(t, ak, "C1", "tpm2/gettime", 1, c)
		change(&cl)
		if r, msg := run(cl); r != structures.Fail {
			t.Errorf("%s: got %v %s, want fail", name, r, msg)
		}
	}

	// tpm2/akpublic overrides the element: the right key there passes even
	// when the element's is wrong.
	cl := signedClaim(t, ak, "C1", "tpm2/quote", 1, c)
	cl.Header.Element.TPM2.AK.Public = other.public
	cl.Header.CallParameters["tpm2/akpublic"] = ak.public
	if r, msg := run(cl); r != structures.Success || !strings.Contains(msg, "tpm2/akpublic") {
		t.Errorf("akpublic override: %v %s", r, msg)
	}
}

func TestCapabilitiesFirmware(t *testing.T) {
	ak := newTestAK(t)
	quote := signedClaim(t, ak, "Q1", "tpm2/quote", 1, clockIn{clock: 1, firmwareVersion: 0x7003d00000000})
	gettime := signedClaim(t, ak, "G1", "tpm2/gettime", 1, clockIn{clock: 1, firmwareVersion: 0x7003d00000000})
	oldQuote := makeClaim("Q0", "tpm2/quote", "E1", "S1", map[string]interface{}{
		"quote": map[string]interface{}{"firmwareVersion": "7003d00000000", "clockinfo": map[string]interface{}{"clock": "1", "resetcount": "0", "restartcount": "0", "safe": "true"}},
	})
	otherElement := signedClaim(t, ak, "Q2", "tpm2/quote", 1, clockIn{firmwareVersion: 0x7003d00000000})
	otherElement.Header.Element.ItemID = "E2"
	pcrs := makeClaim("P1", "tpm2/pcrs", "E1", "S1", map[string]interface{}{})
	fakeClaims(t, quote, gettime, oldQuote, otherElement, pcrs)

	caps := func(fw string) structures.Claim {
		return makeClaim("K1", "tpm2/capabilities", "E1", "S1", map[string]interface{}{
			"summary": map[string]interface{}{"firmwareversion": fw},
		})
	}
	cases := []struct {
		name  string
		claim structures.Claim
		param map[string]interface{}
		want  structures.ResultValue
		in    string
	}{
		{"match, different formats", caps("0007003d00000000"), map[string]interface{}{"quotecid": "Q1"}, structures.Success, "signed attest"},
		{"gettime as reference", caps("0007003d00000000"), map[string]interface{}{"quotecid": "G1"}, structures.Success, "matches"},
		{"old quote without attest", caps("0007003d00000000"), map[string]interface{}{"quotecid": "Q0"}, structures.Success, "decoded quote"},
		{"mismatch", caps("0007003e00000000"), map[string]interface{}{"quotecid": "Q1"}, structures.Fail, "differ"},
		{"missing parameter", caps("0007003d00000000"), map[string]interface{}{}, structures.Fail, "missing"},
		{"other element", caps("0007003d00000000"), map[string]interface{}{"quotecid": "Q2"}, structures.Fail, "another element"},
		{"wrong referenced type", caps("0007003d00000000"), map[string]interface{}{"quotecid": "P1"}, structures.Fail, "tpm2/pcrs"},
		{"wrong own type", quote, map[string]interface{}{"quotecid": "Q1"}, structures.Fail, "tpm2/capabilities claims"},
	}
	for _, c := range cases {
		r, msg, err := CapabilitiesFirmware(c.claim, "tpm2_capabilitiesFirmware", structures.ExpectedValue{}, structures.Session{}, c.param)
		if err != nil || r != c.want || !strings.Contains(msg, c.in) {
			t.Errorf("%s: got %v %q %v, want %v containing %q", c.name, r, msg, err, c.want, c.in)
		}
	}
}

func TestClockProgress(t *testing.T) {
	ak := newTestAK(t)
	prev := signedClaim(t, ak, "P", "tpm2/quote", 100, clockIn{clock: 50000, reset: 5, restart: 2, safe: true})
	oldPrev := makeClaim("O", "tpm2/gettime", "E1", "S0", map[string]interface{}{
		"time": map[string]interface{}{"firmwareVersion": "1", "clockinfo": map[string]interface{}{"clock": "40000", "resetcount": "5", "restartcount": "2", "safe": "true"}},
	})
	oldPrev.Header.Timing.Requested = 50
	pcrs := makeClaim("X", "tpm2/pcrs", "E1", "S1", map[string]interface{}{})
	fakeClaims(t, prev, oldPrev, pcrs)

	param := map[string]interface{}{"previouscid": "P"}
	cases := []struct {
		name  string
		now   clockIn
		param map[string]interface{}
		want  structures.ResultValue
		in    string
	}{
		{"no reset", clockIn{clock: 60000, reset: 5, restart: 2, safe: true}, param, structures.Success, "0 reset(s)"},
		{"two reboots", clockIn{clock: 90000, reset: 7, restart: 0, safe: true}, param, structures.Success, "2 reset(s)"},
		{"resume", clockIn{clock: 70000, reset: 5, restart: 3, safe: true}, param, structures.Success, "1 restart(s)"},
		{"reset count down", clockIn{clock: 90000, reset: 1, restart: 0, safe: true}, param, structures.Fail, "reset count went from 5 to 1"},
		{"restart count down", clockIn{clock: 90000, reset: 5, restart: 0, safe: true}, param, structures.Fail, "restart count went from 2 to 0"},
		{"clock back, safe", clockIn{clock: 1000, reset: 6, restart: 0, safe: true}, param, structures.Fail, "although the TPM reports safe"},
		{"clock back, unsafe", clockIn{clock: 1000, reset: 6, restart: 0, safe: false}, param, structures.Fail, "unclean shutdown"},
		{"previous without attest", clockIn{clock: 60000, reset: 5, restart: 2, safe: true}, map[string]interface{}{"previouscid": "O"}, structures.Success, "decoded clock values"},
		{"missing parameter", clockIn{clock: 60000, reset: 5}, map[string]interface{}{}, structures.Fail, "missing"},
		{"wrong referenced type", clockIn{clock: 60000, reset: 5}, map[string]interface{}{"previouscid": "X"}, structures.Fail, "tpm2/pcrs"},
	}
	for _, c := range cases {
		now := signedClaim(t, ak, "N", "tpm2/gettime", 200, c.now)
		r, msg, err := ClockProgress(now, "tpm2_clockProgress", structures.ExpectedValue{}, structures.Session{}, c.param)
		if err != nil || r != c.want || !strings.Contains(msg, c.in) {
			t.Errorf("%s: got %v %q %v, want %v containing %q", c.name, r, msg, err, c.want, c.in)
		}
	}

	// The previous claim must be earlier.
	early := signedClaim(t, ak, "N", "tpm2/gettime", 100, clockIn{clock: 60000, reset: 5, restart: 2})
	if r, msg, _ := ClockProgress(early, "", structures.ExpectedValue{}, structures.Session{}, param); r != structures.Fail || !strings.Contains(msg, "not earlier") {
		t.Errorf("not earlier: %v %s", r, msg)
	}
}
