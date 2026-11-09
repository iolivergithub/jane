# jp_extract — Jane element extractor (Go)

User documentation: [docs/jp_extract.md](../docs/jp_extract.md).

`jp_extract ELEMENTID FILE` copies an element and everything belonging to it (expected values, claims, results, sessions, log entries, the intents it used, other elements' relationships to it) from Jane's MongoDB into one JSON file with `metadata` and `extract` sections; `--delete` then removes them from Jane, asking first if Jane looks to be running.

## Code

| File | |
|---|---|
| `main.go` | command line and usage |
| `config.go` | Jane's `config.yaml`: database, the ports Jane listens on |
| `store.go` | the `store` interface (find by field, delete by `_id`, pull relationships) and its MongoDB implementation |
| `extract.go` | `collect`: what belongs to the element, exclusive and shared sessions, what `--delete` removes |
| `output.go` | metadata, writing the file, reading it back and checking it |
| `command.go` | the run, the Jane-running check and question, deletion |
| `extract_test.go` | tests against an in-memory `store` with MongoDB's matching through arrays |

Documents are handled as raw BSON throughout, not as janeserver's structures, so fields jp_extract does not know about are copied and deleted unchanged.

## Build and test

    make build
    go test ./...
