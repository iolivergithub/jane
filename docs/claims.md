# Claims

A claim is what a trust agent returned for one intent on one element, together with everything needed to understand it later. Jane makes one claim per attestation request, stores it in the `claims` collection, adds it to the request's session, and never changes it.

| Field | Meaning |
|---|---|
| `itemid` | the claim's identifier, made by Jane |
| `bodytype` | the intent's function (e.g. `tpm2/quote`), or `*ERROR` if attesting failed |
| `header` | the context (below) |
| `body` | what the trust agent returned, as JSON; for an error claim, an `error` field |
| `footer` | hash and signature over `bodytype`, `header` and `body` (see [Signing](signing.md)) |

The **header** is a full copy of what was used, so the claim stays meaningful even if the element or intent is changed or deleted later:

| Header field | Meaning |
|---|---|
| `element` | the element, as it was |
| `intent` | the intent, as it was |
| `endpointname`, `endpoint` | which of the element's endpoints was used (name, URL, protocol) |
| `session` | the session the claim belongs to |
| `timing` | `requested` and `received`, nanoseconds since 1970 (UTC) |
| `aps` | additional parameters given by the caller (attest page, REST, jp) |
| `cps` | the call parameters actually sent to the trust agent: the intent's parameters, values taken from the element (e.g. `tpm2/device`, `tpm2/akhandle`), the additional parameters, and anything Jane generated (e.g. `tpm2/nonce`) |

Rules read the claim: its body, and often its call parameters (a nonce, the AK handle). Because call parameters are stored, secrets must never be passed as parameters; passwords such as the TPM endorsement password stay on the element (see [Running](running.md#tarzan)).

The Web UI shows each claim with a page suited to its function (decoded quotes, PCR tables, event logs) and its raw JSON. The body formats per function are described in [Functions](functions.md).

The Go definition is `janeserver/structures/claims.go`.
