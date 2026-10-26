package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestEKCertIssuerURL(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cert := func(urls ...string) json.RawMessage {
		tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IssuingCertificateURL: urls}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(map[string]any{"certificate": base64.StdEncoding.EncodeToString(der)})
		return b
	}

	if u, why := ekCertIssuerURL(cert("ldap://x/y", "http://pki.example.org/ca.crt", "https://b")); u != "http://pki.example.org/ca.crt" {
		t.Errorf("got %q %q", u, why)
	}
	cases := map[string]json.RawMessage{
		"no CA Issuers URL":    cert(),
		"no EK certificate (":  json.RawMessage(`{"error":"No NV index at 0x01C00002"}`),
		"no EK certificate in": json.RawMessage(`{}`),
		"not base64":           json.RawMessage(`{"certificate":"!!"}`),
		"could not be parsed":  json.RawMessage(`{"certificate":"MAMCAQE="}`),
		"not JSON":             json.RawMessage(`x`),
	}
	for want, body := range cases {
		if u, why := ekCertIssuerURL(body); u != "" || !strings.Contains(why, want) {
			t.Errorf("%s: got %q %q", want, u, why)
		}
	}
}
