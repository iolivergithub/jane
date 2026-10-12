# auth

Authentication and authorisation for Jane's Web UI and REST API.

All user, password, session and cookie handling lives here. The rest of Jane touches it in these places:

| File | Hook |
|---|---|
| `janeserver.go` | `auth.Initialise(*configFile)` after the datalayer is up |
| `services/webui/initwebapi.go` | `auth.RegisterEndpoints(router)` and `router.Use(auth.WebUIMiddleware())` |
| `services/restapi/initrestapi.go` | `router.Use(auth.RESTMiddleware())` |
| `services/webui/templates.go` | adds `auth.TemplateFuncs()` (`authEnabled`, `currentUser`) to the template functions |
| `services/webui/initwebapi.go` `Render` | binds `auth.RequestTemplateFuncs(c)` to a clone of the page, so `currentUser` is the request's user |

## Enabling

Add to `config.yaml` (no `auth:` section means no login, exactly as before):

```yaml
auth:
  web: true            # require a login for the Web UI
  rest: false          # require HTTP Basic auth for the REST API
  sessiontimeout: 8h   # sliding expiry of a Web UI login
```

Create users, or reset a password, with `etc/addjaneuser`. It prompts for the password; the database URL and name are the `database:` values from `config.yaml`:

```
etc/addjaneuser ian mongodb://127.0.0.1:27017 testasvrlocal
```

It needs the Python packages `argon2-cffi` and `pymongo` (`apt install python3-argon2 python3-pymongo`).

Passwords are stored as Argon2id hashes in PHC format (`$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`). Jane only verifies them, in `checkPassword` (`password.go`), reading the parameters from each stored hash. Anything else in the `password` field, including plain text, never matches, so users created before hashing need their password set again with `addjaneuser`.

## Behaviour

**Web UI.** Every route except `/login` and `/logout` needs a valid `jane_session` cookie, including the home page and unknown URLs.
- Without one, GET requests are redirected to `/login?next=...` and other methods get 401.
- The cookie is HttpOnly and SameSite=Lax, and Secure when the Web UI runs over HTTPS (`web.usehttp: false`).
- Sessions are held in memory, so restarting Jane logs everyone out.

**REST API.** HTTP Basic auth against the same users, active only when `rest: true`.
- `OPTIONS` (CORS preflight) and `/health` stay open.
- The provisioner and any scripts must send credentials once this is on.

**Logging.** Logins, failed logins and logouts go to Jane's normal log under channel `AUTH`.

**Authorisation.** The `Authoriser` interface decides whether an authenticated user may make a request; the decision is made on the method and the Echo route pattern. The default allows everything. Install a different policy with `auth.SetAuthoriser(...)`.

## Not covered

- The `/live` page talks directly to the MQTT broker over WebSockets, so that stream is protected only by broker credentials.
- The x3270 interface is not Echo-based and is not protected.
- There is no CSRF token. SameSite=Lax cookies stop cross-site POSTs in current browsers.
