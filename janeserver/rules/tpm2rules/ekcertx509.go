package tpm2rules

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"
)

// X.509 helpers for the EK certificate rules (#63). Standard library only.

// ekCert is an EK certificate taken apart. EK certificates often bend the
// X.509 rules (negative serials, odd extensions) and Go's parser then refuses
// them, so the parts the rules need are also read directly from the DER:
// Cert is nil in that case and ParseError says why.
type ekCert struct {
	Cert       *x509.Certificate
	ParseError error

	TBS                 []byte
	SigAlg              x509.SignatureAlgorithm
	Signature           []byte
	RawIssuer           []byte
	PublicKey           interface{}
	NotBefore, NotAfter time.Time
	SHA256              string
}

type rawCertificate struct {
	TBS       asn1.RawValue
	SigAlg    pkix.AlgorithmIdentifier
	Signature asn1.BitString
}

type rawTBS struct {
	Version  int `asn1:"optional,explicit,default:0,tag:0"`
	Serial   asn1.RawValue
	SigAlg   pkix.AlgorithmIdentifier
	Issuer   asn1.RawValue
	Validity rawValidity
	Subject  asn1.RawValue
	SPKI     asn1.RawValue
}

type rawValidity struct {
	NotBefore, NotAfter time.Time
}

// signature algorithms by OID, for certificates Go's parser refused
var sigAlgOIDs = map[string]x509.SignatureAlgorithm{
	"1.2.840.113549.1.1.5":  x509.SHA1WithRSA,
	"1.2.840.113549.1.1.11": x509.SHA256WithRSA,
	"1.2.840.113549.1.1.12": x509.SHA384WithRSA,
	"1.2.840.113549.1.1.13": x509.SHA512WithRSA,
	"1.2.840.10045.4.1":     x509.ECDSAWithSHA1,
	"1.2.840.10045.4.3.2":   x509.ECDSAWithSHA256,
	"1.2.840.10045.4.3.3":   x509.ECDSAWithSHA384,
	"1.2.840.10045.4.3.4":   x509.ECDSAWithSHA512,
}

// parseEKCert takes a DER certificate apart, with Go's parser if it can and
// directly from the DER otherwise.
func parseEKCert(der []byte) (*ekCert, error) {
	sum := sha256.Sum256(der)
	e := &ekCert{SHA256: fmt.Sprintf("%x", sum)}
	if c, err := x509.ParseCertificate(der); err == nil {
		e.Cert = c
		e.TBS, e.SigAlg, e.Signature = c.RawTBSCertificate, c.SignatureAlgorithm, c.Signature
		e.RawIssuer, e.PublicKey = c.RawIssuer, c.PublicKey
		e.NotBefore, e.NotAfter = c.NotBefore, c.NotAfter
		return e, nil
	} else {
		e.ParseError = err
	}

	var rc rawCertificate
	if _, err := asn1.Unmarshal(der, &rc); err != nil {
		return nil, fmt.Errorf("not a certificate: %v (x509: %v)", err, e.ParseError)
	}
	var tbs rawTBS
	if _, err := asn1.Unmarshal(rc.TBS.FullBytes, &tbs); err != nil {
		return nil, fmt.Errorf("certificate body unreadable: %v (x509: %v)", err, e.ParseError)
	}
	pub, err := x509.ParsePKIXPublicKey(tbs.SPKI.FullBytes)
	if err != nil {
		return nil, fmt.Errorf("public key unreadable: %v (x509: %v)", err, e.ParseError)
	}
	e.TBS, e.Signature = rc.TBS.FullBytes, rc.Signature.RightAlign()
	e.SigAlg = sigAlgOIDs[rc.SigAlg.Algorithm.String()] // UnknownSignatureAlgorithm if not listed
	e.RawIssuer, e.PublicKey = tbs.Issuer.FullBytes, pub
	e.NotBefore, e.NotAfter = tbs.Validity.NotBefore, tbs.Validity.NotAfter
	return e, nil
}

// rsaKeyMatches compares an RSA public key with a TPM's modulus and exponent
// (exponent 0 in a TPMT_PUBLIC means 65537).
func rsaKeyMatches(pub interface{}, modulus []byte, exponent uint32) error {
	k, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("the certificate's key is not RSA (%T)", pub)
	}
	if exponent == 0 {
		exponent = 65537
	}
	if k.N.Cmp(new(big.Int).SetBytes(modulus)) != 0 {
		return fmt.Errorf("the certificate's RSA modulus differs from the EK's")
	}
	if k.E != int(exponent) {
		return fmt.Errorf("the certificate's RSA exponent %d differs from the EK's %d", k.E, exponent)
	}
	return nil
}

const (
	issuerFetchTimeout = 10 * time.Second
	issuerFetchLimit   = 1 << 20 // 1 MiB
)

// fetchURL reads an https, http or file URL, at most issuerFetchLimit bytes.
func fetchURL(raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("bad URL %q: %v", raw, err)
	}
	var r io.Reader
	switch u.Scheme {
	case "https", "http":
		client := &http.Client{Timeout: issuerFetchTimeout}
		rsp, err := client.Get(raw)
		if err != nil {
			return nil, err
		}
		defer rsp.Body.Close()
		if rsp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", raw, rsp.Status)
		}
		r = rsp.Body
	case "file":
		f, err := os.Open(u.Path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	default:
		return nil, fmt.Errorf("URL scheme %q not supported (https, http, file)", u.Scheme)
	}
	b, err := io.ReadAll(io.LimitReader(r, issuerFetchLimit+1))
	if err != nil {
		return nil, err
	}
	if len(b) > issuerFetchLimit {
		return nil, fmt.Errorf("%s is larger than %d bytes", raw, issuerFetchLimit)
	}
	return b, nil
}

// parseCertificates reads one or more certificates, PEM or DER.
func parseCertificates(b []byte) ([]*x509.Certificate, error) {
	if !bytes.Contains(b, []byte("-----BEGIN")) {
		cs, err := x509.ParseCertificates(b)
		if err != nil {
			return nil, fmt.Errorf("not a DER or PEM certificate (PKCS#7 is not supported): %v", err)
		}
		return cs, nil
	}
	var cs []*x509.Certificate
	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)
		if blk == nil {
			break
		}
		if blk.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(blk.Bytes)
		if err != nil {
			return nil, fmt.Errorf("PEM certificate %d: %v", len(cs)+1, err)
		}
		cs = append(cs, c)
	}
	if len(cs) == 0 {
		return nil, fmt.Errorf("no certificates in the PEM data")
	}
	return cs, nil
}

func selfSigned(c *x509.Certificate) bool {
	return bytes.Equal(c.RawIssuer, c.RawSubject) && c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) == nil
}

func certFingerprint(c *x509.Certificate) string {
	return fmt.Sprintf("%x", sha256.Sum256(c.Raw))
}

func validAt(what string, notBefore, notAfter, at time.Time) error {
	if at.Before(notBefore) || at.After(notAfter) {
		return fmt.Errorf("%s is not valid at %s (valid %s to %s)", what, at.UTC().Format(time.RFC3339),
			notBefore.UTC().Format(time.RFC3339), notAfter.UTC().Format(time.RFC3339))
	}
	return nil
}

// maxChain bounds the walk up the chain.
const maxChain = 8

// verifyEKChain checks the EK certificate was issued by the fetched
// certificates (#63), all valid at the given time. Self-signed fetched
// certificates are the roots and the chain must end at one; if none is
// self-signed, the fetched certificates themselves are trusted, and the
// chain ends at the last one found. It returns the chain above the EK
// certificate, its issuer first.
func verifyEKChain(ek *ekCert, fetched []*x509.Certificate, at time.Time) ([]*x509.Certificate, error) {
	if err := validAt("The EK certificate", ek.NotBefore, ek.NotAfter, at); err != nil {
		return nil, err
	}
	haveRoot := false
	for _, c := range fetched {
		if selfSigned(c) {
			haveRoot = true
		}
	}

	var chain []*x509.Certificate
	rawIssuer, tbs, alg, sig := ek.RawIssuer, ek.TBS, ek.SigAlg, ek.Signature
	for len(chain) < maxChain {
		var issuer *x509.Certificate
		for _, c := range fetched {
			if bytes.Equal(c.RawSubject, rawIssuer) && c.CheckSignature(alg, tbs, sig) == nil {
				issuer = c
				break
			}
		}
		if issuer == nil {
			break
		}
		if err := validAt(fmt.Sprintf("Issuer certificate %q", issuer.Subject.String()), issuer.NotBefore, issuer.NotAfter, at); err != nil {
			return nil, err
		}
		chain = append(chain, issuer)
		if selfSigned(issuer) {
			return chain, nil
		}
		rawIssuer, tbs, alg, sig = issuer.RawIssuer, issuer.RawTBSCertificate, issuer.SignatureAlgorithm, issuer.Signature
	}

	switch {
	case len(chain) == 0:
		return nil, fmt.Errorf("none of the %d fetched certificates issued the EK certificate (subject or signature does not match)", len(fetched))
	case len(chain) == maxChain:
		return nil, fmt.Errorf("chain longer than %d certificates", maxChain)
	case haveRoot:
		top := chain[len(chain)-1]
		return nil, fmt.Errorf("the chain ends at %q, which was not issued by any root among the fetched certificates", top.Subject.String())
	}
	return chain, nil
}
