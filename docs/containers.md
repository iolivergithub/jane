# Containers (quick start)

`etc/dockerfiles/` has two examples for trying Jane out (#93): one image with Jane, MongoDB and Mosquitto, and a file to run that image on Kubernetes with `kubectl apply`. They are a **quick start, not a deployment**: experienced administrators adapt them (separate containers, accounts, TLS, storage). More complete versions may follow later.

- [What the image is](#what-the-image-is)
- [Docker](#docker)
- [Kubernetes](#kubernetes)
- [Not covered](#not-covered)

## What the image is

| | |
|---|---|
| Base | the official `mongo:8.0-noble` image (MongoDB 8.0 on Ubuntu 24.04) |
| Added | Mosquitto (Ubuntu package), `janeserver`, `addjaneuser` (with `python3-argon2`, `python3-pymongo`) |
| Configuration | `/etc/opt/jane/config.yaml` as shipped in the deb (`etc/debbuild/config.yaml`): MongoDB and the broker at `127.0.0.1`, listening on all interfaces, plain HTTP, no logins; the `REPLACE_ME` development certificate; `standardintents.json` and `standardobjects.json` |
| Started by | `/opt/jane/start.sh`: `mongod` and `mosquitto` in the background, then `janeserver` in the foreground once MongoDB answers, so the container stops when Jane does |
| Log | `/var/log/jane/jane.log`; MongoDB's in `/var/log/mongod.log` |

**Ports**

| Port | |
|---|---|
| 8520 | REST API (jp, scripts, tarzan's Jane) |
| 8540 | Web UI |
| 3270 | x3270 (tn3270) |
| 1883 | MQTT |
| 8081 | MQTT over WebSockets (the Web UI's Live page) |

**Unhardened, and meant to be:** everything runs as root in one container; no TLS; anonymous MQTT; MongoDB without an account; the development certificate; no logins; no volume unless you add one. See [Hardening](hardening.md) for what a real deployment needs.

**linux/amd64 only.** The image and the Kubernetes file are references for "standard" linux/amd64 machines, where they run. Anything else is up to the administrator of that system: the official `mongo` image exists only for linux/amd64 and linux/arm64, its MongoDB needs AVX on x86-64 (some VMs hide it from the guest: `grep -c avx /proc/cpuinfo`) and ARMv8.2-A on arm64. Use the files as a template there, e.g. with another MongoDB image or MongoDB outside the container.

## Docker

Build janeserver, then the image, from the top of the repository (the build context is the repository; `Dockerfile.dockerignore` limits it to the files the image copies):

```bash
make -C janeserver binary VERSION=1.5.6
docker build -f etc/dockerfiles/Dockerfile -t jane-allinone:1.5.6 .
```

Run it, publishing the five ports:

```bash
docker run -d --name jane -p 8520:8520 -p 8540:8540 -p 3270:3270 \
    -p 1883:1883 -p 8081:8081 jane-allinone:1.5.6
docker logs -f jane
```

MongoDB's data is inside the container and goes with it. To keep it, add a volume: `-v jane-mongodb:/data/db`.

Open `http://<host>:8540`, go to **Intents** and press **Load standard intents** (and **Load standard objects** on the Opaque objects page if you want them). Then provision a machine against it: on the machine, with tarzan running, set `attestationserver: http://<host>:8520` in the provisioning file and run `jp create` (see [jp](jp.md)). Jane must be able to reach tarzan on port 8530, so give the machine's address, not `127.0.0.1`, in the provisioning file's tarzan endpoint.

**Users.** The configuration has no logins. To try them, change `auth:` in a copy of `config.yaml` (mount it over `/etc/opt/jane/config.yaml` with `-v`) and add users in the running container:

```bash
docker exec -it jane /opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
docker exec jane /opt/jane/addjaneuser --apikey jp mongodb://127.0.0.1:27017 asvr
```

To stop and remove it: `docker rm -f jane` (and `docker volume rm jane-mongodb`).

## Kubernetes

`etc/dockerfiles/jane-k8s.yaml` runs the image above: a Namespace `jane`, a Deployment with one replica (one pod: MongoDB and Mosquitto are inside it) on amd64 nodes, and a Service of type `LoadBalancer` with the five ports.

1. Build the image as above and make it available to the cluster:
   - with a registry: `docker tag jane-allinone:1.5.6 registry.example.org/jane/jane-allinone:1.5.6` and `docker push` it;
   - on **K3s** without a registry: `docker save jane-allinone:1.5.6 -o jane.tar`, then on each node `sudo k3s ctr images import jane.tar`, and use the image name `docker.io/library/jane-allinone:1.5.6`.
2. Change the `image:` line in `jane-k8s.yaml` to that name.
3. Apply it and find the addresses:

   ```bash
   kubectl apply -f etc/dockerfiles/jane-k8s.yaml
   kubectl -n jane get pods
   kubectl -n jane get svc jane
   ```

   On K3s the built-in ServiceLB serves `LoadBalancer` services on the nodes' addresses; on clusters without a load balancer change the type to `NodePort` (the ports are then mapped to 30000–32767). With minikube, run `minikube tunnel`.
4. Users, if you turn logins on: `kubectl -n jane exec -it deploy/jane -- /opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr`.

MongoDB's data is on an `emptyDir` and lost when the pod is deleted or moved. To keep it, uncomment the PersistentVolumeClaim at the end of the file and the `persistentVolumeClaim` lines in the Deployment (and remove `emptyDir`). The Deployment uses the `Recreate` strategy, so the old pod has released the volume before the new one starts.

To remove everything: `kubectl delete -f etc/dockerfiles/jane-k8s.yaml` (deletes the namespace and its contents).

## Not covered

TLS for the REST API and the Web UI, accounts for MongoDB and Mosquitto, Secrets for the configuration, a NetworkPolicy, separate containers for MongoDB and Mosquitto (or the official images of both), health checks beyond the readiness probe, and backups. [Hardening](hardening.md) describes these for Jane itself; the container versions are left to the administrator.
