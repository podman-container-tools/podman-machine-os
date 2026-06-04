#!/usr/bin/env bash

# Note this was duplicated form the podman repo.

# Best-effort cleanup of podman machine state and leaked test processes.

set +e -x

pkill -f vfkit
pkill -f krunkit
pkill -f gvproxy
pkill -f ginkgo

rm -rf "$HOME/.local/share/containers/podman/machine"
rm -rf "$HOME/.config/containers/podman"
rm -rf "${TMPDIR:-/private/tmp/ci}/podman"

# Make we never error
true
