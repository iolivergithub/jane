# Protocols

A protocol is how Jane talks to a trust agent. Each element endpoint names one (`endpoints: <name>: protocol:`), and the protocol decides which intent functions it can carry and how. The Web UI's **Protocols** page lists the registered ones.

| Protocol | Trust agent | Functions | Notes |
|---|---|---|---|
| `A10HTTPRESTv2` | [tarzan](trustagents.md#tarzan) (or another agent speaking the same HTTP API) | `tpm2/pcrs`, `tpm2/quote`, `tpm2/gettime`, `tpm2/capabilities`, `tpm2/activatecredential`, `tpm2/getcert`, `uefi/eventlog`, `uefi/efivars`, `uefi/bootconfig`, `ima/asciilog`, `txt/log`, `sys/info` | JSON over HTTP. Jane fills in parameters from the element (TPM device, key handles and public keys) and generates nonces and credentials |
| `RATSD` | [ratsd](trustagents.md#ratsd), Veraison's attestation daemon | `ratsd/subattesters`, `ratsd/chares` | CMW evidence collections from ratsd's leaf attesters (TSM, NVIDIA GPU, ...); Jane generates the nonce |
| `A10NULLPROTOCOL` | none | any (`*/*`) | testing: always returns a fixed test claim |
| `A10NETCONF` | none | `null/good`, `null/test` | proof of concept for NETCONF; returns a fixed claim |
| `TESTCONTAINERPROTOCOL` | none | `testcontainers/repoimageintegrity` | test protocol for containers |

The parameters each function takes, and what it returns, are in [Functions](functions.md); the attest page's **Reference** shows them too, with where each value comes from.

Protocols are compiled into Jane and registered at start-up (`janeserver/protocols/registerProtocols.go`).
