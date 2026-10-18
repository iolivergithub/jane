# Test data

`cos-85-amd-sev.bin` is a TCG crypto-agile event log (SHA-1 and SHA-256 banks)
from a Container-Optimized OS 85 GCE instance with AMD SEV. It is copied
unchanged from Google's go-eventlog
(https://github.com/google/go-eventlog, `testdata/eventlogs/tpm/cos-85-amd-sev.bin`),
licensed under the Apache License 2.0. The PCR values used in the tests come
from go-eventlog's `tpmeventlog/replay_test.go` (`COS85AmdSev`).
