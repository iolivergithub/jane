# Register names

Jane has built-in names and descriptions for measurement registers (#101, since 1.5.6): TPM PCRs under several naming schemes, and the registers of confidential computing platforms. They are shown wherever Jane shows a PCR, so that a reader sees what a register means. The data is compiled into janeserver (`janeserver/registernames/registernames.json`); there is no database collection and nothing to download.

## Choosing the scheme for an element

An element names the scheme for its TPM's PCRs in its `tpm2` section:

```json
"tpm2": { "device": "/dev/tpmrm0", "ekcerthandle": "0x1C000002", "pcrnaming": "linux-uefi", ... }
```

jp takes it from `pcrnaming:` in the provisioning file's `tpm2:` section. Empty or unknown means the default, `tcg-pcclient-uefi`; an unknown key is pointed out on the claim pages. Claims keep a snapshot of the element, so a claim is shown with the scheme the element had when it was made.

## Schemes

| Key | Title | Platform | Registers | Reference |
|---|---|---|---|---|
| `tcg-pcclient-uefi` | TCG PC Client Platform Firmware Profile (UEFI) | tpm2 | 24 | [source](https://tianocore-docs.github.io/edk2-TrustedBootChain/release-1.00/3_TCG_Trusted_Boot_Chain_in_EDKII.html) |
| `linux-uefi` | Linux on UEFI (shim, GRUB, systemd, IMA) | tpm2 | 24 | [source](https://uapi-group.org/specifications/specs/linux_tpm_pcr_registry/) |
| `windows-bitlocker` | Windows BitLocker (UEFI) | tpm2 | 24 | [source](https://tianocore-docs.github.io/edk2-TrustedBootChain/release-1.00/4_Other_Trusted_Boot_Chains.html) |
| `coreboot-vboot` | coreboot with Google vboot | tpm2 | 24 | [source](https://tianocore-docs.github.io/edk2-TrustedBootChain/release-1.00/4_Other_Trusted_Boot_Chains.html) |
| `intel-tdx` | Intel TDX measurement registers | tdx | 8 | [source](https://www.intel.com/content/www/us/en/developer/tools/trust-domain-extensions/documentation.html) |
| `arm-cca` | Arm CCA realm measurements | cca | 6 | [source](https://datatracker.ietf.org/doc/html/draft-ffm-rats-cca-token-04) |
| `amd-sev-snp` | AMD SEV-SNP attestation report | sev-snp | 5 | [source](https://www.amd.com/en/developer/sev.html) |
| `intel-sgx` | Intel SGX enclave identity | sgx | 2 | [source](https://www.intel.com/content/www/us/en/developer/tools/software-guard-extensions/overview.html) |
| `aws-nitro` | AWS Nitro Enclaves PCRs | nitro | 6 | [source](https://docs.aws.amazon.com/enclaves/latest/user/set-up-attestation.html) |

- **`tcg-pcclient-uefi`** (default): the TCG PC Client Platform Firmware Profile as implemented by EDK II: PCRs 0-7 firmware, 8-15 operating system, 16 debug, 17-22 DRTM, 23 application.
- **`linux-uefi`**: PCRs 0-7 with what shim, GRUB, systemd-boot and systemd-stub add, and 8-15 as GRUB (8, 9), IMA (10), systemd (11-13, 15) and shim (14) use them, from the Linux TPM PCR Registry.
- **`windows-bitlocker`**: PCRs 0-7 as TCG, 8-10 reserved under UEFI (legacy BIOS: NTFS boot sector, NTFS boot block, boot manager), 11 BitLocker access control, 12 data and highly volatile events, 13 boot module details, 14 boot authorities.
- **`coreboot-vboot`**: coreboot with Google verified boot (PCRs 0-3).
- **`intel-tdx`**, **`arm-cca`**, **`amd-sev-snp`**, **`intel-sgx`**, **`aws-nitro`**: the measurement registers of those platforms (register names such as `RTMR2` or `MRENCLAVE`). Jane has no trust agent for them yet; the schemes are there for reference and future targets.

## Where the names appear

- **Web UI:** on `tpm2/pcrs` claims each PCR has its short name next to it, with the description on hover; `tpm2/quote` claims list the PCRs in the quote (the intent's `pcrSelection`) with their names. *Structures → Register names* (`/pcrnames`) lists the schemes; `/pcrname/<key>` shows one.
- **REST:** `GET /pcrnames` (every scheme and the default key), `GET /pcrname/<key>` (one scheme; 404 if unknown).
- **x3270:** menu option 10, *Register names*: the schemes and their registers.

## Changing the names

Edit `janeserver/registernames/registernames.json` and rebuild janeserver. Each scheme has `key`, `title`, `platform`, `description`, `reference` and `registers` (`register`, `short`, `long`); the unit tests check that keys are unique, the default exists and every register has a name.
