package actions

import (
	"fmt"

	"tantor/janeapi"
	"tantor/provisioningfile"
)

type postAttestReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

func CreateEVS(eid string) (string, error) {

	s, _ := janeapi.OpenSession("Tantor: Creating EVS for " + eid)

	for _, v := range provisioningfile.ProvisioningData.Evs {
		fmt.Printf("A>>> evs %v\n", v)

		a := janeapi.AttestStr{
			EID: eid,
			EPN: "tarzan",
			PID: v,
			SID: s,
		}

		r, err := janeapi.Attest(a)
		fmt.Printf("<<<A r,err %v %v \n", r, err)

		//TODO:  bind the result to a structure (copy from elsewhere)
		// only handle claims that require EVS, ie: TPM quotes
		//  once that is done, make the EVS
	}

	janeapi.CloseSession(s)

	return "", nil
}
