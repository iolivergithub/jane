package registernames

import (
	"strconv"
	"testing"
)

func TestData(t *testing.T) {
	if len(Schemes()) < 2 {
		t.Fatalf("%d schemes", len(Schemes()))
	}
	// every TPM scheme names PCRs 0-7 at least, by number
	for _, s := range Schemes() {
		if s.Platform != "tpm2" {
			continue
		}
		for i := 0; i < 8; i++ {
			if Lookup(s.Key, strconv.Itoa(i)) == nil {
				t.Errorf("%s: PCR %d not named", s.Key, i)
			}
		}
	}
	for _, key := range []string{"tcg-pcclient-uefi", "linux-uefi", "windows-bitlocker", "intel-tdx", "arm-cca", "amd-sev-snp", "intel-sgx", "aws-nitro"} {
		if _, ok := Get(key); !ok {
			t.Errorf("scheme %s missing", key)
		}
	}
}

func TestResolveAndLookup(t *testing.T) {
	if s, known := Resolve(""); s.Key != Default || known {
		t.Errorf("empty: %s %v", s.Key, known)
	}
	if s, known := Resolve("no-such-scheme"); s.Key != Default || known {
		t.Errorf("unknown: %s %v", s.Key, known)
	}
	if s, known := Resolve("linux-uefi"); s.Key != "linux-uefi" || !known {
		t.Errorf("linux: %s %v", s.Key, known)
	}
	if r := Lookup("linux-uefi", "10"); r == nil || r.Short != "IMA" {
		t.Errorf("linux PCR 10 = %+v", r)
	}
	if r := Lookup("", "7"); r == nil || r.Short != "Secure Boot policy" {
		t.Errorf("default PCR 7 = %+v", r)
	}
	if r := Lookup("windows-bitlocker", "11"); r == nil || r.Short != "BitLocker access control" {
		t.Errorf("windows PCR 11 = %+v", r)
	}
	if r := Lookup("intel-tdx", "RTMR2"); r == nil {
		t.Error("TDX RTMR2 not named")
	}
	if Lookup("", "99") != nil {
		t.Error("PCR 99 named")
	}
}

func TestParseRejects(t *testing.T) {
	for name, b := range map[string]string{
		"not json":     `{`,
		"no default":   `[{"key":"x","title":"X","registers":[]}]`,
		"twice":        `[{"key":"tcg-pcclient-uefi","title":"T"},{"key":"tcg-pcclient-uefi","title":"T"}]`,
		"register dup": `[{"key":"tcg-pcclient-uefi","title":"T","registers":[{"register":"0","short":"a"},{"register":"0","short":"b"}]}]`,
		"no short":     `[{"key":"tcg-pcclient-uefi","title":"T","registers":[{"register":"0"}]}]`,
	} {
		if _, err := parse([]byte(b)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
