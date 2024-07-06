# Automatic Startup at Boot

In this section we present an example distribution for use in a systemd environment. We utilise the file layout given in the follow section.

## Example File Layout (Linux/BSD)

One possible layout is to put everything in `/opt/jane`.  Note, `janeserver` and `tarzan` are put together just for convenience. Set permissions accordingly.

```bash
$ pwd
/opt/jane
$ ls -l
total 27364
-rw-rw-r-- 1 ian ian      706 tammi  21 13:01 config.yaml
-rwxrwxr-x 1 ian ian 19448208 tammi  21 13:00 janeserver
-rwxrwxr-x 1 ian ian  8554460 tammi  21 13:02 tarzan
-rw-rw-r-- 1 ian ian     1440 tammi  21 13:01 temporary.crt
-rw-rw-r-- 1 ian ian     1704 tammi  21 13:01 temporary.key
```

## Janeserver and Systemd on Linux

Place the following systemd configuration in `/etc/systemd/system`  as `jane.service`

```
[Unit]
Description=Jane Attestation Engine
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
User=ian
ExecStart=/opt/jane/janeserver -config=/opt/jane/config.yaml

[Install]
WantedBy=multi-user.target
```

Ensure the `config.yaml` is properly configured for your system and installation.

Start with `systemctl start jane.service` and enable with `systemctl enable jane.service`. Use `journalctl -xe` to check startup and possible errors.


## Tarzan 

This is how to start tarzan. It works on BSDs, Linux, Windows and quite a few others depending upon the binary. Instructions here for starting with systemd and rc.3 which'll probably transfer between many Linux and BSD installations. Windows seems to work too

### Linux with Systemd 

Place the following systemd configuration in `/etc/systemd/system`  as `tarzan.service`

Note tarzan may require root to run. Take note of any security aspects.

```
[Unit]
Description=Tarzan Trust Agent
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
User=root
ExecStart=/opt/jane/tarzan

[Install]
WantedBy=multi-user.target
```

Start with `systemctl start tarzan.service` and enable with `systemctl enable tarzan.service`. Use `journalctl -xe` to check startup and possible errors.

### Windows

This is possible. In the respository in `dist` is a file `TarzanTrustAgent.xml` which provides some hints on this.

### BSD (rc.d)

Yes too. This script placed in `/etc/rc.d` called `tarzan` works for startup, at least on my OpenBSD VM:

```
#!/bin/sh
#
# $OpenBSD: tarzan

daemon="/opt/jane/tarzan"

. /etc/rc.d/rc.subr

rc_cmd $1
```