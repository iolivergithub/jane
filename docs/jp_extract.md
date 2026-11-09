# jp_extract, the element extractor

`jp_extract` copies one element out of Jane's database, with everything that belongs to it, into a single JSON file. With `--delete` it then removes those items from Jane. It is the only way to delete an element's history: Jane itself deletes nothing, because deleting could disrupt digital forensics.

```bash
jp_extract [--delete] [--config PATH] [--mongodb URL] [--database NAME] [--server URL] ELEMENTID FILE
```

It is installed with Jane: `/opt/jane/jp_extract` from the jane deb (amd64, arm64, s390x), `jp_extract.exe` in the Windows zip. `ELEMENTID` is the element's itemid. `FILE` must not exist yet: jp_extract never overwrites a file.

jp_extract reads and writes MongoDB directly, as `addjaneuser` does. Jane's REST API cannot read log entries by element and has no way to delete claims, results or log entries. jp_extract writes nothing into Jane's log.

## What is extracted

| Collection | Documents |
|---|---|
| `elements` | the element |
| `expectedvalues` | expected values with `elementid` = the element |
| `intents` | the intents named by those expected values (`intentid`) and by the element's claims (`header.intent.itemid`): the current documents |
| `claims` | claims with `header.element.itemid` = the element |
| `results` | results with `elementid` = the element, or whose `claimid` is one of those claims |
| `sessions` | sessions listing any of those claims or results, named in their session copies, or opened by an attestation profile run of the element |
| `log` | log entries whose `refid` is the element or any of its expected values, claims, results or sessions (not intents, which are shared) |
| `relationshipsTo` | the relationships other elements have to the element (not a collection: see below) |

Opaque objects are not extracted.

A session is **exclusive** if every claim and result it lists belongs to the element, and **shared** if it also lists another element's. Both kinds are extracted.

## --delete

After the file is written, jp_extract reads it back and checks the counts, hashes and every document it is about to delete. Only then does it delete, in this order: results, claims, exclusive sessions, log entries, expected values, other elements' relationships to the element, and the element last. A crash part way through leaves the element visible, and a second run with a new file picks up what is left.

| Removed | Kept |
|---|---|
| the element, its expected values, claims and results | intents, always, since other elements usually share them |
| other elements' relationships to the element (recorded under `relationshipsTo`) | those other elements, otherwise unchanged |
| exclusive sessions | shared sessions; they keep the IDs of the deleted claims and results, which no longer resolve |
| log entries about the deleted items | log entries about shared sessions; the copies in `jane.log` and on MQTT, which cannot be withdrawn |

Sessions that were not extracted but have a deleted session as `parent` keep it, so their parent no longer resolves. They are listed in the metadata under `sessions.children`.

**Jane should be stopped.** jp_extract checks whether Jane is running: something listening on Jane's REST, Web UI or x3270 ports (taken from `config.yaml`, with a wildcard `listenon` tried on 127.0.0.1) or at `--server`, or systemd reporting `jane` active. Without a configuration file or `--server` it cannot tell. In either case it prints a warning and asks

```
Jane is still running, continue? [yes/no]
```

Only `yes` typed in full goes on. An empty answer asks again; anything else, or the end of the input, stops with nothing written or deleted. A Jane left running may add claims, results or log entries during the deletion; jp_extract then reports that it did not remove what was planned.

**Relationships.** Relationships other elements have to the element (`hosted-on`, `runs-on`) are removed from those elements, which are otherwise unchanged. They are recorded in the file under `relationshipsTo`, extracted with or without `--delete`. The element's own relationships are in its document and go with it.

## The database

The connection string and database name come from Jane's `config.yaml` (`--config`, default `/etc/opt/jane/config.yaml` if it exists). `--mongodb` and `--database` override them. A MongoDB login comes from `JANE_MONGODB_USERNAME`, `JANE_MONGODB_PASSWORD` and `JANE_MONGODB_AUTHSOURCE`, as for Jane. On a deb install, as root:

```bash
systemctl stop jane
set -a; . /etc/opt/jane/jane.env; set +a
/opt/jane/jp_extract --delete 3f2a...e1 /var/tmp/host1-extract.json
```

## The file

```json
{
  "metadata": { ... },
  "extract": {
    "elements": [ ... ],
    "expectedvalues": [ ... ],
    "intents": [ ... ],
    "claims": [ ... ],
    "results": [ ... ],
    "sessions": [ ... ],
    "log": [ ... ],
    "relationshipsTo": [
      {"elementid": "<other element>", "name": "vm1", "relationshiptype": "hosted-on", "itemid": "<this element>"}
    ]
  }
}
```

A `relationshipsTo` entry says that element `elementid` (named `name`) had `{"relationshiptype": ..., "itemid": ...}` in its `relationships` list. Each document of the collections is written as MongoDB canonical Extended JSON v2 (`{"$oid": ...}`, `{"$numberLong": ...}`, `{"$binary": ...}`), so it converts back to exactly the BSON that was in the database, `_id`, signed footers and integer types included. The file is created with mode 0600.

`metadata` holds:

| Key | |
|---|---|
| `tool`, `version`, `build`, `formatversion` | what wrote the file (format 1) |
| `extractid` | a UUID4 for this extract |
| `operation` | `extract` or `extract+delete` |
| `element` | `itemid` and `name` |
| `started`, `finished` | UTC, RFC 3339: when reading the database began and ended |
| `host`, `operator`, `system`, `config`, `database`, `connection` | where it ran, the OS user, Jane's `system: name`, the configuration file, the database, and the connection string without any password |
| `collections` | for each section of the extract, `relationshipsTo` included: `count`, `bytes` (of its entries) and `sha256` |
| `totals` | entries and bytes over all sections |
| `sessions` | `exclusive`, `shared` and `children` session IDs, as above |
| `missingintents` | intents the element used that are no longer in Jane |
| `delete` | with `--delete` only: the documents removed per collection (`remove`), `relationshipsremoved`, and what is `kept` |

A section's `sha256` is taken over each of its entries exactly as written in the file, each followed by a newline. jp_extract signs nothing; to sign an extract, sign the file.

## Exit status

| | |
|---|---|
| 0 | success |
| 1 | error: element not found, database unreachable, `FILE` exists, no `yes` to the question (`--delete`), the file did not read back, or a deletion that did not remove what was planned |
| 2 | bad command line |

## Building

```bash
cd extractor
make build                 # or make binary OS=... ARCH=... as for jp; see Compiling
go test ./...              # needs no MongoDB: the tests use an in-memory store
```
