package tpm2rules

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"a10/structures"

	"github.com/google/go-tpm/tpm2"
)

// tpmRSAPublic is a TPM2B_PUBLIC (base64) for an RSA key, as jp stores the EK.
func tpmRSAPublic(n []byte, exponent uint32) string {
	pub := tpm2.TPMTPublic{
		Type:    tpm2.TPMAlgRSA,
		NameAlg: tpm2.TPMAlgSHA256,
		Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
			Scheme:    tpm2.TPMTRSAScheme{Scheme: tpm2.TPMAlgNull},
			KeyBits:   2048,
			Exponent:  exponent,
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: n}),
	}
	return base64.StdEncoding.EncodeToString(tpm2.Marshal(tpm2.New2B(pub)))
}

func getcertClaim(der []byte, ekPublic string) structures.Claim {
	var c structures.Claim
	c.Header.Intent.Function = "tpm2/getcert"
	c.Header.Element.TPM2.EK = structures.TPMKey{Handle: "0x810100EE", Public: ekPublic}
	c.Header.Timing.Received = structures.Timestamp(testNow.UnixNano())
	c.Body = map[string]interface{}{"certificate": base64.StdEncoding.EncodeToString(der)}
	return c
}

func TestEKCertRules(t *testing.T) {
	root := makeCert(t, "Test Root", 1, testKey(t), nil, true, t2040)
	inter := makeCert(t, "Test EK CA", 2, testKey(t), &root, true, t2040)
	ekKey := testKey(t)
	ekc := makeCert(t, "", 0x1234, ekKey, &inter, false, t2040)
	ekPub := tpmRSAPublic(ekKey.N.Bytes(), 0)

	var noEV structures.ExpectedValue
	var s structures.Session

	// tpm2_ekCertMatchesEK
	claim := getcertClaim(ekc.cert.Raw, ekPub)
	if r, msg, err := EKCertMatchesEK(claim, "", noEV, s, nil); r != structures.Success || err != nil {
		t.Errorf("matching EK: %v %q %v", r, msg, err)
	}
	other := getcertClaim(ekc.cert.Raw, tpmRSAPublic(testKey(t).N.Bytes(), 0))
	if r, msg, _ := EKCertMatchesEK(other, "", noEV, s, nil); r != structures.Fail || !strings.Contains(msg, "modulus") {
		t.Errorf("other EK: %v %q", r, msg)
	}
	noek := getcertClaim(ekc.cert.Raw, "")
	if r, msg, _ := EKCertMatchesEK(noek, "", noEV, s, nil); r != structures.Fail || !strings.Contains(msg, "tpm2.ek.public") {
		t.Errorf("no EK: %v %q", r, msg)
	}
	quote := getcertClaim(ekc.cert.Raw, ekPub)
	quote.Header.Intent.Function = "tpm2/quote"
	if r, msg, _ := EKCertMatchesEK(quote, "", noEV, s, nil); r != structures.Fail || !strings.Contains(msg, "tpm2/getcert") {
		t.Errorf("quote claim: %v %q", r, msg)
	}
	lenient := getcertClaim(negativeSerial(t, ekc.cert, inter), ekPub)
	if r, msg, err := EKCertMatchesEK(lenient, "", noEV, s, nil); r != structures.Success || err != nil {
		t.Errorf("lenient: %v %q %v", r, msg, err)
	}

	// tpm2_ekCertIssuer, with a file URL
	dir := t.TempDir()
	write := func(name string, cs ...*x509.Certificate) string {
		var b []byte
		for _, c := range cs {
			b = append(b, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return "file://" + p
	}
	ev := func(url string) structures.ExpectedValue {
		return structures.ExpectedValue{ItemID: "ev1", EVS: map[string]interface{}{"issuerurl": url}}
	}

	r, msg, err := EKCertIssuer(claim, "", ev(write("chain.pem", inter.cert, root.cert)), s, nil)
	if r != structures.Success || err != nil || !strings.Contains(msg, `"CN=Test EK CA" <- "CN=Test Root"`) || !strings.Contains(msg, certFingerprint(inter.cert)) {
		t.Errorf("chain: %v %q %v", r, msg, err)
	}
	if r, msg, _ := EKCertIssuer(claim, "", ev(write("root.pem", root.cert)), s, nil); r != structures.Fail || !strings.Contains(msg, "none of the") {
		t.Errorf("root only: %v %q", r, msg)
	}
	if r, msg, _ := EKCertIssuer(claim, "", noEV, s, nil); r != structures.Fail || !strings.Contains(msg, "issuerurl") {
		t.Errorf("no issuerurl: %v %q", r, msg)
	}
	// fetch failure is an error (VerifyCallFailure), not a fail
	if _, _, err := EKCertIssuer(claim, "", ev("file://"+filepath.Join(dir, "missing.pem")), s, nil); err == nil {
		t.Error("missing file gave no error")
	}
	if r, msg, err := EKCertIssuer(lenient, "", ev("file://"+filepath.Join(dir, "chain.pem")), s, nil); r != structures.Success || err != nil || !strings.Contains(msg, "read leniently") {
		t.Errorf("lenient: %v %q %v", r, msg, err)
	}
}
