# Attestation profiles

Since 1.6. An **attestation profile** is a named list of attestation instructions stored on an element: which intents to attest, through which endpoint, which rules to run on each claim, and which related elements to go on to attest (**follow**). Jane can run a profile on request — from `jp remoteattest`, the REST API or the Web UI — and records each run in a session.

The language a profile is written in (in a provisioning file), its variables and their scope, and how follow works are described in [The provisioning language](provisioning.md). This page is about Jane's side: where profiles are kept, how they are run, and what a run records.

## Who does what

| | Does | Never does |
|---|---|---|
| **jp** (`create`/`update`, on the device) | Writes the profile its provisioning file names, and makes it the element's default | Changes profiles it didn't write; runs `follow:` itself |
| **Jane** | Stores profiles; runs one on request, each step through the ordinary attest and verify; follows relationships; records the run in its sessions | Decides trust (no verdict); saves an edit made in the Web UI; deletes a profile |
| **tarzan** | Answers Jane's requests, as for any attestation | Knows about profiles |
| **You** (or a scheduler) | Decide when to run what, and what the results of a run mean | |

## On the element

| Field | Meaning |
|---|---|
| `attestationprofiles` | the element's profiles, by name |
| `defaultattestationprofile` | the profile run when none is named |

A profile has a `description`, its `steps` (each: `intent`, `endpoint`, optional `cid`, and `rules`, each with `name` and optional `rid`, `resvar`, `parameters`) and `follow` entries (`relationship`, optional `profile`). In the Web UI it is shown as YAML in this form:

```yaml
description: Nightly check
steps:
  - intent: std::intent::sha256::crtm::pcr0
    endpoint: tarzan
    cid: q_pcr0
    rules:
      - name: tpm2_attestedValue
      - name: tpm2_validSignature
  - intent: std::intent::tpm2::capabilities
    endpoint: tarzan
    rules:
      - name: tpm2_capabilitiesFirmware
        parameters:
          quotecid: $q_pcr0
follow:
  - relationship: hosted-on
    profile: hostcheck
```

This is the same content as a provisioning file's `evs:` and `follow:` (see [The provisioning language](provisioning.md#attestation-profiles-16)).

**Writing.** Only `PUT /element/<itemid>/attestationprofile/<name>`, which `jp create` and `jp update` call: the profile is written (or overwritten) and becomes the default; the element's other profiles are untouched. Names are letters, digits, `_` and `-`, at most 64 characters. A profile is refused (400) if it has neither steps nor follow entries, if a step lacks an intent or endpoint, or if its variables are wrong (defined twice, or used before they are defined). Intents, endpoints and rules are not looked up when a profile is written: a run reports any that don't exist. `PUT /element` keeps the stored profiles and default, whatever it carries; a new element starts with none. Nothing deletes a profile.

**Who may write.** Any holder of a REST API key can call the write endpoint, as for elements and expected values. With one key shared across a cluster, any machine could change any element's profiles; roles (#47) are where this will be restricted.

## Running a profile

| Request | |
|---|---|
| `POST /element/<itemid>/runprov` | run the element's default profile |
| `POST /element/<itemid>/runprov/<name>` | run the named profile |

Query parameters, all optional:

| Parameter | Meaning |
|---|---|
| `followlimit=<n>` | follow at most *n* levels (default 10; `0` = don't follow) |
| `parallel=true` | run followed profiles at the same time, at most 4 at once (the same sessions are made as without it) |
| `parent=<sid>` | open the run's session under the session *sid* (see [Sessions](sessions.md)) |

The request answers when the whole run, with everything it follows, has finished. Errors before anything runs — element or profile not found, no default, bad name or parameter, parent session not found — give 400 or 404. Once it runs, failures are in the report:

```json
{ "elementid": "…", "elementname": "web01", "profile": "nightly", "sessionid": "…", "parent": "",
  "steps": [ { "intent": "…", "endpoint": "tarzan", "claimid": "…", "error": "",
               "rules": [ { "rule": "tpm2_attestedValue", "resultid": "…", "result": 0,
                            "message": "…", "notrun": "", "error": "" } ] } ],
  "vars":  [ { "name": "q_pcr0", "kind": "cid", "value": "…" } ],
  "follow": [ { "relationship": "hosted-on", "target": "<itemid>", "profile": "hostcheck",
                "kind": "run", "message": "", "report": { … the same shape … } } ] }
```

There is no verdict: each rule's result is as the rule gave it. `jp remoteattest` turns the report into an exit code (see [jp](jp.md)).

**How it runs.** The follow tree is planned first, from the relationships and the stored profiles. Then each element's profile runs in its own session: its steps one after another (attest, then that claim's rules, with `$variables`), then the variables and that element's follow decisions are recorded in the session, the session is closed, and the elements it follows run in sessions of their own whose parent is this one. Each step uses Jane's ordinary attest and verify, so claims and results are exactly as from any other attestation. A call to a trust agent that doesn't answer gives up after 2 minutes; the step is reported and the run goes on. The details — variables, NOT RUN, follow decisions and why the run always ends — are in [The provisioning language](provisioning.md).

## In the Web UI

- The element page has a **Profiles** tab: each profile with its description, the default marked, its steps and follow entries. Display only.
- A profile's page shows it as YAML, read-only. **Edit** makes the box editable: the edit is used for one run and **never saved** — the stored profile changes only through jp. A YAML or variable mistake is shown and nothing runs.
- **Follow relationships** (ticked), **Depth** (1–10) and **Run followed profiles in parallel** (unticked), then **Attest**. The run goes on in Jane by itself and the browser goes to the run's session page: leaving the page loses nothing.
- Followed elements always run their own stored profiles; an edit applies only to the profile on the page.

## What a run records

Each session of a run carries an `attestationprofilerun` record, signed with the session:

| Field | Set | Meaning |
|---|---|---|
| `elementid`, `name` | at open | which profile on which element |
| `edited` | at open | the profile was edited in the Web UI for this run |
| `profile` | at open | the profile actually run, as YAML |
| `followlimit`, `depth`, `parallel` | at open | the run's options; `depth` 0 for the first run |
| `variables` | at close | name, kind (`cid`, `rid`, `resvar`) and value of each variable set |
| `follow` | at close | each follow decision: relationship, target, profile, `kind` (`run`, `skipped`, `limit`, `error`) and message |

The session's message says who asked for the run, from where and how, e.g. `runprov nightly (edited) on web01 (<itemid>) by ian via Web UI from 10.0.0.7; followlimit 3`; a followed run's message names the relationship and the session it was followed from. The session page shows the record in a collapsed section whose title sums it up (profile, element, edited, variables, follow counts, depth); open it for the variables (claims and results linked), the follow decisions (linked to the sessions they ran in) and the YAML that ran, and **Edit again** to open the profile page with that YAML.

While a run is going, its session pages reload themselves every `sessions: refresh` seconds (default 2; 0 switches it off — see [Running](running.md)), and stop when the run has finished. A follow marked `run` whose session never appeared — Jane stopped during the run — is shown as "follow run not started". Closing a session of a running profile (the **Close** button on a LONG SESSION) stops that run's remaining steps.

Claims and results copy their session and element, so they also carry the run record and the element's profiles at the time.

## See also

[The provisioning language](provisioning.md) · [jp](jp.md) · [Sessions](sessions.md) · [Elements](elements.md)
