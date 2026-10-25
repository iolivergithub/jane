# Changelog

All notable changes to Jane are listed here, newest first. Days with several
commits are grouped into a single entry. Versions before 1.0 are assigned
retrospectively.

## [1.5.5] – 2026-10-25
- **REST API now uses API keys, not passwords (#89). This is a breaking change: Jane, jp and any scripts calling the REST API must be updated together.** With `auth: rest: true` a client sends `Authorization: Bearer jane_...`; HTTP Basic gets 401 with a message saying so. Keys are made with `addjaneuser --apikey` (printed once; only a SHA-256 hash is stored) and removed with `--revokeapikey`. jp reads its key from `JANE_API_KEY` or `--apikeyfile` (e.g. `/etc/opt/jane/jp.key`, root 600); `JANE_USERNAME`, `JANE_PASSWORD`, `--user` and `--pass` are removed and refused with a message. See `docs/authentication.md` and `docs/jp.md`.
- Failed attempts are counted per client address across the REST API, the Web UI and x3270: after 5 failures in 5 minutes the address is blocked for 5 minutes (REST answers 429 with `Retry-After`). Password checks (Argon2id) are capped at two at a time, so a burst of logins cannot exhaust memory. Successful key use is logged once an hour per user and address; refusals are logged on the `AUTH` channel. The Web UI home page shows how many API keys exist.
- Sessions: claims and results are added atomically and only while the session is open; adding to a closed session fails with a clear error, and the attest and verify calls say the claim or result was stored but not added (#90). Closing signs exactly the lists that are stored, and refuses (try again) if the session changed while it was being closed (#108).
- Sessions open longer than `sessions: longsession` seconds (default 60, minimum 30) are flagged LONG SESSION in the Web UI and x3270, and can be closed there (Web UI Close button; x3270 F9) - e.g. a session left open by a crashed client (#98). A forced close is the normal signed close, plus a `close/FORCED` log entry naming who did it.
- tarzan: `tpm2/capabilities` also reports the commands the TPM implements (`commands`, e.g. `TPM_CC_Quote (0x158)`), shown in a collapsed section of the claim page (#103).
- Web UI: every page has its own title; "Attest" on an element page opens the attest page with that element selected (#110, #111); a light/dark theme switch in the navbar, remembered per browser (#112); copy buttons next to item IDs (#113).
- **deb: systemd units moved to `/lib/systemd/system` and are no longer conffiles (#96).** On upgrade from 1.5.4 or earlier, a locally edited `/etc/systemd/system/jane.service` or `tarzan.service` is moved to `/etc/opt/jane/*.service.pre-1.5.5` with a warning; put local changes in a drop-in (`systemctl edit jane`). Masks are kept. **Upgrading never restarts Jane**: the installer says whether the old version is still running; tarzan is restarted. The install checks that the jane user can read the certificates and keys and write the log named in `config.yaml`, and reports problems; ownership is applied to subdirectories of `/etc/opt/jane` too.
- Removed: the Veraison PSA rule and test protocol (#95); `etc/tpmsetup`, `etc/scripts`, `etc/experimental` and `etc/deprecated` (#97); `tarzan/Makefile.arm6` and `x3270/initx3270.old` (#114).
- Fixes: the x3270 start-up message, doc comments and typos (#106, #107, #109).
- Documentation: new Architecture page with component and data model diagrams. Obsolete docs removed (PR #117, thanks to chinmayee5599) and typos fixed across messages, the build script and docs (PR #119, thanks to SatriaBagusNurhadianto).

## [1.5.4] – 2026-10-20
- New rule `tpm2_validSignature` (on `tpm2/quote` and `tpm2/gettime` claims): verifies the TPM's signature over the raw attestation with the element's AK (or `tpm2/akpublic`), and that the attestation type matches the function (#83). Needs tarzan 1.5.3 or later; RSA AKs.
- New rule `tpm2_clockProgress` (on quote or gettime claims, required parameter `previouscid`): compares the TPM clock, reset and restart counts with an earlier claim; fails if a count went down (TPM cleared or replaced) or the clock went backwards, otherwise reports the reboots in between.
- New rule `tpm2_capabilitiesFirmware` (on `tpm2/capabilities` claims, parameter `quotecid`): the reported firmware version equals the signed one in a quote or gettime claim (#77).
- The provisioning example and skeletons run these rules (the pcr0 quote gets `cid: q_pcr0`).
- MongoDB and MQTT accounts from environment variables `JANE_MONGODB_USERNAME`, `JANE_MONGODB_PASSWORD`, `JANE_MONGODB_AUTHSOURCE`, `JANE_MQTT_USERNAME`, `JANE_MQTT_PASSWORD` (#79, #80); unset means no account, as before. The deb ships `/etc/opt/jane/jane.env` (root:jane 640, commented out). Works with HashiCorp Vault Agent.
- addjaneuser reads the same MongoDB variables; `--dbuser` and `--dbpass` (together) override them.
- jp sends REST credentials from `JANE_USERNAME`/`JANE_PASSWORD` or `--user`/`--pass` (together), so it works with `auth: rest: true` (#46). Failed REST logins are logged on the `AUTH` channel.
- **jp never changes the TPM unless asked:** `tpmclear` and `tpmprovision` in the worklist run only for `create`/`update` with the new `--reprovisiontpm`; otherwise they are skipped and reported. `attest` never touches the TPM. jp no longer asks questions, so `update` and `attest` can run unattended. **`-u`/`--unsafe` is removed: scripts using it must be changed** (`jp create --reprovisiontpm` for a first registration; plain `jp update`). New `docs/jp.md` explains the modes and flags.
- jp: `create` and `update` stop before contacting Jane if they cannot write the element ID file (no root); an unreadable ID file is reported as "permission denied", not missing; `attest` needs no root.
- TPM endorsement hierarchy password from `JANE_TPM_ENDORSEMENT_PASSWORD` for tarzan (credential activation; `/etc/opt/jane/tarzan.env`, root 600) and jp (#76). `tpm2/gettime` cannot use it (a TPM library limitation) and fails while one is set.
- **Web UI login and x3270 idle timeout now 30 minutes** (was 8 hours). An edited `config.yaml` kept on upgrade keeps its value.
- Logging: the database write times out after 5 s instead of waiting forever; failures are reported on stderr; new log files are created 640 (#65).
- **Keylime and MarbleRun removed** (#91), with the element fields `mrcoordinator`/`mrmarbleinstance`, `enclave.json` and the EGo/SGX dependency: janeserver now builds, vets and tests on any platform. Existing `config.yaml` files with a `keylime:` section still load. This also drops `go-jose`, the subject of a Dependabot alert.
- deb: `apt remove` stops and disables the service (it used to keep restarting a missing binary); upgrades restart tarzan and a running Jane; file modes are set inside the packages; `/etc/opt/jane` is 751; an existing ordinary user called `jane` stops the install; calm install messages with documentation links, and a hint when MongoDB or Mosquitto is missing; jane recommends `mosquitto`.
- deb: `jane.service` is sandboxed (no capabilities, read-only file system except `/var/log/jane`, ...) and starts after MongoDB and Mosquitto; both units restart after 5 s.
- deb: tarzan starts with `-sys -tpm2 -uefi -ima`, so the provisioning example works out of the box. The provisioning example's ratsd endpoint is commented out (spelling fixed) and `tpmclear` is explained.
- Web UI: every script and stylesheet loaded from a CDN carries a Subresource Integrity hash (Font Awesome was missing one), and a test fails if a template adds an external resource without one (#92).
- Documentation reorganised: Quick start → Getting started → Hardening, then reference pages (new: rules, claims, results, sessions, signing, protocols, trust agents). `automaticStartup.md` and `security.md` are folded in; screenshots are to be retaken. Help & About links to the Quick start and Hardening.

## [1.5.3] – 2026-10-18
- UEFI event log claims are decoded and shown as a list of events (type, PCR, digest, light decoding of UEFI variables, image paths and text), one bank at a time with a PCR filter (#73). Parsing uses Google's go-eventlog.
- New rule `tpm2_uefiLogReplay` (on a `tpm2/pcrs` claim, parameter `uefilogcid`): replays the UEFI event log against the PCR values, per bank (#71).
- New rule `tpm2_pcrDigest` (on a `tpm2/quote` claim, parameter `pcrscid`): checks the PCR values in a `tpm2/pcrs` claim give the quote's attested value (#70). Together: event log → PCRs → quote.
- New intent functions and tarzan endpoints `tpm2/gettime` (signed TPM clock, reset and restart counters, #68) and `tpm2/capabilities` (TPM identity and configuration, unsigned, #69), with standard intents `std::intent::tpm2::gettime` and `std::intent::tpm2::capabilities` and claim pages. Quotes also carry the raw attestation and signature (`attest`, `rawsignature`).
- jp: variables in the provisioning file (`cid` on an intent, `rid` and `resvar` on a rule, `$name` in rule parameters) so `jp attest` can pass claim itemids to rules; a table of the variables is printed at the end; mistakes stop jp with exit code 2 before anything runs (#72).
- jp: `create` and `update` no longer run rules - run `jp attest` afterwards. `processevs_withrules` is a deprecated alias of `processevs`.
- jp: the element ID file moved to `/etc/opt/jane/janeelementid` and is read-only; `update` and `attest` copy an existing `/etc/janeelementid` there automatically.
- Standard intents: the download verifies TLS certificates and times out; failures are logged and the intents page says "Loading the standard intents failed, see log." The deb installs a copy in `/etc/opt/jane/standardintents.json` (#67). **After upgrading, load the standard intents again** so the new intents are known.
- deb: consistent ownership and permissions - binaries and configuration owned by root, configuration and keys readable by group `jane`, the log owned by `jane` (#78).
- Quote claim page: the Safe badge now shows Unsafe when it should.
- Web UI handlers no longer print debug output; errors they reported only on stdout are now logged.
- Removed for now: Docker support (Dockerfile, compose files and docs; to return in a future release). `aijane` moved to `etc/experimental`. Removed committed binaries and unused files.

## [1.5.2] – 2026-10-14
- New `tpm2/activatecredential` intent function (`std::intent::tpm2::activatecredential`) and tarzan endpoint: Jane makes a credential for the element's AK and EK and the TPM must recover it, showing the AK is in the same TPM as the EK (#49). Keys come from the element and can be overridden.
- New rules `tpm2_credentialActivated` and `tpm2_akAttributes`.
- Provisioning skeletons and the deb example include the new intent.
- A10HTTPRESTv2 error claims show the trust agent's error text instead of a byte list.

## [1.5.1] – 2026-10-14
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
