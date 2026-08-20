#!/bin/bash

source ./podman-rpm-info-vars.sh

CPU_ARCH=$(uname -m)
export DISK_IMAGE_NAME="podman-machine"

declare -A ARCH_TO_IMAGE_ARCH=(
    ["x86_64"]="amd64"
    ["aarch64"]="arm64"
)

# Map architectures to their supported disk image platforms
# Note: WSL is built separately (not via custom-coreos-disk-images)
# applehv is only supported on Apple Silicon Macs (aarch64)
declare -A ARCH_TO_PLATFORMS=(
    ["x86_64"]="hyperv qemu wsl"
    ["aarch64"]="applehv hyperv qemu wsl"
)

# Platforms that are built via custom-coreos-disk-images (excludes WSL)
declare -A ARCH_TO_COREOS_PLATFORMS=(
    ["x86_64"]="hyperv qemu"
    ["aarch64"]="applehv hyperv qemu"
)

declare -A PLATFORM_TO_DISKTYPE=(
    ["applehv"]="raw"
    ["hyperv"]="vhdx"
    ["qemu"]="qcow2"
    ["wsl"]="tar"
)

get_disk_artifact_names() {
  for arch in "${!ARCH_TO_PLATFORMS[@]}"; do
    for platform in ${ARCH_TO_PLATFORMS[$arch]}; do
      echo "$DISK_IMAGE_NAME.$arch.$platform.${PLATFORM_TO_DISKTYPE[$platform]}.zst"
    done
  done
}

get_oci_artifact_names() {
  for arch in "${!ARCH_TO_PLATFORMS[@]}"; do
    echo "$DISK_IMAGE_NAME.$arch.tar"
  done
}

get_all_artifact_names() {
  get_disk_artifact_names
  get_oci_artifact_names
}

_get_part_from_name() {
  local array
  IFS='.' read -r -a array <<< "$1"
  echo "${array[$2]}"
}

get_arch_from_name() {
  _get_part_from_name "$1" 1
}

get_disk_type_from_name() {
  _get_part_from_name "$1" 2
}

# OUTDIR needs to be run in TMT test as a non-root user after initial root
# login
export SRCDIR="${TMT_TREE:-${GITHUB_WORKSPACE:-..}}"
export OUTDIR="${OUTDIR:-${TMT_TEST_DATA:-$(git rev-parse --show-toplevel)/outdir}}"

REPO="${REPO:-quay.io/podman}"
OCI_NAME="machine-os"
# Image version is only x.y so we trim of the .z part here
OCI_VERSION="${PODMAN_VERSION%.*}"
FULL_IMAGE_NAME="${REPO}/${OCI_NAME}:${OCI_VERSION}"

export FULL_IMAGE_NAME_ARCH="$FULL_IMAGE_NAME-${ARCH_TO_IMAGE_ARCH[$CPU_ARCH]}"

# For released images we want the stable stream but for early testing in CI let's use the next stream.
FCOS_STREAM="${FCOS_STREAM:-stable}"
#if [[ "$GITHUB_REF_NAME" == "main" || "$GITHUB_BASE_REF" == "main" ]]; then
#  FCOS_STREAM="next"
#fi

FCOS_BASE_IMAGE="quay.io/fedora/fedora-coreos:$FCOS_STREAM"
export FCOS_BASE_IMAGE
