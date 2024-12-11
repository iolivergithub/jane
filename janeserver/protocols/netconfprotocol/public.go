package netconfprotocol

import (
	"crypto/rand"
	"fmt"
	"log"

	"a10/structures"
)

const nonceSize int = 32

func Registration() structures.Protocol {
	intents := []string{"null/good", "null/test"}

	return structures.Protocol{"A10NETCONF", "POC protocol module for NetConf", Call, intents}
}

func Call(e structures.Element, ep structures.Endpoint, p structures.Intent, s structures.Session, cps map[string]interface{}) (map[string]interface{}, map[string]interface{}, string) {

	// Create a test body

	rtn := map[string]interface{}{
		"foo":     "bar",
		"calling": fmt.Sprintf("with protocol %v I would send an intent to %v", ep.Protocol, p.Function),
		"aNumber": 42,
	}

	nce := make([]byte, nonceSize)
	_, _ = rand.Read(nce)

	ips := map[string]interface{}{
		"nonce": nce,
	}

	// Check if the policy intent was null/null, if so then return with the bodytype being set to null/test
	// or error if the above is false.
	//
	// Claim bodytype should be set to error and a ClaimError structure returned in an error field

	if p.Function == "null/null" {
		log.Println(" null call worked ")
		rtn["worked"] = true
		return rtn, ips, "null/test"
	} else {
		log.Println(" null call bad error ")
		rtn["error"] = "Error here"
		return rtn, ips, structures.CLAIMERROR
	}
}
