package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// ekCertIssuerURL returns the first http(s) CA Issuers URL (authority
// information access) of the EK certificate in a tpm2/getcert claim body, or
// "" and the reason there is none (#63).
func ekCertIssuerURL(body json.RawMessage) (string, string) {
	var b struct {
		Certificate string `json:"certificate"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return "", "claim body is not JSON"
	}
	if b.Certificate == "" {
		if b.Error != "" {
			return "", "no EK certificate (" + b.Error + ")"
		}
		return "", "no EK certificate in the claim"
	}
	der, err := base64.StdEncoding.DecodeString(b.Certificate)
	if err != nil {
		return "", "EK certificate is not base64"
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return "", "EK certificate could not be parsed (" + err.Error() + "); give issuerurl in the evs entry"
	}
	for _, u := range c.IssuingCertificateURL {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u, ""
		}
	}
	return "", "EK certificate has no CA Issuers URL; give issuerurl in the evs entry"
}
