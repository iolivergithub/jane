package ratsdprotocol

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"a10/structures"
	//"a10/utilities"
)

const nonceSize int = 64

func Registration() structures.Protocol {
	intents := []string{"ratsd/chares"}

	return structures.Protocol{"RATSD", "RATSD protcol for ratsd", Call, intents}
}

// THis is the function that is called by operations.attestation --- this is the entry point to the actual protocol part.
// It returns a "json" structure and a string with the body type.
// If requestFromTA returns and error, then it is encoded here and returned.
// The body type is *ERROR in these situations and the body should have a field "error": <some value>
func Call(e structures.Element, ep structures.Endpoint, p structures.Intent, s structures.Session, aps map[string]interface{}) (map[string]interface{}, map[string]interface{}, string) {
	rtn, ips, err := requestFromRATSD(e, ep, p, s, aps)

	if err != nil {
		rtn["error"] = err.Error()
		return rtn, ips, structures.CLAIMERROR
	} else {
		return rtn, ips, p.Function
	}
}

func mergeMaps(m1 map[string]interface{}, m2 map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		merged[key] = value
	}
	return merged
}

// This function performs the actual interaction with ratsd
// This will be highly specific to the actual protocol and its implemented intents
func requestFromRATSD(e structures.Element, ep structures.Endpoint, p structures.Intent, s structures.Session, aps map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {

	var empty map[string]interface{} = make(map[string]interface{})   // this is an  *instantiated* empty map used for error situations
	var bodymap map[string]interface{} = make(map[string]interface{}) // this is used to store the result of the final unmarshalling  of the body received from the TA

	// Parameters
	//
	// Some come from the element itself, eg: UEFI.eventlog
	// Then those supplied by the policy and finally the additional parameters
	// Only certain intents supply parameters and these are dealt with on a case by case basis here
	//
	// First we construct "ips" which is the intial set of parameters
	//
	// For sanity reasons (and Go's strong typing, the parameters is a plain key,value list)
	var ips map[string]interface{} = make(map[string]interface{})

	// create None for RATSD call
	nce := make([]byte, nonceSize)
	_, _ = rand.Read(nce)
	ips["nonce"] = base64.URLEncoding.EncodeToString(nce)
	ips["nonce"] = strings.TrimRight(ips["nonce"].(string), "=")
	fmt.Printf("Nonce is %v\n", ips["nonce"])

	// merge ips with policy parameters. The policy parameters take precidence

	pps := mergeMaps(ips, p.Parameters)
	cps := mergeMaps(pps, aps) // this is the final set of call parameters and should be returned to be part of the claim

	// Construct the call

	postbody, err := json.Marshal(cps)
	if err != nil {
		return empty, cps, fmt.Errorf("JSON Marshalling failed: %w", err)
	}

	url := ep.Endpoint + "/" + p.Function
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postbody))
	req.Header.Set("Content-Type", "application/vnd.veraison.chares+json")
	req.Header.Set("Accept", "application/eat-ucs+json; eat_profile=\"tag:github.com,2024:veraison/ratsd\"")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return empty, cps, err // err will be the error from http.client.Do
	}
	defer resp.Body.Close()

	taResponse, _ := io.ReadAll(resp.Body)
	fmt.Println("*****************")
	fmt.Printf("ratsd reponse is %v type is %v", taResponse, reflect.TypeOf(taResponse))

	var asciiString string
	for _, code := range taResponse {
		asciiString += string(rune(code))
	}
	fmt.Printf("as ascii %v\n", asciiString)

	bodymap["response"] = asciiString

	// at the moment we just get a string of integers back
	//err = json.Unmarshal(taResponse, &bodymap)
	fmt.Println("\nbodymap")
	fmt.Printf("%v", bodymap)
	fmt.Println("*****************")

	if err != nil {
		return empty, cps, fmt.Errorf("JSON Unmarshalling reponse from TA: %w", err)
	}

	switch resp.Status {
	case "200 OK":
		return bodymap, cps, nil

	default:
		return bodymap, cps, fmt.Errorf("RATSD reports error %v with response %v", resp.Status, taResponse)
	}

}
