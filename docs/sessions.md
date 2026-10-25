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

Each claim and result also carries a copy of its session, so they can be traced back even without the session's lists.

Claims and results are added to a session one at a time and atomically, so several clients may add to the same session at once without losing entries. Once a session is closed nothing more can be added: an attest or verify naming a closed session still stores its claim or result, but returns an error saying it was not added to the session. The signed footer of a closed session always covers exactly the lists stored with it.

The Go definition is `janeserver/structures/sessions.go`.
