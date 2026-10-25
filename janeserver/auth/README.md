# auth

Authentication and authorisation for Jane's Web UI, REST API and x3270 interface. User documentation: [docs/authentication.md](../../docs/authentication.md).

All user, password, session and cookie handling lives here. The rest of Jane touches it in these places:

| File | Hook |
|---|---|
| `janeserver.go` | `auth.Initialise(*configFile)` after the datalayer is up |
| `services/webui/initwebapi.go` | `auth.RegisterEndpoints(router)` and `router.Use(auth.WebUIMiddleware())` |
| `services/restapi/initrestapi.go` | `router.Use(auth.RESTMiddleware())` |
| `services/webui/templates.go` | adds `auth.TemplateFuncs()` (`authEnabled`, `currentUser`) to the template functions |
| `services/webui/initwebapi.go` `Render` | binds `auth.RequestTemplateFuncs(c)` to a clone of the page, so `currentUser` is the request's user |
| `services/x3270/session.go` | `auth.X3270Enabled()`, `auth.Authenticate(...)`, `auth.LogLogoff(...)`, `auth.SessionTimeout()` |

## Enabling

Add to `config.yaml` (no `auth:` section means no login, exactly as before):

```yaml
auth:
  web: true            # require a login for the Web UI
  rest: false          # require an API key for the REST API
  x3270: false         # require a logon on the x3270 interface
  sessiontimeout: 30m  # sliding expiry of a Web UI login; idle timeout of an x3270 logon
```

Create users, or reset a password, with `etc/addjaneuser/addjaneuser` (installed as `/opt/jane/addjaneuser` by the deb package). It prompts for the password; the database URL and name are the `database:` values from `config.yaml`:

```
etc/addjaneuser/addjaneuser ian mongodb://127.0.0.1:27017 testasvrlocal
```

It needs the Python packages `argon2-cffi` and `pymongo` (`apt install python3-argon2 python3-pymongo`).

Passwords are stored as Argon2id hashes in PHC format (`$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`). Jane only verifies them, in `checkPassword` (`password.go`), reading the parameters from each stored hash. Anything else in the `password` field, including plain text, never matches, so users created before hashing need their password set again with `addjaneuser`.

## Behaviour

**Web UI.** Every route except `/login` and `/logout` needs a valid `jane_session` cookie, including the home page and unknown URLs.
- Without one, GET requests are redirected to `/login?next=...` and other methods get 401.
- The cookie is HttpOnly and SameSite=Lax, and Secure when the Web UI runs over HTTPS (`web.usehttp: false`).
- Sessions are held in memory, so restarting Jane logs everyone out.

**REST API** (`restkeys.go`, #89). API keys, active only when `rest: true`: `Authorization: Bearer jane_<43 base64url characters>`.
- The format is checked first; then the key's SHA-256 (`sha256:<hex>`) is looked up in the users' `apikey` field (`UserStore.UserForAPIKey`). Keys are made by addjaneuser; one per user.
- HTTP Basic gets 401 with a message that it is no longer supported.
- `OPTIONS` (CORS preflight) and `/health` stay open.
- REST and Web UI are separate in code; they share only the user store and the failed-attempt limiter.

**Failed attempts** (`ratelimit.go`). One counter per client address for all three interfaces: 5 failures in 5 minutes block the address for 5 minutes (REST 429; Web UI and x3270 the usual failure, the log says why). Routers use `echo.ExtractIPDirect()`, so `X-Forwarded-For` is ignored.

**Password checks** (`password.go`). At most 2 Argon2id checks at once (5 s wait, then they fail and log `login/BUSY`); stored hashes may ask for at most 64 MiB.

**Users** (`users.go`). `UserStore` is the only code that reads the `users` collection (`username`, `password`, `apikey`). Another identity source (#48) would be another implementation. The Web UI ends a login whose user has been deleted.

**x3270.** With `x3270: true` a terminal starts at a logon screen and needs a user ID and password from the same users collection.
- `auth.Authenticate` checks them and logs the attempt; after 3 failures the connection is closed.
- A logged-on terminal idle for `sessiontimeout` is disconnected. Logoff, exit, disconnection and timeout are logged with `auth.LogLogoff`.
- tn3270 is not encrypted, so the password crosses the network in clear.
- `auth.Initialise` warns when `web: true` but `x3270: false`, when REST needs a key but no user has one, and when logins or keys go over plain HTTP.

**Logging.** Logins, failed logins and logouts go to Jane's normal log under channel `AUTH`.

**Authorisation.** The `Authoriser` interface decides whether an authenticated user may make a request; the decision is made on the method and the Echo route pattern. The default allows everything. Install a different policy with `auth.SetAuthoriser(...)`.

## Not covered

- The `/live` page talks directly to the MQTT broker over WebSockets, so that stream is protected only by broker credentials.
- The x3270 interface is unencrypted; bind it to `127.0.0.1` (`x3270: listenon:`) and reach it through an SSH tunnel if passwords matter.
- jp sends its key from `JANE_API_KEY` or `--apikeyfile` (since 1.5.5).
- Failed REST requests are logged with `logEntry("AUTH", "login/FAIL", ...)` and the client address; successful key use once an hour per user and address.
- There is no CSRF token. SameSite=Lax cookies stop cross-site POSTs in current browsers.
