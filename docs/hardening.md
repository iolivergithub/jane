# Hardening

The [Quick start](quickstart.md) leaves Jane open: HTTP everywhere, no logins, anonymous database and broker, a TLS key that is published on GitHub, and a signing key that changes at every restart. This page closes those one at a time. The steps are independent: do the ones that matter for your setup, in any order, and check each before moving on.

## Read this first

- **Jane is research software.** Error handling is incomplete in places and it has not had a security review. Use in production: don't, yet.
- The TLS key and certificate shipped as `REPLACE_ME.key`/`.crt` (and `temporary.key`/`.crt` in the source tree) have been public for years. They are placeholders; never rely on them.
- tarzan speaks plain HTTP and has no authentication of its own (issue #58). Anyone who can reach its port can ask it for quotes and logs.
- Never run tarzan with `-unsafe`.

On a deb install, after editing `/etc/opt/jane/config.yaml` or `/etc/opt/jane/jane.env`: `sudo systemctl restart jane`. After editing `/etc/opt/jane/tarzan.env` or the tarzan unit: `sudo systemctl restart tarzan`.

## 1. Restrict where Jane listens

Each of Jane's interfaces binds to `0.0.0.0` (every interface) in the packaged configuration. Bind the ones you don't need from other machines to `127.0.0.1`:

```yaml
rest:
  listenon: 127.0.0.1     # jp on this machine only
web:
  listenon: 0.0.0.0       # Web UI from other machines
x3270:
  listenon: 127.0.0.1
```

Add a host firewall (e.g. `nftables`) allowing only what you use: 8540 (Web UI), 8520 (REST, for jp on other elements), 8081 (MQTT WebSockets, for the Live page).

**Check:** `sudo ss -ltnp | grep janeserver` shows each port on the address you set.

## 2. The x3270 interface

The tn3270 interface (port 3270) is never encrypted, including passwords. If you don't use it, keep it local (step 1) or start Jane without it by adding `--startx3270=false` to `ExecStart` with a drop-in (`sudo systemctl edit jane`):

```ini
[Service]
ExecStart=
ExecStart=/opt/jane/janeserver --config=/etc/opt/jane/config.yaml --startx3270=false
```

To use it from elsewhere, keep it on 127.0.0.1 and tunnel: `ssh -L 3270:127.0.0.1:3270 jane.example`, then connect the terminal to `localhost:3270`.

**Check:** `ss -ltn | grep 3270` shows only 127.0.0.1, or nothing.

## 3. Your own TLS key and certificate

Make a key and certificate (from your CA, or self-signed for a lab):

```bash
sudo openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 825 \
  -keyout /etc/opt/jane/jane.key -out /etc/opt/jane/jane.crt -subj "/CN=jane.example" \
  -addext "subjectAltName=DNS:jane.example"
sudo chown root:jane /etc/opt/jane/jane.key /etc/opt/jane/jane.crt
sudo chmod 640 /etc/opt/jane/jane.key
```

Point both interfaces at them and switch HTTPS on:

```yaml
rest:
  crt: /etc/opt/jane/jane.crt
  key: /etc/opt/jane/jane.key
  usehttp: false
web:
  crt: /etc/opt/jane/jane.crt
  key: /etc/opt/jane/jane.key
  usehttp: false
```

- jp must then use `https://` in `attestationserver:`. For a self-signed certificate, on Linux/BSD run jp with `SSL_CERT_FILE=/path/to/jane.crt`; on Windows add the certificate to the system store.
- **The Live page:** a page loaded over HTTPS may only open secure WebSockets (`wss://`), so Mosquitto's WebSocket listener needs TLS too (step 8), or the Live page stops updating.

**Check:** `https://jane.example:8540` loads with your certificate; `jp attest` works with the `https://` URL.

## 4. Logins

Users are kept in Jane's database. Add one (the password is asked for twice):

```bash
sudo /opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

Then require logins in `config.yaml`:

```yaml
auth:
  web: true
  rest: true
  x3270: true
  sessiontimeout: 30m
```

A Web UI login, or an x3270 logon, expires after `sessiontimeout` without use. Failed logins on all three interfaces are logged on the `AUTH` channel. [Authentication](authentication.md) has the details.

Without HTTPS (step 3), Web UI and REST passwords cross the network in clear.

**Check:** the Web UI asks for a login; `curl http://127.0.0.1:8520/elements` gets 401.

## 5. jp with REST credentials

With `auth: rest: true`, give jp a Jane user, either from the environment (preferred) or on the command line:

```bash
export JANE_USERNAME=ian JANE_PASSWORD='...'
/opt/jane/jp attest /etc/opt/jane/provisioning.yaml
sudo -E /opt/jane/jp update /etc/opt/jane/provisioning.yaml     # -E keeps the variables
```

`--user NAME --pass PASSWORD` (both together) override the variables, but a password on the command line is visible to other users.

**Check:** `jp attest` passes; with a wrong password it says Jane refused the credentials (HTTP 401).

## 6. The footer signing key

Jane signs the footer of every claim, result and session. Without a key file it makes a new key at every start, so nobody can verify those signatures later. Give it a permanent key:

```bash
sudo openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out /etc/opt/jane/signing.key
sudo chown root:jane /etc/opt/jane/signing.key
sudo chmod 640 /etc/opt/jane/signing.key
```

```yaml
signing:
  keyfile: /etc/opt/jane/signing.key
```

Keeping the public key and the key ID somewhere verifiers can find them is up to you. See [Signing](signing.md).

**Check:** on the Web UI home page, the **Signing** tab shows key source `file`, its key type and key ID.

## 7. A MongoDB account

Enable authorization in `/etc/mongod.conf`:

```yaml
security:
  authorization: enabled
```

Create an administrator first if you don't have one, then a user for Jane with access to Jane's database only (`asvr` in the packaged configuration). In `mongosh`:

```javascript
use admin
db.createUser({user: "jane", pwd: passwordPrompt(), roles: [{role: "readWrite", db: "asvr"}]})
```

`sudo systemctl restart mongod`, then put the account in `/etc/opt/jane/jane.env` (step 9):

```bash
JANE_MONGODB_USERNAME=jane
JANE_MONGODB_PASSWORD=...
JANE_MONGODB_AUTHSOURCE=admin
```

addjaneuser needs the account too: as root, `set -a; . /etc/opt/jane/jane.env; set +a` before running it, or pass `--dbuser` and `--dbpass`.

The database connection is unencrypted unless MongoDB is on the same machine or you add `tls=true` to Jane's `database: connection` (with TLS configured in MongoDB).

**Check:** Jane's start-up output (`journalctl -u jane`) says `MongoDB authentication: user jane`.

## 8. Mosquitto accounts, and the Live page

Jane gets an account; the Live page stays anonymous but may only read Jane's log topics. Create the password file:

```bash
sudo mosquitto_passwd -c /etc/mosquitto/passwd jane
sudo chown mosquitto: /etc/mosquitto/passwd
```

Replace `/etc/mosquitto/conf.d/jane.conf` with:

```
per_listener_settings true

# janeserver, on this machine, with an account
listener 1883 127.0.0.1
allow_anonymous false
password_file /etc/mosquitto/passwd
acl_file /etc/mosquitto/jane.acl

# the Web UI's Live page, from browsers: anonymous, read-only
listener 8081
protocol websockets
allow_anonymous true
acl_file /etc/mosquitto/jane.acl
# with the Web UI on HTTPS (step 3) the browser needs wss://:
#certfile /etc/mosquitto/certs/jane.crt
#keyfile /etc/mosquitto/certs/jane.key
```

and `/etc/mosquitto/jane.acl`:

```
# anonymous clients (the Live page) may only read Jane's log
topic read AS/#

user jane
topic write AS/#
```

With `per_listener_settings true` every listener needs its own authentication settings, as above. For TLS on the WebSocket listener, copy Jane's certificate and key where the `mosquitto` user can read them and uncomment the two lines.

`sudo systemctl restart mosquitto`, then add Jane's account to `/etc/opt/jane/jane.env`:

```bash
JANE_MQTT_USERNAME=jane
JANE_MQTT_PASSWORD=...
```

Jane does not support MQTT over TLS yet (issue #87), so keep the broker on Jane's machine.

**Check:** Jane's start-up output says `MQTT authentication: user jane`; the Live page still shows new entries.

## 9. Where secrets live

Jane and tarzan read secrets only from environment variables, never from `config.yaml`. On a deb install systemd fills the environment from two files:

| File | Owner, mode | Read by | Variables |
|---|---|---|---|
| `/etc/opt/jane/jane.env` | root:jane, 640 | jane.service | `JANE_MONGODB_USERNAME`, `JANE_MONGODB_PASSWORD`, `JANE_MONGODB_AUTHSOURCE`, `JANE_MQTT_USERNAME`, `JANE_MQTT_PASSWORD` |
| `/etc/opt/jane/tarzan.env` | root:root, 600 | tarzan.service | `JANE_TPM_ENDORSEMENT_PASSWORD` |

The format is `KEY=value`, one per line; quote values containing spaces or `#`. An unset or empty username means "no account".

**HashiCorp Vault** works without changes to Jane:

- *Vault Agent template*: let the agent render `/etc/opt/jane/jane.env` from Vault and run `systemctl restart jane` when it changes.
- *Vault Agent process supervisor mode*: let the agent start janeserver with `env_template` entries for the variables (a drop-in replacing `ExecStart`), restarting it when a secret changes.

Jane reads the variables once, at start, so rotated secrets need a restart; Web UI logins are lost on restart. Anything else that sets a process's environment works the same way (Kubernetes Secrets, Windows service environment).

Environment variables can be read by root and by the same user (`/proc/<pid>/environ`), the same exposure as the files' modes.

**Check:** `sudo ls -l /etc/opt/jane` shows the owners and modes above.

## 10. tarzan

- Offer only the APIs Jane uses. The packaged unit starts `-sys -tpm2 -uefi -ima`; trim it with a drop-in (`sudo systemctl edit tarzan`):

  ```ini
  [Service]
  ExecStart=
  ExecStart=/opt/jane/tarzan -sys -tpm2
  ```

- Never add `-unsafe`: it lets any caller read any file as root.
- tarzan has no authentication (issue #58): firewall port 8530 so only Jane's machine can reach it.
- If the TPM's endorsement hierarchy has a password (`tpm2_changeauth -c e ...`), put it in `/etc/opt/jane/tarzan.env` as `JANE_TPM_ENDORSEMENT_PASSWORD`, and give jp the same variable (`sudo -E`). tarzan's `tpm2/gettime` cannot use it (a limitation of the TPM library) and fails while a password is set; `tpm2/activatecredential` and jp do use it. `tpm2_changeauth -c e -p <old>` clears it.

**Check:** from another machine, port 8530 is closed (except from Jane's machine); tarzan's start-up output lists only the APIs you want.

## 11. The systemd sandbox

`jane.service` already restricts janeserver: it runs as the `jane` user with no capabilities, cannot gain privileges, sees the file system read-only except `/var/log/jane`, cannot see `/home` or devices, and has its own `/tmp`. `systemd-analyze security jane.service` shows the result.

If you move the log file elsewhere, or keep keys under `/home`, allow it with a drop-in (`sudo systemctl edit jane`):

```ini
[Service]
ReadWritePaths=/srv/jane/log
ProtectHome=read-only
```

tarzan runs as root without a sandbox: it needs the TPM and `/sys/kernel/security`.

**Check:** `journalctl -u jane` shows no `Permission denied` or `Read-only file system` errors.
