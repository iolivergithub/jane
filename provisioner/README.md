# jp — Jane element provisioner (Go)

User documentation: [docs/jp.md](../docs/jp.md) (modes, flags, when to use them). This README covers the provisioning file format in detail and the implementation.

Go port of `provisioner/jp.py`. It registers the machine as an element with a
Jane attestation server, can clear and provision the TPM, and records
expected values for the intents listed in the provisioning file. It can
then attest the registered element, running the rules listed for each
intent. The provisioning file format is unchanged; see the skeletons in
`skeleton/` (`full.yaml`, `full-vm.yaml`, `tpm2-nouefi.yaml`, `sysonly.yaml`;
[docs/jp.md](../docs/jp.md#skeletons)) and the tests' fixture
`testdata/examplecreateprovisioningfile.yaml`.

TPM operations use [go-tpm](https://github.com/google/go-tpm) directly
(package `jp/tpm`), so tpm2-tools is not needed. `jp` builds without cgo for
Linux, Windows, macOS, the BSDs, illumos/Solaris and AIX.

## Build and run

    go build -o jp .                          # or: CGO_ENABLED=0 GOOS=windows go build
    sudo ./jp create --reprovisiontpm provisioning.yaml   # first time: new EK and AK
    sudo ./jp update provisioning.yaml        # leaves the TPM alone; safe to automate
    ./jp attest provisioning.yaml             # attest the registered element (no root needed)

`create` and `update` need root (Administrator on Windows) for the TPM, the
element ID file and the machine ID; without write access to the element ID
file's directory they stop before contacting Jane. `attest` only reads the
element ID file and talks to Jane, so any user can run it. An element ID file
that exists but cannot be read is reported as "permission denied", not as
missing.

### Credentials

If Jane's REST API requires an API key (`auth: rest: true`), jp sends it as
`Authorization: Bearer <key>` on every request, from `JANE_API_KEY` or from
the file given with `--apikeyfile PATH` (the key alone; it overrides the
variable). Keys are made with `addjaneuser --apikey` and look like `jane_`
plus 43 characters; jp refuses anything else before contacting Jane. `sudo`
drops the environment, so use `sudo -E` (or `--preserve-env=JANE_API_KEY`) or
`--apikeyfile` for `create` and `update`. HTTP 401 and 429 answers give a
message saying what to do. The old `--user`/`--pass` and
`JANE_USERNAME`/`JANE_PASSWORD` (1.5.4) are refused with a message. For a
Jane on HTTPS with a self-signed certificate, set `SSL_CERT_FILE` to it
(Linux/BSD).

If the TPM's endorsement hierarchy has a password, set
`JANE_TPM_ENDORSEMENT_PASSWORD` (the same variable tarzan reads); if its
owner hierarchy has one, set `JANE_TPM_OWNER_PASSWORD` (jp only).

| | Unix-likes | Windows |
|---|---|---|
| TPM | `tpm2.device`, default `/dev/tpmrm0` then `/dev/tpm0` | TPM Base Services |
| Element ID file | `/etc/opt/jane/janeelementid` | `%ProgramData%\jane\janeelementid` |
| Machine ID | `/etc/machine-id`, `/var/lib/dbus/machine-id`, `/etc/hostid`; on BSD/macOS then `kern.hostuuid` / `kern.uuid` | registry `MachineGuid` |
| Key files | `$TMPDIR` or `/tmp` | `%TEMP%` |

The element ID file is written read-only (`-r--r--r--`) by `create`, and is
not rewritten by `update` unless the ID changed (only `create --recreate`
replaces it). Before 1.5.3 it was `/etc/janeelementid`; jp 1.5.3 to 1.5.5
moved it, and since 1.5.6 the old location is no longer looked at. A machine
that still has only `/etc/janeelementid` must have it moved by hand (or be
run once with jp 1.5.5).

## Operations

- `create` runs `provisionworklist` and registers a new element, writing its
  ID to the element ID file. It is for the first provisioning only: before
  anything else (in particular before `tpmclear`/`tpmprovision`) it refuses,
  exit 1, if the machine already looks registered: the element ID file exists,
  or Jane already holds an element with the same `element.name` or the same
  machine ID. It also refuses if Jane cannot be asked. `--recreate` lifts
  this: it registers a new
  element and leaves the old one in Jane, whose TPM keys stop matching if the
  worklist re-provisions the TPM. A cloned VM that kept its source's
  `/etc/machine-id` is refused too; give it a new machine ID.
- `update` runs `provisionworklist` and updates the element whose ID is in the
  element ID file, replacing its expected values. Run it after anything that
  changes what is measured, e.g. a firmware update (fwupd) or a shim, SBAT or
  kernel update (apt).
- `create` and `update` do not run rules (since 1.5.3): run `jp attest`
  afterwards to check the element. The old worklist step
  `processevs_withrules` was removed in 1.5.6: it is now reported as
  "Unknown provision work command" and no expected values are recorded, so
  use `processevs`.
- `attest` attests the element whose ID is in the element ID file against
  every intent under `evs`, and verifies every rule listed for each, in one
  Jane session. It does not run `provisionworklist`, touch the TPM, or change
  the element or its expected values, so it is safe to run repeatedly (from
  cron or a systemd timer, say). An intent that cannot be attested or a rule
  that does not pass is reported and the run carries on; it ends with a
  summary:

      std::intent::sha256::crtm::pcr0 on tarzan  (claim C1)
        PASS   tpm2_attestedValue
        FAIL   tpm2_firmware: 9001 Fail
      ...
      Attestation FAILED: 15 rules passed, 3 failed, 1 of 7 intents could not be attested

Exit status: 0 success (for `attest`, every rule passed); 1 error (bad
provisioning file, missing element ID file, Jane server or TPM unreachable);
2 bad command line, or a mistake in the provisioning file's variables (below);
3 `attest` completed but a rule did not pass, a rule could not be run or an
intent could not be attested.

## Variables: passing claims to rules

Some rules check one claim against another and take that claim's itemid as a
rule parameter, e.g. `tpm2_pcrDigest` (`pcrscid`, a `tpm2/pcrs` claim) and
`tpm2_uefiLogReplay` (`uefilogcid`, a `uefi/eventlog` claim). Variables let
`jp attest` pass them automatically:

```yaml
evs:
  - std::intent::uefi::eventlog :
      protocol: tarzan
      cid: uefilog                 # variable: this intent's claim itemid
  - std::intent::tpm::pcrs :
      protocol: tarzan
      cid: pcrs
      rules:
        - tpm2_uefiLogReplay:
            rid: r_uefilog         # variable: the result itemid
            resvar: r_uefilog_code # variable: the result code, e.g. "0 Success"
            parameters: { "uefilogcid": $uefilog }
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      rules:
        - tpm2_attestedValue
        - tpm2_pcrDigest:
            parameters: { "pcrscid": $pcrs }
```

- `cid` (on an intent), `rid` and `resvar` (on a rule) are optional; a rule
  without options is written as a plain name, as before.
- `parameters` is a mapping, usually written as a JSON-style box. A value
  that is exactly `$name` is replaced by that variable (quoted `"$pcrs"` is
  the same); nothing is substituted inside longer strings, and there are no
  type checks (Jane reports a wrong itemid).
- Names are letters, digits and `_`, not starting with a digit. Before
  anything runs, `jp` stops with exit code 2 if a name is defined twice
  (`cid`, `rid` and `resvar` share one namespace), if `$name` refers to a
  variable that is not defined earlier in the file (an intent's own `cid`
  counts for its rules), or if `parameters` is text rather than a mapping.
- If the step that sets a variable failed (e.g. its intent could not be
  attested), a rule using it is reported `NOTRUN` and counts as failed.
- Variables only exist in `jp attest`, for one run. At the end it prints them:

      var                  | kind   | value
      uefilog              | cid    | 6f1c…
      r_uefilog            | rid    | 8a2e…
      r_uefilog_code       | resvar | 0 Success

Collect the event log, the PCRs and the quotes in the same run: a PCR
extended in between (e.g. PCR 10 with IMA) makes `tpm2_pcrDigest` fail
without any tampering.

## Expected value types

An `evs` entry's `type` says how its claim becomes an expected value for the
element, intent and endpoint (created by `create`, replaced by `update`):

- `sysmachineid`: the claim's `machineid`, for the `sys_machineID_` rules.
- `tpm2quote`: the quote's PCR digest and firmware version, for the `tpm2_`
  rules.
- `uefiefivars`: one entry per variable checked by the `UEFI_` rules
  (`janeserver/rules/uefirules`), matched by name and vendor GUID: the value
  as hex if it is at most 32 bytes, otherwise `sha256:<hex>`, or `absent` if
  the variable is not in the claim (tarzan sends every variable efivarfs
  lists). A variable the agent could not read is left out with a warning, so
  its rule fails until it can be read. A claim with no variables at all (the
  agent could not list efivarfs) or from an agent too old to send `rawvalue`
  is refused rather than recorded. The variable list in `uefi.go` must be kept
  in step with janeserver's.

An entry without a `type` is attested but records nothing.

## TPM device

`tpm2.device` in the provisioning file may be:

- a character device, e.g. `/dev/tpmrm0` (the resource manager is
  recommended); empty means the default above
- `unix:///path` or a path to a socket: a simulator such as
  `swtpm socket --tpm2 --server type=unixio,path=/path`
- `tcp://host:port`: a simulator speaking the Microsoft simulator protocol
  (ms-tpm-20-ref, or `swtpm socket --tpm2 --server type=tcp,port=2321
  --ctrl type=tcp,port=2322 --flags startup-clear`), with the platform port
  at port+1. It must already be powered on and started.

On Windows a device path is ignored (TBS is always used unless `tcp://` is
given), so one provisioning file can serve both platforms.

## TPM worklist steps

`tpmclear` and `tpmprovision` change the TPM, so they run only for `create`
or `update` with `--reprovisiontpm`; otherwise they are skipped (and reported),
leaving the TPM as it is. `attest` never runs them and never opens the TPM.
jp asks no questions, and there is no `-u`/`--unsafe` any more (removed in
1.5.4; scripts using it must drop it). This lets `jp update` and `jp attest` run unattended (cron, package
hooks) without any risk to the TPM keys.


- `tpmclear` evicts the persistent objects at `tpm2.ek.handle` and
  `tpm2.ak.handle` (not TPM2_Clear: the rest of the TPM is untouched). An
  empty handle is reported, not an error.
- `tpmprovision` creates the TCG default RSA-2048 EK (`tpm2.RSAEKTemplate`)
  in the endorsement hierarchy and an RSA-2048 restricted signing AK
  (RSASSA, SHA-256) under it, persists both at the configured handles, writes
  `ek.pub` (TPM2B_PUBLIC), `ak.pub` (PEM) and `ak.name` to the temp directory,
  and lists persistent handles. This matches the old `tpm2_createek -G rsa`
  and `tpm2_createak -G rsa -g sha256 -s rsassa -f pem` calls. A handle that
  is already in use is an error, so put `tpmclear` first to re-provision.
- `collecttpm2` sends the `tpm2` section and now also fills in
  `ek`/`ak` `public` (base64 TPM2B_PUBLIC, as janeserver's
  `utilities.ParseTPMKey` reads it) and `name` (hex) from the TPM, unless the
  file sets them. If the TPM or a key isn't available it notes that and sends
  the section as written.

Handles must be persistent handles (0x81xxxxxx), in hex or decimal.
The owner hierarchy is authorised with `JANE_TPM_OWNER_PASSWORD` and the
endorsement hierarchy with `JANE_TPM_ENDORSEMENT_PASSWORD`; each is the empty
password if unset, as with tpm2-tools' defaults.

## Tests

    go test ./...

Unit tests use a fake Jane server and an in-memory TPM. The TPM integration
test (`tpm.TestProvisioning`) needs a TPM and is skipped unless `JP_TEST_TPM`
is set, to a device (`/dev/tpmrm0`), `unix://…`, `tcp://…`, or empty for the
default. It creates and evicts keys at 0x817FFFE0 and 0x817FFFE1, and checks
that a quote from the new AK verifies with its exported PEM key.

## Differences from jp.py

- TPM steps use go-tpm, not tpm2-tools, and TPM failures stop the run (the
  Python printed tpm2-tools failures and carried on). The key files no longer
  include `ak.ctx`, which only tpm2-tools can use.
- Errors stop the run with exit code 1 and a message instead of a traceback;
  a missing `itemid` in a server response is reported with the server's body.
  User-requested stops (`q`, or declining the `--recreate` check) exit 0
  as before.
- The session opened for `processevs` is always closed, even on error.
- `processevs` without an earlier `processelement`
  in the worklist is an error (the Python sent an empty element ID).
- An `evs` entry with more than one intent key is rejected rather than
  silently using the first.
- Unknown expected-value `type`s are reported instead of silently ignored.
- Claim values (machine ID, PCR digest, firmware version) are passed through
  as raw JSON, so large integers are preserved exactly.
- TPM handles are kept as written in the YAML, quoted or not. PyYAML turned an
  unquoted `0x810100EE` into a decimal string (still accepted).
- `host.os` is `sysname-release-machine` from uname (e.g.
  `Linux-6.12.48-amd64-x86_64`), or `Windows-10.0.<build>`; Python's
  `platform.platform()` also appended a libc suffix.
- `create` refuses on an already registered machine (above) instead of
  asking, and `-u` no longer skips that check; `--recreate` is new.
- New `uefiefivars` expected-value type.
- New `attest` operation (above), which prints rule results by name
  (`9001 Fail`); `create`/`update` no longer run rules (1.5.3).
- jp asks no questions (1.5.4): the destructive TPM steps need
  `--reprovisiontpm`; `-u`/`--unsafe` is removed.
- HTTP requests time out after 60 seconds.

Kept deliberately: quote expected-value names still repeat the intent ID
(`<element>---<intent>-<intent>`), so they match EVs created by the Python
version; `uefi.eventlog` and `ima.asciilog` are still the Linux securityfs
paths on every platform, since they describe where the Linux agent reads.
