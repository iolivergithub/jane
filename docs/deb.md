# Building and Installing with DEB and RPM files

First ensure you know how to build things manually...or not.

## Generating the DEB and RPM files

Change to the `etc/debbuild` directory, check that the last line of `versions.txt` is the version you want, then run the build script:

```bash
./debbuild.sh
```

This compiles janeserver, tarzan and the provisioner (`jp`), builds two deb packages, lints them and converts them to rpms.

NB: linting only works if `lintian` is installed, and rpm generation only if `alien` is installed. Otherwise you'll get errors from those steps, but you will still get the deb files.

The packages are built in `/tmp/janedebbuild`, named by version and architecture, e.g. `jane_1.5.1_amd64.deb.gz` and `tarzan_1.5.1_amd64.deb.gz`.

## What is in the packages

**jane** (package `janeattestationengine`)

| File | |
|---|---|
| `/opt/jane/janeserver` | the attestation engine |
| `/opt/jane/addjaneuser` | adds Jane users or resets passwords, see [Authentication](authentication.md) |
| `/etc/opt/jane/config.yaml` | configuration: every option, optional ones commented out, all logins off |
| `/etc/opt/jane/REPLACE_ME.key`, `.crt` | placeholder TLS key and certificate - replace them |
| `/etc/systemd/system/jane.service` | runs Jane as the `jane` user |
| `/var/log/jane/` | Jane's log file |
| `/usr/share/doc/janeattestationengine/addjaneuser.md` | how to use `addjaneuser` |

It depends on `adduser` and recommends `python3`, `python3-argon2` and `python3-pymongo` (for `addjaneuser`). MongoDB and an MQTT broker (e.g. Mosquitto) are needed but not installed by the package.

**tarzan** (package `tarzantrustagent`)

| File | |
|---|---|
| `/opt/jane/tarzan` | the trust agent |
| `/opt/jane/jp` | the provisioner |
| `/etc/opt/jane/provisioning.yaml.example` | an example provisioning file for `jp` - copy it and edit the copy |
| `/etc/systemd/system/tarzan.service` | runs tarzan as root |

The configuration files are dpkg conffiles: an upgrade will not overwrite your changes to them without asking.

## The jane user

Jane runs as the unprivileged system user `jane`, created when the package is installed. It owns `/var/log/jane`, and `config.yaml` and `REPLACE_ME.key` are readable by group `jane` (mode 640). Any keys you add, such as a TLS key or a signing key, must be readable by group `jane` too:

```bash
chown root:jane /etc/opt/jane/mykey.key
chmod 640 /etc/opt/jane/mykey.key
```

## Installation with DEB

As root, unzip the packages and run `dpkg -i ./jane_1.5.1_amd64.deb` and/or `dpkg -i ./tarzan_1.5.1_amd64.deb` (or `apt install ./jane_1.5.1_amd64.deb`, which also installs the recommended Python packages).

In both cases the jane and tarzan services will be enabled with systemd. Tarzan will also be started - check with journalctl. Jane is not started: edit `/etc/opt/jane/config.yaml` and replace the keys first, then `systemctl start jane.service`.

Example output from an older version is below (file names, paths and messages have since changed slightly):

```bash
$ su -
Password: 
root@Debian:~# cd /tmp/janedebbuild/
root@Debian:/tmp/janedebbuild# gunzip jane.deb.gz
root@Debian:/tmp/janedebbuild# gunzip tarzan.deb.gz
root@Debian:/tmp/janedebbuild# dpkg -i ./jane.deb 
Selecting previously unselected package janeattestationengine.
(Reading database ... 206615 files and directories currently installed.)
Preparing to unpack ./jane.deb ...
Unpacking janeattestationengine (1.0) ...
Setting up janeattestationengine (1.0) ...
Running post installation scripts
Enabling Jane with systemd
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
   *** CHECK!!!!! **** 
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
 Edit the /etc/opt/jane/config.yaml
 Replace keys in /etc/opt/jane
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
Post installation scripts complete
root@Debian:/tmp/janedebbuild# dpkg -i ./tarzan.deb 
Selecting previously unselected package tarzantrustagent.
(Reading database ... 206623 files and directories currently installed.)
Preparing to unpack ./tarzan.deb ...
Unpacking tarzantrustagent (1.0) ...
Setting up tarzantrustagent (1.0) ...
Running post installation scripts
Enabling Tarzan with systemd
Starting Tarzan with systemd
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
   *** CHECK!!!!! **** 
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
 Edit the /etc/systemd/system/tarzan.service 
 Ensure Tarzan is runnign the correct services
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
Post installation scripts complete
```

The results of checking the status with systemd are below. Note that jane *has not* been started - only enabled.

```bash
root@Debian:/tmp/janedebbuild# systemctl status jane.service 
○ jane.service - Jane Attestation Engine
     Loaded: loaded (/etc/systemd/system/jane.service; enabled; preset: enabled)
     Active: inactive (dead)
root@Debian:/tmp/janedebbuild# systemctl status tarzan.service 
● tarzan.service - Tarzan Trust Agent
     Loaded: loaded (/etc/systemd/system/tarzan.service; enabled; preset: enabled)
     Active: active (running) since Mon 2024-12-02 20:26:30 EET; 1min 51s ago
   Main PID: 7224 (tarzan)
      Tasks: 6 (limit: 38436)
     Memory: 3.3M
        CPU: 2ms
     CGroup: /system.slice/tarzan.service
             └─7224 /opt/jane/tarzan --sys

Dec 02 20:26:30 Debian tarzan[7224]: +========================================================
Dec 02 20:26:30 Debian tarzan[7224]: |  TA10 version - Starting
Dec 02 20:26:30 Debian tarzan[7224]: |   + linux O/S on amd64
Dec 02 20:26:30 Debian tarzan[7224]: |   + version v0.2, build Mon Dec  2 07:40:59 PM EET 2024 main.VERSION=locally_compiled
Dec 02 20:26:30 Debian tarzan[7224]: |   + session identifier is 7e050497-9570-45d8-9445-addfe8c6226c
Dec 02 20:26:30 Debian tarzan[7224]: |   + unsafe mode? false
Dec 02 20:26:30 Debian tarzan[7224]: +========================================================
Dec 02 20:26:30 Debian tarzan[7224]:    +-- Sys attestation API enabled
Dec 02 20:26:30 Debian tarzan[7224]:    +-- HTTP interface on port :8530 enabled
Dec 02 20:26:30 Debian tarzan[7224]: ⇨ http server started on [::]:8530
```

## Removal with DEB

Simply stop and disable the services, then purge. For example:

```bash
root@Debian:/tmp/janedebbuild# systemctl stop jane.service
root@Debian:/tmp/janedebbuild# systemctl stop tarzan.service
root@Debian:/tmp/janedebbuild# systemctl disable jane.service 
Removed "/etc/systemd/system/multi-user.target.wants/jane.service".
root@Debian:/tmp/janedebbuild# systemctl disable tarzan.service 
Removed "/etc/systemd/system/multi-user.target.wants/tarzan.service".
root@Debian:/tmp/janedebbuild# dpkg -P janeattestationengine 
(Reading database ... 206625 files and directories currently installed.)
Removing janeattestationengine (1.0) ...
Purging configuration files for janeattestationengine (1.0) ...
dpkg: warning: while removing janeattestationengine, directory '/etc/opt' not empty so not removed
root@Debian:/tmp/janedebbuild# dpkg -P tarzantrustagent 
(Reading database ... 206618 files and directories currently installed.)
Removing tarzantrustagent (1.0) ...
Purging configuration files for tarzantrustagent (1.0) ...
```

## Installation with RPM

Probably similar to DEB, you can figure this out.