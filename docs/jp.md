# jp, the provisioner

jp runs on each element (machine to be attested). It registers the machine with Jane, provisions its TPM keys, records the machine's expected values, and attests it. Everything jp does is described by a **provisioning file** (YAML); the tarzan package installs an example as `/etc/opt/jane/provisioning.yaml.example`.

jp is built to run unattended (from cron, a systemd timer, a package hook) as well as by hand. It never asks questions. Anything destructive needs its own flag on the command line, so a scheduled or scripted jp can never wipe the TPM keys or re-register the machine by accident.

- [Synopsis](#synopsis)
- [The three modes](#the-three-modes): create, update, attest
- [Flags](#flags)
- [When to use what](#when-to-use-what)
- [Environment variables](#environment-variables)
- [Exit status and output](#exit-status-and-output)
- [The provisioning file](#the-provisioning-file)
- [Files and permissions](#files-and-permissions)
- [Changes in 1.5.4](#changes-in-154)

## Synopsis

```
jp [-h] [--reprovisiontpm] [--recreate] [--apikeyfile PATH] create|update|attest PROVISIONINGFILE
```

Installed from the deb it is `/opt/jane/jp`. Options may come before or after the mode and file.

## The three modes

| Mode | Needs root | Changes Jane | Can change the TPM | Typical use |
|---|---|---|---|---|
| `create` | yes | registers a new element, records expected values | only with `--reprovisiontpm` | once, when a machine is first set up |
| `update` | yes | updates the element, replaces its expected values | only with `--reprovisiontpm` | after firmware, bootloader or kernel updates |
| `attest` | no | adds claims and results (one session) | never | checking the machine, as often as you like |

### create

Registers this machine with Jane for the first time:

1. **Checks first that the machine is not already registered.** jp refuses (exit 1, nothing changed) if the element ID file exists, or if Jane already has an element with the provisioning file's `element: name` or this machine's machine ID, or if it cannot ask Jane. Only `--recreate` lifts this.
2. Runs the file's `provisionworklist`, in order: collecting host and TPM information, and, with `--reprovisiontpm`, removing old TPM keys and creating new ones.
3. Registers the element in Jane (`processelement`) and writes its item ID to the element ID file.
4. Attests every intent under `evs` once and records an expected value for each entry that has a `type` (`processevs`).

`create` runs no rules: run `jp attest` afterwards to check the result.

Without `--reprovisiontpm` the TPM is used as it is. That is right if the EK and AK already exist at the handles in the file (made by an earlier jp, or by hand). On a TPM without them, jp notes `no key at 0x…`, the element is registered without key public parts, and quotes cannot be checked: use `--reprovisiontpm` for a first registration.

### update

Refreshes the element registered earlier (the one in the element ID file) and **replaces all its expected values** with what the machine reports now:

1. Runs the same worklist; the TPM steps only with `--reprovisiontpm`.
2. Updates the element in Jane (host information, TPM keys, endpoints, description, tags from the file).
3. Attests every intent under `evs` and replaces the expected values.

Run it after anything that legitimately changes what is measured: a firmware update (fwupd), a new shim, bootloader or kernel, Secure Boot database updates. **Whatever the machine reports at that moment becomes the new known-good state**, so run `jp attest` first if you want to be sure the machine was in a good state before accepting its new measurements.

Without `--reprovisiontpm`, `update` never touches the TPM keys and is safe to run unattended, e.g. from an apt hook after kernel updates. If the element ID file is missing it stops: run `create`.

### attest

Checks the machine against Jane's expected values, without changing anything:

1. Opens one session in Jane.
2. Asks Jane to attest every intent under `evs`, through the endpoint named in the entry.
3. Asks Jane to run every rule listed for that intent, passing rule parameters (including claim item IDs from earlier in the same run, see [variables](#variables-passing-claims-to-rules)).
4. Prints a report and exits 0 only if every rule passed.

`attest` does not run the worklist, never opens the TPM, and does not change the element or its expected values. It does not need root: it only reads the element ID file and talks to Jane. Run it as often as you like, by hand or on a schedule.

## Flags

| Flag | Modes | Meaning |
|---|---|---|
| `--reprovisiontpm` | create, update | Allow the worklist's TPM steps: `tpmclear` deletes the keys at the EK and AK handles, `tpmprovision` creates a new EK and AK there. Without it both are skipped (reported as `skipped, the TPM is left unchanged`). Rejected with `attest` |
| `--recreate` | create | Register a new element although this machine is already registered. The old element is left in Jane, orphaned; with `--reprovisiontpm` its keys no longer match the TPM |
| `--apikeyfile PATH` | all | Read the Jane REST API key from a file holding the key alone (e.g. `/etc/opt/jane/jp.key`), when Jane requires one (`auth: rest: true`). Overrides `JANE_API_KEY`. See [Authentication](authentication.md#rest-api) |
| `-h`, `--help` | | Print the usage and exit |

There is no `-u`/`--unsafe` any more (see [Changes in 1.5.4](#changes-in-154)).

## When to use what

| Situation | Command |
|---|---|
| First registration of a machine (TPM without jp's keys) | `sudo -E jp create --reprovisiontpm provisioning.yaml` |
| First registration, keys already at the handles (made by hand, or kept from an earlier install) | `sudo -E jp create provisioning.yaml` |
| After a firmware, shim, bootloader or kernel update | `sudo -E jp update provisioning.yaml` |
| The same, automatically (apt hook, fwupd hook, cron) | `jp update` as above; never add `--reprovisiontpm` to an automated job |
| Regular checks (cron, systemd timer, monitoring) | `jp attest provisioning.yaml`, as any user |
| New TPM keys for a registered machine (keys lost, TPM cleared or replaced, handles reused) | `sudo -E jp update --reprovisiontpm provisioning.yaml` |
| Register the machine again as a new element (cloned VM given a new identity, reinstalled machine) | `sudo -E jp create --recreate --reprovisiontpm provisioning.yaml`, then remove the old element in the Web UI |

A cloned VM that kept its source's `/etc/machine-id` is refused by `create`, because Jane already has an element with that machine ID: give the clone a new machine ID first.

Examples:

```bash
# first registration
sudo -E /opt/jane/jp create --reprovisiontpm /etc/opt/jane/provisioning.yaml
/opt/jane/jp attest /etc/opt/jane/provisioning.yaml

# /etc/cron.d/jane-attest: attest every hour, as an unprivileged user
0 * * * *  nobody  /opt/jane/jp attest /etc/opt/jane/provisioning.yaml >/var/tmp/jp-attest.log 2>&1
```

## Environment variables

| Variable | Meaning |
|---|---|
| `JANE_API_KEY` | the Jane REST API key (`jane_...`, made with `addjaneuser --apikey`), when Jane's `auth: rest: true` |
| `JANE_TPM_ENDORSEMENT_PASSWORD` | the TPM's endorsement hierarchy password, only if one is set (`tpm2_changeauth -c e`); used when creating the EK. Unset means the empty password |
| `JANE_TPM_OWNER_PASSWORD` | the TPM's owner hierarchy password, only if one is set (`tpm2_changeauth -c o`); used when making the EK and AK persistent and when `tpmclear` evicts them. Unset means the empty password. tarzan does not need it |
| `SSL_CERT_FILE` | (Linux/BSD) a certificate to trust for Jane's HTTPS, e.g. Jane's self-signed certificate |

`sudo` drops the environment by default: use `sudo -E`, or `sudo --preserve-env=JANE_API_KEY,JANE_TPM_ENDORSEMENT_PASSWORD,JANE_TPM_OWNER_PASSWORD`, or give the key with `--apikeyfile`.

`JANE_USERNAME`, `JANE_PASSWORD`, `--user` and `--pass` were removed in 1.5.5: jp stops with a message if it is given them without a key. Jane and jp must both be 1.5.5 or later.

## Exit status and output

| Status | Meaning |
|---|---|
| 0 | success; for `attest`, every rule passed |
| 1 | error: bad provisioning file, element ID file missing or unreadable, Jane or the TPM unreachable, Jane refused the API key (HTTP 401) or this address made too many failed attempts (HTTP 429), no permission (create/update without root), machine already registered (create) |
| 2 | bad command line (unknown option, missing argument, the removed `--user`/`--pass`, an API key that is unreadable or not a Jane key, `--recreate` or `--reprovisiontpm` with the wrong mode), or a mistake in the provisioning file's variables |
| 3 | `attest` only: a rule did not pass, could not be run, or an intent could not be attested |

`attest` prints each intent with its claim and each rule's result, then a summary and the variables:

```
std::intent::sha256::crtm::pcr0 on tarzan  (claim C1)
  PASS   tpm2_attestedValue
  FAIL   tpm2_firmware: 9001 Fail
...
Attestation FAILED: 15 rules passed, 3 failed, 1 of 7 intents could not be attested
```

The session, claims and results are in Jane's Web UI too.

## The provisioning file

```yaml
attestationserver: http://127.0.0.1:8520      # Jane's REST API
element:
  name: Debian13VM                            # must be unique in Jane
  description: Debian 13 VM
  tags: [linux, x86, tpm2, uefi]
  elementtype: virtual-machine                # optional (1.5.6): physical-machine, virtual-machine, ...
endpoints:                                    # where Jane reaches this machine's trust agents
  tarzan:
      endpoint: http://127.0.0.1:8530
      protocol: A10HTTPRESTv2
tpm2:
  device: /dev/tpmrm0
  ekcerthandle: '0x1C000002'
  ek:
    handle: '0x810100EE'
  ak:
    handle: '0x810100AA'
provisionworklist:                            # create and update run these, in order
  - tpmclear
  - tpmprovision
  - collecthostinfo
  - collecttpm2
  - collectuefi
  - collectima
  - makebetterdescription
  - processelement
  - processevs
evs:                                          # intents to attest, and what to record or check
  - std::intent::sys::info :
      protocol: tarzan
      type: sysmachineid
      rules:
        - sys_taRunningSafely
        - sys_machineID_Agent
  - ...
```

Use the endpoint names in `evs: protocol:` (the key is called `protocol` for historical reasons; it names an endpoint). If Jane and tarzan are on different machines, `attestationserver` must reach Jane from this machine and `endpoints` must reach tarzan from Jane's machine.

### Worklist steps

| Step | What it does |
|---|---|
| `tpmclear` | deletes (evicts) the persistent keys at `tpm2.ek.handle` and `tpm2.ak.handle`. Not TPM2_Clear: the rest of the TPM is untouched. Runs only with `--reprovisiontpm` |
| `tpmprovision` | creates the TCG default RSA-2048 EK and an RSA-2048 restricted signing AK under it, at those handles; fails if a handle is in use (hence `tpmclear` before it). Runs only with `--reprovisiontpm` |
| `collecthostinfo` | OS, architecture, hostname and machine ID, into the element's `host` |
| `collecttpm2` | the `tpm2` section, with the EK and AK public parts and names read from the TPM (read only) |
| `collectuefi`, `collectima` | the standard UEFI event log and IMA log locations |
| `makebetterdescription` | adds the element name and a timestamp to the description |
| `processelement` | registers (`create`) or updates (`update`) the element, and writes the element ID file |
| `processevs` | attests each `evs` intent and records or replaces expected values |

A worklist without the TPM steps is fine for machines whose keys are managed elsewhere.

### evs entries

Each entry names a standard (or your own) intent and says:

- `protocol:` which endpoint to use;
- `type:` what expected value to record on `create`/`update` (`sysmachineid`, `tpm2quote`, `uefiefivars`; none: record nothing);
- `rules:` which rules `attest` runs, each a name or a name with options (`parameters`, `rid`, `resvar`);
- `cid:` a variable holding this intent's claim item ID during `attest`.

### Variables: passing claims to rules

Rules that compare two claims take the other claim's item ID as a parameter. Within one `jp attest` run, a variable carries it:

```yaml
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      cid: q_pcr0
      type: tpm2quote
      rules:
        - tpm2_attestedValue
        - tpm2_validSignature
  - std::intent::tpm2::gettime :
      protocol: tarzan
      rules:
        - tpm2_clockProgress:
            parameters: { "previouscid": $q_pcr0 }
```

A variable must be defined earlier in the file; mistakes stop jp with exit 2 before anything runs. Variables last for one run; to compare with an earlier session, run the rule from the Web UI's attest page with an older claim's item ID. The full rules for variables and expected-value types are in the [jp README](../provisioner/README.md); the rules themselves in [Rules](rules.md).

## Files and permissions

| File | Notes |
|---|---|
| `/etc/opt/jane/janeelementid` | the element's item ID, written by `create` (read-only, 444). Windows: `%ProgramData%\jane\janeelementid`. (Before 1.5.3 it was `/etc/janeelementid`; since 1.5.6 jp no longer moves it — move it by hand.) |
| provisioning file | anywhere; `/etc/opt/jane/provisioning.yaml` by convention |
| `ek.pub`, `ak.pub`, `ak.name` | written to the temporary directory by `tpmprovision` |

`create` and `update` need root (Administrator on Windows) for the TPM, the element ID file and the machine ID. They check before contacting Jane that they can write the element ID file, and stop with "run it as root" if not, so a failed run never leaves a half-registered element. `attest` needs no root; on a host with both packages `/etc/opt/jane` is 751, so any user can read the element ID file.

## Changes in 1.5.4

- **The TPM is changed only with `--reprovisiontpm`.** Before, `tpmclear` and `tpmprovision` ran whenever they were in the worklist, after a y/n question, or without one under `-u`; an automated `jp update -u` re-created the TPM keys every time.
- **No questions, and `-u`/`--unsafe` is removed.** jp now fails with "unknown option" (exit 2) if given `-u`. Update your scripts:

  | Before | Now |
  |---|---|
  | `jp create -u file` (keys made afresh) | `jp create --reprovisiontpm file` |
  | `jp update -u file` (keys re-created each time) | `jp update file` (keys kept), or `jp update --reprovisiontpm file` if new keys are really wanted |
  | `jp attest file` | unchanged |
  | `jp create --recreate -u file` | `jp create --recreate --reprovisiontpm file` |

- REST credentials (`JANE_USERNAME`/`JANE_PASSWORD`, `--user`/`--pass`) and the endorsement password variable are new; `attest` no longer needs root.
