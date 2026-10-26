package tpm2

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"
)

// testEKCert makes a certificate shaped like an EK certificate: empty
// subject, critical SAN with the TPM attributes as a directoryName, and an
// AIA CA Issuers URL.
func testEKCert(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := asn1.Marshal(pkix.RDNSequence{
		{{Type: oidTPMManufacturer, Value: "id:49465800"}},
		{{Type: oidTPMModel, Value: "SLB9670"}},
		{{Type: oidTPMVersion, Value: "id:0007003D"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	san, err := asn1.Marshal([]asn1.RawValue{{Class: asn1.ClassContextSpecific, Tag: 4, IsCompound: true, Bytes: dir}})
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(0x1234),
		Issuer:                pkix.Name{CommonName: "Test EK CA"},
		NotBefore:             time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		IssuingCertificateURL: []string{"http://pki.example.org/ekca.crt"},
		ExtraExtensions:       []pkix.Extension{{Id: oidSAN, Critical: true, Value: san}},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func TestEKCertSummary(t *testing.T) {
	der := testEKCert(t)

	// NV indices are padded: trimDER drops what follows the certificate
	padded := append(append([]byte{}, der...), make([]byte, 100)...)
	trimmed, err := trimDER(padded)
	if err != nil || len(trimmed) != len(der) {
		t.Fatalf("trimDER: %d bytes, %v", len(trimmed), err)
	}
	if _, err := trimDER([]byte{0xff, 0xff, 0xff}); err == nil {
		t.Error("trimDER accepted rubbish")
	}

	s := summariseEKCert(trimmed)
	if s.ParseError != "" {
		t.Fatalf("parse error %v", s.ParseError)
	}
	if s.KeyType != "RSA" || s.KeyBits != 2048 || s.Serial != "1234" {
		t.Errorf("key/serial: %+v", s)
	}
	if s.Manufacturer != "id:49465800" || s.Model != "SLB9670" || s.Version != "id:0007003D" {
		t.Errorf("TPM attributes: %q %q %q", s.Manufacturer, s.Model, s.Version)
	}
	if len(s.IssuerURLs) != 1 || s.IssuerURLs[0] != "http://pki.example.org/ekca.crt" {
		t.Errorf("AIA: %v", s.IssuerURLs)
	}
	if s.NotAfter != "2040-01-01T00:00:00Z" || len(s.SHA256) != 64 {
		t.Errorf("validity/fingerprint: %+v", s)
	}

	// a certificate x509 refuses still gets its fingerprint
	bad := summariseEKCert([]byte{0x30, 0x03, 0x02, 0x01, 0x01})
	if bad.ParseError == "" || len(bad.SHA256) != 64 {
		t.Errorf("unparseable: %+v", bad)
	}
}
