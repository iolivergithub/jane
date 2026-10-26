package tpm2rules

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"a10/structures"
	"a10/utilities"

	"github.com/google/go-tpm/tpm2"
)

// Rules for tpm2/getcert claims (#63). The certificate itself is unsigned:
// these rules tie it to the element's EK and to its issuer.

// ekCertFromClaim decodes the certificate in a tpm2/getcert claim.
func ekCertFromClaim(claim structures.Claim) (*ekCert, string) {
	if f := claim.Header.Intent.Function; f != "tpm2/getcert" {
		return nil, fmt.Sprintf("Rule applies to tpm2/getcert claims, not %s", f)
	}
	b64, _ := claim.Body["certificate"].(string)
	if b64 == "" {
		return nil, "Claim has no certificate"
	}
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, "Claim certificate is not base64"
	}
	ek, err := parseEKCert(der)
	if err != nil {
		return nil, fmt.Sprintf("Claim certificate: %v", err)
	}
	return ek, ""
}

// lenientNote says when Go's parser refused the certificate and its raw
// parts were used instead.
func lenientNote(ek *ekCert) string {
	if ek.ParseError == nil {
		return ""
	}
	return fmt.Sprintf(" (read leniently: %v)", ek.ParseError)
}

// EKCertMatchesEK checks the public key in the EK certificate is the EK in
// the element's tpm2 section, as recorded in the claim. No expected value and
// no network: the element's EK is trusted because it was there at
// provisioning. RSA only for now.
func EKCertMatchesEK(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	ek, msg := ekCertFromClaim(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	pub, err := utilities.ParseTPMPublic(claim.Header.Element.TPM2.EK.Public)
	if err != nil {
		return structures.Fail, fmt.Sprintf("Element has no usable tpm2.ek.public: %v", err), nil
	}
	if pub.Type != tpm2.TPMAlgRSA {
		return structures.Fail, "The element's EK is not RSA: ECC EK certificates are not supported yet", nil
	}
	n, err := pub.Unique.RSA()
	if err != nil {
		return structures.Fail, fmt.Sprintf("Element EK has no RSA modulus: %v", err), nil
	}
	parms, err := pub.Parameters.RSADetail()
	if err != nil {
		return structures.Fail, fmt.Sprintf("Element EK has no RSA parameters: %v", err), nil
	}
	if err := rsaKeyMatches(ek.PublicKey, n.Buffer, parms.Exponent); err != nil {
		return structures.Fail, fmt.Sprintf("EK certificate does not match the element's EK %v: %v%s", claim.Header.Element.TPM2.EK.Handle, err, lenientNote(ek)), nil
	}
	return structures.Success, fmt.Sprintf("EK certificate (SHA-256 %s) holds the element's EK %v (RSA %d bits)%s",
		ek.SHA256, claim.Header.Element.TPM2.EK.Handle, len(n.Buffer)*8, lenientNote(ek)), nil
}

// EKCertIssuer checks the EK certificate was issued by the certificates at
// the expected value's issuerurl (https, http or file): one certificate or a
// bundle, DER or PEM. Self-signed ones are the roots the chain must reach; a
// bundle without a root is trusted as it is. Validity is checked at the time
// the claim was received. Failing to fetch is an error, not a fail.
func EKCertIssuer(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	ek, msg := ekCertFromClaim(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	url, _ := ev.EVS["issuerurl"].(string)
	if url == "" {
		return structures.Fail, fmt.Sprintf("Expected value %s has no issuerurl", ev.ItemID), nil
	}
	b, err := fetchURL(url)
	if err != nil {
		return structures.Fail, fmt.Sprintf("Could not fetch the issuer certificates from %s", url), err
	}
	fetched, err := parseCertificates(b)
	if err != nil {
		return structures.Fail, fmt.Sprintf("Could not read the issuer certificates from %s: %v", url, err), nil
	}

	at := time.Now()
	if r := claim.Header.Timing.Received; r > 0 {
		at = time.Unix(0, int64(r))
	}
	chain, err := verifyEKChain(ek, fetched, at)
	if err != nil {
		return structures.Fail, fmt.Sprintf("EK certificate not verified against %s: %v%s", url, err, lenientNote(ek)), nil
	}

	names := make([]string, len(chain))
	for i, c := range chain {
		names[i] = fmt.Sprintf("%q", c.Subject.String())
	}
	issuer := chain[0]
	return structures.Success, fmt.Sprintf("EK certificate issued by %q (SHA-256 %s), chain %s, from %s%s",
		issuer.Subject.String(), certFingerprint(issuer), strings.Join(names, " <- "), url, lenientNote(ek)), nil
}
