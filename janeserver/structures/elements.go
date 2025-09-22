package structures

type Element struct {
	ItemID      string `json:"itemid,omitempty" bson:"itemid,omitempty" yaml:"itemid,omitempty"`
	Name        string `json:"name" bson:"name" yaml:"name"`
	Description string `json:"description" bson:"description" yaml:"description"`

	// endpoints, map a name to a string containing the URL - this allows multiple endpoints for a single
	// element, supporting multiple protocols
	Endpoints map[string]Endpoint `json:"endpoints" bson:"endpoints" yaml:"endpoints"`

	Tags []string `json:"tags" bson:"tags" yaml:"tags"`

	Sshkey           SSHKEY           `json:"sshkey,omitempty" bson:"sshkey,omitempty"  yaml:"sshkey,omitempty"`
	TPM2             TPM2             `json:"tpm2,omitempty" bson:"tpm2,omitempty" yaml:"tpm2,omitempty"`
	UEFI             UEFI             `json:"uefi,omitempty" bson:"uefi,omitempty" yaml:"uefi,omitempty"`
	IMA              IMA              `json:"ima,omitempty" bson:"ima,omitempty" yaml:"ima,omitempty"`
	TXT              TXT              `json:"txt,omitempty" bson:"txt,omitempty" yaml:"txt,omitempty"`
	Host             HostMachine      `json:"host,omitempty" bson:"host,omitempty"  yaml:"host,omitempty"`
	MRCoordinator    MRCoordinator    `json:"mrcoordinator,omitempty" bson:"mrcoordinator,omitempty"  yaml:"mrcoordinator,omitempty"`
	MRMarbleInstance MRMarbleInstance `json:"mrmarbleinstance,omitempty" bson:"mrmarbleinstance,omitempty"  yaml:"mrmarbleinstance,omitempty"`

	Archive Archive `json:"archive,omitempty" bson:"archive,omitempty"  yaml:"archive,omitempty"`
}

// ELEMENTSUMMARY MUST BE A SUBSET OF THE ELEMENT TYPE
type ElementSummary struct {
	ItemID    string              `json:"itemid,omitempty" bson:"itemid,omitempty"  yaml:"itemid,omitempty"`
	Name      string              `json:"name" bson:"name"  yaml:"name"`
	Endpoints map[string]Endpoint `json:"endpoints" bson:"endpoints"  yaml:"endpoints"`
}

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
}

type TPMKey struct {
	Handle string `json:"handle" bson:"handle"  yaml:"handle"`
	// Public portion of the key marshalled as TPM2BPublic
	Public string `json:"public" bson:"public"  yaml:"public"`
	Name   string `json:"name" bson:"name"  yaml:"name"`
}

type MRCoordinator struct {
	Certs []string `json:"certs" bson:"certs"  yaml:"certs"`
}

type MRMarbleInstance struct {
	ExpectedNonce    string `json:"expectednonce" bson:"expectednonce"  yaml:"expectednonce"`
	RequestData      string `json:"requestdata" bson:"requestdata"  yaml:"requestdata"`
	RequestSignature string `json:"requestsignature" bson:"requestsignature"  yaml:"requestsignature"`
}
