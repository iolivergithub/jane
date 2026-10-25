# Getting started

A tour of Jane's Web UI, using the element registered in the [Quick start](quickstart.md). It assumes Jane, MongoDB, Mosquitto and tarzan are running, the standard intents are loaded, and `jp create` and `jp attest` have been run once.

- [The home page](#the-home-page)
- [The element](#the-element)
- [Sessions, claims and results](#sessions-claims-and-results)
- [Attesting from the Web UI](#attesting-from-the-web-ui)
- [Expected values](#expected-values)
- [Checking claims against each other](#checking-claims-against-each-other)
- [Opaque objects](#opaque-objects)
- [The Live page and the log](#the-live-page-and-the-log)
- [Creating an element by hand](#creating-an-element-by-hand)

## The home page

Open `http://<jane-host>:8540`. The home page shows what is in the database and how Jane is configured: the services and the addresses they listen on, the message bus, the signing key and authentication.

> **📷 SCREENSHOT NEEDED — `images/gs-home.png`**
> Web UI home page straight after the quick start (one element, standard intents loaded, one session). Show the counts at the top and the configuration tabs below, with the **Services** tab open. Browser width about 1400 px; full page.

The menu bar has **Elements**, **Attest** and **Sessions** for everyday use, **Structures** for every kind of item (intents, expected values, claims, results, objects, protocols, rules, the log), **New** for creating items, **Help**, the **Live** page (screen icon), and the theme button: the Web UI follows your system's light or dark setting, and the button switches between automatic, light and dark (remembered in your browser). Each page has its own title in the browser tab, and the IDs at the top of element, intent, expected value, claim, result and session pages have a copy button next to them.

## The element

**Elements** lists what can be attested. Open the element jp registered.

> **📷 SCREENSHOT NEEDED — `images/gs-element.png`**
> **Elements** → the quick-start element. Show the top of the page: name, description, tags, the `tarzan` endpoint, the host information and the TPM section with EK and AK handles. Browser width about 1400 px; crop below the TPM section.

The element records where its trust agent is (endpoints), what jp found about the machine (host, TPM keys, log locations), and links to its expected values, claims and results. Its TPM section holds the AK's public key that `tpm2_validSignature` checks quotes against.

## Sessions, claims and results

`jp attest` ran in one **session**. Open **Sessions** and the newest one.

> **📷 SCREENSHOT NEEDED — `images/gs-session.png`**
> **Sessions** → the session made by `jp attest`. Show the session header (opened, closed, message) and the list of claims and results with their pass/fail badges. Browser width about 1400 px; full page if it fits, otherwise the first screen.

- A **claim** is what tarzan returned for one intent, with a full copy of the element, intent and parameters used. See [Claims](claims.md).
- A **result** is one rule's verdict on one claim, with its message. See [Results](results.md) and [Rules](rules.md).

Open a quote claim (intent *x86 UEFI CRTM only*):

> **📷 SCREENSHOT NEEDED — `images/gs-claim-quote.png`**
> The `std::intent::sha256::crtm::pcr0` claim from that session. Show the decoded quote (magic, type, nonce, clock info, firmware version, PCR digest) and the start of the raw attest/signature fields. Browser width about 1400 px; first screen.

and one of its results:

> **📷 SCREENSHOT NEEDED — `images/gs-result.png`**
> The `tpm2_validSignature` result of that quote claim: rule name, result **pass**, the message naming the AK, and the link back to the claim. Browser width about 1400 px.

## Attesting from the Web UI

**Attest** does by hand what `jp attest` does for every intent in its file.

1. Select the element and its endpoint (`tarzan`). **Attest now** on an element's page opens this page with the element already selected.
2. Select an intent, e.g. *System Information*.
3. Choose **Attest and Verify** and select rules, e.g. `sys_taRunningSafely`, `sys_machineID_Agent` and `sys_machineID_CrossRef`; or **Attest Only** to just collect a claim.
4. Press **Submit**. With Attest Only you go to the claim; otherwise to the session with its results.

> **📷 SCREENSHOT NEEDED — `images/gs-attest-form.png`**
> Web UI → **Attest**. The quick-start element selected, intent *System Information* (`std::intent::sys::info`), **Attest and Verify** chosen, rules `sys_taRunningSafely`, `sys_machineID_Agent` and `sys_machineID_CrossRef` ticked, before pressing **Submit**. Crop to the form; browser width about 1400 px.

Below the form, **Reference** lists every function's parameters and every rule, with the parameters it takes. **Intent Parameters** and **Rule Parameters** take JSON objects, e.g. `{"previouscid": "<claim item id>"}` for `tpm2_clockProgress`.

## Expected values

Rules such as `tpm2_attestedValue` and `sys_machineID_Agent` compare a claim with an **expected value**: the known-good values for one element, intent and endpoint. jp recorded them during `jp create`; `jp update` refreshes them after firmware or OS updates. **Structures → Expected Values** lists them.

> **📷 SCREENSHOT NEEDED — `images/gs-expectedvalue.png`**
> **Structures → Expected Values** → the expected value for the quick-start element and `std::intent::sha256::crtm::pcr0`. Show its element, intent, endpoint and the `evs` with `attestedValue` and `firmwareVersion`, and the results that used it. Browser width about 1400 px.

You can also create one by hand (**New → Expected Value**): choose the element, intent and endpoint, and fill in `evs` as the rule expects (see [Expected values](expectedvalues.md)). Attesting with **Attest Only** first shows the values to copy.

## Checking claims against each other

Some rules compare two claims of the same element, named by item ID in the rule parameters:

- `tpm2_uefiLogReplay` on a PCRs claim, with `uefilogcid`: the event log explains the PCRs;
- `tpm2_pcrDigest` on a quote, with `pcrscid`: the PCRs give the quote's digest;
- `tpm2_clockProgress` on a quote or gettime claim, with `previouscid`: no TPM reset went missing and the clock moved forwards;
- `tpm2_capabilitiesFirmware` on a capabilities claim, with `quotecid`: the reported firmware matches the signed one.

jp fills these in from the claims of the same run. To compare with an older session, copy a claim's item ID from its page and give it on the attest page, e.g. `{"previouscid": "..."}` with intent *TPM 2.0 Time* and rule `tpm2_clockProgress`. After a reboot the result reports one reset.

> **📷 SCREENSHOT NEEDED — `images/gs-clockprogress.png`**
> The result of `tpm2_clockProgress` on a *TPM 2.0 Time* claim, run from the attest page with `previouscid` set to a gettime or quote claim taken **before a reboot**: result **pass**, message "1 reset(s) (reboots)…". Browser width about 1400 px.

## Opaque objects

An opaque object gives a name to a value Jane might see: a PCR value, a hash, a key. Claim pages then show the name when the value appears (hover for the description). Create one with **New → Opaque Object**, e.g. the PCR 0 value of a known firmware version.

> **📷 SCREENSHOT NEEDED — `images/gs-opaqueobject.png`**
> A `tpm2/pcrs` claim page where a PCR 0 value is recognised as an opaque object: the highlighted value with its tooltip showing the object's name. Browser width about 1400 px; crop to the PCR table.

## The Live page and the log

The **Live** page (screen icon) shows Jane's log as it happens, with statistics for the last three hours. Your browser reads it straight from the MQTT broker over WebSockets (port 8081); if it stays empty, check the broker's WebSocket listener ([Quick start](quickstart.md#2-mosquitto)). Run `jp attest` while it is open.

> **📷 SCREENSHOT NEEDED — `images/gs-live.png`**
> The **Live** page during a `jp attest` run: new entries arriving at the top and the statistics chart. Browser width about 1400 px; full page.

**Structures → Log** shows the stored log for a period.

## Creating an element by hand

For a machine jp cannot run on, create the element with **New → Element** and fill in the template: name, description, endpoints (URL and protocol of its trust agent), tags, and the sections that apply (host, TPM with key handles and public keys, UEFI and IMA log locations). See [Elements](elements.md) for every field. Expected values can then be added by hand as above.
