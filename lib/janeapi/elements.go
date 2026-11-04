package janeapi

// Endpoint is one of an element's endpoints: where its trust agent listens
// and the protocol to use.
type Endpoint struct {
	Endpoint string `json:"endpoint" bson:"endpoint"  yaml:"endpoint"`
	Protocol string `json:"protocol" bson:"protocol"  yaml:"protocol"`
}

// HostMachine is an element's host section, as jp collects it.
type HostMachine struct {
	OS        string `json:"os" bson:"os"  yaml:"os"`
	Arch      string `json:"arch" bson:"arch"  yaml:"arch"`
	Hostname  string `json:"hostname" bson:"hostname"  yaml:"hostname"`
	MachineID string `json:"machineid" bson:"machineid"  yaml:"machineid"`
}

// UEFI is an element's UEFI section.
type UEFI struct {
	Eventlog string `json:"eventlog" bson:"eventlog" yaml:"eventlog"`
}

// IMA is an element's IMA section.
type IMA struct {
	ASCIILog string `json:"asciilog" bson:"asciilog" yaml:"asciilog"`
}

// TPM2 is an element's TPM 2.0 section. Handles are strings; in jp's
// provisioning file yaml.v3 keeps the literal text of an unquoted hex scalar
// such as 0x810100EE.
type TPM2 struct {
	Device       string `json:"device" bson:"device"  yaml:"device"`
	EKCertHandle string `json:"ekcerthandle" bson:"ekcerthandle"  yaml:"ekcerthandle"`
	EK           TPMKey `json:"ek" bson:"ek"  yaml:"ek"`
	AK           TPMKey `json:"ak" bson:"ak"  yaml:"ak"`
	// PCRNaming names the register naming scheme for this TPM's PCRs (see
	// janeserver's package registernames, #101), e.g. tcg-pcclient-uefi (the
	// default), linux-uefi, windows-bitlocker; empty or unknown means the
	// default.
	PCRNaming string `json:"pcrnaming,omitempty" bson:"pcrnaming,omitempty" yaml:"pcrnaming,omitempty"`
}

// TPMKey is a TPM key at a persistent handle.
type TPMKey struct {
	Handle string `json:"handle" bson:"handle"  yaml:"handle"`
	// Public portion of the key marshalled as TPM2BPublic
	Public string `json:"public" bson:"public"  yaml:"public"`
	Name   string `json:"name" bson:"name"  yaml:"name"`
}
