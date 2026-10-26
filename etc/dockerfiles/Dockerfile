# Jane, MongoDB and Mosquitto in one container: a QUICK START for trying Jane
# out (#93). NOT HARDENED: everything runs as root in one container, no TLS,
# anonymous MQTT, the development certificate, MongoDB without an account, and
# no volume unless you add one. Experienced administrators adapt it; see
# docs/containers.md and docs/hardening.md.
#
# linux/amd64 only. It is a reference for "standard" linux/amd64, where it
# runs; anywhere else (other architectures, CPUs without AVX, which MongoDB
# needs, other base images) use it as a template.
#
# Build janeserver first, then the image, from the top of the repository:
#
#   make -C janeserver binary VERSION=1.5.6
#   docker build -f etc/dockerfiles/Dockerfile -t jane-allinone:1.5.6 .
#
# Run:
#
#   docker run -d --name jane -p 8520:8520 -p 8540:8540 -p 3270:3270 \
#       -p 1883:1883 -p 8081:8081 jane-allinone:1.5.6

# The official MongoDB image (Ubuntu 24.04 "noble").
FROM mongo:8.0-noble

# Mosquitto for MQTT; python3-argon2 and python3-pymongo for addjaneuser.
RUN apt-get update \
 && apt-get install -y --no-install-recommends mosquitto python3 python3-argon2 python3-pymongo \
 && rm -rf /var/lib/apt/lists/*

# Jane, laid out as the deb package installs it.
COPY janeserver/janeserver /opt/jane/janeserver
COPY etc/addjaneuser/addjaneuser /opt/jane/addjaneuser
COPY etc/debbuild/config.yaml etc/debbuild/REPLACE_ME.crt etc/debbuild/REPLACE_ME.key /etc/opt/jane/
COPY etc/standardintents/standardintents.json etc/standardobjects/standardobjects.json /etc/opt/jane/
COPY etc/dockerfiles/mosquitto.conf /etc/opt/jane/mosquitto.conf
COPY etc/dockerfiles/start.sh /opt/jane/start.sh
RUN chmod 755 /opt/jane/janeserver /opt/jane/addjaneuser /opt/jane/start.sh \
 && mkdir -p /var/log/jane

# 8520 REST API, 8540 Web UI, 3270 x3270, 1883 MQTT, 8081 MQTT over WebSockets
EXPOSE 8520 8540 3270 1883 8081

ENTRYPOINT ["/opt/jane/start.sh"]
