# The provisioning language

A provisioning file is a YAML file, read by [jp](jp.md), that describes one element (a machine) completely:

- how to register it in Jane, and which trust agents reach it;
- which intents to attest, and which **expected values** to record from them;
- which **rules** check those claims;
- since 1.6, the **attestation profile** Jane stores for the element, including which related elements to **follow**.

This page is the reference for the language: every key, how rules take parameters, **variables and their scope**, and **follow**. How jp's commands are run is in [jp](jp.md); what Jane does with a stored profile (REST, Web UI, sessions) is in [Attestation profiles](attestationprofiles.md).

Sections marked **(1.6)** are new in Jane 1.6.

## Who reads what

| Part of the file | `jp create` / `jp update` | `jp attest` | `jp remoteattest` (1.6) | Stored in Jane |
|---|---|---|---|---|
| `attestationserver` | ✓ | ✓ | ✓ unless `--server` or `JANE_SERVER` is given | — |
| `element`, `endpoints`, `tpm2` | ✓ (registers or updates the element) | — | — | in the element |
| `provisionworklist` | ✓ | — | — | — |
| `evs:` — intent, `protocol` | ✓ (attest, to record expected values) | ✓ | — | in the profile, as steps (1.6) |
| `evs:` — `type`, `issuerurl` | ✓ (how to record the expected value) | — | — | the expected values themselves, not the type |
| `evs:` — `cid`, `rules` (with `rid`, `resvar`, `parameters`) | — | ✓ | — | in the profile, as steps (1.6) |
| `defaultattestationprofile`, `attestationprofiledescription` (1.6) | ✓ (names and describes the profile written) | — | — | profile name, description, element's default |
| `follow` (1.6) | ✓ (written into the profile) | — (ignored) | — | in the profile |

`jp remoteattest` reads nothing from the file but `attestationserver`: everything it runs is the profile already stored in Jane.

## A complete file

```yaml
attestationserver: http://jane.example.org:8520   # Jane's REST API, as seen from this machine

element:
  name: web01                          # must be unique in Jane
  description: Web server 1
  tags: [linux, x86, tpm2, uefi]
  elementtype: virtual-machine         # optional: physical-machine, virtual-machine, ...

endpoints:                             # how Jane reaches this machine's trust agents
  tarzan:
    endpoint: http://web01.example.org:8530
    protocol: A10HTTPRESTv2

tpm2:
  device: /dev/tpmrm0
  ekcerthandle: '0x01C00002'
  pcrnaming: tcg-pcclient-uefi         # optional: PCR names (registernames.md)
  ek: { handle: '0x810100EE' }
  ak: { handle: '0x810100AA' }

provisionworklist:                     # create and update run these, in order
  - tpmclear                           # only with --reprovisiontpm
  - tpmprovision                       # only with --reprovisiontpm
  - collecthostinfo
  - collecttpm2
  - collectuefi
  - collectima
  - makebetterdescription
  - processelement
  - processevs

defaultattestationprofile: nightly                   # (1.6) the profile this file writes
attestationprofiledescription: Nightly full check    # (1.6) optional

evs:                                   # what to attest, record and check, in order
  - std::intent::uefi::eventlog :
      protocol: tarzan
      cid: uefilog                     # variable: this claim's item ID
  - std::intent::tpm::pcrs :
      protocol: tarzan
      rules:
        - tpm2_uefiLogReplay:
            parameters: { "uefilogcid": $uefilog }
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      type: tpm2quote                  # create/update record the expected value from this claim
      cid: q_pcr0
      rules:
        - tpm2_attestedValue
        - tpm2_validSignature:
            rid: r_sig                 # variable: the result's item ID
            resvar: r_sig_code         # variable: the result code
  - std::intent::tpm2::capabilities :
      protocol: tarzan
      rules:
        - tpm2_capabilitiesFirmware:
            parameters: { "quotecid": $q_pcr0 }

follow:                                # (1.6) after this element, attest the ones it is related to
  - relationship: hosted-on            # every element this one is hosted-on
    profile: hostcheck                 # run their profile "hostcheck"
```

## Top-level keys

| Key | Required | Meaning |
|---|---|---|
| `attestationserver` | yes | URL of Jane's REST API, reachable from this machine |
| `element` | for `create`/`update` | `name` (unique in Jane), `description`, `tags`, `elementtype` — see [Elements](elements.md) |
| `endpoints` | for `create`/`update` | map from endpoint name to `{endpoint, protocol}`: where Jane reaches this machine's trust agents, and with which [protocol](protocols.md). Must be reachable **from Jane's machine** |
| `tpm2` | if the machine has a TPM | device, EK certificate handle, PCR naming scheme, EK and AK handles |
| `provisionworklist` | for `create`/`update` | the steps `create` and `update` run, in order — table in [jp](jp.md#worklist-steps) |
| `evs` | no (may be empty) | the intents, in order, with what to record and check — below |
| `defaultattestationprofile` (1.6) | no | name of the profile `create`/`update` write; missing → `default`, with a warning |
| `attestationprofiledescription` (1.6) | no | free-text description of that profile |
| `follow` (1.6) | no | which related elements a run of this profile goes on to attest — below |

## `evs` entries

Each entry is a single-key map: the key is an intent's item ID (a [standard intent](standardintents.md) or your own), the value says what to do with it. Entries are processed **in file order**.

| Key | Used by | Meaning |
|---|---|---|
| `protocol` | all | the **endpoint name** (a key of `endpoints:`) to attest through. Called `protocol` for historical reasons |
| `type` | `create`/`update` | how to turn the claim into an expected value: `sysmachineid`, `tpm2quote`, `tpm2ekcert`, `uefiefivars` ([expected value types](../provisioner/README.md#expected-value-types)). None: the claim is collected, nothing is recorded |
| `issuerurl` | `create`/`update`, type `tpm2ekcert` | where the EK certificate's issuer certificates are (`https://`, `http://`, `file://`); overrides the certificate's own CA Issuers URL |
| `cid` | runs | a variable that receives this intent's claim item ID |
| `rules` | runs | the rules to run on the claim, in order |

`create`/`update` attest every entry (to record expected values) but run no rules. A run — `jp attest`, or a profile run in Jane — attests every entry and runs its rules.

## Rules and parameters

A rule is written either as its name, or as a single-key map from its name to its options:

```yaml
rules:
  - tpm2_attestedValue                 # plain name
  - tpm2_pcrDigest:                    # name with options
      parameters: { "pcrscid": $pcrs } # rule parameters (a mapping)
      rid: r_digest                    # variable: the result's item ID
      resvar: r_digest_code            # variable: the result code
```

- `parameters` must be a mapping (usually written as a JSON-style box). Each value is passed to the rule as it is, except a value that is exactly `$name` (quoted or not), which is replaced by that variable's value. Nothing is substituted inside longer strings.
- Which parameters a rule takes is in [Rules](rules.md). Rules that compare two claims take the other claim's item ID — that is what variables are for.

## Variables

Variables pass the item IDs (and result codes) produced earlier in a run to rules later in the same run.

| Defined by | Kind | Value |
|---|---|---|
| `cid:` on an `evs` entry | `cid` | the item ID of the claim made for that intent |
| `rid:` on a rule | `rid` | the item ID of the result that rule produced |
| `resvar:` on a rule | `resvar` | that result's code and name, e.g. `0 Success` |

**Rules for variables** — checked before anything runs; a mistake stops jp with exit 2 (and, 1.6, Jane refuses such a profile):

- **Names** are letters, digits and `_`, not starting with a digit. `cid`, `rid` and `resvar` share one namespace.
- **Single assignment:** a name is defined **once** in a file. Defining it again — in any kind — is an error. A variable never changes its value during a run.
- **Defined before use:** `$name` may only refer to a variable defined **earlier in the file**. An intent's own `cid` counts as earlier for its own rules; a rule's `rid`/`resvar` are set after that rule runs, so later rules (and later intents) can use them.
- **No types:** a variable holds a string; the rule that receives it checks it (a wrong item ID gives a known rule error).

**At run time:**

- If the step that sets a variable fails (the intent could not be attested, say), the variable has no value, and every rule that uses it is reported **NOTRUN**, with the reason; it counts as not passed. The run carries on.
- At the end of a run the variables are listed with their kinds and values — printed by `jp attest` and `jp remoteattest`, and (1.6) stored in the run's session and shown on the session page.

### Scope

**A variable exists for one run of one profile on one element, and nowhere else.**

- **One run:** variables start empty at the beginning of each `jp attest`, each `jp remoteattest`, each run from the Web UI. Nothing carries over between runs or sessions.
- **One element:** a run that follows relationships (below) starts each followed element's run with **no variables**, and nothing comes back from it. Variables are never passed along `follow`, in either direction.
- **What to do instead:**
  - to compare with a claim from an **earlier session**, run the rule from the Web UI's attest page with that claim's item ID as the parameter;
  - to compare **across elements** (a VM's values against its host's), use a rule that reads the related element itself, or do the comparison outside Jane from the two sessions' results.

Keeping variables inside one run keeps every profile self-contained: it can be run on its own (by `jp remoteattest`, REST or the Web UI) and gives the same answer however it was reached.

## Attestation profiles (1.6)

On `create` and `update`, jp turns the file into an **attestation profile** and stores it on the element in Jane:

- the profile's **name** is `defaultattestationprofile` (`default` if missing, with a warning);
- its **description** is `attestationprofiledescription`;
- its **steps** are the `evs:` entries, in order, each with its intent, endpoint (`protocol:`), `cid` and rules — **without** `type:` and `issuerurl:`, which only concern recording expected values;
- its **follow** entries are the file's `follow:`.

The profile is written (or overwritten) and becomes the element's **default**; other profiles already on the element are left alone. So running `jp update` with files that name different profiles builds up several profiles on one element, together with all the expected values they need. A file whose `evs:` and `follow:` are both empty writes no profile (warning); a profile may have no steps if it has `follow:` entries — an element that only fans out to others.

Profiles change **only** through `jp create`/`jp update` on the device. Jane can run them (`jp remoteattest`, REST, the Web UI — where a profile can also be edited for one run without saving), but never stores changes to them. In Jane a profile is shown and edited as YAML in Jane's own form (`steps:` with `intent`, `endpoint`, `cid`, `rules`), which is the same content as the `evs:` shorthand here — see [Attestation profiles](attestationprofiles.md).

`jp attest` does not use the stored profile: it runs the `evs:` in the file, as before.

## Follow (1.6)

`follow:` says which related elements a run goes on to attest after its own steps. It uses the element's [relationships](elements.md#element-type-and-relationships) (`hosted-on`, `runs-on`).

```yaml
follow:
  - relationship: hosted-on      # for every element this one is hosted-on ...
    profile: hostcheck           # ... run its profile "hostcheck"
  - relationship: runs-on        # for every element this one runs-on ...
                                 # ... run its default profile (no profile given)
```

| Key | Meaning |
|---|---|
| `relationship` | a relationship type; followed **outwards**: from this element to the elements it is related to |
| `profile` | the profile to run on each related element; missing or empty: that element's **default** profile |

**What happens when a profile with `follow:` runs** (in Jane: `jp remoteattest`, REST `runprov`, the Web UI):

1. The element's own steps run, in its own session, with its own variables.
2. The follow entries are resolved: for each entry, **every** relationship of that type gives a target element and profile. Each is decided before anything else runs:
   - `run` — it will be run;
   - `skipped (already run in this request)` — that element and profile has already run, or is about to, in this request;
   - `not followed (follow limit n reached)`;
   - an error — the element has no relationship of that type, the related element is missing, or it has no such profile (or no default).
3. The variables and these decisions are recorded in the session, and the session is closed.
4. Each `run` target is run the same way, in a **new session whose parent is this one** — and may follow further.

**It always ends.** Within one request, each (element, profile) pair runs at most once, so cycles in the relationships (A hosted-on B, B runs-on A) are harmless: the second visit is skipped. A **follow limit** (default 10 levels: the starting run is level 0, the elements it follows level 1, …) bounds how far it spreads; `jp remoteattest --followlimit n`, `?followlimit=n` on REST, the depth selector in the Web UI. `--nofollow` (`followlimit 0`) runs the element alone.

**Errors don't stop it.** A follow error is logged, recorded in the session and reported at the end; the remaining follow entries still run. `jp remoteattest` exits with 3 if there was any follow error (something asked for was not attested), but not for skips or the limit.

**Nothing passes along follow:** each followed run has its own session, its own variables (starting empty), its own results. Combining the sessions of one request into a verdict is up to whoever asked for it; Jane reports, it does not judge.

**Sessions form a tree.** Each followed run's session points to the session it was followed from (`parent`); the session page lists both directions. See [Sessions](sessions.md).

### Example: a VM and its host

The host's file (on the host) writes a profile `hostcheck`:

```yaml
defaultattestationprofile: hostcheck
evs:
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      type: tpm2quote
      rules: [ tpm2_attestedValue, tpm2_validSignature ]
```

The VM's file (on the VM) writes `nightly` and follows its host:

```yaml
defaultattestationprofile: nightly
evs:
  - std::intent::sha256::crtm::pcr0 :
      protocol: tarzan
      type: tpm2quote
      rules: [ tpm2_attestedValue, tpm2_validSignature ]
follow:
  - relationship: hosted-on
    profile: hostcheck
```

With the relationship *VM hosted-on host* in Jane, `jp remoteattest` on the VM gives two sessions: the VM's (`nightly`), and the host's (`hostcheck`) with the VM's session as its parent. The VM's session records `hosted-on → host: run`.

## Checks before anything runs

jp reads and checks the whole file before contacting Jane, and stops if it is wrong:

- **exit 2** (a mistake in the variables): a variable name is invalid, defined twice, or used before it is defined; (1.6) `defaultattestationprofile` is not a valid profile name (letters, digits, `_`, `-`; at most 64 characters);
- **exit 1** (the file can't be used): it isn't valid YAML, `attestationserver` is missing, an `evs` entry isn't a single-key map, a rule entry has more than one key, or a rule's `parameters` is text rather than a mapping.

Unknown intents, endpoints or rules are not checked in advance: they are reported when the run reaches them, and the run carries on.

## See also

- [jp](jp.md): the commands, flags, environment variables, exit codes, worklist steps and skeletons.
- [Attestation profiles](attestationprofiles.md) (1.6): profiles in Jane — REST, Web UI, editing for one run, session records.
- [Rules](rules.md): every rule and its parameters. [Standard intents](standardintents.md). [Expected values](expectedvalues.md).
- [Elements](elements.md): element types and relationships. [Sessions](sessions.md): sessions and parents.
