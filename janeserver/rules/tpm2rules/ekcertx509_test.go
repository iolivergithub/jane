package tpm2rules

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testCA struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var (
	t2020 = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	t2040 = time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	testNow   = time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
)

// makeCert issues a certificate for key; parent nil means self-signed.
func makeCert(t *testing.T, cn string, serial int64, key *rsa.PrivateKey, parent *testCA, isCA bool, notAfter time.Time) testCA {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},
		NotBefore: t2020, NotAfter: notAfter,
		BasicConstraintsValid: isCA, IsCA: isCA,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign
	}
	pc, pk := tmpl, key
	if parent != nil {
		pc, pk = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, pc, &key.PublicKey, pk)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return testCA{c, key}
}

// negativeSerial re-signs a certificate with its serial made negative, which
// Go's parser refuses (as for some real EK certificates).
func negativeSerial(t *testing.T, c *x509.Certificate, issuer testCA) []byte {
	t.Helper()
	tbs := append([]byte{}, c.RawTBSCertificate...)
	ser := c.SerialNumber.Bytes()
	i := bytes.Index(tbs, append([]byte{0x02, byte(len(ser))}, ser...))
	if i < 0 || ser[0]&0x80 != 0 {
		t.Fatal("serial not found")
	}
	tbs[i+2] |= 0x80
	h := sha256.Sum256(tbs)
	sig, err := rsa.SignPKCS1v15(rand.Reader, issuer.key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	der, err := asn1.Marshal(rawCertificate{
		TBS:       asn1.RawValue{FullBytes: tbs},
		SigAlg:    pkix.AlgorithmIdentifier{Algorithm: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}, Parameters: asn1.NullRawValue},
		Signature: asn1.BitString{Bytes: sig, BitLength: len(sig) * 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func TestVerifyEKChain(t *testing.T) {
	root := makeCert(t, "Test Root", 1, testKey(t), nil, true, t2040)
	inter := makeCert(t, "Test EK CA", 2, testKey(t), &root, true, t2040)
	otherRoot := makeCert(t, "Other Root", 3, testKey(t), nil, true, t2040)
	ekKey := testKey(t)
	ekc := makeCert(t, "", 0x1234, ekKey, &inter, false, t2040)

	ek, err := parseEKCert(ekc.cert.Raw)
	if err != nil || ek.ParseError != nil || ek.Cert == nil {
		t.Fatalf("parse: %v %v", err, ek.ParseError)
	}

	cases := []struct {
		name    string
		fetched []*x509.Certificate
		at      time.Time
		chain   int
		err     string
	}{
		{"root and intermediate", []*x509.Certificate{root.cert, inter.cert}, testNow, 2, ""},
		{"bundle in any order", []*x509.Certificate{inter.cert, otherRoot.cert, root.cert}, testNow, 2, ""},
		{"intermediate only, trusted", []*x509.Certificate{inter.cert}, testNow, 1, ""},
		{"root only", []*x509.Certificate{root.cert}, testNow, 0, "none of the 1 fetched"},
		{"wrong root", []*x509.Certificate{inter.cert, otherRoot.cert}, testNow, 0, "not issued by any root"},
		{"expired", []*x509.Certificate{root.cert, inter.cert}, t2040.Add(time.Hour), 0, "not valid at"},
	}
	for _, c := range cases {
		chain, err := verifyEKChain(ek, c.fetched, c.at)
		if c.err == "" {
			if err != nil || len(chain) != c.chain {
				t.Errorf("%s: chain %d, err %v", c.name, len(chain), err)
			}
		} else if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.err)
		}
	}

	// expired intermediate
	oldInter := makeCert(t, "Test EK CA", 2, inter.key, &root, true, t2020.Add(time.Hour))
	if _, err := verifyEKChain(ek, []*x509.Certificate{root.cert, oldInter.cert}, testNow); err == nil || !strings.Contains(err.Error(), "Issuer certificate") {
		t.Errorf("expired intermediate: %v", err)
	}

	// an EK certificate Go refuses is checked from its raw parts
	bad := negativeSerial(t, ekc.cert, inter)
	lek, err := parseEKCert(bad)
	if err != nil {
		t.Fatalf("lenient parse: %v", err)
	}
	if lek.ParseError == nil || lek.Cert != nil {
		t.Skip("this Go accepts negative serials; lenient path not exercised")
	}
	if chain, err := verifyEKChain(lek, []*x509.Certificate{root.cert, inter.cert}, testNow); err != nil || len(chain) != 2 {
		t.Errorf("lenient chain: %d %v", len(chain), err)
	}
	if err := rsaKeyMatches(lek.PublicKey, ekKey.N.Bytes(), 0); err != nil {
		t.Errorf("lenient key: %v", err)
	}
	if !lek.NotAfter.Equal(t2040) {
		t.Errorf("lenient validity %v", lek.NotAfter)
	}
	// a tampered signature fails
	bad[len(bad)-1] ^= 1
	lek, _ = parseEKCert(bad)
	if _, err := verifyEKChain(lek, []*x509.Certificate{root.cert, inter.cert}, testNow); err == nil {
		t.Error("tampered signature accepted")
	}
}

func TestRSAKeyMatches(t *testing.T) {
	k := testKey(t)
	if err := rsaKeyMatches(&k.PublicKey, k.N.Bytes(), 0); err != nil {
		t.Errorf("exponent 0 (65537): %v", err)
	}
	if err := rsaKeyMatches(&k.PublicKey, k.N.Bytes(), 65537); err != nil {
		t.Errorf("exponent 65537: %v", err)
	}
	if err := rsaKeyMatches(&k.PublicKey, k.N.Bytes(), 3); err == nil || !strings.Contains(err.Error(), "exponent") {
		t.Errorf("exponent 3: %v", err)
	}
	if err := rsaKeyMatches(&k.PublicKey, testKey(t).N.Bytes(), 0); err == nil || !strings.Contains(err.Error(), "modulus") {
		t.Errorf("other key: %v", err)
	}
	if err := rsaKeyMatches("ecc", k.N.Bytes(), 0); err == nil {
		t.Error("non-RSA key accepted")
	}
}

func TestParseCertificatesAndFetch(t *testing.T) {
	root := makeCert(t, "Test Root", 1, testKey(t), nil, true, t2040)
	inter := makeCert(t, "Test EK CA", 2, testKey(t), &root, true, t2040)
	bundle := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: inter.cert.Raw}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.cert.Raw})...)

	if cs, err := parseCertificates(bundle); err != nil || len(cs) != 2 {
		t.Errorf("PEM bundle: %d %v", len(cs), err)
	}
	if cs, err := parseCertificates(inter.cert.Raw); err != nil || len(cs) != 1 {
		t.Errorf("DER: %d %v", len(cs), err)
	}
	if _, err := parseCertificates([]byte("<html>not found</html>")); err == nil {
		t.Error("HTML accepted")
	}
	if _, err := parseCertificates([]byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n")); err == nil {
		t.Error("PEM without certificates accepted")
	}

	// file
	dir := t.TempDir()
	p := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(p, bundle, 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := fetchURL("file://" + p); err != nil || !bytes.Equal(b, bundle) {
		t.Errorf("file: %v", err)
	}
	if _, err := fetchURL("file://" + filepath.Join(dir, "missing")); err == nil {
		t.Error("missing file accepted")
	}

	// http
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ca.crt":
			w.Write(inter.cert.Raw)
		case "/big":
			w.Write(make([]byte, issuerFetchLimit+10))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if b, err := fetchURL(srv.URL + "/ca.crt"); err != nil || !bytes.Equal(b, inter.cert.Raw) {
		t.Errorf("http: %v", err)
	}
	if _, err := fetchURL(srv.URL + "/nothing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
	if _, err := fetchURL(srv.URL + "/big"); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("limit: %v", err)
	}
	if _, err := fetchURL("mongodb://localhost/certs"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("mongodb: %v", err)
	}
}
