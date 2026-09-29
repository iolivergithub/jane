# jp — Jane element provisioner (Go)

Go port of `provisioner/jp.py`. It registers the machine as an element with a
Jane attestation server, can clear and provision the TPM with tpm2-tools, and
records expected values and runs rules for the intents listed in the
provisioning file. The provisioning file format is unchanged; see
`testdata/examplecreateprovisioningfile.yaml`.

## Build and run

    go build -o jp .
    sudo ./jp create provisioning.yaml        # asks before TPM steps
    sudo ./jp update provisioning.yaml -u     # -u / --unsafe: no prompts

Root is needed for `/etc/janeelementid`, `/etc/machine-id` and the TPM.
Linux only at runtime (it builds elsewhere; `collecthostinfo` then fails).

    go test ./...

## Differences from jp.py

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
  unquoted `0x810100EE` into a decimal string.
- `host.os` is `sysname-release-machine` from uname (e.g.
  `Linux-6.12.48-amd64-x86_64`); Python's `platform.platform()` also appended a
  libc suffix.
- `-u` prints "unsafe mode" rather than the confusingly named
  "safe mode is True".
- HTTP requests time out after 60 seconds.

Kept deliberately: quote expected-value names still repeat the intent ID
(`<element>---<intent>-<intent>`), so they match EVs created by the Python
version; tpm2-tools failures are printed but not fatal,
as `tpmclear` routinely fails on empty handles.
