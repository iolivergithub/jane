package ratsdprotocol

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"a10/structures"
)

// taCallTimeout bounds a call to ratsd, so an agent that never answers gives
// an error instead of blocking the attestation forever.
const taCallTimeout = 2 * time.Minute

// RATSD requires a 64 byte nonce, sent base64url encoded without padding
const nonceSize int = 64

// Intent functions supported by this protocol. These are also the paths on the ratsd daemon.
const (
	fnSubattesters = "ratsd/subattesters"
	fnChares       = "ratsd/chares"
)

// Intent parameter keys. As with the tpm2/ parameters of A10HTTPRESTv2 these are namespaced.
// The un-namespaced "attester-selection" (as written in ratsd's own API) is also accepted.
const (
	paramNonce             = "ratsd/nonce"
	paramAttesterSelection = "ratsd/attester-selection"
	rawAttesterSelection   = "attester-selection"
)

// Media types used by ratsd
const (
	charesRequestType  = "application/vnd.veraison.chares+json"
	charesResponseType = `application/eat-ucs+json; eat_profile="tag:github.com,2024:veraison/ratsd"`
)

func Registration() structures.Protocol {
	intents := []string{fnSubattesters, fnChares}

	return structures.Protocol{"RATSD", "RATSD protcol for ratsd", Call, intents}
}

// THis is the function that is called by operations.attestation --- this is the entry point to the actual protocol part.
// It returns a "json" structure and a string with the body type.
// If requestFromRATSD returns and error, then it is encoded here and returned.
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

	var empty map[string]interface{} = make(map[string]interface{}) // this is an  *instantiated* empty map used for error situations

	// Parameters
	//
	// "ips" is the initial set of parameters. Both intents get a fresh nonce, in the same way that
	// tpm2/quote does in A10HTTPRESTv2. Then the policy (intent) parameters are merged in and finally
	// the additional parameters, eg: from the Attest page. Later ones take precedence.
	var ips map[string]interface{} = make(map[string]interface{})

	nce := make([]byte, nonceSize)
	_, _ = rand.Read(nce)
	ips[paramNonce] = base64.RawURLEncoding.EncodeToString(nce)

	pps := mergeMaps(ips, p.Parameters)
	cps := mergeMaps(pps, aps) // this is the final set of call parameters and should be returned to be part of the claim

	// Construct the call

	var req *http.Request
	var err error
	url := strings.TrimRight(ep.Endpoint, "/") + "/" + p.Function

	switch p.Function {
	case fnSubattesters:
		// A plain GET; ratsd takes no body here so the nonce is only recorded in the claim
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return empty, cps, fmt.Errorf("could not construct request: %w", err)
		}

	case fnChares:
		charesbody, err := makeCharesBody(cps)
		if err != nil {
			return empty, cps, err
		}
		postbody, err := json.Marshal(charesbody)
		if err != nil {
			return empty, cps, fmt.Errorf("JSON Marshalling failed: %w", err)
		}
		req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(postbody))
		if err != nil {
			return empty, cps, fmt.Errorf("could not construct request: %w", err)
		}
		req.Header.Set("Content-Type", charesRequestType)
		req.Header.Set("Accept", charesResponseType)

	default:
		return empty, cps, fmt.Errorf("intent function %v is not supported by the RATSD protocol", p.Function)
	}

	client := &http.Client{Timeout: taCallTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return empty, cps, err // err will be the error from http.client.Do
	}
	defer resp.Body.Close()

	ratsdResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, cps, fmt.Errorf("reading response from RATSD: %w", err)
	}

	// Errors from ratsd are problem details (RFC 9457) JSON objects; return them as the body if they parse
	if resp.StatusCode != http.StatusOK {
		var problem map[string]interface{}
		if json.Unmarshal(ratsdResponse, &problem) == nil {
			return problem, cps, fmt.Errorf("RATSD reports error %v: %v", resp.Status, problem["detail"])
		}
		return empty, cps, fmt.Errorf("RATSD reports error %v with response %v", resp.Status, string(ratsdResponse))
	}

	// Successful responses
	var bodymap map[string]interface{} = make(map[string]interface{})

	switch p.Function {
	case fnSubattesters:
		// ratsd returns a JSON array of {name, options}; claim bodies are maps so wrap it
		var subattesters []interface{}
		if err := json.Unmarshal(ratsdResponse, &subattesters); err != nil {
			return empty, cps, fmt.Errorf("JSON Unmarshalling response from RATSD: %w", err)
		}
		bodymap["subattesters"] = subattesters

	case fnChares:
		// ratsd returns a JSON object with cmw, eat_nonce and eat_profile
		if err := json.Unmarshal(ratsdResponse, &bodymap); err != nil {
			return empty, cps, fmt.Errorf("JSON Unmarshalling response from RATSD: %w", err)
		}
	}

	return bodymap, cps, nil
}

// Builds the body of the chares request from the call parameters.
// Only the nonce and attester selection are sent to ratsd.
func makeCharesBody(cps map[string]interface{}) (map[string]interface{}, error) {
	body := make(map[string]interface{})

	nonce, ok := cps[paramNonce].(string)
	if !ok || nonce == "" {
		return nil, fmt.Errorf("parameter %v must be a non-empty base64url string", paramNonce)
	}
	body["nonce"] = nonce

	// The namespaced key takes precedence over the raw ratsd key
	selection, present := cps[paramAttesterSelection]
	if !present {
		selection, present = cps[rawAttesterSelection]
	}
	if present {
		attesters, err := toAttesterList(selection)
		if err != nil {
			return nil, err
		}
		if len(attesters) > 0 {
			body["attester-selection"] = attesters
		}
	}

	return body, nil
}

// Accepts the attester selection as a JSON list of names, e.g. ["mock-tsm","tsm-report"],
// or as a single comma separated string, e.g. "mock-tsm,tsm-report"
func toAttesterList(v interface{}) ([]string, error) {
	attesters := []string{}

	switch sel := v.(type) {
	case []interface{}:
		for _, a := range sel {
			name, ok := a.(string)
			if !ok {
				return nil, fmt.Errorf("%v must contain only attester names (strings), got %v", paramAttesterSelection, a)
			}
			if name = strings.TrimSpace(name); name != "" {
				attesters = append(attesters, name)
			}
		}
	case []string:
		for _, name := range sel {
			if name = strings.TrimSpace(name); name != "" {
				attesters = append(attesters, name)
			}
		}
	case string:
		for _, name := range strings.Split(sel, ",") {
			if name = strings.TrimSpace(name); name != "" {
				attesters = append(attesters, name)
			}
		}
	default:
		return nil, fmt.Errorf("%v must be a list of attester names or a comma separated string, got %v", paramAttesterSelection, v)
	}

	return attesters, nil
}
