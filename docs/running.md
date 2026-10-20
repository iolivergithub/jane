# Running Jane, tarzan and jp

This is the reference for every option, flag and environment variable of the programs, and for running them by hand or on systems other than Debian. For a first installation follow the [Quick start](quickstart.md); for securing an installation see [Hardening](hardening.md); for what the deb packages install and how systemd runs them see [Installing with dpkg/apt](deb.md).

- [What runs where](#what-runs-where)
- [MongoDB and Mosquitto](#mongodb-and-mosquitto)
- [janeserver](#janeserver)
- [addjaneuser](#addjaneuser)
- [tarzan](#tarzan)
- [jp](#jp)
- [All environment variables](#all-environment-variables)

## What runs where

| Program | Runs on | Talks to |
|---|---|---|
| janeserver | the Jane host | MongoDB, the MQTT broker, trust agents (tarzan, ratsd) |
| addjaneuser | wherever it can reach Jane's MongoDB | MongoDB |
| tarzan | each element (machine being attested), as root | answers janeserver |
| jp | each element | janeserver's REST API; the local TPM |

Installed from the deb packages, the programs are in `/opt/jane` and their configuration in `/etc/opt/jane`. Built from source (see [Compiling](compiling.md)) they can live anywhere.

## MongoDB and Mosquitto

janeserver needs both before it starts, and panics (and, under systemd, restarts) until it can reach them.

| Jane setting | Meaning |
|---|---|
| `database: connection` | MongoDB connection string, e.g. `mongodb://127.0.0.1:27017`; options such as `tls=true` or `authSource` may be added |
| `database: name` | the database Jane uses; created on first use |
| `messaging: broker`, `port` | MQTT broker address and port for janeserver (plain TCP; TLS is not supported yet, #87) |
| `messaging: portws` | the broker's WebSocket port, used by the browser for the Web UI's Live page |
| `messaging: clientid` | MQTT client ID; must be unique per Jane instance |

Setting them up without accounts is in the [Quick start](quickstart.md) (steps 1 and 2); with accounts, and with TLS for the Live page, in [Hardening](hardening.md) (steps 7 and 8). Accounts are given to Jane through environment variables, never in `config.yaml`.

## janeserver

### Command line

| Flag | Default | Meaning |
|---|---|---|
| `--config=FILE` | `./config.yaml` | the configuration file |
| `--startREST=false` | true | don't start the REST API |
| `--startWebUI=false` | true | don't start the Web UI |
| `--startx3270=false` | true | don't start the x3270 (tn3270) interface |

### Configuration file

The packaged `/etc/opt/jane/config.yaml` lists every option with comments; a commented example for running from the source tree is `janeserver/config.yaml`.

| Section | Keys | Notes |
|---|---|---|
| `system` | `name` | a name for this Jane instance, shown in the UIs and logs |
| `database` | `connection`, `name` | see above |
| `messaging` | `broker`, `port`, `portws`, `clientid` | see above |
| `rest` | `port` (8520), `listenon`, `usehttp`, `crt`, `key` | REST API. `listenon`: address to bind, `0.0.0.0` or empty for all, `127.0.0.1` for this machine only. `usehttp: true` serves plain HTTP; otherwise HTTPS with `crt`/`key` |
| `web` | `port` (8540), `listenon`, `usehttp`, `crt`, `key` | Web UI, as for `rest` |
| `x3270` | `port` (3270), `listenon` | tn3270 terminal interface; never encrypted |
| `logging` | `logfilelocation`, `sessionupdatelogging` | Jane's log file (CSV, one entry per line), and whether to log every claim and result added to a session |
| `signing` (optional) | `keyfile` | PEM private key for claim, result and session footers; without it an ephemeral key is made at each start. See [Signing](signing.md) |
| `auth` (optional) | `web`, `rest`, `x3270`, `sessiontimeout` (30m) | logins per interface; without the section nothing asks for a login. See [Authentication](authentication.md) |

Restart Jane after changing the file.

### Environment variables

| Variable | Meaning | Unset or empty |
|---|---|---|
| `JANE_MONGODB_USERNAME` | MongoDB account | no account (or whatever the connection string says) |
| `JANE_MONGODB_PASSWORD` | its password | empty |
| `JANE_MONGODB_AUTHSOURCE` | database the account is defined in | `admin`, or the connection string's `authSource` |
| `JANE_MQTT_USERNAME` | MQTT broker account | anonymous |
| `JANE_MQTT_PASSWORD` | its password | none |
| `JANE_X3270_DEBUG` | any value: log the x3270 data stream (debugging) | off |

An account in the environment replaces a username in the connection string. Jane reads the variables once, at start.

### Starting it

**By hand** (from the source tree, or for testing):

```bash
export JANE_MONGODB_USERNAME=jane JANE_MONGODB_PASSWORD='...'    # only if MongoDB needs an account
./janeserver --config=config.yaml
```

**Deb install (systemd):** `systemctl start|stop|restart|status jane`; the variables come from `/etc/opt/jane/jane.env`. See [Installing with dpkg/apt](deb.md).

**Windows:** run `janeserver.exe` as a service (e.g. with the Service Control Manager or a service wrapper) and set the variables in the service's environment.

**HashiCorp Vault:** either let Vault Agent render `jane.env` and restart Jane when it changes, or run janeserver under Vault Agent's process supervisor mode with `env_template` entries for the variables. See [Hardening](hardening.md#9-where-secrets-live).

### Start-up output

```
JANE: Configuration file location:  /etc/opt/jane/config.yaml
JANE: initialising database MONGO connection
JANE: MongoDB authentication: none
JANE: Database infrastructure MONGO is running
JANE: Initialising message infrastructure MQTT connection
JANE: MQTT authentication: none
JANE: MQTT connected
JANE: Message infrastructure MQTT is running
JANE: Authentication: web false, rest false, x3270 false, session timeout 30m0s
...
```

The authentication lines show the accounts used (`user jane`), never passwords. Jane warns if a login is required but there are no users, or if no signing key is configured. Under systemd: `journalctl -u jane`.

### The log

Every operation is logged three ways: to the `log` collection in MongoDB, to MQTT topic `AS/<channel>`, and to `logging: logfilelocation` as CSV. The file is created with mode 640 and opened for each entry, so log rotation works. If MongoDB does not answer within 5 seconds the entry is reported on standard error (the journal) instead.

## addjaneuser

Adds a Jane user, or sets a new password for an existing one. The password is stored as an Argon2id hash. Installed as `/opt/jane/addjaneuser`; in the source tree `etc/addjaneuser/addjaneuser`. Needs Python 3 with `argon2-cffi` and `pymongo` (`apt install python3-argon2 python3-pymongo`).

```
addjaneuser [--dbuser NAME --dbpass PASSWORD] USERNAME MONGODB_URL DATABASE
```

`MONGODB_URL` and `DATABASE` are Jane's `database: connection` and `database: name` (`asvr` in the packaged configuration).

```bash
sudo /opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr       # asks for the password twice
echo 'secret' | /opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr   # from a script
```

If MongoDB requires an account, addjaneuser reads `JANE_MONGODB_USERNAME`, `JANE_MONGODB_PASSWORD` and `JANE_MONGODB_AUTHSOURCE`, like Jane. On a deb install, as root:

```bash
set -a; . /etc/opt/jane/jane.env; set +a
/opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

`--dbuser` and `--dbpass` (both together) override the variables. A password on the command line is visible to other users and kept in shell history; the variables are the safer way.

Users are removed in `mongosh`: `db.users.deleteOne({username: "ian"})`. Jane needs no restart after users change. See [Authentication](authentication.md).

## tarzan

tarzan is the reference trust agent: it answers Jane over HTTP (protocol `A10HTTPRESTv2`) with TPM 2.0 quotes and other evidence, event logs and system information. It runs as root (the TPM device, `/sys/kernel/security`).

### Flags

At least one API must be enabled.

| Flag | Meaning |
|---|---|
| `-sys` | system information (OS, architecture, hostname, machine ID, ...) |
| `-tpm2` | TPM 2.0: PCRs, quotes, gettime, capabilities, credential activation |
| `-uefi` | UEFI event log and UEFI variables |
| `-ima` | Linux IMA measurement log |
| `-txt` | Intel TXT log (not implemented yet) |
| `-port=N` | listen on port N (default 8530) |
| `-unsafe` | **never use**: lets callers ask for any file instead of the standard log locations, i.e. read any file as root |

Flags may be written with one or two dashes (`-tpm2` or `--tpm2`).

### Environment

| Variable | Meaning |
|---|---|
| `JANE_TPM_ENDORSEMENT_PASSWORD` | the TPM's endorsement hierarchy password, only if one has been set (`tpm2_changeauth -c e`). Unset means the empty password. Used by credential activation; `tpm2/gettime` cannot use it and fails while a password is set |

### Running it

**By hand:**

```bash
sudo ./tarzan -sys -tpm2 -uefi -ima
```

**Deb install (systemd):** the packaged unit starts `-sys -tpm2 -uefi -ima` and reads `/etc/opt/jane/tarzan.env`; see [Installing with dpkg/apt](deb.md).

**OpenBSD (rc.d):** save as `/etc/rc.d/tarzan` and enable with `rcctl enable tarzan`:

```sh
#!/bin/ksh
#
# $OpenBSD: tarzan

daemon="/opt/jane/tarzan"
daemon_flags="-sys -tpm2"

. /etc/rc.d/rc.subr

rc_bg=YES
rc_cmd $1
```

**Windows:** run `tarzan.exe` (e.g. `tarzan.exe -sys -tpm2`) as Administrator, or as a service. Windows controls the TPM itself: PCRs can be read, but quotes need an AK the system allows, and UEFI and IMA logs are not available.

## jp

jp registers this machine with Jane as an element, provisions its TPM keys, records its expected values, and attests it. It reads a provisioning file (YAML); the deb installs an example as `/etc/opt/jane/provisioning.yaml.example`, and `provisioner/README.md` describes the file format.

```
jp [-h] [-u] [--recreate] [--user NAME --pass PASSWORD] create|update|attest PROVISIONINGFILE
```

| Operation | Needs root | What it does |
|---|---|---|
| `create` | yes | first registration: runs the worklist (TPM clear and provisioning ask first), registers the element, writes the element ID file, records expected values. Refuses if the machine is already registered |
| `update` | yes | refreshes the registered element and all its expected values (after firmware or OS updates) |
| `attest` | no | attests the element against every intent in the file and runs their rules |

| Option | Meaning |
|---|---|
| `-u`, `--unsafe` | don't ask before TPM and overwrite steps |
| `--recreate` | `create` only: register a new element although this machine is registered (asks first unless `-u`) |
| `--user NAME --pass PASSWORD` | Jane user for the REST API; both together; override `JANE_USERNAME`/`JANE_PASSWORD` |

Exit status: 0 success (attest: every rule passed), 1 error, 2 bad command line, 3 attest: a rule did not pass.

The element ID is kept in `/etc/opt/jane/janeelementid` (Windows: `%ProgramData%\jane\janeelementid`). `create` and `update` stop straight away, before contacting Jane, if they cannot write it.

### Environment

| Variable | Meaning |
|---|---|
| `JANE_USERNAME`, `JANE_PASSWORD` | Jane user and password, when Jane's REST API requires a login (`auth: rest: true`) |
| `JANE_TPM_ENDORSEMENT_PASSWORD` | the TPM's endorsement hierarchy password, if one is set |
| `SSL_CERT_FILE` | (Linux/BSD) a CA or self-signed certificate to trust for Jane's HTTPS |

`sudo` drops the environment by default: use `sudo -E`, or `sudo --preserve-env=JANE_USERNAME,JANE_PASSWORD,JANE_TPM_ENDORSEMENT_PASSWORD`, for `create` and `update`.

## All environment variables

| Variable | Program | Meaning | Unset or empty |
|---|---|---|---|
| `JANE_MONGODB_USERNAME` | janeserver, addjaneuser | MongoDB account | no account |
| `JANE_MONGODB_PASSWORD` | janeserver, addjaneuser | its password | empty |
| `JANE_MONGODB_AUTHSOURCE` | janeserver, addjaneuser | database the account is defined in | `admin` / connection string |
| `JANE_MQTT_USERNAME` | janeserver | MQTT account | anonymous |
| `JANE_MQTT_PASSWORD` | janeserver | its password | none |
| `JANE_X3270_DEBUG` | janeserver | log the x3270 data stream | off |
| `JANE_USERNAME` | jp | Jane user for the REST API | no credentials |
| `JANE_PASSWORD` | jp | its password | empty |
| `JANE_TPM_ENDORSEMENT_PASSWORD` | tarzan, jp | TPM endorsement hierarchy password | empty password |
| `SSL_CERT_FILE` | jp (Go) | extra trusted certificates for HTTPS | system store |
