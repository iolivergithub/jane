# Table of contents

- [Running JANESERVER](#running-janeserver)
  - [JANESERVER Configuration File](#janeserver-configuration-file)
  - [Using Keylime for Measured Boot evaluation](#using-keylime-for-measured-boot-evaluation)
- [Running TARZAN](#running-tarzan)
  - [Command line options](#command-line-options)
  - [Unsafe operation - Here be a good way to open your system to every hacker ever](#unsafe-operation---here-be-a-good-way-to-open-your-system-to-every-hacker-ever)

# Running JANESERVER

Janeserver requires a configuration file and optionally keys for the https certs. We've supplied a temporary key in the dist folder...don't use these unless you're crazy. We also like triggering github to give us private key warnings because we've stored them there. Browsers will complain unless your certs a signed by a suitable authority, eg: LetsTrust.

NOTE: this document only deals with running things natively, if you want [docker.md](docker) instructions please go to that section.

An example configuration file with explanation is given below. Store this in some known place, eg: /etc

Generate some keys with a suitable name (not temporary.key) and store these in a suitable place too:

```bash
openssl genrsa 2048 > temporary.key
chmod 400 temporary.key 
openssl req -new -x509 -nodes -sha256 -days 365 -key temporary.key -out temporary.crt
```

NOTE: you'll find `temporary.key` and `temporary.crt` in a few places in this repository. *DO NOT USE THESE KEYS* - they're just for demonstration purposes and if you do use them then you are crazy and this will result in a VERY insecure system. *You have been warned*.

EDIT the `config.yaml` configuration file so that the locations of these keys are recorded there.

To start `janeserver` run and point the config option to where the config file is

```bash
./janeserver -config=config.yaml
```

You should see something similar to the following. If a config file is not found it will exit immediately. If Mosquitto or Mongo are not available then it will eventually time out. Finally if you are using https for the webui service, if the keys are not found it will exit with an error. The name of the database, location of config file and session identifier will all be different of course.

```bash
generating private, public key pair for claim signing - just for this session so no chance to verify later. THese keys MUST be external
JANE: Configuration file location:  /home/ian/config.yaml
JANE: initialising database MONGO connection
JANE: Database infrastructure MONGO is running
JANE: Initialising message infrastructure MQTT connection
JANE: Message infrastructure MQTT is running
JANE: MQTT connected

+========================================================================================
|  JANE version
|   + linux O/S on amd64
|   + version v1.0rc1, build not set
|   + runing with name ASVR_GO_1_TEST
|   + session identifier is 5eddb86d-ce05-4319-91aa-d4815d61b008
+========================================================================================

X3270 service listening on port 3270
⇨ http server started on [::]:8520
⇨ http server started on [::]:8540
```

If that works, point your browser at the machine where this is running and port 8540.

## JANESERVER Configuration File

Note the lines with "CHANGE ME" - review these for your system.

   * The name of the system can set to anything you want.
   * The MQTT client ID must be unique if you indend running more than one instance
   * The MQTT port probably doesn't need chaning
   * The MOTT server must allow anonymous connections, read the Mosquitto documentation for this
   * Address of the monogoDB server must be set
   * Choose any name you want for your attestation database
   * crt and key files should have the full path to the keys generated earlier
   * usehttp -- set this to true if you want less#CHANGE ME security
   * X3270 -- don't worry about this, but if you have an X3270 terminal, the TUI might be useful (when we finish it)
   * Logging goes to a default place in /var/log or somewhere suitable on Windows

The lines with "DEFAULT VALUE" most likely do not need to change, but review for your system just in case.

Read the sections on security and keys

```yaml
#Some general naming
system:
  name: ASVR_GO_1_TEST                              #CHANGE ME

#MongoDB Configuration
database:
  connection: mongodb://192.168.1.203:27017         #CHANGE ME
  name: attestationDatabase                         #CHANGE ME

#MQTT Configuration
messaging:
  broker: 192.168.1.203                             #CHANGE ME
  port: 1883                                        #DEFAULT VALUE
  clientid: attestationMQTTclient                   #CHANGE ME

#REST Interface Configuration
rest:
  port: 8520                                        #DEFAULT VALUE
  crt: temporary.crt                                #CHANGE ME
  key: temporary.key                                #CHANGE ME
  usehttp: false                                    #DEFAULT VALUE

#Web Interface Configuration
web:
  port: 8540                                        #DEFAULT VALUE
  crt: temporary.crt                                #CHANGE ME
  key: temporary.key                                #CHANGE ME
  usehttp: false                                    #DEFAULT VALUE

#X3270
x3270:
  port: 3270                                        #DEFAULT VALUE
  
#Log file
logging:
  logfilelocation: /var/log/jane.log                #DEFAULT VALUE
  sessionupdatelogging: false                       #DEFAULT VALUE
```

## Using Keylime for Measured Boot evaluation

JANESERVER supports calling Keylime to validate a Measured Boot log.  For this currently it requires a special version found here: https://github.com/THS-on/keylime/tree/api/standalone-validation

The simplest way using it is to build a local Keylime Docker container:
```
git clone https://github.com/THS-on/keylime.git
cd keylime
git checkout standalone-validation 
cd docker/release
./build_locally.sh
```
After this JANE can be used with the included docker-compose.yml file. See [docker.md](docker) for more information on this.

To include Keylime access add the following section to the `config.yaml` file:

```yaml
#Keylime
keylime:
  apiurl: https://127.0.0.1:30000/keylime                   #CHANGE ME
```





# Running TARZAN

Tarzan is a reference trust agent implementation that responds to the A10HTTPREST protocol. Running tarzan is simple, just speicfy which services you want started, eg: sys and tpm2...

```bash
./tarzan --tpm2 --sys
```

```bash
+========================================================================================
|  tarzan version - Starting
|   + linux O/S on amd64
|   + version v0.1, build not set
|   + session identifier is 19a14951-76c3-4641-b9ac-fa65683e5c36
|   + unsafe mode? false
+========================================================================================

⇨ http server started on [::]:8530
```

If you are running on Linux and need access to files such as the UEFI log file then you will need to run tarzan as sudo.

tarzan requires access to the TPM device, eg `/dev/tpm0` on Linux (Windows handles this internally), and so whichever user tarzan is running as needs access to that device.

```bash
sudo ./tarzan --tpm2 --sys
```


## Command line options

Tarzan's services and configuration is all done by command line flags. At least one of these must be specified for tarzan to respond to anything at all. At minimum the sys service just to report what system you are running on should be enabled.

| Flag |  Description |
| --- | --- | --- |
| --tpm2 | Start the services to respond to TPM2 attestation requests |
| --uefi | Start the services to respond to UEFI attestation requests |
| --ima | Start the services to respond to Linux IMA attestation requests |
| --txt | Start the services to respond to Intel TXT attestation requests |
| --sys | Start the services to respond to TPM2 attestation requests |


tarzan by default listens on port 8530, this can be changed using the --port option

For example, to start tarzan on port 4789 and reporting on uefi, ima and sys you would use (possibly with sudo):

```bash
tarzan --sysy --uefi --ima --port=4789
```

## Unsafe operation - Here be a good way to open your system to every hacker ever

tarzan can read UEFI and IMA logs in non-standard places, but in order to do this, the element description in the Jane's database would have to refer to those specifically. tarzan by default operates in a *safe* mode where it will only use the standard locations in Linux's securityfs. You can turn off this mode:

```bash
sudo ./tarzan -unsafe=true
```

which responds with

```bash
$ sudo ./tarzan -unsafe=true
+========================================================================================
|  tarzan version - Starting
|   + linux O/S on amd64
|   + version v0.1, build not set
|   + session identifier is 4e85a08d-7d1c-450d-9a7a-659f29ab8380
|   + unsafe mode? true
+========================================================================================


!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
tarzan is running in UNSAFE file access mode.  Unsafe is set to true
Requests for log files, eg: UEFI, IMA, that supply a non default location will happily read that file
This is a HUGE security issue. YOU HAVE BEEN WARNED
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
⇨ http server started on [::]:8530
```

Really, *don't do this*...you've now given everyone with access to port 8530 (and it is all over http) root access to every file on your system.

