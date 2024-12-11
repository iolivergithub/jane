package veraisonpsaprotocol

import (
	//"fmt"
	"crypto/rand"
	"log"

	"a10/structures"
)

const nonceSize int = 32

func Registration() structures.Protocol {
	intents := []string{"*/*"}

	return structures.Protocol{"EVCLI_PSA", "EVCLI Protocol - TEST VERSION, always returns a PSA test claim", Call, intents}
}

func Call(e structures.Element, ep structures.Endpoint, p structures.Intent, s structures.Session, cps map[string]interface{}) (map[string]interface{}, map[string]interface{}, string) {

	// Create a test body

	/* Need to add this at some point
		[
	            {
	                "measurement-type": "BL",
	                "measurement-value": "AAECBAABAgQAAQIEAAECBAABAgQAAQIEAAECBAABAgQ=",
	                "signer-id": "UZIA/1GSAP9RkgD/UZIA/1GSAP9RkgD/UZIA/1GSAP8="
	            },
	            {
	                "measurement-type": "PRoT",
	                "measurement-value": "BQYHCAUGBwgFBgcIBQYHCAUGBwgFBgcIBQYHCAUGBwg=",
	                "signer-id": "UZIA/1GSAP9RkgD/UZIA/1GSAP9RkgD/UZIA/1GSAP8="
	            }
	        ],
	*/

	rtn := map[string]interface{}{
		"eat-profile":                        "http://arm.com/psa/2.0.0",
		"psa-client-id":                      1,
		"psa-security-lifecycle":             12288,
		"psa-implementation-id":              "UFFSU1RVVldQUVJTVFVWV1BRUlNUVVZXUFFSU1RVVlc=",
		"psa-boot-seed":                      "3q2+796tvu/erb7v3q2+796tvu/erb7v3q2+796tvu8=",
		"psa-hardware-version":               "1234567890123",
		"psa-software-components":            " ",
		"psa-nonce":                          "AAECAwABAgMAAQIDAAECAwABAgMAAQIDAAECAwABAgM=",
		"psa-instance-id":                    "AaChoqOgoaKjoKGio6ChoqOgoaKjoKGio6ChoqOgoaKj",
		"psa-verification-service-indicator": "https://psa-verifier.org",
		"psa-certification-reference":        "1234567890123-12345"}

	nce := make([]byte, nonceSize)
	_, _ = rand.Read(nce)

	ips := map[string]interface{}{
		"nonce": nce,
	}

	// Check if the policy intent was null/null, if so then return with the bodytype being set to null/test
	// or error if the above is false.
	//
	// Claim bodytype should be set to error and a ClaimError structure returned in an error field

	if p.Function == "veraison/psa/get_token" {
		log.Println(" psa call worked ")
		rtn["worked"] = true
		return rtn, ips, "veraison/psa/get_token"
	} else {
		log.Println(" psa call bad error ")
		rtn["error"] = "Unknown PSA function name"
		return rtn, ips, structures.CLAIMERROR
	}
}
