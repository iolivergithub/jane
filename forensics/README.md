# jane-forensics — Jane element forensics (Go)

User documentation: [docs/jane-forensics.md](../docs/jane-forensics.md).

`jane-forensics ELEMENTID [--from TIME] [--until TIME]` reads an element and everything belonging to it (expected values, the intents it used, claims, results, sessions, log entries) through Jane's REST API, compares each claim's element, intent, endpoint and body (as it is, and decoded as Jane's rules read it), and each result's outcome and expected value, with the one before it, and writes the lot as one self-contained HTML page with a searchable, filterable timeline, series and value histories. It only ever sends GET requests.

## Code

| File | |
|---|---|
| `main.go` | command line and usage |
| `timerange.go` | `--from` / `--until`: times, durations ago, the range |
| `client.go` | the REST client (GET only, API key as for jp) and `doc`, a document with exact numbers (Jane's timestamps are nanoseconds) |
| `collect.go` | what is read: the documents in the range and the baselines before it |
| `diff.go` | value-by-value comparison by path; arrays of named objects matched by name |
| `views.go` | claim bodies decoded as the rules read them (TPM quote and time, UEFI event log via `lib/uefi`, UEFI variables, IMA log), volatile values, PCR and expected value names |
| `summary.go` | the summary in words of each change set |
| `timeline.go` | events, change sets, series, links: the report |
| `command.go` | the run; the report put into `report.html` |
| `report.html` | the page (HTML, CSS and JavaScript, no external resources), embedded in the binary; `/*JANE_FORENSICS_REPORT*/null` is replaced by the report as JSON |
| `fakejane_test.go` | a fake Jane REST API and a story: an element whose PCR 7, SecureBoot variable, description, intent and expected value change over three days, with quotes (a reboot), IMA logs and sys/info |
| `forensics_test.go`, `units_test.go`, `views_test.go` | tests; `views_test.go` decodes `lib/uefi/testdata`'s event log |

Documents are handled as decoded JSON (`map[string]any`, numbers as `json.Number`), not as janeserver's structures, so fields jane-forensics does not know about are compared and shown too.

## Build and test

    make build
    go test ./...
