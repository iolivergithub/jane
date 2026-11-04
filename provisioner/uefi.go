package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/iolivergithub/jane/lib/uefi"
)

type efivarsBody struct {
	Efivars *[]efivarEntry `json:"efivars"`
}

type efivarEntry struct {
	Name       string  `json:"name"`
	GUID       string  `json:"guid"`
	ErrorValue string  `json:"errorvalue"`
	RawValue   *string `json:"rawvalue"`
}

// uefiEVs builds the evs map for the UEFI_ rules from the body of a
// uefi/efivars claim: one entry per rule variable, as hex, sha256:<hex> or
// "absent".
//
// tarzan sends every variable efivarfs lists, so a variable missing from the
// claim is absent on the machine. tarzan does not report a failure to list
// variables, though, so a claim with no variables at all is refused rather
// than recorded as "everything absent". A variable the agent could not read
// is left out with a warning (its rule will then fail), and a claim from an
// agent that does not send rawvalue is refused, since its values are lossy.
func uefiEVs(body json.RawMessage, out io.Writer) (map[string]any, error) {
	var b efivarsBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	if b.Efivars == nil {
		return nil, errors.New("claim body has no efivars")
	}
	if len(*b.Efivars) == 0 {
		return nil, errors.New("claim lists no UEFI variables; the agent may not have been able to read efivarfs, so no expected value was recorded")
	}

	type key struct{ name, guid string }
	byKey := map[key]efivarEntry{}
	for _, v := range *b.Efivars {
		byKey[key{v.Name, strings.ToLower(v.GUID)}] = v
	}

	evs := map[string]any{}
	for _, rv := range uefi.RuleVariables {
		v, ok := byKey[key{rv.Name, rv.GUID}]
		if !ok {
			evs[rv.Name] = uefi.Absent
			continue
		}
		if v.ErrorValue != "" {
			fmt.Fprintf(out, "  ... warning: %s could not be read by the agent (%s); left out of the expected value, so UEFI_%s will fail\n", rv.Name, v.ErrorValue, rv.Name)
			continue
		}
		if v.RawValue == nil {
			return nil, fmt.Errorf("%s has no rawvalue; the agent is too old to send UEFI variables exactly, update tarzan", rv.Name)
		}
		data, err := base64.StdEncoding.DecodeString(*v.RawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: rawvalue is not base64: %w", rv.Name, err)
		}
		evs[rv.Name] = uefi.Value(data)
	}
	return evs, nil
}
