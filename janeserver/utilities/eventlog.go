package utilities

import (
	"strings"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// Helpers for TCG PC Client (UEFI) event logs, shared by the claim view
// (#73) and the tpm2_uefiLogReplay rule (#71). Decoding a claim's log is
// uefi.DecodeEventLog (lib/uefi); parsing and replay are done by
// github.com/google/go-eventlog.

// EventLogBanks returns the banks the log has digests for, keyed by the bank
// names tarzan uses in tpm2/pcrs claims ("sha1", "sha256", "sha384").
func EventLogBanks(el *tcg.EventLog) map[string]register.HashAlg {
	banks := make(map[string]register.HashAlg, len(el.Algs))
	for _, a := range el.Algs {
		banks[strings.ToLower(a.String())] = a
	}
	return banks
}
