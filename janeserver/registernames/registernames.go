// Package registernames holds names and descriptions of measurement
// registers: TPM PCRs under several naming schemes (TCG PC Client UEFI,
// Linux, Windows BitLocker, coreboot) and the registers of confidential
// computing platforms (Intel TDX, Arm CCA, AMD SEV-SNP, Intel SGX, AWS Nitro)
// (#101). The data is built in (registernames.json, embedded); there is no
// database collection and nothing to download.
//
// An element names its TPM's scheme in tpm2.pcrnaming; empty or unknown means
// the default, tcg-pcclient-uefi.
package registernames

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// Default is the scheme used when an element names none, or an unknown one.
const Default = "tcg-pcclient-uefi"

// Register is one measurement register: its index or name (a string, so
// that "RTMR2" or "MRTD" fit as well as "7"), a short name and a description.
type Register struct {
	Register string `json:"register"`
	Short    string `json:"short"`
	Long     string `json:"long"`
}

// Scheme is a set of register names for one platform or boot chain.
type Scheme struct {
	Key         string     `json:"key"`
	Title       string     `json:"title"`
	Platform    string     `json:"platform"` // tpm2, tdx, cca, sev-snp, sgx, nitro
	Description string     `json:"description"`
	Reference   string     `json:"reference"`
	Registers   []Register `json:"registers"`
}

//go:embed registernames.json
var data []byte

var (
	schemes []Scheme
	byKey   map[string]int
)

func init() {
	s, err := parse(data)
	if err != nil {
		panic("registernames: " + err.Error())
	}
	schemes = s
	byKey = map[string]int{}
	for i, sc := range schemes {
		byKey[sc.Key] = i
	}
}

// parse reads and checks the data: unique scheme keys, the default present,
// unique registers within a scheme, no empty names.
func parse(b []byte) ([]Scheme, error) {
	var s []Scheme
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, sc := range s {
		if sc.Key == "" || sc.Title == "" {
			return nil, fmt.Errorf("a scheme has no key or title")
		}
		if keys[sc.Key] {
			return nil, fmt.Errorf("scheme %s listed twice", sc.Key)
		}
		keys[sc.Key] = true
		regs := map[string]bool{}
		for _, r := range sc.Registers {
			if r.Register == "" || r.Short == "" {
				return nil, fmt.Errorf("scheme %s: a register has no name", sc.Key)
			}
			if regs[r.Register] {
				return nil, fmt.Errorf("scheme %s: register %s listed twice", sc.Key, r.Register)
			}
			regs[r.Register] = true
		}
	}
	if !keys[Default] {
		return nil, fmt.Errorf("the default scheme %s is missing", Default)
	}
	return s, nil
}

// Schemes returns every scheme, in the order of the data file.
func Schemes() []Scheme {
	return schemes
}

// Get returns the scheme with this key.
func Get(key string) (Scheme, bool) {
	i, ok := byKey[key]
	if !ok {
		return Scheme{}, false
	}
	return schemes[i], true
}

// Resolve returns the scheme an element uses: key, or the default if key is
// empty or unknown; known says whether key itself was found.
func Resolve(key string) (s Scheme, known bool) {
	if s, ok := Get(key); ok {
		return s, true
	}
	s, _ = Get(Default)
	return s, false
}

// Lookup returns the register's names in the scheme key (resolved as by
// Resolve), or nil if the scheme does not name it.
func Lookup(key, register string) *Register {
	s, _ := Resolve(key)
	for i := range s.Registers {
		if s.Registers[i].Register == register {
			return &s.Registers[i]
		}
	}
	return nil
}
