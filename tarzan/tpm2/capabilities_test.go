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

func TestCommandNames(t *testing.T) {
	cases := []struct {
		a    tpm2.TPMACC
		want string
	}{
		{tpm2.TPMACC{CommandIndex: 0x158}, "TPM_CC_Quote (0x158)"},
		{tpm2.TPMACC{CommandIndex: 0x11F, NV: true, CHandles: 2}, "TPM_CC_NV_UndefineSpaceSpecial (0x11f)"},
		{tpm2.TPMACC{CommandIndex: 0x1A1}, "0x1a1"},
		{tpm2.TPMACC{CommandIndex: 0x0002, V: true}, "vendor command (0x20000002)"},
	}
	for _, c := range cases {
		if got := commandName(commandCode(c.a)); got != c.want {
			t.Errorf("command %+v: got %q, want %q", c.a, got, c.want)
		}
	}
	// every name is a TPM_CC_ name and its code is in the TPM_CC range
	for code, name := range tpmCommandNames {
		if code < 0x11F || code > 0x1FF || len(name) < 8 || name[:7] != "TPM_CC_" {
			t.Errorf("bad table entry 0x%x %q", code, name)
		}
	}
}

// #124: a TPM without ECC is recognised from its algorithm list, and a
// refused optional query is recorded rather than failing the claim.
func TestOptionalSections(t *testing.T) {
	// algorithm list of an Infineon SLB 9665, firmware 5.0 (no ECC)
	// (rsa, sha1, hmac, aes, keyedhash, xor, sha256, rsassa, rsaes, rsapss,
	// oaep, kdf1_sp800_108, symcipher, cfb)
	slb9665 := []tpm2.TPMAlgID{0x0001, 0x0004, 0x0005, 0x0006, 0x0008, 0x000A, 0x000B,
		0x0014, 0x0015, 0x0016, 0x0017, 0x0022, 0x0025, 0x0043}
	if hasAlgorithm(slb9665, tpm2.TPMAlgECC) {
		t.Error("SLB 9665 list: ECC reported")
	}
	if !hasAlgorithm(append(slb9665, tpm2.TPMAlgECC), tpm2.TPMAlgECC) {
		t.Error("ECC in the list but not found")
	}
	if hasAlgorithm(nil, tpm2.TPMAlgECC) {
		t.Error("empty list: ECC reported")
	}

	var r tpm2capabilitiesReturn
	if r.Unavailable != nil {
		t.Fatal("Unavailable should start nil, so it is left out of the JSON")
	}
	r.unavailable("ecccurves", eccNotSupported)
	r.unavailable("commands", "TPM2_GetCapability(TPM_CAP_COMMANDS, 0x11f) failed: TPM_RC_VALUE")
	if r.Unavailable["ecccurves"] != eccNotSupported || len(r.Unavailable) != 2 {
		t.Errorf("unavailable = %v", r.Unavailable)
	}
}
