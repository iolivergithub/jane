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