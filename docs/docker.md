# Building and Running with Docker

First ensure you know how to build things manually...or not. 

NOTE: make sure you are familiar with Docker.

## Building the JANESERVER container

In the janeserver directory is a Dockerfile, run the following

```bash
sudo docker buildx build . --network=host -t janeserver
```

This will generate a large amount of output and eventually store a container called `janeserver` in your local Docker repository cache. The output for an example build looks like this:

```bash
ian@ian-virtual-machine:~/jane/janeserver$ sudo docker buildx build . --network=host -t janeserver
[+] Building 47.9s (26/26) FINISHED                                                                                               docker:default
 => [internal] load build definition from Dockerfile                                                                                        0.0s
 => => transferring dockerfile: 1.55kB                                                                                                      0.0s
 => [internal] load .dockerignore                                                                                                           0.0s
 => => transferring context: 2B                                                                                                             0.0s
 => [internal] load metadata for docker.io/amd64/ubuntu:23.04                                                                               1.4s
 => [build-stage  1/19] FROM docker.io/amd64/ubuntu:23.04@sha256:ea1285dffce8a938ef356908d1be741da594310c8dced79b870d66808cb12b0f           0.0s
 => [internal] load build context                                                                                                           0.0s
 => => transferring context: 8.56kB                                                                                                         0.0s
 => CACHED [build-stage  2/19] RUN apt-get update                                                                                           0.0s
 => CACHED [build-stage  3/19] RUN apt-get install -y wget                                                                                  0.0s
 => CACHED [build-stage  4/19] RUN ["wget","https://go.dev/dl/go1.22.4.linux-amd64.tar.gz"]                                                 0.0s
 => CACHED [build-stage  5/19] RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz                               0.0s
 => CACHED [build-stage  6/19] RUN export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin               0.0s
 => CACHED [build-stage  7/19] RUN mkdir -p /etc/apt/keyrings                                                                               0.0s
 => CACHED [build-stage  8/19] RUN ["wget",  "https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key","-O","/etc/apt/keyrings  0.0s
 => CACHED [build-stage  9/19] RUN echo "deb [signed-by=/etc/apt/keyrings/intel-sgx-keyring.asc arch=amd64] https://download.01.org/intel-  0.0s
 => CACHED [build-stage 10/19] RUN apt update                                                                                               0.0s
 => CACHED [build-stage 11/19] RUN ["wget","https://github.com/edgelesssys/edgelessrt/releases/download/v0.4.1/edgelessrt_0.4.1_amd64_ubun  0.0s
 => CACHED [build-stage 12/19] RUN apt-get install -y ./edgelessrt_0.4.1_amd64_ubuntu-22.04.deb  build-essential cmake libssl-dev libsgx-d  0.0s
 => CACHED [build-stage 13/19] WORKDIR /app                                                                                                 0.0s
 => [build-stage 14/19] COPY . /app                                                                                                         0.1s
 => [build-stage 15/19] COPY go.mod go.sum ./                                                                                               0.0s
 => [build-stage 16/19] RUN /usr/local/go/bin/go mod download                                                                              19.7s
 => [build-stage 17/19] RUN /usr/local/go/bin/go mod tidy                                                                                   2.5s
 => [build-stage 18/19] RUN . /opt/edgelessrt/share/openenclave/openenclaverc && GOOS=linux GOARCH=amd64 /usr/local/go/bin/go build -ldfl  23.2s 
 => [build-stage 19/19] RUN strip /janeserver                                                                                               0.4s 
 => CACHED [stage-1 3/4] RUN apt-get install -y libssl-dev                                                                                  0.0s 
 => CACHED [stage-1 4/4] COPY --from=build-stage /janeserver /janeserver                                                                    0.0s 
 => exporting to image                                                                                                                      0.0s 
 => => exporting layers                                                                                                                     0.0s
 => => writing image sha256:446009731622517876c409ccc68c7192a8c60c4a21cdc562907f8f16b1ac9e99                                                0.0s
 => => naming to docker.io/library/janeserver                                                                                               0.0s
ian@ian-virtual-machine:~/jane/janeserver$ 

```

To check the image:

```bash
ian@ian-virtual-machine:~/jane/janeserver$ sudo docker images
REPOSITORY          TAG             IMAGE ID       CREATED          SIZE
janeserver          latest          446009731622   40 minutes ago   145MB
```


## Running with Docker Compose

In the directory `jane/etc/dockercompose` you can find everything required to run including a suitable `config.yaml` file which can be edited if necessary. Do not edit however the locations of mosquitto (messagebus) or mongo as these entries are provided by the local domain name services of Docker.

Refer to the section on generating keys in [running.md](running janeserver). We provide temporary keys for testing and running insane levels of unsecurity.

If necessary create a network and volume

```bash
sudo docker volume create attestationdata
sudo docker network create attestationnetwork
```

To bring up the service:

```bash
sudo docker compose up -d
```

## Security, Resiliency, Reliability, Load Balancing etc

Nope....if you want to add LetsTrust and ngnix to it feel free. I have plans so contact me, or go see the issues on the gitlab pages.

But as an example, mongo-express has this when you start it up...

```bash
databaseui-1  | Server is open to allow connections from anyone (0.0.0.0)
databaseui-1  | basicAuth credentials are "admin:pass", it is recommended you change this in your config.js!
```

and the mosquitto.conf has the lovely `allow_anonymous` and the config file for Jane sets everything to use http and uses the temporary keys

I tried, but I got really really fed up of trying to do reverse proxy, URL rewriting and all that with nginx and traefik, I never want to see those every again - and another Medium post which is a cut down, ChatGPT written, error filled "tutorial"....just no


The `-d` can be omitted during testing - including this starts in daemon mode

To stop the service (if in daemon mode), else Ctrl-C works fine.

```bash
sudo docker compose down
```


### Services Provided

The following services are running and available externally by default

| Service | Port | Description |
| --- | --- | --- |
| Jane Web UI | 8540 | The web UI for interacting in a basic manner with Jane |
| REST API | 8520 | The REST API for Jane |
| X3270 | 3270 | The X3270 interface |
| Mongo Express | 8555 | This is really useful...mongo-express for interacting directly with Jane's database |



## TARZAN

Refer back to [compiling](compiling.md) to understand how to build tarzan. This normally doesn't need to be run in a container (and we don't really recommend it anyway)