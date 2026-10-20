# Installing with dpkg/apt

Jane comes as two Debian packages: **jane** (the attestation engine, for the Jane host) and **tarzan** (the trust agent and the provisioner jp, for each element). This page describes what they install, what installing, upgrading and removing do, and how systemd runs the services afterwards. For a step-by-step first installation see the [Quick start](quickstart.md); for building the packages see [Compiling](compiling.md#building-the-packages).

## Before you install

The jane package needs two services it cannot install itself:

- **MongoDB**, which is not in Debian: install it from MongoDB's repository (see the [Quick start](quickstart.md#1-mongodb)).
- **an MQTT broker**: the package recommends `mosquitto`, which `apt` installs with it.

The jane package depends on `adduser` and recommends `mosquitto`, `python3`, `python3-argon2` and `python3-pymongo` (the last three for `addjaneuser`). The tarzan package has no dependencies.

## Installing

Release files are gzipped (`jane_<version>_amd64.deb.gz`); unzip them first with `gunzip`. Then, as root:

```bash
apt install ./jane_<version>_amd64.deb ./tarzan_<version>_amd64.deb
```

`apt install` also installs the recommended packages; `dpkg -i` installs only the package itself.

**Installing jane:**

1. creates the system user and group `jane` if they don't exist. If an ordinary (non-system) user called `jane` already exists, the installation stops: rename that account first;
2. sets the owners and modes in the table below;
3. enables `jane.service`, but does **not** start it;
4. prints how to start it and where the documentation is, and says so if MongoDB or Mosquitto seem to be missing on this machine (only checked when `config.yaml` points at a local database or broker).

The packaged configuration runs as it is: HTTP, no logins, MongoDB and Mosquitto on 127.0.0.1. Start Jane with `systemctl start jane`, then go on with the [Quick start](quickstart.md) and [Hardening](hardening.md).

**Installing tarzan:** sets the owners and modes of its files, then enables and starts `tarzan.service` (APIs `-sys -tpm2 -uefi -ima` on port 8530) and prints how to register the machine with jp.

## What is in the packages

**jane** (package `janeattestationengine`)

| File | |
|---|---|
| `/opt/jane/janeserver` | the attestation engine |
| `/opt/jane/addjaneuser` | adds Jane users or resets passwords (see [Running](running.md#addjaneuser)) |
| `/etc/opt/jane/config.yaml` | configuration: every option, optional ones commented out, all logins off |
| `/etc/opt/jane/jane.env` | environment for janeserver (MongoDB and MQTT accounts), all commented out |
| `/etc/opt/jane/REPLACE_ME.key`, `.crt` | placeholder TLS key and certificate, only used when you switch to HTTPS; replace them (see [Hardening](hardening.md#3-your-own-tls-key-and-certificate)) |
| `/etc/opt/jane/standardintents.json` | the standard intents matching this version, for loading by hand if downloading them fails |
| `/etc/systemd/system/jane.service` | runs janeserver as the `jane` user, sandboxed |
| `/var/log/jane/` | Jane's log file |
| `/usr/share/doc/janeattestationengine/addjaneuser.md` | how to use `addjaneuser` |

**tarzan** (package `tarzantrustagent`)

| File | |
|---|---|
| `/opt/jane/tarzan` | the trust agent |
| `/opt/jane/jp` | the provisioner |
| `/etc/opt/jane/provisioning.yaml.example` | example provisioning file for jp: copy it and edit the copy |
| `/etc/opt/jane/tarzan.env` | environment for tarzan (TPM endorsement password), commented out |
| `/etc/systemd/system/tarzan.service` | runs tarzan as root |

`config.yaml`, `jane.env`, `tarzan.env`, the keys, the provisioning example and both units are dpkg conffiles: an upgrade never overwrites your changes without asking.

## Owners and permissions

Every install and upgrade of jane sets these, so wrong owners left from earlier versions or by hand are corrected. Binaries and configuration belong to root, so the `jane` user cannot change them; `jane` reads its configuration and keys through its group and owns only its log.

| Path | Owner | Mode |
|---|---|---|
| `/opt/jane` (directory) | `root:root` | 755 |
| `/opt/jane/janeserver`, `addjaneuser`, `tarzan`, `jp` | `root:root` | 755 |
| `/etc/opt/jane` (directory) | `root:jane` | 751 |
| `config.yaml`, `jane.env`, `*.key`, `*.pem` in `/etc/opt/jane` | `root:jane` | 640 |
| `*.crt`, `standardintents.json` in `/etc/opt/jane` | `root:jane` | 644 |
| `/etc/opt/jane/tarzan.env` | `root:root` | 600 |
| `/etc/opt/jane/provisioning.yaml.example` | `root:root` | 644 |
| `/var/log/jane` (directory) and its files | `jane:jane` | 750 / 640 |
| `/etc/systemd/system/jane.service`, `tarzan.service` | `root:root` | 644 |

- `/etc/opt/jane` is 751: other users may open a file whose name they know (jp reading the element ID file) but cannot list the directory; secrets are protected by their own modes. The modes are already set inside the packages, so secrets are never readable by others, not even during installation.
- Keys you add or replace (TLS, signing) must be readable by group `jane`: `chown root:jane FILE; chmod 640 FILE`. Files named `*.key` and `*.pem` are fixed on every upgrade; other names are left as you set them.
- To read Jane's log as another user, add that user to group `jane`.
- The element ID file `/etc/opt/jane/janeelementid` is written by jp as root, mode 444.

## Running under systemd

| | jane | tarzan |
|---|---|---|
| Unit | `/etc/systemd/system/jane.service` | `/etc/systemd/system/tarzan.service` |
| Runs as | `jane` | `root` |
| Command | `/opt/jane/janeserver --config=/etc/opt/jane/config.yaml` | `/opt/jane/tarzan -sys -tpm2 -uefi -ima` |
| Environment file | `/etc/opt/jane/jane.env` | `/etc/opt/jane/tarzan.env` |
| Started on install | no | yes |
| Restarts after failure | every 5 s | every 5 s |

```bash
systemctl start|stop|restart|status jane      # the same for tarzan
journalctl -u jane                            # its output; -f to follow
```

`jane.service` starts after the network, MongoDB (`mongod.service`) and Mosquitto, when they are on the same machine. Until it can reach both it exits and is restarted every 5 seconds.

**Environment files.** systemd reads them (`EnvironmentFile=`), not Jane or tarzan; the programs only see the variables. Format: `KEY=value`, one per line, `#` for comments; quote values containing spaces or `#`. A missing file is fine. After editing: `systemctl restart jane` (or `tarzan`). The variables are listed in [Running](running.md#all-environment-variables).

**Changing a unit.** Don't edit the packaged units; add a drop-in, which upgrades leave alone:

```bash
sudo systemctl edit tarzan
```

```ini
[Service]
ExecStart=
ExecStart=/opt/jane/tarzan -sys -tpm2
```

The empty `ExecStart=` clears the packaged command. Drop-ins also suit extra Jane flags (`--startx3270=false`), running Jane under Vault Agent, or the sandbox settings below.

**The sandbox.** `jane.service` restricts janeserver: no capabilities, no new privileges, the file system read-only except `/var/log/jane`, no access to `/home` or devices, its own `/tmp`, no kernel or cgroup changes. If you move the log file or keep keys elsewhere, allow it with a drop-in, e.g. `ReadWritePaths=/srv/jane/log` or `ProtectHome=read-only`. `systemd-analyze security jane.service` summarises the restrictions. tarzan is not sandboxed: it needs root, the TPM and `/sys/kernel/security`.

## Upgrading

Install the new packages the same way. Your changed conffiles are kept (dpkg asks if the packaged version changed too). Then:

- jane is restarted if it was running, otherwise left stopped;
- tarzan is restarted;
- owners and modes are set again (table above).

Read the [changelog](../CHANGELOG.md) for each release; for example, load the standard intents again when a release adds intents.

## Removing

`apt remove janeattestationengine` (or `tarzantrustagent`) stops and disables the service and removes the programs. Configuration, keys, the element ID file and `/var/log/jane` stay; `apt purge` removes the conffiles too. The `jane` user is kept, as is usual for system users. MongoDB data is untouched.

## RPM

The build also converts the packages to RPMs with `alien`. They are not tested: the install scripts assume Debian tools (`adduser`) and systemd.
