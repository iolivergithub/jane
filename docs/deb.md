# Building and Installaing with DEB and RPM files

First ensure you know how to build things manually...or not. 

## Generating the DEB and RPM files

Change to the `etc\debbuild` director, then run the build script:

```bash
./debbuild.sh 
```

This will generate the gzipped deb and rpm files for jane and tarzan. It will compile both, build the deb, lint it and convert them to rpms.

NB: linting will only work with `lintian` is installed, and the rpm generation only if `alien` is installed. Otherwise you'll get errors. At minimum you'll get gzipped deb files.

The installer creates a directory in `/tmp` where the deb and rpm files will be found, eg:

```bash
ian@Debian:/tmp/janedebbuild$ ls -l
total 20216
drwxr-xr-x 5 ian ian    4096 Dec  2 19:40 jane
-rw-r--r-- 1 ian ian 5859658 Dec  2 19:41 janeattestationengine-1.0-2.x86_64.rpm.gz
-rw-r--r-- 1 ian ian 4420056 Dec  2 19:41 jane.deb.gz
drwxr-xr-x 5 ian ian    4096 Dec  2 19:40 tarzan
-rw-r--r-- 1 ian ian 4866350 Dec  2 19:41 tarzan.deb.gz
-rw-r--r-- 1 ian ian 5535737 Dec  2 19:41 tarzantrustagent-1.0-2.x86_64.rpm.gz
```


## Installation with DEB

As root run `dpkg -i ./jane.deb` and/or `dpkg -i ./tarzan.deb`

In both cases the jane and tarzan services will the enabled with systemd. Tarzan will also run - check with journalctl.

Example output is below:

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
 Edit the /opt/etc/jane/config.yaml 
 Regenerate keys in /opt/etc/jane
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

## Installation and RPM with RPM

Probably similar to DEB, you can figure this out.