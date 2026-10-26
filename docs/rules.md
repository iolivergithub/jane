# Rules

A rule checks one claim and gives a result: pass (`0`) or fail (`9001`), or one of the error values below. Rules are built into Jane; the Web UI's **Rules** page lists the registered ones, and the attest page's **Reference** has the same information as this page.

A rule may need:

- an **expected value** (EV) for the claim's element and intent, holding the "known good" values it compares with (see [Expected values](expectedvalues.md)). jp records these when it provisions an element;
- **rule parameters**, a JSON object given when the rule is run (attest page, REST `/verify`, or `parameters:` in a jp provisioning file). Several rules take the item ID of a second claim, so that two claims of the same element can be checked against each other.

## Result values

| Value | Name | Meaning |
|---|---|---|
| 0 | Success | the check passed |
| 9001 | Fail | the check failed; the message says why |
| 9010 | VerifyCallFailure | the rule could not run (e.g. it returned an error) |
| 9098 | VerifyClaimErrorAttempt | the claim is an error claim; there is nothing to verify |
| 9099 | NoResult | no result was produced |
| 9997 | MissingExpectedValue | the rule needs an expected value and there is none for this element and intent |
| 9998 | RuleCallFailure | the rule does not exist or has no function |
| 9999 | UnsetResultValue | internal: the result was never set |

## TPM 2.0

| Rule | Claim | EV | Parameters | Checks |
|---|---|---|---|---|
| `tpm2_attestedValue` | `tpm2/quote` | `attestedValue` | | the quote's attested value (PCR digest) equals the expected value |
| `tpm2_firmware` | `tpm2/quote` | `firmwareVersion` | | the TPM firmware version equals the expected value |
| `tpm2_magicNumber` | `tpm2/quote` | | | the attest structure's magic number is `ff544347` |
| `tpm2_type` | `tpm2/quote` | | | the attest type is `8018` (a quote) |
| `tpm2_safe` | `tpm2/quote` | | | the TPM reports `safe` (no unclean shutdown since the clock was last saved) |
| `tpm2_validNonce` | `tpm2/quote` | | | the quote's qualifying data equals the nonce Jane sent (`tpm2/nonce`) |
| `tpm2_validSignature` | `tpm2/quote`, `tpm2/gettime` | | | the signature over the raw attest verifies with the AK, and the attest type matches the function (see below) |
| `tpm2_pcrDigest` | `tpm2/quote` | | `pcrscid` | the PCR values in the `tpm2/pcrs` claim named by `pcrscid` hash to this quote's attested value |
| `tpm2_uefiLogReplay` | `tpm2/pcrs` | | `uefilogcid` | replaying the UEFI event log in the `uefi/eventlog` claim named by `uefilogcid` gives this claim's PCR values, per bank |
| `tpm2_clockProgress` | `tpm2/quote`, `tpm2/gettime` | | `previouscid` | clock, reset and restart counts compared with an earlier claim (see below) |
| `tpm2_capabilitiesFirmware` | `tpm2/capabilities` | | `quotecid` | the reported firmware version equals the signed one in a quote or gettime claim (see below) |
| `tpm2_credentialActivated` | `tpm2/activatecredential` | | | the TPM recovered the credential's secret: the AK and EK are in the same TPM |
| `tpm2_akAttributes` | `tpm2/activatecredential` | | | the AK is a restricted signing key with fixedTPM, fixedParent and sensitiveDataOrigin |
| `tpm2_ekCertMatchesEK` | `tpm2/getcert` | | | the EK certificate holds the element's EK (`tpm2.ek.public`); RSA only (see below) |
| `tpm2_ekCertIssuer` | `tpm2/getcert` | `issuerurl` | | the EK certificate was issued by the certificates at `issuerurl` and is valid (see below) |

Parameters naming another claim (`pcrscid`, `uefilogcid`, `previouscid`, `quotecid`) take its item ID. The claim must be of the stated function, for the same element, and not an error claim. If the two claims are from different sessions the message says how far apart they were taken.

**Chains.** `tpm2_uefiLogReplay` (event log → PCRs) and `tpm2_pcrDigest` (PCRs → quote), with `tpm2_validSignature` on the quote, check that the event log explains a quote signed by the TPM.

### tpm2_validSignature

Since 1.5.3 tarzan returns, with every quote and gettime claim, the raw `TPMS_ATTEST` the TPM signed (`attest`) and its signature (`rawsignature`). The rule:

- takes the AK from the element (`tpm2.ak.public`, recorded by jp), or from `tpm2/akpublic` if it was passed as an additional parameter when attesting;
- verifies the signature over `attest`;
- checks the attest is a quote (`8018`) for a `tpm2/quote` claim and a time attestation (`8019`) for a `tpm2/gettime` claim.

It fails for claims made by tarzan before 1.5.3 (no `attest`), elements without an AK public key (run `jp update`), and ECC AKs (only RSA is supported).

**Limitation:** the other quote rules (`tpm2_validNonce`, `tpm2_attestedValue`, `tpm2_firmware`, `tpm2_safe`, `tpm2_magicNumber`, `tpm2_type`, `tpm2_pcrDigest`) read tarzan's decoded copy of the quote, which the signature does not cover.

### tpm2_ekCertMatchesEK and tpm2_ekCertIssuer

Both run on a `tpm2/getcert` claim, the EK certificate read from the TPM's NV storage (#63). The certificate is not signed by the TPM, so on its own it proves nothing; the two rules tie it to the element's EK and to the certificate's issuer.

- **`tpm2_ekCertMatchesEK`** compares the certificate's public key with the EK recorded in the element (`tpm2.ek.public`, as it was when the claim was made). No expected value and no network access: the element's EK is trusted because it was there at provisioning. Only RSA EKs are supported; an ECC EK fails with "ECC EK certificates are not supported yet".
- **`tpm2_ekCertIssuer`** fetches the certificates at the expected value's `issuerurl` (`https://`, with the server's TLS certificate verified, `http://` or `file://`; 10 s timeout, at most 1 MiB) and checks the EK certificate's signature and validity against them. The URL may give one certificate or a bundle, DER or PEM (PKCS#7 `.p7c` files are not read). Self-signed certificates in it are roots, the others intermediates, and the chain from the EK certificate must reach a root; a bundle without a self-signed certificate is trusted as it is (the chain ends at the last certificate found). Validity is checked at the time the claim was received. The message gives the issuer's subject and SHA-256 fingerprint and the chain. Failing to fetch the URL is an error (result code 9010, `VerifyCallFailure`), not a fail: it says nothing about the TPM.

jp records `issuerurl` (expected-value type `tpm2ekcert`) from the certificate's own authority information access (*CA Issuers*) URL, or from `issuerurl:` in the provisioning file, e.g. a file or web server where the certificates from a manufacturer's download were put. A TPM without an EK certificate has none: the claim is an error claim and neither rule can pass.

EK certificates often bend the X.509 rules (negative serial numbers, unusual extensions) and Go's parser refuses some of them. Both rules then read the key, issuer, validity and signature directly from the DER, and the message says "read leniently" and why.

### tpm2_clockProgress

Runs on a quote or gettime claim and compares it with the earlier quote or gettime claim named by `previouscid`. The parameter is required: Jane never looks for a previous claim itself. Values come from the signed attest (or, for claims without one, from the decoded copy, and the message says so).

| Since the previous claim | Result |
|---|---|
| reset count lower | fail: TPM cleared, or a different TPM |
| same reset count, restart count lower | fail: TPM cleared, or a different TPM |
| clock lower, TPM reports safe | fail |
| clock lower, TPM reports not safe | fail: the clock was not saved before power was lost |
| otherwise | pass, reporting the resets (reboots) and restarts in between and how far the clock advanced |

The previous claim must have been requested earlier. In a jp provisioning file the previous claim is usually one from the same run (`previouscid: $q_pcr0`); to compare across sessions, run the rule from the attest page or REST `/verify` with an older claim's item ID.

### tpm2_capabilitiesFirmware

Runs on a `tpm2/capabilities` claim, which is not signed, and compares its `summary.firmwareversion` with the firmware version in the signed attest of the quote or gettime claim named by `quotecid`. The two are written differently by tarzan (`%08x%08x` and plain hex) and compared as numbers. This ties the reported TPM identity to the TPM that signs, as far as the firmware version goes. Run `tpm2_validSignature` on the quote too: this rule does not check the signature itself.

## UEFI

One rule per UEFI variable, each comparing the variable in a `uefi/efivars` claim with the expected value of type `uefiefivars` recorded by jp:

`UEFI_SecureBoot`, `UEFI_SetupMode`, `UEFI_AuditMode`, `UEFI_DeployedMode`, `UEFI_VendorKeys`, `UEFI_PK`, `UEFI_KEK`, `UEFI_db`, `UEFI_dbx`, `UEFI_MokSBStateRT`, `UEFI_SbatLevelRT`, `UEFI_MokListRT`, `UEFI_MokListXRT`, `UEFI_PKDefault`, `UEFI_KEKDefault`, `UEFI_dbDefault`, `UEFI_dbxDefault`.

The Rules page shows each one's description.

## System

| Rule | Claim | EV | Checks |
|---|---|---|---|
| `sys_taRunningSafely` | `sys/info` | | tarzan is not running with `-unsafe` |
| `sys_machineID_Agent` | `sys/info` | `machineid` | the machine ID reported by tarzan equals the expected value |
| `sys_machineID_CrossRef` | `sys/info` | | the machine ID reported by tarzan equals the element's `host.machineid` |

## RATSD

| Rule | Claim | Checks |
|---|---|---|
| `chares_validNonce` | `ratsd/chares` | the `eat_nonce` returned by ratsd equals the `ratsd/nonce` Jane sent (compared as bytes) |
| `chares_validAttesterNonces` | `ratsd/chares` | each leaf attester's evidence contains the nonce ratsd derived for it (TSM reports, NVIDIA GPU evidence) |

## Test and null rules

| Rule | Checks |
|---|---|
| `testcontainer_LIFE`, `testcontainer_43`, `testcontainer_FOOBAR` | fixed checks on the test container protocol's claims |
| `null_success`, `null_fail`, `null_noresult`, `null_verifycallfail`, `null_verifycallerrorattempt`, `null_missingEV`, `null_rulecallfailure`, `null_unsetresultvalue` | always return the result in their name, without looking at the claim; for testing result handling |
