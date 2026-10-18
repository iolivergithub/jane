//go:build !windows

package tpm2

import (
	"encoding/base64"
	"slices"
	"testing"

	"github.com/google/go-tpm/tpm2"
)

// TestGetTimeTPM signs the time with an AK created under the EK, as the
// provisioner does. Needs TARZAN_TEST_TPM (transient objects only).
func TestGetTimeTPM(t *testing.T) {
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

	parent := tpm2.AuthHandle{Handle: ek.ObjectHandle, Name: ek.Name, Auth: ekSession()}
	created, err := tpm2.Create{ParentHandle: parent, InPublic: tpm2.New2B(akTemplate)}.Execute(tpm)
	if err != nil {
		t.Fatalf("creating AK: %v", err)
	}
	parent.Auth = ekSession()
	ak, err := tpm2.Load{ParentHandle: parent, InPrivate: created.OutPrivate, InPublic: created.OutPublic}.Execute(tpm)
	if err != nil {
		t.Fatalf("loading AK: %v", err)
	}
	defer tpm2.FlushContext{FlushHandle: ak.ObjectHandle}.Execute(tpm)

	nonce := []byte("0123456789abcdef01234567")
	rtn, err := getTime(tpm, ak.ObjectHandle, nonce)
	if err != nil {
		t.Fatalf("getTime: %v", err)
	}
	if rtn.Time.Magic != "ff544347" || rtn.Time.Type != "8019" {
		t.Errorf("magic %s type %s", rtn.Time.Magic, rtn.Time.Type)
	}
	if rtn.Time.ExtraData != base64.StdEncoding.EncodeToString(nonce) {
		t.Errorf("extradata %s", rtn.Time.ExtraData)
	}
	raw, err := base64.StdEncoding.DecodeString(rtn.Attest)
	if err != nil || len(raw) == 0 {
		t.Errorf("attest %q: %v", rtn.Attest, err)
	}
	if rtn.RawSignature == "" {
		t.Error("no raw signature")
	}
}

// TestCapabilitiesTPM needs TARZAN_TEST_TPM; it only reads.
func TestCapabilitiesTPM(t *testing.T) {
	tpm := openTestTPM(t)
	defer tpm.Close()

	rtn, err := capabilities(tpm)
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if rtn.Summary.Manufacturer == "" || rtn.Summary.Family != "2.0" {
		t.Errorf("summary %+v", rtn.Summary)
	}
	if _, ok := rtn.Properties["TPM_PT_PCR_COUNT"]; !ok {
		t.Errorf("no TPM_PT_PCR_COUNT in %v", rtn.Properties)
	}
	if rtn.PCRBanks["sha256"] == 0 {
		t.Errorf("PCR banks %v", rtn.PCRBanks)
	}
	if !slices.Contains(rtn.Algorithms, "TPM_ALG_SHA256") {
		t.Errorf("algorithms %v", rtn.Algorithms)
	}
}
