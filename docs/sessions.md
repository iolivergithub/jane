# Sessions

A session groups the claims and results of one piece of work: an attestation from the attest page, a jp run, or a series of REST calls. It is opened, claims and results are added to it, and it is closed.

| Field | Meaning |
|---|---|
| `itemid` | the session's identifier |
| `timing` | `opened` and `closed`, nanoseconds since 1970 (UTC); `closed` is 0 while open |
| `claimlist`, `resultlist` | item IDs of the claims and results added, in order |
| `message` | a free-text description given when it was opened |
| `footer` | hash and signature, made when the session is closed (see [Signing](signing.md)) |

Over REST: `POST /session` opens one (with a `message`), `DELETE /session/<id>` closes it, and the attest and verify calls name the session to add to. The Web UI's **Sessions** page lists them, newest first, with their claims and results.

Each claim and result also carries a copy of its session, so they can be traced back even without the session's lists. Adding many claims to one session at the same moment from several REST clients can lose entries from the lists (#90); Jane's own interfaces and jp add one at a time.

The Go definition is `janeserver/structures/sessions.go`.
