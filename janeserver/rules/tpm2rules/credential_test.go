package tpm2rules

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"

	"a10/structures"
	"a10/utilities"

	"github.com/google/go-tpm/tpm2"
)

func credentialClaim(cps, body map[string]interface{}) structures.Claim {
	var c structures.Claim
	c.Header.CallParameters = cps
	c.Body = body
	return c
}

func TestCredentialActivated(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	hash := utilities.CredentialSecretHash(secret)
	certinfo := base64.StdEncoding.EncodeToString(secret)
	other := base64.StdEncoding.EncodeToString([]byte("something else"))

	cases := []struct {
		name string
		cps  map[string]interface{}
		body map[string]interface{}
		want structures.ResultValue
	}{
		{"matches", map[string]interface{}{"tpm2/credentialsecrethash": hash}, map[string]interface{}{"certinfo": certinfo}, structures.Success},
		{"wrong secret", map[string]interface{}{"tpm2/credentialsecrethash": hash}, map[string]interface{}{"certinfo": other}, structures.Fail},
		{"no hash", map[string]interface{}{}, map[string]interface{}{"certinfo": certinfo}, structures.Fail},
		{"no certinfo", map[string]interface{}{"tpm2/credentialsecrethash": hash}, map[string]interface{}{}, structures.Fail},
		{"certinfo not base64", map[string]interface{}{"tpm2/credentialsecrethash": hash}, map[string]interface{}{"certinfo": "!!"}, structures.Fail},
	}
	for _, c := range cases {
		got, msg, _ := CredentialActivated(credentialClaim(c.cps, c.body), "", structures.ExpectedValue{}, structures.Session{}, nil)
		if got != c.want {
			t.Errorf("%s: got %v (%v), want %v", c.name, got, msg, c.want)
		}
	}
}

func publicWith(t *testing.T, attrs tpm2.TPMAObject) string {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub := tpm2.TPMTPublic{
		Type:             tpm2.TPMAlgRSA,
		NameAlg:          tpm2.TPMAlgSHA256,
		ObjectAttributes: attrs,
		Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
			Scheme:    tpm2.TPMTRSAScheme{Scheme: tpm2.TPMAlgNull},
			KeyBits:   2048,
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: k.N.Bytes()}),
	}
	return base64.StdEncoding.EncodeToString(tpm2.Marshal(tpm2.New2B(pub)))
}

func TestAKAttributes(t *testing.T) {
	good := tpm2.TPMAObject{FixedTPM: true, FixedParent: true, SensitiveDataOrigin: true, UserWithAuth: true, Restricted: true, SignEncrypt: true}
	notFixed := good
	notFixed.FixedTPM = false
	decrypts := good
	decrypts.Decrypt = true
	unrestricted := good
	unrestricted.Restricted = false

	cases := []struct {
		name  string
		akpub interface{}
		want  structures.ResultValue
	}{
		{"proper AK", publicWith(t, good), structures.Success},
		{"not fixedTPM", publicWith(t, notFixed), structures.Fail},
		{"can decrypt", publicWith(t, decrypts), structures.Fail},
		{"not restricted", publicWith(t, unrestricted), structures.Fail},
		{"missing", nil, structures.Fail},
		{"garbage", "AAAA", structures.Fail},
	}
	for _, c := range cases {
		cps := map[string]interface{}{}
		if c.akpub != nil {
			cps["tpm2/akpublic"] = c.akpub
		}
		got, msg, _ := AKAttributes(credentialClaim(cps, nil), "", structures.ExpectedValue{}, structures.Session{}, nil)
		if got != c.want {
			t.Errorf("%s: got %v (%v), want %v", c.name, got, msg, c.want)
		}
	}
}
