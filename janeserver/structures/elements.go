package structures

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

type Endpoint struct {
	Endpoint string `json:"endpoint" bson:"endpoint"  yaml:"endpoint"`
	Protocol string `json:"protocol" bson:"protocol"  yaml:"protocol"`
}

type HostMachine struct {
	OS        string `json:"os" bson:"os"  yaml:"os"`
	Arch      string `json:"arch" bson:"arch"  yaml:"arch"`
	Hostname  string `json:"hostname" bson:"hostname"  yaml:"hostname"`
	MachineID string `json:"machineid" bson:"machineid"  yaml:"machineid"`
}

type SSHKEY struct {
	Key      string `json:"key" bson:"key"  yaml:"key"`
	Timeout  int16  `json:"timeout" bson:"timeout"  yaml:"timeout"`
	Username string `json:"username" bson:"username"  yaml:"username"`
}

type UEFI struct {
	Eventlog string `json:"eventlog" bson:"eventlog" yaml:"eventlog"`
}

type IMA struct {
	ASCIILog string `json:"asciilog" bson:"asciilog" yaml:"asciilog"`
}

type TXT struct {
	Log string `json:"log" bson:"log"  yaml:"log"`
}

type TPM2 struct {
	Device       string `json:"device" bson:"device"  yaml:"device"`
	EKCertHandle string `json:"ekcerthandle" bson:"ekcerthandle"  yaml:"ekcerthandle"`
	EK           TPMKey `json:"ek" bson:"ek"  yaml:"ek"`
	AK           TPMKey `json:"ak" bson:"ak"  yaml:"ak"`
	// PCRNaming names the register naming scheme for this TPM's PCRs (see
	// package registernames, #101); empty or unknown means the default.
	PCRNaming string `json:"pcrnaming,omitempty" bson:"pcrnaming,omitempty" yaml:"pcrnaming,omitempty"`
}

type TPMKey struct {
	Handle string `json:"handle" bson:"handle"  yaml:"handle"`
	// Public portion of the key marshalled as TPM2BPublic
	Public string `json:"public" bson:"public"  yaml:"public"`
	Name   string `json:"name" bson:"name"  yaml:"name"`
}
