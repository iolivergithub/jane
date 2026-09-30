package main

import (
	"fmt"
	"os"

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
}

type TPMKey struct {
	Handle string `yaml:"handle" json:"handle"`
	Public string `yaml:"public,omitempty" json:"public,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
}

// EVSpec is the body of one entry in the evs list.
type EVSpec struct {
	Protocol string   `yaml:"protocol"`
	Type     string   `yaml:"type"`
	Rules    []string `yaml:"rules"`
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
	return &p, nil
}
