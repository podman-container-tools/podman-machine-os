#!/usr/bin/env bash

# 1. PODMAN_VERSION is always used to derive the machine-os image tag (x.y) so this must be valid
# at any given time.
# 2. Both PODMAN_VERSION and PODMAN_PR_NUM will have to be updated manually on release
# PRs.
# 3. If PODMAN_PR_NUM is empty, rpms will be fetched from the `rhcontainerbot/podman-next` copr.
export PODMAN_VERSION="6.0.0-dev"
export PODMAN_PR_NUM=""

# Temporary work around due the fact that podman 6 needs the new nv/av 2.0 which is also not in
# the fedora stable repos until f45
export NETAVARK_PR_NUM="1464"
export AARDVARK_DNS_PR_NUM="706"
export CONTAINERS_COMMON_PR_NUM="864"
export SKOPEO_PR_NUM="2881"
