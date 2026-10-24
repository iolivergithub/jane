# Architecture

A short overview for the curious: what the parts of Jane are, how they fit together, and how the data links up. For using Jane, start with the [Quick start](quickstart.md); the other pages in the [contents](contents.md) cover each part in detail.

## The parts

![Jane: deployment and components](architecture/deployment.svg "Jane: deployment and components")

**Jane** (`janeserver`) runs on a server, as the `jane` user under systemd. It has three front ends:

| Front end | Port | Used by |
|---|---|---|
| REST API (`services/restapi`) | 8520 | jp, scripts, automation; JSON; API key |
| Web UI (`services/webui`) | 8540 | people in a browser; username and password |
| x3270 (`services/x3270`) | 3270 | people on a tn3270 terminal; username and password |

Every request passes the `auth` package (logins, API keys, the users collection) and then reaches `operations`, the core. `operations` stores and reads elements, intents, expected values, claims, results, sessions and opaque objects, and does the two things Jane exists for:

- **attest**: a **protocol** (`protocols`) asks a trust agent on the element for evidence, and Jane stores the answer as a **claim**;
- **verify**: a **rule** (`rules`) checks a claim, usually against an expected value, and Jane stores the outcome as a **result**.

Claims, results and closed sessions carry a signed footer (`signing`). Every operation writes a log entry (`logging`) to MongoDB, to the MQTT broker (topics `AS/<channel>`) and to `/var/log/jane/jane.log`. `datalayer` holds the MongoDB and MQTT connections; `configuration` reads `config.yaml`.

Jane needs **MongoDB** (all data, including the log and the users) and an **MQTT broker** such as Mosquitto (the log as a live feed). Both usually run on the same machine.

On each **element**, the machine being attested:

- **tarzan**, the trust agent, answers Jane's requests over HTTP (port 8530): TPM 2.0 quotes and PCRs, UEFI and IMA logs, system information. It runs as root to reach the TPM and the logs.
- **ratsd** (optional) gathers evidence from other attesters, e.g. confidential VMs and GPUs.
- **jp**, the provisioner, registers the element with Jane (`jp create`, `jp update`, which set up the TPM keys and record expected values) and runs attestations (`jp attest`), through the REST API.

**addjaneuser**, on the Jane server, creates users, sets their passwords and API keys. It is the only tool that writes the users collection.

## The data model

![Jane: data model](architecture/datamodel.svg "Jane: data model")

- An **element** is a machine, with one or more **endpoints** (an address and the protocol used to reach its trust agent), and its TPM keys and log locations.
- An **intent** says what to ask for: a function such as `tpm2/quote` and its parameters.
- An **expected value** holds the known-good values for one element, endpoint and intent.
- A **claim** is what a trust agent returned for one intent, plus the context it was asked in.
- A **result** is one rule's verdict on one claim.
- A **session** groups the claims and results of one piece of work, e.g. one `jp attest` run. It is opened, claims and results are added, and it is closed and signed.

Expected values, results and sessions refer to other items by their `itemid`. Claims and results also keep **copies** of the element, intent, session and expected value they were made from. This is deliberate, for forensics: each copy is a snapshot of that moment, so the history of how an element changed over time can be reconstructed from its claims and results, even after the element, its intents or its expected values have been edited.

**Log entries** refer to any item by `refid` and `reftype`. **Opaque objects** are named values (e.g. well-known hashes or identifiers) for reference. **Users** have a password (Web UI, x3270), an API key (REST), or both; only their hashes are stored.

## One attestation, end to end

1. jp opens a session over REST.
2. For each intent, jp asks Jane to attest: Jane looks up the element's endpoint and its protocol, the protocol calls tarzan, tarzan asks the TPM (or reads the logs), and Jane stores the claim with a signed footer and adds it to the session.
3. For each claim, jp asks Jane to verify with a rule: the rule compares the claim with the expected value, and Jane stores the signed result and adds it to the session.
4. jp closes the session, and Jane signs its footer over the lists of claims and results.

The Web UI and x3270 attest pages do the same steps for one element and intent at a time.

## Where to read more

- Structures: [elements](elements.md), [intents](intents.md), [expected values](expectedvalues.md), [claims](claims.md), [results](results.md), [sessions](sessions.md)
- Attestation: [functions](functions.md), [protocols](protocols.md), [trust agents](trustagents.md), [rules](rules.md), [signing](signing.md)
- Running it: [installing](deb.md), [running](running.md), [jp](jp.md), [authentication](authentication.md), [hardening](hardening.md)
- The code: `janeserver/` (one directory per package above), `tarzan/`, `provisioner/` (jp), `etc/addjaneuser/`
