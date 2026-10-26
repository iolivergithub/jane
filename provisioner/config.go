package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProvisioningFile is the YAML provisioning file (see examplecreateprovisioningfile.yaml).
type ProvisioningFile struct {
	AttestationServer string              `yaml:"attestationserver"`
	Element           ElementConfig       `yaml:"element"`
	Endpoints         map[string]Endpoint `yaml:"endpoints"`
	TPM2              TPM2                `yaml:"tpm2"`
	ProvisionWorklist []string            `yaml:"provisionworklist"`
	EVS               []EVEntry           `yaml:"evs"`
}

type ElementConfig struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	// ElementType, e.g. physical-machine or virtual-machine (#118); optional
	ElementType string `yaml:"elementtype"`
}

// Endpoint matches janeserver's structures.Endpoint.
type Endpoint struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Protocol string `yaml:"protocol" json:"protocol"`
}

// TPM2 matches janeserver's structures.TPM2. Handles are strings; yaml.v3
// keeps the literal text of an unquoted hex scalar such as 0x810100EE.
type TPM2 struct {
	Device       string `yaml:"device" json:"device"`
	EKCertHandle string `yaml:"ekcerthandle" json:"ekcerthandle"`
	EK           TPMKey `yaml:"ek" json:"ek"`
	AK           TPMKey `yaml:"ak" json:"ak"`
	// PCRNaming: the register naming scheme for this TPM's PCRs, e.g.
	// tcg-pcclient-uefi (the default), linux-uefi, windows-bitlocker (#101)
	PCRNaming string `yaml:"pcrnaming,omitempty" json:"pcrnaming,omitempty"`
}

type TPMKey struct {
	Handle string `yaml:"handle" json:"handle"`
	Public string `yaml:"public,omitempty" json:"public,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
}

// EVSpec is the body of one entry in the evs list. CID, if set, names a
// variable that holds the itemid of the claim made for this intent (#72).
type EVSpec struct {
	Protocol string     `yaml:"protocol"`
	Type     string     `yaml:"type"`
	CID      string     `yaml:"cid"`
	Rules    []RuleSpec `yaml:"rules"`
	// IssuerURL, for type tpm2ekcert, is where the EK certificate's issuer
	// certificates are (https, http or file); it overrides the certificate's
	// own AIA URL (#63).
	IssuerURL string `yaml:"issuerurl"`
}

// RuleNames lists the names of the entry's rules.
func (s EVSpec) RuleNames() []string {
	names := make([]string, len(s.Rules))
	for i, r := range s.Rules {
		names[i] = r.Name
	}
	return names
}

// RuleSpec is one rule of an evs entry, written in YAML either as a plain
// rule name or as a single-key map from the rule name to its options (#72):
//
//   - tpm2_attestedValue
//   - tpm2_pcrDigest:
//     rid: r_quote          # variable for the result itemid
//     resvar: r_quote_code  # variable for the result code
//     parameters: { "pcrscid": $pcrs }
type RuleSpec struct {
	Name       string
	RID        string
	ResVar     string
	Parameters map[string]any
}

type ruleOptions struct {
	RID        string    `yaml:"rid"`
	ResVar     string    `yaml:"resvar"`
	Parameters yaml.Node `yaml:"parameters"`
}

func (r *RuleSpec) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		r.Name = n.Value
		return nil
	}
	var m map[string]ruleOptions
	if err := n.Decode(&m); err != nil {
		return err
	}
	if len(m) != 1 {
		return fmt.Errorf("line %d: each rule must be a name or a single-key map, found %d keys", n.Line, len(m))
	}
	for k, v := range m {
		r.Name, r.RID, r.ResVar = k, v.RID, v.ResVar
		switch v.Parameters.Kind {
		case 0:
		case yaml.MappingNode:
			if err := v.Parameters.Decode(&r.Parameters); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: parameters of rule %s must be a mapping, e.g. { \"pcrscid\": $pcrs }, not text", v.Parameters.Line, k)
		}
	}
	return nil
}

// EVEntry is one item of the evs list, written in YAML as a single-key map
// from intent ID to its spec:
//
//   - std::intent::sys::info :
//     protocol: tarzan
type EVEntry struct {
	IntentID string
	Spec     EVSpec
}

func (e *EVEntry) UnmarshalYAML(n *yaml.Node) error {
	var m map[string]EVSpec
	if err := n.Decode(&m); err != nil {
		return err
	}
	if len(m) != 1 {
		return fmt.Errorf("line %d: each evs entry must have exactly one intent, found %d", n.Line, len(m))
	}
	for k, v := range m {
		e.IntentID, e.Spec = k, v
	}
	return nil
}

func loadProvisioningFile(path string) (*ProvisioningFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p ProvisioningFile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.AttestationServer == "" {
		return nil, fmt.Errorf("attestationserver is not set")
	}
	if err := p.checkVars(); err != nil {
		return nil, err
	}
	return &p, nil
}

// VarError is a mistake in the variables of a provisioning file (#72). jp
// stops with exit code 2 before contacting Jane.
type VarError struct{ Msg string }

func (e *VarError) Error() string { return e.Msg }

var varName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// varRef returns the variable named by a parameter value that is exactly
// "$name".
func varRef(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || !strings.HasPrefix(s, "$") {
		return "", false
	}
	return s[1:], true
}

// checkVars checks, in file order, that every cid, rid and resvar name is
// valid and defined once, and that every $name in rule parameters refers to
// a variable defined earlier (an intent's own cid counts as earlier for its
// rules).
func (p *ProvisioningFile) checkVars() error {
	defined := map[string]bool{}
	define := func(name, what string) error {
		if name == "" {
			return nil
		}
		if !varName.MatchString(name) {
			return &VarError{fmt.Sprintf("%s: variable name %q must be letters, digits and _, not starting with a digit", what, name)}
		}
		if defined[name] {
			return &VarError{fmt.Sprintf("%s: variable %s is defined more than once", what, name)}
		}
		defined[name] = true
		return nil
	}
	for _, ev := range p.EVS {
		if err := define(ev.Spec.CID, ev.IntentID+" cid"); err != nil {
			return err
		}
		for _, r := range ev.Spec.Rules {
			where := ev.IntentID + " rule " + r.Name
			for k, v := range r.Parameters {
				if name, ok := varRef(v); ok && !defined[name] {
					return &VarError{fmt.Sprintf("%s: parameter %s refers to $%s, which is not defined before it", where, k, name)}
				}
			}
			if err := define(r.RID, where+" rid"); err != nil {
				return err
			}
			if err := define(r.ResVar, where+" resvar"); err != nil {
				return err
			}
		}
	}
	return nil
}
