package verify

import (
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("run image tests", Ordered, ContinueOnFailure, func() {
	var (
		machineName string
		session     *machineSession
		mb          *imageTestBuilder
		testDir     string
	)

	BeforeAll(func() {
		testDir, mb = setup()
		machineName, session, err = mb.initNowWithName(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		DeferCleanup(func() {
			// stop and remove all machines first before deleting the processes
			clean := []string{"machine", "reset", "-f"}
			session, err := mb.setCmd(clean).run()

			teardown(originalHomeDir, testDir)

			// check errors only after we called teardown() otherwise it is not called on failures
			Expect(err).ToNot(HaveOccurred(), "cleaning up after test")
			Expect(session).To(Exit(0))
		})
	})

	Context("when the podman machine is up and running", func() {
		It("should serve the Podman rootful API", func() {
			curlCmd := []string{"machine", "ssh", machineName, "sudo", "curl", "-sS", "--unix-socket", "/run/podman/podman.sock", "http://d/_ping"}
			curlSession, err := mb.setCmd(curlCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(curlSession).To(Exit(0))
			Expect(curlSession.outputToString()).To(Equal("OK"))
		})
		It("should serve the Podman rootless API", func() {
			// The host user ID is used as the `core` user ID in the Podman
			// machine (except for Windows where the ID is hardcoded).
			uid := os.Getuid()
			if uid == -1 { // windows compensation
				uid = 1000
			}
			rootlessSocket := "/run/user/" + strconv.Itoa(uid) + "/podman/podman.sock"
			curlCmd := []string{"machine", "ssh", machineName, "curl", "-sS", "--unix-socket", rootlessSocket, "http://d/_ping"}
			curlSession, err := mb.setCmd(curlCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(curlSession).To(Exit(0))
			Expect(curlSession.outputToString()).To(Equal("OK"))
		})
		It("should apply the `10-inotify-instances.conf` successfully", func() {
			maxUserInstancesCmd := []string{"machine", "ssh", machineName, "sysctl", "fs.inotify.max_user_instances"}
			maxUserInstancesSession, err := mb.setCmd(maxUserInstancesCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(maxUserInstancesSession).To(Exit(0))
			Expect(maxUserInstancesSession.outputToString()).To(ContainSubstring("fs.inotify.max_user_instances = 524288"))
		})

		It("podman-user-wait-network-online.service works", func() {
			skipIfVmtype(WSLVirt, "no network manager in WSL")

			cmd := []string{"machine", "ssh", machineName, "systemctl", "--user", "start", "podman-user-wait-network-online.service"}
			session, err := mb.setCmd(cmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))

			cmd = []string{"machine", "ssh", machineName, "systemctl", "--user", "-P", "ActiveState", "show", "podman-user-wait-network-online.service"}
			session, err = mb.setCmd(cmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))
			Expect(session.outputToString()).To(Equal("active"))
		})

		It("should apply the `10-autologin.conf` successfully", func() {
			skipIfVmtype(WSLVirt, "no tty for WSL")
			autologinArgv := "argv[]=/usr/sbin/agetty --autologin root --noclear ttyS0 $TERM"
			autologinCmd := []string{"machine", "ssh", machineName, "systemctl", "-P", "ExecStart", "show", "getty@ttyS0"}
			autologinSerialCmd := []string{"machine", "ssh", machineName, "systemctl", "-P", "ExecStart", "show", "serial-getty@ttyS0.service"}
			autologinSession, err := mb.setCmd(autologinCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(autologinSession).To(Exit(0))
			autologinSerialSession, err := mb.setCmd(autologinSerialCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(autologinSerialSession).To(Exit(0))
			Expect(autologinSession.outputToString()).To(And(ContainSubstring(autologinArgv), Equal(autologinSerialSession.outputToString())))
		})
		It("should have zero failed services", func() {
			skipIfVmtype(WSLVirt, "systemd-binfmt.service failing, see https://github.com/containers/podman/issues/19961")

			getFailedSvcCmd := []string{"machine", "ssh", machineName, "systemctl", "--failed", "-q"}
			getFailedSvcSession, err := mb.setCmd(getFailedSvcCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(getFailedSvcSession).To(Exit(0))
			Expect(getFailedSvcSession.outputToString()).To(BeEmpty())
		})
		It("should load RHEL subscription manager service", func() {
			subscriptionManagerCmd := []string{"machine", "ssh", machineName, "systemctl", "-P", "LoadState", "show", "rhsmcertd"}
			subscriptionManagerSession, err := mb.setCmd(subscriptionManagerCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(subscriptionManagerSession).To(Exit(0))
			Expect(subscriptionManagerSession.outputToString()).To(Equal("loaded"))
		})
		It("should load Rosetta activation service", func() {
			skipIfNotVmtype(AppleHvVirt, "Rosetta service is activated when the provider is applehv")
			rosettaCmd := []string{"machine", "ssh", machineName, "systemctl", "-P", "ActiveState", "show", "rosetta-activation.service"}
			rosettaSession, err := mb.setCmd(rosettaCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(rosettaSession).To(Exit(0))
			Expect(rosettaSession.outputToString()).To(Equal("active"))
		})
		It("should ship qemu-guest-agent for vsock (FCOS images)", func() {
			skipIfVmtype(WSLVirt, "WSL image does not use the CoreOS Containerfile customizations")
			rpmCmd := []string{"machine", "ssh", machineName, "rpm", "-q", "qemu-guest-agent"}
			rpmSession, err := mb.setCmd(rpmCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(rpmSession).To(Exit(0))
			Expect(rpmSession.outputToString()).To(ContainSubstring("qemu-guest-agent"))

			catCmd := []string{"machine", "ssh", machineName, "systemctl", "cat", "qemu-guest-agent.service"}
			catSession, err := mb.setCmd(catCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(catSession).To(Exit(0))
			out := catSession.outputToString()
			Expect(out).To(ContainSubstring("vsock-listen"))
			Expect(out).To(ContainSubstring("--path=3:1234"))
		})
		It("should have qemu-guest-agent active on macOS providers", func() {
			skipIfVmtype(WSLVirt, "WSL image does not include qemu-guest-agent")
			skipIfVmtype(HyperVVirt, "HyperV does not expose a vsock channel for qemu-guest-agent")
			skipIfVmtype(QemuVirt, "QEMU does not need qemu-guest-agent for time sync")
			cmd := []string{"machine", "ssh", machineName, "systemctl", "-P", "ActiveState", "show", "qemu-guest-agent.service"}
			session, err := mb.setCmd(cmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))
			// "active" when the host exposes the vsock channel, "activating" when
			// qemu-ga is in a restart loop because the host side is not yet wired up.
			// Both confirm that the systemd conditions matched and the service was started.
			Expect(session.outputToString()).To(Or(Equal("active"), Equal("activating")))
		})
		It("should have zero critical error messages journalctl", func() {
			skipIfVmtype(LibKrun, "TODO: analyze the error messages in journalctl when using libkrun")
			skipIfVmtype(AppleHvVirt, "TODO: analyze the error messages in journalctl when using applehv")

			// seeing this logged on WSL: Exception:
			// 		unknown: Operation canceled @p9io.cpp:258 (AcceptAsync)
			skipIfVmtype(WSLVirt, "WSL kernel seems to log 9p warning by default")
			// Inspect journalctl for any message of priority
			// "emerg" (0), "alert" (1) or "crit" (2). Messages with
			// priority "err" (3) or "warning" (4) are tolerated.
			journalctlCmd := []string{"machine", "ssh", machineName, "journalctl", "-p", "0..2", "-q"}
			journalctlSession, err := mb.setCmd(journalctlCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(journalctlSession).To(Exit(0))
			Expect(journalctlSession.outputToString()).To(BeEmpty())
		})
		It("should have zero warning/error messages in podman or gvforwarder logs", func() {
			journalctlPodmanCmd := []string{"machine", "ssh", machineName, "journalctl", "-p", "0..4", "-q", "/usr/libexec/podman/gvforwarder", "/usr/bin/podman"}
			journalctlPodmanSession, err := mb.setCmd(journalctlPodmanCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(journalctlPodmanSession).To(Exit(0))
			Expect(journalctlPodmanSession.outputToString()).To(BeEmpty())
		})
		It("should start gvforwarder services successfully on hyperV", func() {
			skipIfNotVmtype(HyperVVirt, "gvforwarder is started on hyperV only")
			journalctlGvforwarderCmd := []string{"machine", "ssh", machineName, "journalctl", "/usr/libexec/podman/gvforwarder"}
			journalctlGvforwarderSession, err := mb.setCmd(journalctlGvforwarderCmd).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(journalctlGvforwarderSession).To(Exit(0))
			Expect(journalctlGvforwarderSession.outputToString()).To(ContainSubstring("waiting for packets..."))
		})

		It("no MOTD warnings logged", func() {
			skipIfVmtype(WSLVirt, "wsl image does not have the FCOS modt messages")
			// https://github.com/containers/podman-machine-os/issues/155
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName}).runWithStdin(strings.NewReader("id"))
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			output := sshSession.outputToString()
			Expect(output).To(ContainSubstring("Fedora CoreOS"), "should contain FCOS motd message")
			Expect(output).ToNot(ContainSubstring("Warning"), "no warnings in motd message seen")
			// no coreos-oci-migration-motd.service output should be seen
			Expect(output).ToNot(ContainSubstring("zincati"), "no zincati migrate wanring in motd message seen")
		})

		It("check systemd resolved is not in use", func() {
			// https://github.com/containers/podman-machine-os/issues/18
			// Note the service should not be installed as we removed the package at build time.
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "sudo", "systemctl", "is-active", "systemd-resolved.service"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(4))
			Expect(sshSession.outputToString()).To(Equal("inactive"))
		})

		It("check coreos-platform-chrony-config.service is not in use", func() {
			skipIfVmtype(WSLVirt, "wsl image does not have coreos-platform-chrony-config.service")
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "sudo", "systemctl", "is-active", "coreos-platform-chrony-config.service"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(3))
			Expect(sshSession.outputToString()).To(Equal("inactive"))
		})

		It("iptables module should be loaded", func() {
			skipIfVmtype(WSLVirt, "wsl kernel modules are statically defined in the kernel")
			// https://github.com/containers/podman/issues/25153
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "sudo", "lsmod"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(sshSession.outputToString()).To(And(ContainSubstring("ip_tables"), ContainSubstring("ip6_tables")))
		})

		It("verify custom podman config settings", func() {
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "cat", "/usr/share/containers/registries.conf.d/999-podman-machine.conf", "/usr/share/containers/containers.conf.d/999-podman-machine.conf"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(sshSession.outputToString()).To(And(ContainSubstring("unqualified-search-registries"), ContainSubstring("helper_binaries_dir")))
		})

		It("verify custom os-release variant", func() {
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "cat", "/etc/os-release"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(sshSession.outputToString()).To(And(ContainSubstring(`VARIANT="Podman Machine OS"`), ContainSubstring(`VARIANT_ID=podman-machine-os`)))

			archSession, err := mb.setCmd([]string{"info", "--format", "{{.Host.Distribution.Variant}}"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(archSession).To(Exit(0))
			Expect(archSession.outputToString()).To(Equal("podman-machine-os"))
		})

		It("check podman coreos image version", func() {
			skipIfVmtype(WSLVirt, "wsl does not use ostree updates")
			// set by podman-rpm-info-vars.sh
			version := os.Getenv("PODMAN_VERSION")
			if version == "" {
				Skip("PODMAN_VERSION undefined")
			}
			// When we have an rc package fedora uses "~rc" while the upstream version is "-rc".
			// As such we have to replace it so we can match the real version below.
			version = strings.ReplaceAll(version, "~", "-")
			// version is x.y.z while image uses x.y, remove .z so we can match
			imageVersion := version
			index := strings.LastIndex(version, ".")
			if index >= 0 {
				imageVersion = version[:index]
			}
			// verify the bootc image inside uses the proper podman image reference
			sshSession, err := mb.setCmd([]string{"machine", "ssh", machineName, "sudo bootc status --json | jq -r '.status.booted.image.image | .transport + \" \" +  .image'"}).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))
			Expect(sshSession.outputToString()).
				To(Equal("registry quay.io/podman/machine-os:" + imageVersion))

			// TODO: there is no 5.5 in the copr yet as podman main would need to be bumped.
			// But in order to do that it needs working machine images, catch-22.
			// Skip this check for now, we should consider only doing this check on release branches.
			// check the server version so we know we have the right version installed in the VM
			// server, err := mb.setCmd([]string{"version", "--format", "{{.Server.Version}}"}).run()
			// Expect(err).ToNot(HaveOccurred())
			// Expect(server).To(Exit(0))
			// Expect(server.outputToString()).To(Equal(version))
		})
	})
})
