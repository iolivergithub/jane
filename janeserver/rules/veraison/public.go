package veraison

import (
	"fmt"

	"a10/structures"
	// include veraison libraries here
)

func Registration() []structures.Rule {

	ruleS := structures.Rule{"veraisin_psa_token_verify", "Implements the PSA token verification", VeraisonPSATokenVerify, true}

	return []structures.Rule{ruleS}
}

func VeraisonPSATokenVerify(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {

	psatoken := claim.Body

	fmt.Printf("PSA token is %v\n", psatoken)

	// syntax/semanic checks

	// CBOR/JSON to Go synatax stuff

	// Call veraison function

	// return
	return structures.Success, "Veraison Says Yes", nil

}
