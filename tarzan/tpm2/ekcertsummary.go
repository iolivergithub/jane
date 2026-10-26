package tpm2

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"time"
)

// ekCertSummary is a readable summary of an EK certificate (#63). It is for
// people looking at the claim; Jane's rules parse the DER themselves.
type ekCertSummary struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Serial       string   `json:"serial"`
	NotBefore    string   `json:"notbefore"`
	NotAfter     string   `json:"notafter"`
	KeyType      string   `json:"keytype"`
	KeyBits      int      `json:"keybits,omitempty"`
	Manufacturer string   `json:"tpmmanufacturer,omitempty"` // SAN 2.23.133.2.1
	Model        string   `json:"tpmmodel,omitempty"`        // SAN 2.23.133.2.2
	Version      string   `json:"tpmversion,omitempty"`      // SAN 2.23.133.2.3
	IssuerURLs   []string `json:"issuingcertificateurl,omitempty"`
	OCSP         []string `json:"ocsp,omitempty"`
	SHA256       string   `json:"sha256"`
	ParseError   string   `json:"parseerror,omitempty"`
}

// TCG EK credential profile: TPM attributes in the subject alternative name.
var (
	oidSAN             = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidTPMManufacturer = asn1.ObjectIdentifier{2, 23, 133, 2, 1}
	oidTPMModel        = asn1.ObjectIdentifier{2, 23, 133, 2, 2}
	oidTPMVersion      = asn1.ObjectIdentifier{2, 23, 133, 2, 3}
)

// trimDER returns the certificate without anything after its outer SEQUENCE:
// NV indices are often larger than the certificate and padded.
func trimDER(data []byte) ([]byte, error) {
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("NV data is not a DER certificate: %v", err)
	}
	return raw.FullBytes, nil
}

// summariseEKCert fills a summary. A certificate Go's x509 refuses (EK
// certificates often bend the rules) still gets its fingerprint and the
// parse error.
func summariseEKCert(der []byte) ekCertSummary {
	s := ekCertSummary{SHA256: fmt.Sprintf("%x", sha256.Sum256(der))}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		s.ParseError = err.Error()
		return s
	}
	s.Subject = c.Subject.String()
	s.Issuer = c.Issuer.String()
	s.Serial = fmt.Sprintf("%x", c.SerialNumber)
	s.NotBefore = c.NotBefore.UTC().Format(time.RFC3339)
	s.NotAfter = c.NotAfter.UTC().Format(time.RFC3339)
	s.IssuerURLs = c.IssuingCertificateURL
	s.OCSP = c.OCSPServer
	switch k := c.PublicKey.(type) {
	case *rsa.PublicKey:
		s.KeyType, s.KeyBits = "RSA", k.N.BitLen()
	case *ecdsa.PublicKey:
		s.KeyType, s.KeyBits = "ECC "+k.Curve.Params().Name, k.Curve.Params().BitSize
	default:
		s.KeyType = c.PublicKeyAlgorithm.String()
	}
	for _, e := range c.Extensions {
		if e.Id.Equal(oidSAN) {
			s.Manufacturer, s.Model, s.Version = tpmSANAttributes(e.Value)
		}
	}
	return s
}

// tpmSANAttributes reads the TPM manufacturer, model and version from the
// directoryName entries of a subject alternative name extension.
func tpmSANAttributes(ext []byte) (manufacturer, model, version string) {
	var names []asn1.RawValue
	if _, err := asn1.Unmarshal(ext, &names); err != nil {
		return
	}
	for _, n := range names {
		if n.Class != asn1.ClassContextSpecific || n.Tag != 4 { // directoryName [4]
			continue
		}
		var rdns pkix.RDNSequence
		if _, err := asn1.Unmarshal(n.Bytes, &rdns); err != nil {
			continue
		}
		for _, rdn := range rdns {
			for _, atv := range rdn {
				v := fmt.Sprint(atv.Value)
				switch {
				case atv.Type.Equal(oidTPMManufacturer):
					manufacturer = v
				case atv.Type.Equal(oidTPMModel):
					model = v
				case atv.Type.Equal(oidTPMVersion):
					version = v
				}
			}
		}
	}
	return
}
