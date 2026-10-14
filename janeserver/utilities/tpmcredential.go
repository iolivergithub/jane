package utilities

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/google/go-tpm/tpm2"
)

// credentialSecretSize is the size of the random secret in a credential. It
// must not exceed the digest size of the EK's name algorithm (32 for SHA-256).
const credentialSecretSize = 32

// Credential is a TPM2_MakeCredential result for tpm2/activatecredential
// (#49), encoded for call parameters. The secret itself is not kept: only its
// SHA-256, so it can be checked when the TPM returns it.
type Credential struct {
	Blob            string // TPM2B_ID_OBJECT contents, base64
	EncryptedSecret string // TPM2B_ENCRYPTED_SECRET contents, base64
	SecretHash      string // SHA-256 of the secret, hex
}

// MakeCredential makes a credential for the AK, encrypted to the EK, both
// given as base64 marshalled TPM2B_PUBLIC (the element's tpm2.ak.public and
// tpm2.ek.public). Only the TPM holding the EK can recover the secret, and
// only for an object with the AK's name. This is done in software; Jane needs
// no TPM.
func MakeCredential(akPublic, ekPublic string) (Credential, error) {
	c, _, err := makeCredential(rand.Reader, akPublic, ekPublic)
	return c, err
}

// makeCredential also returns the secret, for tests.
func makeCredential(rnd io.Reader, akPublic, ekPublic string) (Credential, []byte, error) {
	ak, err := ParseTPMPublic(akPublic)
	if err != nil {
		return Credential{}, nil, fmt.Errorf("AK public: %w", err)
	}
	ek, err := ParseTPMPublic(ekPublic)
	if err != nil {
		return Credential{}, nil, fmt.Errorf("EK public: %w", err)
	}

	akName, err := tpm2.ObjectName(ak)
	if err != nil {
		return Credential{}, nil, fmt.Errorf("computing the AK name: %w", err)
	}
	ekKey, err := tpm2.ImportEncapsulationKey(ek)
	if err != nil {
		return Credential{}, nil, fmt.Errorf("EK cannot be used for a credential: %w", err)
	}

	secret := make([]byte, credentialSecretSize)
	if _, err := io.ReadFull(rnd, secret); err != nil {
		return Credential{}, nil, err
	}

	blob, encSecret, err := tpm2.CreateCredential(rnd, ekKey, akName.Buffer, secret)
	if err != nil {
		return Credential{}, nil, fmt.Errorf("making the credential: %w", err)
	}

	return Credential{
		Blob:            base64.StdEncoding.EncodeToString(blob),
		EncryptedSecret: base64.StdEncoding.EncodeToString(encSecret),
		SecretHash:      CredentialSecretHash(secret),
	}, secret, nil
}

// CredentialSecretHash is the SHA-256 of a credential secret in hex, the form
// kept in the call parameters and checked by tpm2_credentialActivated.
func CredentialSecretHash(secret []byte) string {
	h := sha256.Sum256(secret)
	return hex.EncodeToString(h[:])
}

// ParseTPMPublic decodes a base64 marshalled TPM2B_PUBLIC, the form the
// provisioner stores in an element's tpm2.ek.public and tpm2.ak.public.
func ParseTPMPublic(b64 string) (*tpm2.TPMTPublic, error) {
	if b64 == "" {
		return nil, fmt.Errorf("empty")
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding failed: %w", err)
	}
	pub2B, err := tpm2.Unmarshal[tpm2.TPM2BPublic](b)
	if err != nil {
		return nil, fmt.Errorf("not a TPM2B_PUBLIC: %w", err)
	}
	pub, err := pub2B.Contents()
	if err != nil {
		return nil, fmt.Errorf("not a TPMT_PUBLIC: %w", err)
	}
	return pub, nil
}
