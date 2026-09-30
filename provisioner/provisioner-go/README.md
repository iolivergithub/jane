# jp — Jane element provisioner (Go)

Go port of `provisioner/jp.py`. It registers the machine as an element with a
Jane attestation server, can clear and provision the TPM, and records
expected values and runs rules for the intents listed in the provisioning
file. It can then attest the registered element. The provisioning file format is unchanged; see
`testdata/examplecreateprovisioningfile.yaml`.

TPM operations use [go-tpm](https://github.com/google/go-tpm) directly
(package `jp/tpm`), so tpm2-tools is not needed. `jp` builds without cgo for
Linux, Windows, macOS, the BSDs, illumos/Solaris and AIX.

## Build and run

    go build -o jp .                          # or: CGO_ENABLED=0 GOOS=windows go build
    sudo ./jp create provisioning.yaml        # asks before TPM steps
    sudo ./jp update provisioning.yaml -u     # -u / --unsafe: no prompts
    sudo ./jp attest provisioning.yaml        # attest the registered element

Run as root (Administrator on Windows) for the TPM, the element ID file and
the machine ID.

| | Unix-likes | Windows |
|---|---|---|
| TPM | `tpm2.device`, default `/dev/tpmrm0` then `/dev/tpm0` | TPM Base Services |
| Element ID file | `/etc/janeelementid` | `%ProgramData%\jane\janeelementid` |
| Machine ID | `/etc/machine-id`, `/var/lib/dbus/machine-id`, `/etc/hostid`; on BSD/macOS then `kern.hostuuid` / `kern.uuid` | registry `MachineGuid` |
| Key files | `$TMPDIR` or `/tmp` | `%TEMP%` |

## Operations

- `create` runs `provisionworklist` and registers a new element, writing its
  ID to the element ID file. If that file exists it asks first.
- `update` runs `provisionworklist` and updates the element whose ID is in the
  element ID file.
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
2 bad command line; 3 `attest` completed but a rule did not pass or an intent
could not be attested.

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

- `tpmclear` evicts the persistent objects at `tpm2.ek.handle` and
  `tpm2.ak.handle`. An empty handle is reported, not an error.
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
Owner and endorsement hierarchy authorisation is the empty password, as with
tpm2-tools' defaults.

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
  User-requested stops (`q`, or declining the create-overwrite check) exit 0
  as before.
- The session opened for `processevs` is always closed, even on error.
- `processevs` / `processevs_withrules` without an earlier `processelement`
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
- New `attest` operation (above), and rule results are printed by name
  (`9001 Fail`) during `processevs_withrules` rather than as HTTP statuses.
- `-u` prints "unsafe mode" rather than the confusingly named
  "safe mode is True".
- HTTP requests time out after 60 seconds.

Kept deliberately: quote expected-value names still repeat the intent ID
(`<element>---<intent>-<intent>`), so they match EVs created by the Python
version; `uefi.eventlog` and `ima.asciilog` are still the Linux securityfs
paths on every platform, since they describe where the Linux agent reads.
