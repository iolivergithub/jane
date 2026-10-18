# Standard Intents

The standard intents are a set of intents which are common across all installations and express typical scenarios. The file https://github.com/iolivergithub/jane/etc/standardintents/standardintents.json contains a standard list of common intents and instructions on loading these are on [intents](intents.md) page.

## List of Standard Intents

   * x86 UEFI CRTM only
   * x86 UEFI CRTM Firmware
   * x86 UEFI Bootloader Only
   * x86 UEFI SRTM
   * x86 Intel TXT
   * x86 Grub Full
   * Linux IMA with PCR 0
   * TPM 2.0 Credential Activation (`std::intent::tpm2::activatecredential`), checked with the rules `tpm2_credentialActivated` and `tpm2_akAttributes`
   * TPM 2.0 Time (`std::intent::tpm2::gettime`): signed TPM clock and reset/restart counters; no rules yet
   * TPM 2.0 Capabilities (`std::intent::tpm2::capabilities`): the TPM's identity and configuration, unsigned; no rules yet (#77)

After upgrading Jane, load the standard intents again (Intents, **Load standard intents**) so that new intents are known. If Jane cannot download them, the page says "Loading the standard intents failed, see log." and the reason is in Jane's log; the deb installs a copy matching the installed version in `/etc/opt/jane/standardintents.json`.
