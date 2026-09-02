#!/usr/bin/env bash

# 1. PODMAN_VERSION is always used to derive the machine-os image tag (x.y) so this must be valid
# at any given time.
# 2. Both PODMAN_VERSION and PODMAN_PR_NUM will have to be updated manually on release
# PRs.
# 3. If PODMAN_PR_NUM is empty, rpms will be fetched from the `rhcontainerbot/podman-next` copr.
export PODMAN_VERSION="6.1.1"
export PODMAN_PR_NUM="29700"
