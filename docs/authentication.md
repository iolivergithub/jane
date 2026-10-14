# Authentication

Jane can require a login for each of its three interfaces separately: the Web UI, the REST API and the x3270 terminal interface. All three use the same users, held in Jane's MongoDB database. With no `auth:` section in `config.yaml`, nothing asks for a login.

Read [Security](security.md) as well: logins do not make Jane safe to expose, and two of the interfaces can carry passwords unencrypted.

## Switching it on

In `config.yaml` (`/etc/opt/jane/config.yaml` when installed from the deb package):

```yaml
auth:
  web: true            # require a login for the Web UI
  rest: false          # require HTTP Basic auth for the REST API
  x3270: true          # require a logon on the x3270 interface
  sessiontimeout: 8h   # a Web UI login, or an x3270 logon, expires after this long without use
```

Restart Jane after changing this section. At startup Jane logs which interfaces need a login, and warns if one is enabled but there are no users, or if the Web UI needs a login while the x3270 interface does not.

The Web UI home page has an **Authentication** tab showing the current settings, the number of users and the number of Web UI logins in progress.

## Users

Add a user, or set a new password for one, with `addjaneuser`:

| Installed from | Command |
|---|---|
| deb package | `/opt/jane/addjaneuser` |
| source | `etc/addjaneuser/addjaneuser` |

```bash
/opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

The second and third arguments are the `database: connection` and `database: name` values from `config.yaml` (`asvr` in the packaged configuration). The password is asked for twice and never appears on the command line; to script it, pipe it in on standard input.

The script needs Python 3 with `argon2-cffi` and `pymongo`. The deb package recommends them; otherwise install them with `apt install python3-argon2 python3-pymongo`. More detail is in the script's [README](../etc/addjaneuser/README.md).

- Passwords are stored as Argon2id hashes. Jane only verifies them; it never stores a password.
- Anyone who can run `addjaneuser` against the database can reset any password; the old one is not needed.
- There is no delete command: remove the user from the `users` collection, e.g. in `mongosh`: `db.users.deleteOne({username: "ian"})`.
- Jane does not need a restart after users change.
- All users can do everything; there are no roles yet.

## Web UI

With `web: true`, every page except the login page needs a login.

- Logging in sets a session cookie. It is HttpOnly and SameSite=Lax, and marked Secure when the Web UI runs over HTTPS (`web: usehttp: false`).
- The login expires after `sessiontimeout` without use. Logins are kept in memory, so restarting Jane logs everyone out.
- The logged-in user is shown in the navigation bar, with a logout button.
- The **Live** page reads MQTT directly from the broker over WebSockets, so that stream is protected only by the broker's own settings.

Over plain HTTP (`usehttp: true`) the password crosses the network unencrypted.

## REST API

With `rest: true`, REST requests need HTTP Basic authentication with a Jane user, e.g.

```bash
curl -u ian https://jane.example:8520/elements
```

- `/health` and CORS preflight (`OPTIONS`) requests stay open.
- Anything calling the API must then send credentials. **The provisioner (`jp`) does not do this yet**, so it cannot be used while `rest: true`.
- Basic authentication sends the password with every request, so use HTTPS (`rest: usehttp: false`).

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

Web UI and x3270 logins, failed logins and logoffs go to Jane's normal log (database, MQTT and log file) on the `AUTH` channel, with the client's address. x3270 entries say how the session ended: logoff, exit, disconnected or timed out. REST requests are not logged individually.
