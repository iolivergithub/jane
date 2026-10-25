# Authentication

Jane can require authentication for each of its three interfaces separately: a login with username and password for the Web UI and the x3270 terminal interface, and an **API key** for the REST API (since 1.5.5). All three use the same users, held in Jane's MongoDB database. With no `auth:` section in `config.yaml`, nothing asks for a login.

Read [Hardening](hardening.md) as well: logins do not make Jane safe to expose on their own, and two of the interfaces can carry passwords unencrypted.

## Switching it on

In `config.yaml` (`/etc/opt/jane/config.yaml` when installed from the deb package):

```yaml
auth:
  web: true            # require a login for the Web UI
  rest: false          # require an API key for the REST API
  x3270: true          # require a logon on the x3270 interface
  sessiontimeout: 30m  # a Web UI login, or an x3270 logon, expires after this long without use (default 30m)
```

Restart Jane after changing this section. At startup Jane logs which interfaces need a login, and warns if one is enabled but there are no users, if the REST API needs a key but no user has one, if the Web UI needs a login while the x3270 interface does not, or if logins or keys would cross the network over plain HTTP.

The Web UI home page has an **Authentication** tab showing the current settings, the number of users, the number of users with an API key and the number of Web UI logins in progress.

## Users

Add a user, set a new password, or make or revoke an API key with `addjaneuser`:

| Installed from | Command |
|---|---|
| deb package | `/opt/jane/addjaneuser` |
| source | `etc/addjaneuser/addjaneuser` |

```bash
/opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

The second and third arguments are the `database: connection` and `database: name` values from `config.yaml` (`asvr` in the packaged configuration). The password is asked for twice and never appears on the command line; to script it, pipe it in on standard input.

The script needs Python 3 with `argon2-cffi` and `pymongo`. The deb package recommends them; otherwise install them with `apt install python3-argon2 python3-pymongo`. If MongoDB itself needs an account, addjaneuser reads it from `JANE_MONGODB_USERNAME`/`JANE_MONGODB_PASSWORD` (or `--dbuser`/`--dbpass`); see [Running](running.md#addjaneuser).

- Passwords are stored as Argon2id hashes, API keys as SHA-256 hashes. Jane only verifies them; it never stores a password or a key.
- A user may have a password (Web UI, x3270), an API key (REST), or both. A user record holds `username`, `password` and `apikey`.
- Anyone who can run `addjaneuser` against the database can reset any password or key; the old one is not needed.
- There is no delete command: remove the user from the `users` collection, e.g. in `mongosh`: `db.users.deleteOne({username: "ian"})`.
- Jane does not need a restart after users change. A deleted user's Web UI login ends at their next request, and a deleted user's or revoked API key stops working at once.
- All users can do everything; there are no roles yet.

## Web UI

With `web: true`, every page except the login page needs a login.

- Logging in sets a session cookie. It is HttpOnly and SameSite=Lax, and marked Secure when the Web UI runs over HTTPS (`web: usehttp: false`).
- The login expires after `sessiontimeout` without use. Logins are kept in memory, so restarting Jane logs everyone out.
- The logged-in user is shown in the navigation bar, with a logout button.
- The **Live** page reads MQTT directly from the broker over WebSockets, so that stream is protected only by the broker's own settings.

Over plain HTTP (`usehttp: true`) the password crosses the network unencrypted.

## REST API

With `rest: true`, every REST request needs an **API key**, sent in the standard header:

```bash
curl -H "Authorization: Bearer jane_...43 characters..." https://jane.example:8520/elements
```

**Making a key.** Keys are made by addjaneuser, never by hand:

```bash
/opt/jane/addjaneuser --apikey cluster-a mongodb://127.0.0.1:27017 asvr
```

prints a new key, alone on standard output (so `KEY=$(addjaneuser --apikey ...)` captures it). It is shown only once; only its SHA-256 is stored. The user is created if it does not exist; a user with only a key cannot log in to the Web UI. Each user has **at most one key**: making a new one replaces the old, which stops working at once. `addjaneuser --revokeapikey cluster-a ...` removes the key and keeps the user.

**What a key looks like.** `jane_` followed by 43 characters (32 random bytes, base64url). Jane refuses anything else before even looking it up, so a key cannot be made up or chosen by hand: only keys from addjaneuser work. The `jane_` prefix also makes a leaked key easy to recognise.

**Sharing a key.** Typically one user and its key are shared by all the machines of a cluster (each machine's jp sends the same key). That is convenient, with two consequences:

- If one machine leaks the key, the whole cluster's key has to be replaced.
- Replacing the key cuts off every machine using it until the new one is deployed. To change keys without an outage, create a second user with its own key, roll that key out, then delete the old user.

Jane's log records which address used which user's key (below), so machines sharing a key can still be told apart.

**jp** sends the key from `JANE_API_KEY`, or from a file given with `--apikeyfile` (see [jp](jp.md)). A convenient convention for a machine: the key alone in `/etc/opt/jane/jp.key`, owned by root with the group of the users who run jp, mode 640, deployed e.g. by Ansible:

```bash
sudo install -m 640 -o root -g jpusers /dev/stdin /etc/opt/jane/jp.key <<< "$KEY"
/opt/jane/jp attest --apikeyfile /etc/opt/jane/jp.key /etc/opt/jane/provisioning.yaml
```

The packages do not create this file; the jane package's install script leaves `jp.key` alone.

**Details**

- `/health` and CORS preflight (`OPTIONS`) requests stay open.
- A request without a key, with a malformed or unknown key, or with a revoked one gets 401.
- **HTTP Basic is no longer accepted** (it was until 1.5.4): a request with a username and password gets 401 with a message saying so. jp and Jane must both be 1.5.5 or later.
- The key travels with every request, so use HTTPS (`rest: usehttp: false`). Jane warns at startup when REST keys would cross the network over plain HTTP.
- Checking a key costs one SHA-256 and one database read: no slow password hashing, so many machines running jp at once are no problem for logins.
- With `rest: false` no key is needed, and a key that is sent is ignored.

## Failed attempts

Failed attempts are counted per client address, together for all three interfaces: a wrong API key, a wrong Web UI password, a wrong x3270 password. After **5 failures within 5 minutes** the address is refused for **5 minutes**: the REST API answers 429 with a `Retry-After` header; the Web UI and x3270 show the usual "login failed" message, and the log records the real reason. Machines sharing an address (e.g. behind NAT) share the count.

The client address is always the one the connection comes from. `X-Forwarded-For` and `X-Real-IP` headers are ignored, since any client can set them; if Jane runs behind a reverse proxy, every request appears to come from the proxy.

Password checks (Web UI, x3270) are deliberately slow and use memory (Argon2id, about 30 ms and 40 MiB each). At most two run at once; a login that cannot get a turn within 5 seconds fails with the usual message and the log says Jane was busy. Stored hashes asking for more than 64 MiB are refused.

## x3270 interface

With `x3270: true`, a terminal connecting to Jane's tn3270 port (default 3270) gets a logon screen asking for a user ID and password.

- After three failed attempts the connection is closed; there is a one-second pause after each failure.
- A logged-on terminal left idle for `sessiontimeout` is disconnected.
- Row 21 of every screen is a status line: the user, the logon time (UTC) and duration, the terminal type, the client address and the system name.
- On the Primary Option Menu, **L** logs off (back to the logon screen) and **X** exits (disconnects). F3 and F12 go back one screen everywhere else, but do nothing on the menu, so a stray F3 cannot log you off.

**tn3270 is not encrypted**: the password crosses the network in clear. If this matters, bind the interface to the local machine and reach it through an SSH tunnel:

```yaml
x3270:
  port: 3270
  listenon: 127.0.0.1
```

```bash
ssh -L 3270:127.0.0.1:3270 jane.example     # then connect the terminal to localhost:3270
```

With `x3270: false` the interface starts at the Primary Option Menu, as it did before 1.5.1.

## What is logged

On the `AUTH` channel of Jane's normal log (database, MQTT and log file), with the client's address:

- Web UI and x3270 logins, failed logins and logoffs; x3270 entries say how the session ended (logoff, exit, disconnected, timed out). A Web UI login ended because the user was deleted says so.
- Failed REST requests: every unknown or malformed key; requests without a key, or with HTTP Basic, once an hour per address.
- Successful REST key use: the first request of each user from each address in an hour (not every request).
- Addresses refused after too many failures, and password checks not done because Jane was busy.
