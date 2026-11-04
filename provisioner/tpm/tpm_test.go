package tpm

import (
	"testing"

	"github.com/google/go-tpm/tpm2"
)

func TestParseHandle(t *testing.T) {
	for in, want := range map[string]tpm2.TPMHandle{
		"0x810100EE":  0x810100EE,
		"0x810100aa":  0x810100AA,
		" 0x81000001": 0x81000001,
		"2164326638":  0x810100EE, // decimal, as PyYAML turned unquoted hex into
	} {
		got, err := ParseHandle(in)
		if err != nil || got != want {
			t.Errorf("ParseHandle(%q) = %s, %v", in, Hex(got), err)
		}
	}
	for _, bad := range []string{"", "EE", "0x01", "0x80000001", "0x1C000002", "0x1FFFFFFFF"} {
		if _, err := ParseHandle(bad); err == nil {
			t.Errorf("ParseHandle(%q) succeeded", bad)
		}
	}
}

func TestAKTemplate(t *testing.T) {
	a := AKTemplate.ObjectAttributes
	if !a.Restricted || !a.SignEncrypt || a.Decrypt || !a.FixedTPM || !a.FixedParent ||
		!a.SensitiveDataOrigin || !a.UserWithAuth || AKTemplate.NameAlg != tpm2.TPMAlgSHA256 {
		t.Errorf("AK attributes = %+v", a)
	}
	rsaParms, err := AKTemplate.Parameters.RSADetail()
	if err != nil {
		t.Fatal(err)
	}
	scheme, err := rsaParms.Scheme.Details.RSASSA()
	if err != nil || rsaParms.KeyBits != 2048 || scheme.HashAlg != tpm2.TPMAlgSHA256 {
		t.Errorf("AK RSA params = %+v, %v", rsaParms, err)
	}
}

// The owner hierarchy password comes from JANE_TPM_OWNER_PASSWORD; unset or
// empty means the empty password (#86).
func TestOwnerPassword(t *testing.T) {
	t.Setenv(OwnerPasswordVar, "")
	if p := ownerPassword(); len(p) != 0 {
		t.Errorf("empty variable: password %q, want empty", p)
	}
	t.Setenv(OwnerPasswordVar, "s3cret")
	if p := ownerPassword(); string(p) != "s3cret" {
		t.Errorf("password %q, want s3cret", p)
	}
	if h := ownerAuth().Handle; h != tpm2.TPMRHOwner {
		t.Errorf("owner auth handle %v, want TPM_RH_OWNER", h)
	}
}
