#!/bin/bash
set -eo pipefail

mkdir -p /etc/containers/registries.conf.d \
         /etc/systemd/system.conf.d \
         /etc/environment.d \
         /etc/containers/registries.conf.d \
         /etc/ssh/sshd_config.d \
         /etc/sysctl.d \
         /etc/chrony.d \
         /etc/systemd/system/user@.service.d

# Install config files

cat >/etc/chrony.d/50-podman-makestep.conf <<EOF
makestep 1 -1
EOF

echo "confdir /etc/chrony.d" >> /etc/chrony.conf

cat >/etc/profile.d/docker-host.sh <<'EOF'
export DOCKER_HOST="unix://$(podman info -f "{{.Host.RemoteSocket.Path}}")"
EOF

cat >/etc/containers/registries.conf.d/999-podman-machine.conf <<EOF
# Issue #11489: make sure that we can inject a custom registries.conf
# file on the system level to force a single search registry.
# The remote client does not yet support prompting for short-name
# resolution, so we enforce a single search registry (i.e., docker.io)
# as a workaround.

unqualified-search-registries=["docker.io"]
EOF

cat >/etc/sysctl.d/10-inotify-instances.conf <<EOF
fs.inotify.max_user_instances=524288
EOF

cat >/etc/ssh/sshd_config.d/99-podman-sshd.conf <<EOF
# There seems to be big problem on macos with connecting to ssh to early and
# it seems to count that as auth failure locking us out. Podman machine only
# runs locally and there ar eno remote users that can connect so just disable
# it.
PerSourcePenalties authfail:0

# If many podman commands are run simultaneously, sshd may drop some of the
# connections. There are no remote users so set the limit very high.
MaxStartups 65535
EOF

## Set delegate.conf so cpu,io subsystem is delegated to non-root users as well for cgroupv2
## by default
cat >/etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
Delegate=memory pids cpu io
EOF


# 1. For main branch builds, replace aardvark-dns, conmon, crun, netavark, podman, containers-common
# 2. For release branch builds, fetch the build from the copr job on the podman
# release PR.
# 3. Remove moby-engine, containerd, runc, zincati for both dev and release builds
# Note: Currently does not result in a size reduction for the container image
# 4. Even though the URLs mention `rawhide`, the repo and gpg files are Fedora
# release agnostic and such `rawhide` URLs are unlikely to change compared to URLs
# containing Fedora release numbers.
if [[ ${PODMAN_PR_NUM} == "" ]]; then \
    curl --fail -o /etc/yum.repos.d/rhcontainerbot-podman-next-fedora.repo https://copr.fedorainfracloud.org/coprs/rhcontainerbot/podman-next/repo/fedora-rawhide/rhcontainerbot-podman-next-fedora-rawhide.repo
    curl --fail -o /etc/pki/rpm-gpg/rhcontainerbot-podman-next-fedora.gpg https://download.copr.fedorainfracloud.org/results/rhcontainerbot/podman-next/pubkey.gpg
    dnf install --best -y \
    aardvark-dns crun netavark podman containers-common containers-common-extra crun-wasm
else
    shopt -s nullglob
    FILE="/var/tmp/rpms/*.rpm"
    if [[ -n $(echo $FILE) ]]; then dnf update -y --best --allowerasing $FILE; fi
    curl --fail -o /etc/yum.repos.d/podman-release-copr.repo https://copr.fedorainfracloud.org/coprs/packit/podman-container-tools-podman-${PODMAN_PR_NUM}/repo/fedora-rawhide/packit-podman-container-tools-podman-${PODMAN_PR_NUM}-fedora-rawhide.repo
    curl --fail -o /etc/pki/rpm-gpg/podman-release-copr.gpg https://download.copr.fedorainfracloud.org/results/packit/podman-container-tools-podman-${PODMAN_PR_NUM}/pubkey.gpg
    dnf install --best -y podman
fi

# Install subscription-manager and enable service to refresh certificates
# Install qemu-user-static for bootc/user emulation
# Install device-mapper (this satisfies the deps for qemu-user-static
# We don't want all weak deps for these packages here.
dnf install -y --setopt=install_weak_deps=false \
    subscription-manager device-mapper qemu-user-static-aarch64 qemu-user-static-x86


# Package list to install
PACKAGES=(
    # for hyperV and WSL user mode networking
    gvisor-tap-vsock-gvforwarder

    # ansible for post-install configuration (podman machine init --playbook)
    ansible-core

    # WSL specific deps (most of them are already in the coreos base so this is a NOP there)
    procps-ng
    openssh-server
    cifs-utils
    nfs-utils-coreos
    iptables-nft
    iproute
    dhcp-client

    # cpp for buildah.in support
    cpp

    # git-core for Containerfile `ADD <gitrepo>` clone feature
    git-core
)

dnf install -y "${PACKAGES[@]}"

# Remove unwanted packages (mainly relevant for coreos)
dnf remove -y moby-engine containerd runc toolbox qed-firmware docker-cli

# Remove man pages (man binary is not present)
rm -fr /var/cache /usr/share/man
dnf -y clean all

systemctl enable rhsmcertd.service
# Patching qemu backed binfmt configurations to use the actual executable's permissions and not the interpreter's
for x in /usr/lib/binfmt.d/*.conf; do sed 's/\(:[^C:]*\)$/\1PC/' "$x" | tee /etc/binfmt.d/"$(basename "$x")"; done
