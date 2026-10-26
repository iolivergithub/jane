# Installing with dpkg/apt

Jane comes as two Debian packages: **jane** (the attestation engine, for the Jane host) and **tarzan** (the trust agent and the provisioner jp, for each element). This page describes what they install, what installing, upgrading and removing do, and how systemd runs the services afterwards. For a step-by-step first installation see the [Quick start](quickstart.md); for building the packages see [Compiling](compiling.md#building-the-packages).

## Before you install

The jane package needs two services it cannot install itself:

- **MongoDB**, which is not in Debian: install it from MongoDB's repository (see the [Quick start](quickstart.md#1-mongodb)).
- **an MQTT broker**: the package recommends `mosquitto`, which `apt` installs with it.

The jane package depends on `adduser` and recommends `mosquitto`, `python3`, `python3-argon2` and `python3-pymongo` (the last three for `addjaneuser`). The tarzan package has no dependencies.

## Which package for which machine

From 1.5.6 there is a package per architecture (#116). Debian 13 supports all of them; for armel it is the last release.

| Machine | jane | tarzan |
|---|---|---|
| 64-bit PC (x86-64) | `jane_<v>_amd64.deb` | `tarzan_<v>_amd64.deb` |
| 64-bit ARM (servers, Raspberry Pi 3/4/5 with a 64-bit OS) | `jane_<v>_arm64.deb` | `tarzan_<v>_arm64.deb` |
| IBM Z (s390x) | `jane_<v>_s390x.deb` | `tarzan_<v>_s390x.deb` |
| IBM POWER, little-endian (ppc64el) | — | `tarzan_<v>_ppc64el.deb` |
| 64-bit RISC-V | — | `tarzan_<v>_riscv64.deb` |
| 32-bit ARMv7 with FPU (Debian `armhf`) | — | `tarzan_<v>_armhf-armv7.deb` |
| 32-bit ARMv6 with FPU: Raspberry Pi Zero/1 on Raspberry Pi OS | — | `tarzan_<v>_armhf-armv6.deb` |
| 32-bit ARM without FPU, ARMv5 (Debian `armel`) | — | `tarzan_<v>_armel-armv5.deb` |

- The two `armhf` packages both declare `Architecture: armhf`, so the file name says which is which. Raspberry Pi OS 32-bit calls its ARMv6 port `armhf`, but an ARMv7 binary does not run on a Pi Zero/1: use `armhf-armv6` there. On ARMv7 boards either works.
- The `armel-armv5` binaries use software floating point and run on any 32-bit ARM, including boards without an FPU.
- s390x, ppc64el and riscv64 machines rarely have a TPM; tarzan still serves `sys/info` there.
- Jane only needs to *reach* a MongoDB, which can run on another machine. Current MongoDB needs ARMv8.2-A or later on arm64 (a Raspberry Pi 5, but **not** a Pi 4) and an AVX-capable CPU on x86-64, and MongoDB's own packages do not cover every Debian architecture; where it can't run locally, point `database: connection` in `config.yaml` at a MongoDB elsewhere.
- Windows amd64 is built as zip files instead (see [Compiling](compiling.md#windows)).

## Installing

Release files are gzipped (`jane_<version>_<arch>.deb.gz`); unzip them first with `gunzip`. Then, as root, e.g. on amd64:

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
| `/lib/systemd/system/jane.service` | runs janeserver as the `jane` user, sandboxed |
| `/var/log/jane/` | Jane's log file |
| `/usr/share/doc/janeattestationengine/addjaneuser.md` | how to use `addjaneuser` |

**tarzan** (package `tarzantrustagent`)

| File | |
|---|---|
| `/opt/jane/tarzan` | the trust agent |
| `/opt/jane/jp` | the provisioner (see [jp](jp.md)) |
| `/etc/opt/jane/provisioning.yaml.example` | example provisioning file for jp: copy it and edit the copy |
| `/etc/opt/jane/tarzan.env` | environment for tarzan (TPM endorsement password), commented out |
| `/lib/systemd/system/tarzan.service` | runs tarzan as root |

`config.yaml`, `jane.env`, `tarzan.env`, the keys and the provisioning example are dpkg conffiles: an upgrade never overwrites your changes without asking. The two systemd units are ordinary package files (since 1.5.5): an upgrade always installs the new unit, and your changes go in drop-ins (see [Running under systemd](#running-under-systemd)).

## Owners and permissions

Every install and upgrade of jane sets these, so wrong owners left from earlier versions or by hand are corrected. Binaries and configuration belong to root, so the `jane` user cannot change them; `jane` reads its configuration and keys through its group and owns only its log.

| Path | Owner | Mode |
|---|---|---|
| `/opt/jane` (directory) | `root:root` | 755 |
| `/opt/jane/janeserver`, `addjaneuser`, `tarzan`, `jp` | `root:root` | 755 |
| `/etc/opt/jane` (directory) | `root:jane` | 751 |
| directories inside `/etc/opt/jane` | `root:jane` | 750 |
| `config.yaml`, `jane.env`, `*.key`, `*.pem` in `/etc/opt/jane` and its subdirectories (not `jp.key`) | `root:jane` | 640 |
| `*.crt`, `standardintents.json` in `/etc/opt/jane` and its subdirectories | `root:jane` | 644 |
| `/etc/opt/jane/tarzan.env` | `root:root` | 600 |
| `/etc/opt/jane/provisioning.yaml.example` | `root:root` | 644 |
| `/var/log/jane` (directory) and its files | `jane:jane` | 750 / 640 |
| `/lib/systemd/system/jane.service`, `tarzan.service` | `root:root` | 644 |

- `/etc/opt/jane` is 751: other users may open a file whose name they know (jp reading the element ID file) but cannot list the directory; secrets are protected by their own modes. The modes are already set inside the packages, so secrets are never readable by others, not even during installation.
- Keys you add or replace (TLS, signing) must be readable by group `jane`: `chown root:jane FILE; chmod 640 FILE`. Files named `*.key`, `*.pem` and `*.crt` in `/etc/opt/jane` and its subdirectories are fixed on every upgrade; other names are left as you set them. jp's API key file `/etc/opt/jane/jp.key` belongs to the users who run jp and is left alone (see [Authentication](authentication.md)).
- Every install and upgrade also checks the files `config.yaml` names (the TLS certificates and keys, the signing key, the log file) as the `jane` user, and prints a warning for each one Jane cannot read or write: missing, unreadable, under a home directory (hidden by the sandbox), or a log outside `/var/log/jane`. Files outside `/etc/opt/jane` and `/var/log/jane` are never changed.
- To read Jane's log as another user, add that user to group `jane`.
- The element ID file `/etc/opt/jane/janeelementid` is written by jp as root, mode 444.

## Running under systemd

| | jane | tarzan |
|---|---|---|
| Unit | `/lib/systemd/system/jane.service` | `/lib/systemd/system/tarzan.service` |
| Runs as | `jane` | `root` |
| Command | `/opt/jane/janeserver --config=/etc/opt/jane/config.yaml` | `/opt/jane/tarzan -sys -tpm2 -uefi -ima` |
| Environment file | `/etc/opt/jane/jane.env` | `/etc/opt/jane/tarzan.env` |
| Started on install | no (enabled) | yes |
| Restarted on upgrade | no: restart it yourself | yes |
| Restarts after failure | every 5 s | every 5 s |

```bash
systemctl start|stop|restart|status jane      # the same for tarzan
journalctl -u jane                            # its output; -f to follow
```

`jane.service` starts after the network, MongoDB (`mongod.service`) and Mosquitto, when they are on the same machine. Until it can reach both it exits and is restarted every 5 seconds.

**Environment files.** systemd reads them (`EnvironmentFile=`), not Jane or tarzan; the programs only see the variables. Format: `KEY=value`, one per line, `#` for comments; quote values containing spaces or `#`. A missing file is fine. After editing: `systemctl restart jane` (or `tarzan`). The variables are listed in [Running](running.md#all-environment-variables).

**Changing a unit.** Don't edit the packaged units (an upgrade replaces them) and don't copy them to `/etc/systemd/system` (a copy there overrides the packaged unit and misses later changes). Add a drop-in, which upgrades leave alone; it lands in `/etc/systemd/system/<unit>.d/`:

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

- **jane is not restarted**: a running Jane carries on as the old version until you restart it (`systemctl restart jane`); the package says so. It stays enabled.
- tarzan is restarted; a warning is printed if it does not come up.
- owners and modes are set again, and the files `config.yaml` names are checked (above).

Jane and jp should be upgraded together, as the REST API and jp change together.

### Upgrading from 1.5.4 or earlier

Until 1.5.4 the units were conffiles in `/etc/systemd/system`. From 1.5.5 they are in `/lib/systemd/system`, and only that one unit file is used for each service:

- an unchanged old unit in `/etc/systemd/system` is removed;
- an **edited** old unit (or one made by hand) is moved to `/etc/opt/jane/jane.service.pre-1.5.5` (or `tarzan.service.pre-1.5.5`), readable by root only, and a warning is printed. Move what you still need into a drop-in: compare with `diff /etc/opt/jane/jane.service.pre-1.5.5 /lib/systemd/system/jane.service`, then `systemctl edit jane`;
- dpkg's leftovers (`.dpkg-dist`, `.dpkg-old`, `.dpkg-bak`) are removed, the services are enabled again from the new location, and drop-ins you already have keep working. A masked unit stays masked.

With a non-interactive upgrade (e.g. unattended-upgrades) nobody sees the warnings on screen; apt keeps them in `/var/log/apt/term.log`.

Read the [changelog](../CHANGELOG.md) for each release; for example, load the standard intents again when a release adds intents.

## Removing

`apt remove janeattestationengine` (or `tarzantrustagent`) stops and disables the service and removes the programs. Configuration, keys, the element ID file and `/var/log/jane` stay; `apt purge` removes the conffiles too. The `jane` user is kept, as is usual for system users. MongoDB data is untouched.

## RPM

The build also converts the packages to RPMs with `alien`. They are not tested: the install scripts assume Debian tools (`adduser`) and systemd.
