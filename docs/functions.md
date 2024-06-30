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


## uefi\eventlog
This returns the UEFI Eventlog.

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



