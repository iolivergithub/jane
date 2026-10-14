//go:build linux

package utilities

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

// openTestTPM opens the TPM for TestMakeCredentialTPM: the device in
// JANE_TEST_TPM (empty for /dev/tpmrm0). Builds can replace it, e.g. with an
// in-process simulator.
var openTestTPM = func(t *testing.T) transport.TPMCloser {
	dev, ok := os.LookupEnv("JANE_TEST_TPM")
	if !ok {
		t.Skip("set JANE_TEST_TPM to a TPM device (e.g. /dev/tpmrm0) to run; it only uses transient objects")
	}
	if dev == "" {
		dev = "/dev/tpmrm0"
	}
	tpm, err := linuxtpm.Open(dev)
	if err != nil {
		t.Fatal(err)
	}
	return tpm
}

func ekSessionForTest() tpm2.Session {
	return tpm2.Policy(tpm2.TPMAlgSHA256, 16,
		func(t transport.TPM, s tpm2.TPMISHPolicy, nonce tpm2.TPM2BNonce) error {
			_, err := tpm2.PolicySecret{
				AuthHandle:    tpm2.AuthHandle{Handle: tpm2.TPMRHEndorsement, Auth: tpm2.PasswordAuth(nil)},
				PolicySession: s,
				NonceTPM:      nonce,
			}.Execute(t)
			return err
		})
}

// TestMakeCredentialTPM checks a credential from MakeCredential, made from
// the keys as the provisioner stores them, is activated by a real TPM and the
// recovered secret has the recorded hash.
func TestMakeCredentialTPM(t *testing.T) {
	tpm := openTestTPM(t)
	defer tpm.Close()

	ek, err := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.AuthHandle{Handle: tpm2.TPMRHEndorsement, Auth: tpm2.PasswordAuth(nil)},
		InPublic:      tpm2.New2B(tpm2.RSAEKTemplate),
	}.Execute(tpm)
	if err != nil {
		t.Fatalf("creating EK: %v", err)
	}
	defer tpm2.FlushContext{FlushHandle: ek.ObjectHandle}.Execute(tpm)

	parent := tpm2.AuthHandle{Handle: ek.ObjectHandle, Name: ek.Name, Auth: ekSessionForTest()}
	created, err := tpm2.Create{ParentHandle: parent, InPublic: tpm2.New2B(testAKTemplate)}.Execute(tpm)
	if err != nil {
		t.Fatalf("creating AK: %v", err)
	}
	parent.Auth = ekSessionForTest()
	ak, err := tpm2.Load{ParentHandle: parent, InPrivate: created.OutPrivate, InPublic: created.OutPublic}.Execute(tpm)
	if err != nil {
		t.Fatalf("loading AK: %v", err)
	}
	defer tpm2.FlushContext{FlushHandle: ak.ObjectHandle}.Execute(tpm)

	b64 := func(p tpm2.TPM2BPublic) string { return base64.StdEncoding.EncodeToString(tpm2.Marshal(&p)) }
	c, err := MakeCredential(b64(created.OutPublic), b64(ek.OutPublic))
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := base64.StdEncoding.DecodeString(c.Blob)
	encSecret, _ := base64.StdEncoding.DecodeString(c.EncryptedSecret)

	rsp, err := tpm2.ActivateCredential{
		ActivateHandle: tpm2.AuthHandle{Handle: ak.ObjectHandle, Name: ak.Name, Auth: tpm2.PasswordAuth(nil)},
		KeyHandle:      tpm2.AuthHandle{Handle: ek.ObjectHandle, Name: ek.Name, Auth: ekSessionForTest()},
		CredentialBlob: tpm2.TPM2BIDObject{Buffer: blob},
		Secret:         tpm2.TPM2BEncryptedSecret{Buffer: encSecret},
	}.Execute(tpm)
	if err != nil {
		t.Fatalf("ActivateCredential: %v", err)
	}
	if got := CredentialSecretHash(rsp.CertInfo.Buffer); got != c.SecretHash {
		t.Errorf("recovered secret hash %v, want %v", got, c.SecretHash)
	}
}
