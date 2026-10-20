# Quick start

This page takes one Debian 13 machine with a TPM 2.0 from nothing to its first attestation. It uses the packaged defaults throughout: HTTP, no logins, no keys of your own, and no database or broker accounts.

> **This is for a test machine.** Everything here is unencrypted and open to anyone who can reach the machine. When it works, go through [Hardening](hardening.md) before Jane is reachable from a network you don't fully trust.

Jane, tarzan and jp all run on the same machine here. Later you would normally run Jane on one machine and tarzan and jp on each machine (element) you want to attest.

| Program | What it is | Runs on |
|---|---|---|
| janeserver | the attestation engine: Web UI (port 8540), REST API (8520), x3270 terminal (3270) | the Jane host |
| MongoDB | Jane's database | the Jane host (or anywhere Jane can reach) |
| Mosquitto | MQTT broker: Jane publishes its log there; the Web UI's Live page reads it | the Jane host |
| tarzan | the trust agent: answers Jane's requests for TPM quotes, event logs and system information (port 8530) | each element |
| jp | the provisioner: registers the element with Jane, sets up its TPM keys, records expected values, attests | each element |

You need `sudo` (or a root shell), network access for `apt`, and the two packages `jane_<version>_amd64.deb` and `tarzan_<version>_amd64.deb` (from a release, or built as in [Compiling](compiling.md)).

## 1. MongoDB

MongoDB is not in Debian. Install MongoDB 8.0 from MongoDB's own repository. MongoDB publishes it for Debian 12 ("bookworm"); that repository is used here on Debian 13. For other systems see [MongoDB's installation guide](https://www.mongodb.com/docs/v8.0/tutorial/install-mongodb-on-debian/).

```bash
sudo apt-get install -y gnupg curl
curl -fsSL https://pgp.mongodb.com/server-8.0.asc | sudo gpg -o /usr/share/keyrings/mongodb-server-8.0.gpg --dearmor
echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/debian bookworm/mongodb-org/8.0 main" | sudo tee /etc/apt/sources.list.d/mongodb-org-8.0.list
sudo apt-get update
sudo apt-get install -y mongodb-org
sudo systemctl enable --now mongod
```

MongoDB listens on 127.0.0.1 only and needs no account, which is what Jane's packaged configuration expects.

## 2. Mosquitto

```bash
sudo apt-get install -y mosquitto
```

Mosquitto 2 accepts only local, anonymous clients until it is told otherwise. Jane connects locally on port 1883; the Web UI's **Live** page connects from your browser over WebSockets on port 8081. Create `/etc/mosquitto/conf.d/jane.conf`:

```
# Jane: MQTT for janeserver on this machine, WebSockets for the Web UI's Live page
listener 1883 127.0.0.1
allow_anonymous true

listener 8081
protocol websockets
allow_anonymous true
```

```bash
sudo systemctl restart mosquitto
```

## 3. Jane and tarzan

Release files are gzipped; unzip them first, then install both packages:

```bash
gunzip jane_<version>_amd64.deb.gz tarzan_<version>_amd64.deb.gz
sudo apt install ./jane_<version>_amd64.deb ./tarzan_<version>_amd64.deb
```

The jane package creates the `jane` system user, installs to `/opt/jane` (programs) and `/etc/opt/jane` (configuration), and enables Jane without starting it. The tarzan package starts tarzan straight away. [Installing with dpkg/apt](deb.md) describes exactly what is installed.

Start Jane:

```bash
sudo systemctl start jane
systemctl status jane          # should be "active (running)"
```

If it keeps restarting, `journalctl -u jane` shows why; usually MongoDB or Mosquitto is not running.

## 4. The Web UI and the standard intents

Open `http://<machine>:8540` in a browser (`http://127.0.0.1:8540` on the machine itself). The home page shows the database, messaging and configuration.

Go to **Intents** and press **Load standard intents**. Intents describe what to ask a trust agent for (a quote over certain PCRs, an event log, system information); jp refers to them by name.

## 5. Register this machine (jp create)

Copy the example provisioning file and look through it:

```bash
sudo cp /etc/opt/jane/provisioning.yaml.example /etc/opt/jane/provisioning.yaml
sudo nano /etc/opt/jane/provisioning.yaml
```

At least change `element: name` to this machine's name. The defaults expect Jane and tarzan on this machine and the TPM at `/dev/tpmrm0`.

Then register the machine:

```bash
sudo /opt/jane/jp create --reprovisiontpm /etc/opt/jane/provisioning.yaml
```

`--reprovisiontpm` lets jp run the two worklist steps that change the TPM:

- **tpmclear** deletes the keys stored at the EK and AK handles (0x810100EE and 0x810100AA), so they can be made afresh. It does not clear the rest of the TPM, but anything another program stored at exactly those handles is lost.
- **tpmprovision** creates the endorsement key (EK) and attestation key (AK) at those handles.

Without the flag both steps are skipped and the TPM is left as it is; use that if the TPM already has jp's keys. jp never asks questions, so every command here can also run unattended: only `--reprovisiontpm` (and `--recreate`) allow anything destructive.

jp then collects information about the machine, registers it with Jane as an element, writes the element's ID to `/etc/opt/jane/janeelementid`, and records the expected values (the "known good" PCR values, firmware version, machine ID, UEFI variables) for each intent in the file.

## 6. Attest (jp attest)

```bash
/opt/jane/jp attest /etc/opt/jane/provisioning.yaml
```

`attest` needs no root. It asks Jane to attest the element against every intent in the file and to run every rule listed there, and prints a table of results. Exit status 0 means every rule passed.

## 7. Look at the results

In the Web UI:

- **Sessions**: the attestation you just ran, with its claims and results.
- **Elements**: this machine, its TPM keys and its expected values.
- **Live** (the screen icon in the menu bar): Jane's log as it happens (run `jp attest` again to watch it).

[Getting started](gettingstarted.md) is a tour of the Web UI from here.

## Next

- [Hardening](hardening.md): your own TLS keys, logins, database and broker accounts, restricting addresses, the signing key. Do this before Jane is reachable from other machines.
- [Running Jane, tarzan and jp](running.md): every option, flag and environment variable, and running on other systems.
- [Contents](contents.md): all the documentation.
