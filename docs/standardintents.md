# Standard Intents

The standard intents are a set of intents which are common across all installations and express typical scenarios. The file https://github.com/iolivergithub/jane/blob/main/etc/standardintents/standardintents.json contains a standard list of common intents and instructions on loading these are on [intents](intents.md) page.

## List of Standard Intents

| Item ID | Name | Function | Rules usually run |
| --- | --- | --- | --- |
| `std::intent::sys::info` | System Information | sys/info | `sys_taRunningSafely`, `sys_machineID_Agent`, `sys_machineID_CrossRef` |
| `std::intent::tpm::pcrs` | TPM 2.0 PCRs | tpm2/pcrs | `tpm2_uefiLogReplay` |
| `std::intent::sha256::crtm::pcr0` | x86 UEFI CRTM only | tpm2/quote | quote rules, `tpm2_validSignature`, `tpm2_pcrDigest` |
| `std::intent::sha256::crtm::firmware` | x86 UEFI CRTM Firmware | tpm2/quote | as above |
| `std::intent::sha256::crtm::bootloaderonly` | x86 UEFI Bootloader Only | tpm2/quote | as above |
| `std::intent::sha256::crtm::crtmplusbootloader` | x86 UEFI Bootloader and CRTM | tpm2/quote | as above |
| `std::intent::sha256::crtm::srtmbootloader` | x86 UEFI SRTM and Bootloader | tpm2/quote | as above |
| `std::intent::sha256::srtm` | x86 UEFI SRTM | tpm2/quote | as above |
| `std::intent::sha256::txt` | x86 Intel TXT | tpm2/quote | as above |
| `std::intent::sha256::grubfull` | x86 Grub Full | tpm2/quote | as above |
| `std::intent::sha256::linuxIMA` | Linux IMA with PCR 0 | tpm2/quote | as above |
| `std::intent::tpm2::gettime` | TPM 2.0 Time | tpm2/gettime | `tpm2_validSignature`, `tpm2_clockProgress` |
| `std::intent::tpm2::capabilities` | TPM 2.0 Capabilities | tpm2/capabilities | `tpm2_capabilitiesFirmware` |
| `std::intent::tpm2::getcert` | TPM 2.0 EK Certificate | tpm2/getcert | `tpm2_ekCertMatchesEK`, `tpm2_ekCertIssuer` |
| `std::intent::tpm2::activatecredential` | TPM 2.0 Credential Activation | tpm2/activatecredential | `tpm2_credentialActivated`, `tpm2_akAttributes` |
| `std::intent::uefi::eventlog` | UEFI Eventlog | uefi/eventlog | (used by `tpm2_uefiLogReplay`) |
| `std::intent::uefi::efivars` | UEFI EFI Variables | uefi/efivars | the `UEFI_*` rules |
| `std::intent::uefi::bootconfig` | UEFI Boot Configuration | uefi/bootconfig | |
| `std::intent::linux::ima::asciilog` | Linux IMA ASCII Log | ima/asciilog | |
| `std::intent::ratsd::subattesters` | RATSD Subattesters | ratsd/subattesters | |
| `std::intent::ratsd::chares` | RATSD CMC | ratsd/chares | `chares_validNonce`, `chares_validAttesterNonces` |

"Quote rules" are `tpm2_attestedValue`, `tpm2_firmware`, `tpm2_magicNumber`, `tpm2_safe` and `tpm2_validNonce`. The jp provisioning example runs these combinations; see [Rules](rules.md).

After upgrading Jane, load the standard intents again (Intents, **Load standard intents**) so that new intents are known. If Jane cannot download them, the page says "Loading the standard intents failed, see log." and the reason is in Jane's log; the deb installs a copy matching the installed version in `/etc/opt/jane/standardintents.json`.
