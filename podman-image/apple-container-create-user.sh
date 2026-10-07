#!/bin/sh
# First boot user setup for Apple container machines. `container machine` runs
# /etc/machine/create-user.sh instead of its built-in script when it exists.
# This does what ignition does for the other providers: on top of creating the
# user it configures what is needed for rootless podman.
set -e

/sbin.machine/create-user.sh

for f in /etc/subuid /etc/subgid; do
    grep -q "^${CONTAINER_USER}:" "$f" 2>/dev/null || echo "${CONTAINER_USER}:100000:1000000" >>"$f"
done

# Start the user session (and thus the rootless podman.socket) at boot.
mkdir -p /var/lib/systemd/linger
touch "/var/lib/systemd/linger/${CONTAINER_USER}"
loginctl enable-linger "${CONTAINER_USER}" || true
