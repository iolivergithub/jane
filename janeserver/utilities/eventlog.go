package utilities

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// Parsing of TCG PC Client (UEFI) event logs, shared by the claim view
// (#73) and the tpm2_uefiLogReplay rule (#71). Parsing and replay are done
// by github.com/google/go-eventlog.

// DecodeEventLog base64-decodes an event log as sent by tarzan in a
// uefi/eventlog claim (body field "eventlog") and parses it. The result is
// not verified: replay it against PCR values before trusting it.
func DecodeEventLog(b64 string) (*tcg.EventLog, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding the event log: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("the event log is empty")
	}
	el, err := tcg.ParseEventLog(raw, tcg.ParseOpts{AllowPadding: true})
	if err != nil {
		return nil, fmt.Errorf("parsing the event log: %w", err)
	}
	return el, nil
}

// EventLogBanks returns the banks the log has digests for, keyed by the bank
// names tarzan uses in tpm2/pcrs claims ("sha1", "sha256", "sha384").
func EventLogBanks(el *tcg.EventLog) map[string]register.HashAlg {
	banks := make(map[string]register.HashAlg, len(el.Algs))
	for _, a := range el.Algs {
		banks[strings.ToLower(a.String())] = a
	}
	return banks
}
