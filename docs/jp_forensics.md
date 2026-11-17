# jp_forensics, element forensics

`jp_forensics` reads one element's history from Jane's REST API and writes it as a single HTML page: a timeline of the element, its expected values, the intents it used, its claims, results, sessions and log entries, with every change marked. It answers questions such as *when did PCR 7 change?*, *when was Secure Boot turned off?*, *which `jp update` changed this expected value, and when did a rule first fail after it?*

```bash
jp_forensics [--from TIME] [--until TIME] [--server URL] [--apikeyfile PATH]
             [--out FILE] [--max N] [--nodocuments] ELEMENTID
```

It is installed with Jane: `/opt/jane/jp_forensics` from the jane deb (amd64, arm64, s390x), `jp_forensics.exe` in the Windows Jane zip. It needs only the REST API, so it can be run anywhere Jane can be reached.

jp_forensics only reads (HTTP GET). It changes nothing in Jane and writes nothing into Jane's log. It never overwrites a file.

## Examples

```bash
export JANE_SERVER=http://jane:8520
export JANE_API_KEY=jane_...                 # if Jane's REST API needs a key

jp_forensics 0b8e...                         # the element's whole history
jp_forensics 0b8e... --from 7d               # the last seven days
jp_forensics 0b8e... --from 2026-11-01 --until 2026-11-10
jp_forensics 0b8e... --until 2026-11-10      # from 10 November until now
jp_forensics 0b8e... --from 36h --out /tmp/incident-42.html
```

## Options

| Option | |
|---|---|
| `ELEMENTID` | the element's itemid |
| `--from TIME`, `--until TIME` | the two ends of the time range. An end that is not given is **now**; the range runs from the earlier to the later of the two, whichever way round they are given. With neither, the report covers the element's whole history |
| `--server URL` | Jane's REST API, e.g. `http://jane:8520`; else `JANE_SERVER` |
| `--apikeyfile PATH` | read the REST API key from PATH; overrides `JANE_API_KEY`. Needed if Jane's REST API requires a key (`auth: rest: true`), see [Authentication](authentication.md) |
| `--out FILE` | the page to write; default `jp_forensics_<ELEMENTID>_<UTC time>.html` in the current directory. `-` writes to standard output |
| `--max N` | the most claims, results and log entries asked of Jane per list (default 100000). The report warns when a list may have been cut short |
| `--nodocuments` | leave the documents themselves (claims, results, ...) out of the page, keeping the timeline and the differences: a much smaller page when claims hold large event logs |

A `TIME` is one of:

| Form | Example | Meaning |
|---|---|---|
| `now` | | now |
| a duration ago | `90m`, `36h`, `7d`, `2w` | that long before now |
| a date | `2026-11-17` | the start of that day, local time; as the **later** end of the range, the end of that day |
| a date and time | `2026-11-17T09:30`, `2026-11-17 09:30:00` | local time |
| RFC 3339 | `2026-11-17T09:30:00Z`, `2026-11-17T09:30:00+02:00` | that zone |

Exit status: 0 success; 1 Jane unreachable or refusing, element not found, the output file exists; 2 bad command line.

## What is read

| From Jane | REST | |
|---|---|---|
| the element | `GET /element/:id` | as it is now |
| its expected values | `GET /expectedValues/element/:id`, `GET /expectedValue/:id` | as they are now |
| its claims | `GET /claims/element/:id?max=N`, `GET /claim/:id` | in the range, and a baseline before it (below) |
| its results | `GET /results/element/:id?max=N`, `GET /result/:id` | in the range, and a baseline before it |
| the intents used | `GET /intent/:id` | as they are now; those named by the claims and expected values |
| sessions | `GET /session/:id` | those of the claims and results in the range |
| log entries | `GET /log/since?duration=...`, or `GET /log?max=N` for the whole history | those whose `refid` is the element, one of its expected values, or a claim, result or session in the range |

Log entries about intents are left out, as in [jp_extract](jp_extract.md): intents are shared with other elements.

**Baselines.** So that the first claims and results in the range are compared with something, jp_forensics also reads, going back in time from the start of the range, the latest earlier claim for the element copy, for each intent and for each series that appears in the range, and likewise the latest earlier result for each result series and each expected value. It stops as soon as it has them. Baselines are used for the comparisons and included as documents, but are not shown as events.

## What is compared

A claim holds copies of the element and the intent as they were when it was made, and a result a copy of the expected value it was checked against (see [Claims](claims.md) and [Results](results.md)). jp_forensics compares each copy with the one before it, value by value:

| Change | In | Compared with |
|---|---|---|
| `element` | each claim's element copy | the previous claim, of any intent |
| `intent` | each claim's intent copy | the previous claim of the same intent |
| `body` | each claim's body: PCR values, UEFI variables, event logs, IMA logs, system information, ... | the previous claim of the same intent on the same endpoint (a **series**) |
| `endpoint` | each claim's endpoint | the previous claim in its series |
| `outcome` | each result's value (`0 Success`, `9001 Fail`, ...) | the previous result of the same rule, intent and endpoint |
| `expectedvalue` | each result's expected value copy | the previous result with the same expected value |

If the range reaches now, the element, intents and expected values as they are in Jane are also compared with their last copies ("now" events), which shows a change made since the last attestation, for instance by `jp update`. The element's and expected values' `recordhistory` (created, last updated) appear as events too.

Values are compared leaf by leaf, by path: `sha256.7` is PCR 7 of the SHA-256 bank, `efivars[name=SecureBoot].value` the SecureBoot variable. Arrays whose items are objects with distinct names (UEFI variables, for instance) are matched by name, with the GUID as well where names repeat, so a variable that moves in the list is not reported as changed; other arrays are matched by position. A value longer than 1024 bytes is shortened in the report, with its full length and SHA-256.

Some values change on every claim by design, such as the nonce and signature of a TPM quote. Hide them in the page with **Ignore paths** rather than in jp_forensics, so that nothing is lost from the report.

jp_forensics, like Jane, makes no trust decisions: it shows what changed and when.

## The page

The page is one self-contained HTML file: no scripts, styles or fonts from elsewhere, and it makes no requests, so it can be opened offline, archived or attached to an incident report. It has:

- **Timeline**: every event in time order, grouped by day. Each claim and result shows tags for what changed (`body: 3 changed`, `element: 2 changed`, `outcome: 1 changed`) or `first` where there was nothing earlier in the report to compare with. Click an event to see its details, the differences (before and after, with the differing characters of equal-length values such as digests marked), links to related claims, results, sessions and expected values, and the documents themselves.
- **Series**: each intent on each endpoint, with one box per claim (unchanged, changed, first, error), and each rule with one box per result (success, fail, other; outcome and expected value changes marked). Click a box to go to the event.
- **Value history**: for the element, an intent, a claim series, a result series or an expected value, every path that changed and each change in order, starting from the value before the first change. This is where to look for *when did PCR 7 change, and to what?*
- **About**: what was compared, and the report's details (Jane's address, range, counts, the version of jp_forensics).

The filters apply to every view:

| Filter | |
|---|---|
| Search | case-insensitive; every word must match (`"quoted phrase"` matches exactly); matches titles, item IDs, intent, endpoint and rule names, changed paths and their values |
| Type chips | element, intent, expected value, claim, result, session, log; double-click a chip to show only that type |
| Intent, endpoint, rule, outcome | `Not success` is any result other than 0; `Errors` are 9010 and above |
| Only changes | events with at least one change that is not ignored (with *incl. first*, also those with nothing to compare with) |
| Ignore paths | a regular expression matched against each difference's part and path, such as `body:quote`, `element:recordhistory.lastupdated`; `nonce\|signature` hides those wherever they are |
| From, until | narrows the time shown, within the report's range |
| UTC / Local time, Oldest / Newest first | display only |

**CSV** downloads the events shown. **Copy link** on an event gives a link that opens the page at that event.

Large reports show the first 1500 events and a button for more. Claims with large bodies (UEFI or IMA event logs) make large pages; `--nodocuments` leaves the documents out.

## Code

`forensics/` in the repository; see its [README](../forensics/README.md).
