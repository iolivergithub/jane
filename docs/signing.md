# Signing

Jane signs every claim, result and session it stores, so that tampering with them later can be detected. The signature goes in the item's **footer**:

| Field | Meaning |
|---|---|
| `hash` | SHA-256 of the item's canonical JSON (the claim's bodytype, header and body; the result's fields; the session's fields) |
| `alg` | the signature algorithm, as a JOSE name: `PS256` (RSA), `ES256`/`ES384`/`ES512` (ECDSA), `EdDSA` (Ed25519) |
| `kid` | the key ID: `sha256:` and the hex SHA-256 of the public key (PKIX DER) |
| `signature` | the signature over the canonical JSON of `{"alg", "hash", "kid"}`, so neither the algorithm nor the key ID can be swapped |

Canonical JSON follows RFC 8785 (JCS), except that integers are written exactly as they are: Jane's timestamps are nanoseconds and would lose precision as IEEE doubles. ECDSA signatures are raw `r||s`, as in JOSE.

## The key

Set `signing: keyfile:` in `config.yaml` to a PEM private key (PKCS#8, PKCS#1 or SEC1; RSA, ECDSA P-256/384/521 or Ed25519), readable by group `jane` (see [Hardening](hardening.md#6-the-footer-signing-key)). Without it Jane generates an ephemeral RSA-2048 key at each start and warns: footers signed with it cannot be verified once Jane stops.

The Web UI home page's **Signing** tab shows the key in use: source, type, algorithm and key ID.

## Verifying

Jane signs but does not verify footers; there is no verifier yet. Keeping the public key and its key ID where a verifier can find them is up to you. A verifier must hash the item as stored (the canonical JSON of the stored fields), not a re-encoding by a different version of Jane, since fields have been added and removed between versions.

Jane contains code to sign with a PKCS#11 device (tested with a YubiHSM), but it is not used yet; other key sources are issue #42.
