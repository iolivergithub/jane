# jp_forensics — Jane element forensics (Go)

User documentation: [docs/jp_forensics.md](../docs/jp_forensics.md).

`jp_forensics ELEMENTID [--from TIME] [--until TIME]` reads an element and everything belonging to it (expected values, the intents it used, claims, results, sessions, log entries) through Jane's REST API, compares each claim's element, intent, endpoint and body, and each result's outcome and expected value, with the one before it, and writes the lot as one self-contained HTML page with a searchable, filterable timeline, series and value histories. It only ever sends GET requests.

## Code

| File | |
|---|---|
| `main.go` | command line and usage |
| `timerange.go` | `--from` / `--until`: times, durations ago, the range |
| `client.go` | the REST client (GET only, API key as for jp) and `doc`, a document with exact numbers (Jane's timestamps are nanoseconds) |
| `collect.go` | what is read: the documents in the range and the baselines before it |
| `diff.go` | value-by-value comparison by path; arrays of named objects matched by name |
| `timeline.go` | events, change sets, series, links: the report |
| `command.go` | the run; the report put into `report.html` |
| `report.html` | the page (HTML, CSS and JavaScript, no external resources), embedded in the binary; `/*JP_FORENSICS_REPORT*/null` is replaced by the report as JSON |
| `fakejane_test.go` | a fake Jane REST API and a story: an element whose PCR 7, SecureBoot variable, description, intent and expected value change over three days |
| `forensics_test.go`, `units_test.go` | tests |

Documents are handled as decoded JSON (`map[string]any`, numbers as `json.Number`), not as janeserver's structures, so fields jp_forensics does not know about are compared and shown too.

## Build and test

    make build
    go test ./...
