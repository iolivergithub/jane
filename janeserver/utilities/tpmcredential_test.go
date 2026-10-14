package utilities

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"github.com/google/go-tpm/tpm2"
)

// testAKTemplate is the provisioner's AK: a restricted RSA-2048 RSASSA/SHA-256
// signing key.
var testAKTemplate = tpm2.TPMTPublic{
	Type:    tpm2.TPMAlgRSA,
	NameAlg: tpm2.TPMAlgSHA256,
	ObjectAttributes: tpm2.TPMAObject{
		FixedTPM: true, FixedParent: true, SensitiveDataOrigin: true,
		UserWithAuth: true, Restricted: true, SignEncrypt: true,
	},
	Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
		Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
		Scheme: tpm2.TPMTRSAScheme{
			Scheme:  tpm2.TPMAlgRSASSA,
			Details: tpm2.NewTPMUAsymScheme(tpm2.TPMAlgRSASSA, &tpm2.TPMSSigSchemeRSASSA{HashAlg: tpm2.TPMAlgSHA256}),
		},
		KeyBits: 2048,
	}),
	Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{}),
}

// softwarePublic fills tmpl with a fresh RSA key and returns it as base64
// TPM2B_PUBLIC, the way the provisioner stores keys in an element.
func softwarePublic(t *testing.T, tmpl tpm2.TPMTPublic) string {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl.Unique = tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: k.N.Bytes()})
	return base64.StdEncoding.EncodeToString(tpm2.Marshal(tpm2.New2B(tmpl)))
}

func TestMakeCredential(t *testing.T) {
	ak := softwarePublic(t, testAKTemplate)
	ek := softwarePublic(t, tpm2.RSAEKTemplate)

	c, secret, err := makeCredential(rand.Reader, ak, ek)
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != credentialSecretSize {
		t.Errorf("secret is %d bytes", len(secret))
	}
	if c.SecretHash != CredentialSecretHash(secret) || len(c.SecretHash) != 64 {
		t.Errorf("SecretHash = %q", c.SecretHash)
	}
	for name, v := range map[string]string{"blob": c.Blob, "encrypted secret": c.EncryptedSecret} {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(b) == 0 {
			t.Errorf("%s: %q, %v", name, v, err)
		}
	}
	// RSA-2048 EK: the encrypted seed is one RSA block
	if b, _ := base64.StdEncoding.DecodeString(c.EncryptedSecret); len(b) != 256 {
		t.Errorf("encrypted secret is %d bytes, want 256", len(b))
	}
	if bytes.Contains([]byte(c.Blob+c.EncryptedSecret), []byte(base64.StdEncoding.EncodeToString(secret))) {
		t.Error("secret appears in the credential")
	}

	c2, err := MakeCredential(ak, ek)
	if err != nil {
		t.Fatal(err)
	}
	if c2.SecretHash == c.SecretHash || c2.Blob == c.Blob {
		t.Error("two credentials are the same")
	}
}

func TestMakeCredentialBadKeys(t *testing.T) {
	ak := softwarePublic(t, testAKTemplate)
	ek := softwarePublic(t, tpm2.RSAEKTemplate)
	cases := map[string][2]string{
		"no AK":         {"", ek},
		"no EK":         {ak, ""},
		"AK not base64": {"!!", ek},
		"EK garbage":    {ak, base64.StdEncoding.EncodeToString([]byte("garbage"))},
	}
	for name, c := range cases {
		if _, err := MakeCredential(c[0], c[1]); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
