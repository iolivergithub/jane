# Changelog

All notable changes to Jane are listed here, newest first. Days with several
commits are grouped into a single entry. Versions before 1.0 are assigned
retrospectively.

## [1.5.1] – unreleased
- x3270 logon (`auth: x3270`), with an ASCII-art logon screen, a status line on every screen, L (logoff) and X (exit) on the menu, and F3/F12 no longer disconnecting from the menu.
- `listenon` is now honoured for REST, Web UI and the new `x3270: listenon`; REST no longer reads the Web UI's value.
- Attest page lists sorted case-insensitively, with filter boxes, rules grouped by type and a selected-rule count.
- Element page history matrix tighter, with collapsible rule groups showing the worst result per session.
- Elements page result summaries are thin bars instead of squares.
- deb packages: Jane runs as a `jane` system user; `addjaneuser` installed as `/opt/jane/addjaneuser`; complete example `config.yaml`; up-to-date `provisioning.yaml.example`.
- Builds no longer run `go get -u`; the `jp` binary is no longer in the repository.
- New authentication guide; security, running, deb and compiling docs updated.

## [1.5] – 2026-10-13
- Footer signing with key id, algorithm, canonical hash and a configurable key.
- UEFI variables in `uefi/efivars` claims now render correctly.
- New `UEFI_` rules checking UEFI variables against an expected value (fail when a variable is missing).
- Provisioner: expected values for UEFI efivars; `jp create` refuses to re-register a known machine.
- Updated deb package configuration.

## [1.4] – 2026-10-12
- RATSD protocol supports subattesters and chares intents; RATSD test intent replaced with standard ones.
- New chares rules: `chares_validNonce` and `chares_validAttesterNonces`.
- Attest page gains an intent and rule parameter reference.

## [1.3] – 2026-10-12
- Login and authorisation for the Web UI, with optional REST basic auth.
- User passwords stored as Argon2id hashes; `etc/addjaneuser` added.
- Reworked home page (count tiles, warning when no intents are loaded, Authentication tab), logged-in user shown in the navbar.
- Version, build and run session shown; About, History and Help merged into "Help and About"; updated disclaimer and licence link.
- Live monitor connects to the correct broker host, explains failures, formats log entries and shows 3-hour statistics.
- Element page timeline charts removed; rules, protocols, opaque object and log pages tidied.
- Build flags fixed across janeserver, tarzan and provisioner so `BUILD` and `VERSION` are set correctly.

## [1.2] – 2026-10-09
- Web UI redesign: elements as a card grid with recent results; element page with session history and tabbed lists (capped at 100); reworked sessions, claims, results, intents and expected value pages.
- New x3270 text interface: screens for elements, intents, expected values, sessions, claims, results, attest, log and configuration.
- Fixes to deb package naming.

## [1.1.1] – 2026-10-06
- Working deb generation script.

## [1.1] – 2026-10-05
- Reworked deb builder with version numbering.

## [1.0.5] – 2026-10-01
- Live page, with Mosquitto configuration parameters for websockets.
- Added `aijane`.
- Debbuild refactored; README installation and usage steps revised.

## [1.0.4] – 2026-09-30
- Attest mode added to the provisioner (`jp`).

## [1.0.3] – 2026-09-29
- Provisioner ported from Python to Go.

## [1.0.2] – 2026-02-05
- Dependency on Marblerun removed; Jane now compiles and runs on any platform.
- Documentation added.

## [1.0.1] – 2025-12-10
- Dockerfile fixed.

## [1.0] – 2025-10-16
- First binary release.
- REST API returns expected values correctly; provisioner safety checks.
- Provisioner build split from the rest of the debbuild scripts.

## [0.9.16] – 2025-09-22 to 2025-09-25
- Archiving structure recording the history of elements and expected values.
- Fixed provisioner zipapp generation and machine-id comparison in the sys rules.

## [0.9.15] – 2025-09-04 to 2025-09-20
- Provisioner added (Python zipapp), with debbuild support.
- REST API: get elements by tag.
- Graceful shutdown.

## [0.9.14] – 2025-08-28 to 2025-08-29
- RATSD protocol: working nonce for the chares call.

## [0.9.13.1] – 2025-07-29 to 2025-08-01
- TPM2 Quote fixed, PCR selection error checking, base64 fix in the TPM2 rules, nil-error crash in `GetExpectedValues` fixed.
- Session page shows duration.

## [0.9.13] – 2025-07-22
- Attestation of `/etc/machine-id`.

## [0.9.12.1] – 2025-07-17
- gzip middleware removed from the Web UI to ease nginx URL rewriting.

## [0.9.12] – 2025-04-16 to 2025-06-03
- rima added, with its own database, an extra endpoint and deb installation; later refactored.

## [0.9.11] – 2025-03-28 to 2025-04-13
- TPM2 Quote support with PCR selections, displayed in the Web UI.
- Attestation rules for TPM2 quotes.
- Tarzan PCR collector handles missing banks (e.g. SHA384/512).

## [0.9.10] – 2025-01-31 to 2025-03-27
- New PCR handling.

## [0.9.9] – 2025-01-15
- RATSD support, with standard intents for testing.

## [0.9.8] – 2025-01-04 to 2025-01-07
- LCARS-style interface, stardate calculation and XML output from the REST API.
- Shell scripts updated for new schemas.

## [0.9.7] – 2024-12-16 to 2024-12-18
- TPM test tooling (EK, AK, evictcontrol) and experimental Tantor agent with TPM provisioning.

## [0.9.6] – 2024-12-11
- Multiple endpoints per element.

## [0.9.5] – 2024-12-01 to 2024-12-07
- Deb packaging (debbuild), RPMs via alien, Makefiles; go vet clean-up.

## [0.9.4] – 2024-11-29
- UEFI efivars collection; standard intent loading refactored.

## [0.9.3] – 2024-11-19 to 2024-11-22
- PCR digests and expected values use base64 throughout.
- More help pages, updated claim page, attest-and-verify scripting.
- Opaque object descriptions for PCRs and firmware quotes.

## [0.9.2] – 2024-11-10
- New expected values page; opaque objects.

## [0.9.1] – 2024-11-07 to 2024-11-09
- New element form; loading of standard intents, with more intents added.

## [0.9.0] – 2024-06-30
- Initial upload; trust agent renamed from ta10 to tarzan (2024-07-06).
