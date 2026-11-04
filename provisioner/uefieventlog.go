package main

// Expected-value type uefieventlog (#134): the measured UEFI state in a
// uefi/eventlog claim, for janeserver's uefi_log rules, computed by the same
// code (lib/uefi) as the rules use.

import (
	"encoding/json"
	"fmt"

	"github.com/iolivergithub/jane/lib/uefi"
)

// uefiEventLogEVs returns the expected value for a uefi/eventlog claim's
// body. An error means the log can't be read; jp then records nothing.
func uefiEventLogEVs(body json.RawMessage) (map[string]any, error) {
	var b struct {
		Eventlog string `json:"eventlog"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	if b.Eventlog == "" {
		return nil, fmt.Errorf("the claim has no event log")
	}
	el, err := uefi.DecodeEventLog(b.Eventlog)
	if err != nil {
		return nil, err
	}
	st, err := uefi.MeasuredState(el)
	if err != nil {
		return nil, err
	}
	return st.EVS(), nil
}
