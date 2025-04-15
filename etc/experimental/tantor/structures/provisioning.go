package structures

type Provisioning struct {
	AttestationServer string  `yaml:"attestationserver"`
	Element           Element `yaml:"element"`

	ProvisioningWorkList []string `yaml:"provisionworklist"`
	Evs                  []string `yaml:"evs"`
}
