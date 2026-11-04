package structures

import "github.com/iolivergithub/jane/lib/janeapi"

type Element struct {
	ItemID      string `json:"itemid,omitempty" bson:"itemid,omitempty" yaml:"itemid,omitempty"`
	Name        string `json:"name" bson:"name" yaml:"name"`
	Description string `json:"description" bson:"description" yaml:"description"`

	// endpoints, map a name to a string containing the URL - this allows multiple endpoints for a single
	// element, supporting multiple protocols
	Endpoints map[string]Endpoint `json:"endpoints" bson:"endpoints" yaml:"endpoints"`

	Tags []string `json:"tags" bson:"tags" yaml:"tags"`

	// ElementType says what the element is: a free string, by convention
	// "physical-machine" or "virtual-machine" (#118). Empty means not stated.
	ElementType string `json:"elementtype,omitempty" bson:"elementtype,omitempty" yaml:"elementtype,omitempty"`

	// Relationships to other elements, a set (#118). Changed only by the
	// relationship operations; UpdateElement keeps the stored ones.
	Relationships []ElementRelationship `json:"relationships,omitempty" bson:"relationships,omitempty" yaml:"relationships,omitempty"`

	// AttestationProfiles are named attestation instructions (#128), written
	// only by jp create/update; DefaultAttestationProfile names the one run
	// when none is named. Both are kept by UpdateElement and set only by
	// SetAttestationProfile.
	AttestationProfiles       map[string]AttestationProfile `json:"attestationprofiles,omitempty" bson:"attestationprofiles,omitempty" yaml:"attestationprofiles,omitempty"`
	DefaultAttestationProfile string                        `json:"defaultattestationprofile,omitempty" bson:"defaultattestationprofile,omitempty" yaml:"defaultattestationprofile,omitempty"`

	Sshkey SSHKEY      `json:"sshkey,omitempty" bson:"sshkey,omitempty"  yaml:"sshkey,omitempty"`
	TPM2   TPM2        `json:"tpm2,omitempty" bson:"tpm2,omitempty" yaml:"tpm2,omitempty"`
	UEFI   UEFI        `json:"uefi,omitempty" bson:"uefi,omitempty" yaml:"uefi,omitempty"`
	IMA    IMA         `json:"ima,omitempty" bson:"ima,omitempty" yaml:"ima,omitempty"`
	TXT    TXT         `json:"txt,omitempty" bson:"txt,omitempty" yaml:"txt,omitempty"`
	Host   HostMachine `json:"host,omitempty" bson:"host,omitempty"  yaml:"host,omitempty"`

	RecordHistory RecordHistory `json:"recordhistory,omitempty" bson:"recordhistory,omitempty"  yaml:"recordhistory,omitempty"`
}

// ELEMENTSUMMARY MUST BE A SUBSET OF THE ELEMENT TYPE
type ElementSummary struct {
	ItemID    string              `json:"itemid,omitempty" bson:"itemid,omitempty"  yaml:"itemid,omitempty"`
	Name      string              `json:"name" bson:"name"  yaml:"name"`
	Endpoints map[string]Endpoint `json:"endpoints" bson:"endpoints"  yaml:"endpoints"`
}

// ElementRelationship relates an element to another, e.g. a VM that is
// "hosted-on" its host (#118). It is stored on the source element.
type ElementRelationship struct {
	RelationshipType string `json:"relationshiptype" bson:"relationshiptype" yaml:"relationshiptype"`
	ItemID           string `json:"itemid" bson:"itemid" yaml:"itemid"`
}

// The known relationship types (#118).
const (
	RelationshipRunsOn   = "runs-on"   // software runs on the target element
	RelationshipHostedOn = "hosted-on" // a VM runs on the target element
)

// RelationshipTypes lists the known relationship types.
var RelationshipTypes = []string{RelationshipRunsOn, RelationshipHostedOn}

// Endpoint, HostMachine, UEFI, IMA, TPM2 and TPMKey are shared with jp
// (lib/janeapi), so what jp sends is what Jane stores.
type Endpoint = janeapi.Endpoint

type HostMachine = janeapi.HostMachine

type SSHKEY struct {
	Key      string `json:"key" bson:"key"  yaml:"key"`
	Timeout  int16  `json:"timeout" bson:"timeout"  yaml:"timeout"`
	Username string `json:"username" bson:"username"  yaml:"username"`
}

type UEFI = janeapi.UEFI

type IMA = janeapi.IMA

type TXT struct {
	Log string `json:"log" bson:"log"  yaml:"log"`
}

type TPM2 = janeapi.TPM2

type TPMKey = janeapi.TPMKey
