# uefi

What janeserver and jp must agree on about UEFI, in one place:

- `RuleVariables`: the variables checked by janeserver's `UEFI_<name>` rules,
  with their vendor GUIDs. jp records an `uefiefivars` expected value entry
  for each, written by `Value` (hex up to `InlineLimit` bytes, else
  `sha256:<hex>`) or `Absent`.
- `MeasuredState`: the Secure Boot state measured in a UEFI event log (#134),
  checked by the `uefi_log` rules and recorded by jp as the `uefieventlog`
  expected value type. `DecodeEventLog` reads a `uefi/eventlog` claim's log.

Up to 1.6 each program had its own copy, kept in step by hand.

It is the module `github.com/iolivergithub/jane/lib/uefi`, used through a
`replace` to `../lib/uefi` in `janeserver/go.mod` and `provisioner/go.mod`,
so it is never downloaded. `testdata/` holds the COS 85 event log that
janeserver's tests also use.
