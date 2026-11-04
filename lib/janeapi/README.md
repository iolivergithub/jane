# janeapi

What jp and Jane must agree on over Jane's REST API, in one place:

- the parts of an element jp writes: `Endpoint`, `TPM2`, `TPMKey`,
  `HostMachine`, `UEFI`, `IMA`, and the attestation profile types
  (`AttestationProfile`, `ProfileStep`, `ProfileRule`, `ProfileFollow`);
- `ValidProfileName` and `ValidAPIKey`, the formats Jane enforces and jp
  checks before it sends anything.

janeserver's `structures` package and jp use the types through type aliases
(`structures.TPM2`, jp's `HostInfo`, `FollowSpec`, ... keep their names), so
the JSON jp sends is by construction what Jane stores. The tags are
janeserver's. The element itself is not shared: jp's `Element` uses
pointers so that it leaves out sections it does not set, and janeserver's
carries history and fields jp never writes. jp's `Claim`, `ElementSummary`
and `ExpectedValue` are partial views of Jane's and stay in jp.

It is the module `github.com/iolivergithub/jane/lib/janeapi`, used through a
`replace` to `../lib/janeapi` in `janeserver/go.mod` and
`provisioner/go.mod`.
