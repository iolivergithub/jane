package provisioningfile

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"tantor/structures"
)

var ProvisioningData *structures.Provisioning

func ReadProvisioningFile(f string) {
	fmt.Println("Provisioning file location: ", f)

	pfile, err := ioutil.ReadFile(f)
	if err != nil {
		panic(fmt.Sprintf("Provisioning file missing. Exiting with error %w", err))
	}

	err = yaml.Unmarshal(pfile, &ProvisioningData)
	if err != nil {
		panic(fmt.Sprintf("Unable to parse provisioning file. Exiting with error %w", err))
	}
}
