package actions

import (
	"fmt"

	"tantor/janeapi"
	"tantor/provisioningfile"
)

func CreateElement() (string, error) {

	element := provisioningfile.ProvisioningData.Element

	fmt.Printf("ELEMENT %v\n", element)

	r, err := janeapi.AddElement(element)

	fmt.Printf("RESPONSE %v %w\n", r, err)

	return r, nil
}
