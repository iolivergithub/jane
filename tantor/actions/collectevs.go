package actions

import (
	"fmt"

	"tantor/janeapi"
	"tantor/provisioningfile"
)

func CreateEVS(eid string) (string, error) {

	eid := 
	s, err := janeapi.OpenSession("test session")

	for k, v := range provisioningfile.ProvisioningData.Evs {
		fmt.Printf(" evs %v, %v\n", k, v)

		a := janeapi.AttestStr{
			EID: eid,
			EPN: "tarzan",
			PID: v,
			SID: s,
		}

		r, err := janeapi.Attest(a)
		fmt.Printf(" r,err %v %v", r, err)
	}

	janeapi.CloseSession(s)

	return "", nil
}
