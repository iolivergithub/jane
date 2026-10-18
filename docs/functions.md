# Functions

This document lists all the currently implemented functions that invoke some attestaton operation on an element.

## tpm2\quote
This function requests a TPM 2.0 quote.


| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| bank | Yes |String | Specifies the particular bank to take the PCR measurements from, eg: sha1, sha256 |
| pcrSelection | Yes | String | A comma separated list as a string of PCR register numbers ||
| tpm2/nonce | | String | A nonce used in the quote for prevention of reply attacks |
| tpm2/akhandle | Y | String | The handle of the attestation signing key |
| tpm2/device | Y | String | The TPM device to read from |

Note, some parameters may be set in the element defintion or overridden by any explicitly supplied parameters.

On Windows machines the tpm2/device parameter has no effect as the operating system decides which device to use.

The allowed banks are sha1, sha256, sha384 and sha512. Not all TPMs support all banks.

## tpm2\pcrs

The function returns the current state of all PCR banks on a TPM 2.0 device.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| tpm2/device | Y | String | The TPM device to read from |

Note, some parameters may be set in the element defintion or overridden by any explicitly supplied parameters.

On Windows machines the tpm2/device parameter has no effect as the operating system decides which device to use.

## tpm2\activatecredential

Checks that the attestation key (AK) is held in the same TPM as the endorsement key (EK), using TPM2_MakeCredential and TPM2_ActivateCredential.

Jane makes a credential in software: a fresh random 32-byte secret, encrypted to the EK and bound to the AK's name. The trust agent asks its TPM to recover the secret. The TPM only does so if it holds the EK's private key and the object at the AK handle has exactly that name. The secret is never stored or sent: Jane keeps only its SHA-256, and the `tpm2_credentialActivated` rule compares that with the hash of the secret the trust agent returned. `tpm2_akAttributes` checks the AK is a restricted signing key that cannot leave its TPM, which activation itself does not check.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| tpm2/device | Y | String | The TPM device to use, from the element's `tpm2.device` |
| tpm2/akhandle | Y | String | Handle of the AK, from the element's `tpm2.ak.handle` |
| tpm2/ekhandle | Y | String | Handle of the EK, from the element's `tpm2.ek.handle` |
| tpm2/akpublic | Y | String | The AK's public area (base64 TPM2B_PUBLIC), from the element's `tpm2.ak.public`; the credential is made for this key's name |
| tpm2/ekpublic | Y | String | The EK's public area (base64 TPM2B_PUBLIC), from the element's `tpm2.ek.public`; the credential is encrypted to this key |
| tpm2/credentialblob | | String | Set by Jane: the credential (TPM2B_ID_OBJECT contents, base64) |
| tpm2/encryptedsecret | | String | Set by Jane: the encrypted seed (TPM2B_ENCRYPTED_SECRET contents, base64) |
| tpm2/credentialsecrethash | | String | Set by Jane: SHA-256 of the secret, hex |

Normally all of these come from the element, as written by the provisioner (`jp create` / `jp update`); an element without `tpm2.ak.public` and `tpm2.ek.public` gives an error claim. Any of them can be overridden by the intent or the caller, and the credential is made after the overrides are applied. Overriding only a handle checks whether the key at that handle is the element's AK or EK.

The EK must use the standard EK policy (PolicySecret on the endorsement hierarchy with the empty endorsement password), as the provisioner creates it.

The claim body contains `certinfo` (the recovered secret, base64) and `akname`, `ekname` (the names of the keys the trust agent used, hex).

Activation shows the AK and EK are in the same TPM. It does not show the EK belongs to a genuine TPM; that needs the EK certificate checked against the manufacturer (#63).

## tpm2\gettime

Runs TPM2_GetTime: the TPM's clock, reset and restart counters, `safe` flag, time and firmware version, signed by the AK (RSASSA/SHA-256) and qualified by a fresh nonce, like a quote. No PCRs are involved, so it is cheap to run often; across claims it shows reboots (reset count), resumes (restart count) and whether the TPM clock runs forwards.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| tpm2/device | Y | String | The TPM device to use, from the element's `tpm2.device` |
| tpm2/akhandle | Y | String | Handle of the AK, from the element's `tpm2.ak.handle` |
| tpm2/nonce | | String | Set by Jane: a fresh nonce, base64 |

The endorsement hierarchy is authorised with the empty password (#76). The claim body has `time` (magic, type `8019`, qualifiedsigner, extradata, firmwareVersion, clockinfo, time), `signature`, and the raw `attest` (TPMS_ATTEST) and `rawsignature` (TPMT_SIGNATURE), base64, for verifying the signature (#62). Quotes now carry `attest` and `rawsignature` too.

## tpm2\capabilities

Runs TPM2_GetCapability and reports which TPM the element has and how it is configured. Nothing is signed, so it describes rather than proves.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| tpm2/device | Y | String | The TPM device to use, from the element's `tpm2.device` |

The claim body has `summary` (manufacturer, vendorstring, firmwareversion, family, level, revision, fips140_2), `properties` (every fixed and variable TPM property by its TCG name, e.g. `TPM_PT_MANUFACTURER`), `pcrbanks` (bank: number of PCRs allocated), `algorithms`, `ecccurves` and `persistenthandles`.

## uefi\eventlog
This returns the UEFI Eventlog. The web UI decodes it into a list of events, one bank at a time; the `tpm2_uefiLogReplay` rule checks it against a `tpm2/pcrs` claim.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| location | Y | String | The name of the file containing the eventlog |

Note: because of the security implications of returning files, this parameter may be overriden by the trust agent.

The default location of the UEFI eventlog is: /sys/kernel/security/tpm0/binary_bios_measurements

## ima\asciilog
This returns the IMA ASCII Log.

| Field | Compulsory | Type | Description |
| --- | --- | --- | --- |
| location | Y | String | The name of the file containing the ascii log |

Note: because of the security implications of returning files, this parameter may be overriden by the trust agent.

The default location of the UEFI eventlog is: /sys/kernel/security/ima/ascii_runtime_measurements

## txt\log

## sys\info
Returns some relevant information about the system running the trust agent.

Takes no parameters but returns a structure containing

   * The operating system name
   * The CPU architecture
   * The number of CPUs (this may vary: cores, vCPUs, physical CPUs)
   * The hostname
   * The process ID of the trust agent
   * The parent process ID of the trust agent

The contents will vary by operating system.



