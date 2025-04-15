package actions

import (
	"fmt"
	"tantor/provisioningfile"
)

var eid string

func RunWorklist(ws []string) {
	for j, v := range ws {
		fmt.Printf("Running item %v : %v\n", j, v)

		switch {
		case v == "collectsysinfo":
			h, _ := CollectSystemInfo()
			provisioningfile.ProvisioningData.Element.Host = h
			successmessage("Host identified")
		case v == "collectuefi":
			f, err := CollectUEFIEventLogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.UEFI.Eventlog = f
				successmessage(f)
			} else {
				errormessage(err)
			}
		case v == "collectuefi":
			f, err := CollectIMALogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.IMA.ASCIILog = f
				successmessage(f)
			} else {
				errormessage(err)
			}
		case v == "collectima":
			f, err := CollectIMALogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.IMA.ASCIILog = f
				successmessage(f)
			} else {
				errormessage(err)
			}
		case v == "tpmclear":
			_, err := TPMClear()
			if err == nil {
				successmessage("TPM cleared")
			} else {
				errormessage(err)
			}
		case v == "tpmprovision":
			_, err := TPMProvision()
			if err == nil {
				successmessage("TPM Provisioned")
			} else {
				errormessage(err)
			}
		case v == "createevs":
			_, err := CreateEVS(eid)
			if err == nil {
				successmessage("EVS Created")
			} else {
				errormessage(err)
			}
		case v == "createelement":
			e, err := CreateElement()
			eid = e
			if err == nil {
				successmessage("Element Created " + eid)
			} else {
				errormessage(err)
			}
		default:
			fmt.Println("Error: unknown work request")
		}
	}
}

func successmessage(msg string) {
	fmt.Printf("* success: %v\n", msg)
}

func errormessage(e error) {
	fmt.Printf("X error %w\n", e.Error())
}
