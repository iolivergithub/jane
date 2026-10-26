#!/bin/sh
# Starts MongoDB and Mosquitto in the background, waits for MongoDB, then runs
# janeserver in the foreground, so that the container stops when Jane does
# (#93). Quick start only: see docs/containers.md.
set -e

mongod --bind_ip 127.0.0.1 --dbpath /data/db --logpath /var/log/mongod.log --fork
mosquitto -c /etc/opt/jane/mosquitto.conf -d

echo "waiting for MongoDB..."
i=0
until mongosh --quiet --eval 'db.runCommand({ ping: 1 }).ok' >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -ge 60 ]; then
        echo "MongoDB did not start; see /var/log/mongod.log" >&2
        exit 1
    fi
    sleep 1
done

echo "starting janeserver"
exec /opt/jane/janeserver --config=/etc/opt/jane/config.yaml
