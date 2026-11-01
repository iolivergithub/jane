# Test data

`cos-85-amd-sev.bin` is a TCG crypto-agile event log (SHA-1 and SHA-256 banks)
from a Container-Optimized OS 85 GCE instance with AMD SEV. It is copied
unchanged from Google's go-eventlog
(https://github.com/google/go-eventlog, `testdata/eventlogs/tpm/cos-85-amd-sev.bin`),
licensed under the Apache License 2.0. The PCR values used in the tests come
from go-eventlog's `tpmeventlog/replay_test.go` (`COS85AmdSev`).

`cos-85-uefistate.json` (1.6) is the measured UEFI state of that log, as the
`uefi_log` rules and jp's expected-value type `uefieventlog` compute it. Both
`utilities/uefistate_test.go` and jp's `uefieventlog_test.go` check against
it, so the two implementations stay in step.
