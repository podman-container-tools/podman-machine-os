#!/bin/bash

source ./podman-rpm-info-vars.sh

CPU_ARCH=$(uname -m)
declare -A ARCH_TO_IMAGE_ARCH=(
    ["x86_64"]="amd64"
    ["aarch64"]="arm64"
)


disk_format_from_flavor () {
  if [[ -z $1 ]]; then
    echo "no disk flavor passed"
    exit
  fi

  case $1 in
    "applehv")
      echo "raw"
      ;;
    "hyperv")
      echo "vhdx"
      ;;
    "qemu")
      echo "qcow2"
      ;;
    "wsl")
      echo "tar"
      ;;
    *)
      echo "unknown flavor $1"
      exit 1
      ;;
  esac
}


DISK_FLAVORS=("applehv" "hyperv" "qemu" "wsl")

# OUTDIR needs to be run in TMT test as a non-root user after initial root
# login
export SRCDIR="${TMT_TREE:-${GITHUB_WORKSPACE:-..}}"
export OUTDIR="${OUTDIR:-${TMT_TEST_DATA:-$(git rev-parse --show-toplevel)/outdir}}"
export DISK_IMAGE_NAME="podman-machine"

REPO="${REPO:-quay.io/podman}"
OCI_NAME="machine-os"
# Image version is only x.y so we trim of the .z part here
OCI_VERSION="${PODMAN_VERSION%.*}"
FULL_IMAGE_NAME="${REPO}/${OCI_NAME}:${OCI_VERSION}"

export FULL_IMAGE_NAME_ARCH="$FULL_IMAGE_NAME-${ARCH_TO_IMAGE_ARCH[$CPU_ARCH]}"

# For released images we want the stable stream but for early testing in CI let's use the next stream.
FCOS_STREAM="${FCOS_STREAM:-stable}"
if [[ "$GITHUB_REF_NAME" == "main" || "$GITHUB_BASE_REF" == "main" ]]; then
  FCOS_STREAM="next"
fi

FCOS_BASE_IMAGE="quay.io/fedora/fedora-coreos:$FCOS_STREAM"
export FCOS_BASE_IMAGE
