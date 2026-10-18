//go:build !windows

package tpm2

import (
	"testing"

	"github.com/google/go-tpm/tpm2"
)

func TestCapabilityDecoding(t *testing.T) {
	if got := fourChars(0x49465800); got != "IFX" { // Infineon
		t.Errorf("manufacturer %q", got)
	}
	if got := fourChars(0x322e3000); got != "2.0" {
		t.Errorf("family %q", got)
	}
	if got := propertyName(tpm2.TPMPTManufacturer); got != "TPM_PT_MANUFACTURER" {
		t.Errorf("name %q", got)
	}
	if got := propertyName(tpm2.TPMPT(0x00000115)); got != "0x00000115" {
		t.Errorf("unknown name %q", got)
	}
	if got := propertyValue(tpm2.TPMPTFirmwareVersion1, 0x0007003d); got != "0x0007003d" {
		t.Errorf("firmware %q", got)
	}
	if got := propertyValue(tpm2.TPMPTPCRCount, 24); got != "24" {
		t.Errorf("PCR count %q", got)
	}
	if got := algName(tpm2.TPMAlgSHA256); got != "TPM_ALG_SHA256" {
		t.Errorf("alg %q", got)
	}
	if got := curveName(tpm2.TPMECCNistP256); got != "TPM_ECC_NIST_P256" {
		t.Errorf("curve %q", got)
	}
}
