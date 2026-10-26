# Example container files (quick start)

Jane, MongoDB and Mosquitto in one image, and a file to run it on Kubernetes
(#93). **Quick start only, not hardened, linux/amd64 only** (a template
anywhere else). Full description: [docs/containers.md](../../docs/containers.md).

| File | |
|---|---|
| `Dockerfile` | the all-in-one image, on the official `mongo` image |
| `Dockerfile.dockerignore` | its build context: only the files it copies |
| `start.sh` | starts MongoDB and Mosquitto, then Jane in the foreground |
| `mosquitto.conf` | MQTT on 1883, WebSockets on 8081, anonymous |
| `jane-k8s.yaml` | Namespace, Deployment and Service for `kubectl apply -f` |

From the top of the repository:

```bash
make -C janeserver binary VERSION=1.5.6
docker build -f etc/dockerfiles/Dockerfile -t jane-allinone:1.5.6 .
docker run -d --name jane -p 8520:8520 -p 8540:8540 -p 3270:3270 \
    -p 1883:1883 -p 8081:8081 jane-allinone:1.5.6
```

Then open `http://<host>:8540` and press **Load standard intents** on the
Intents page.
