# auth

Authentication and authorisation for Jane's Web UI and REST API.

All user, password, session and cookie handling lives here. The rest of Jane touches it in four places:

| File | Hook |
|---|---|
| `janeserver.go` | `auth.Initialise(*configFile)` after the datalayer is up |
| `services/webui/initwebapi.go` | `auth.RegisterEndpoints(router)` and `router.Use(auth.WebUIMiddleware())` |
| `services/restapi/initrestapi.go` | `router.Use(auth.RESTMiddleware())` |
| `services/webui/templates.go` | `"authEnabled": auth.WebEnabled`, used by `base.html` to show the logout button |

## Enabling

Add to `config.yaml` (no `auth:` section means no login, exactly as before):

```yaml
auth:
  web: true            # require a login for the Web UI
  rest: false          # require HTTP Basic auth for the REST API
  sessiontimeout: 8h   # sliding expiry of a Web UI login
```

Create users directly in Mongo for now:

```
db.users.insertOne({username: "ian", password: "secret"})
```

Passwords are plain text at the moment. Only `checkPassword` in `users.go` knows this, so hashing can be added there.

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
