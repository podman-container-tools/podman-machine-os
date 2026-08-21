package verify

import (
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const TESTIMAGE = "quay.io/libpod/testimage:20241011"

var _ = Describe("run basic podman commands", func() {
	var (
		mb      *imageTestBuilder
		testDir string
	)
	BeforeEach(func() {
		testDir, mb = setup()
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

	It("Basic ops rootless", func() {
		machineName, session, err := mb.initNowWithName(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Pull an image
		pull := []string{"pull", TESTIMAGE}
		pullSession, err := mb.setCmd(pull).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(pullSession).To(Exit(0))

		// Check Images
		checkCmd := []string{"images", "--format", "{{.Repository}}:{{.Tag}}"}
		checkImages, err := mb.setCmd(checkCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(checkImages).To(Exit(0))
		Expect(len(checkImages.outputToStringSlice())).To(Equal(1))
		Expect(checkImages.outputToStringSlice()).To(ContainElement(TESTIMAGE))

		// Run simple container and check that host-gateway works
		// https://github.com/containers/podman/issues/21681
		runCmdDate := []string{"run", "-it", "--add-host=foobar123:host-gateway", TESTIMAGE, "cat", "/etc/hosts"}
		runCmdDateSession, err := mb.setCmd(runCmdDate).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runCmdDateSession).To(Exit(0))
		Expect(runCmdDateSession.outputToString()).To(ContainSubstring("foobar123"))

		// Run container in background
		runCmdTop := []string{"run", "-dt", TESTIMAGE, "top"}
		runTopSession, err := mb.setCmd(runCmdTop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runTopSession).To(Exit(0))

		// Check containers
		psCmd := []string{"ps", "-q"}
		psCmdSession, err := mb.setCmd(psCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(psCmdSession).To(Exit(0))
		Expect(len(psCmdSession.outputToStringSlice())).To(Equal(1))

		// Check all containers
		psCmdAll := []string{"ps", "-aq"}
		psCmdSessionAll, err := mb.setCmd(psCmdAll).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(psCmdSessionAll).To(Exit(0))
		Expect(len(psCmdSessionAll.outputToStringSlice())).To(Equal(2))

		// Stop all containers
		stopCmd := []string{"stop", "-a"}
		stopSession, err := mb.setCmd(stopCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Check container stopped
		doubleCheckCmd := []string{"ps", "-q"}
		doubleCheckCmdSession, err := mb.setCmd(doubleCheckCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(doubleCheckCmdSession).To(Exit(0))
		Expect(len(doubleCheckCmdSession.outputToStringSlice())).To(Equal(0))

		// pasta and bridge networks use different port forwarding logic, we must validate both
		verifyNetworking(mb, false, "pasta", "8080")

		networkCmdSession, err := mb.setCmd([]string{"network", "create", "--ipv6", "testnet"}).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(networkCmdSession).To(Exit(0))
		verifyNetworking(mb, false, "testnet", "8081")

		// systemd-binfmt.service is failing to configure emulation, so we cannot test that there yet
		// https://github.com/containers/podman/issues/19961
		if vmTestProvider != WSLVirt {
			// Test emulation so we know it always works, we had a kernel update
			// broke rosetta on applehv so we like to catch that the next time.
			var expectedArch string
			var goArch string
			switch runtime.GOARCH {
			case "amd64":
				goArch = "arm64"
				expectedArch = "aarch64"
			case "arm64":
				goArch = "amd64"
				expectedArch = "x86_64"
			}
			// quiet to not get the pull output
			archCommand := []string{"run", "--quiet", "--platform", "linux/" + goArch, TESTIMAGE, "arch"}
			archSession, err := mb.setCmd(archCommand).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(archSession).To(Exit(0))
			Expect(archSession.outputToString()).To(Equal(expectedArch))

			// check that argv[0] is preserved
			argvTestCommand := []string{"run", "--quiet", "--platform", "linux/" + goArch, TESTIMAGE, "sh", "-c", "echo $0"}
			argvSession, err := mb.setCmd(argvTestCommand).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(argvSession).To(Exit(0))
			// Equal is important as we need an exact match for "sh" which means the emulator preserved argv[0].
			// Previously it would show the executable path "/bin/sh".
			Expect(argvSession.outputToString()).To(Equal("sh"))
		}

		// Stop machine
		stopMachineCmd := []string{"machine", "stop", machineName}
		StopMachineSession, err := mb.setCmd(stopMachineCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(StopMachineSession).To(Exit(0))

		// Remove machine
		removeMachineCmd := []string{"machine", "rm", "-f", machineName}
		removeMachineSession, err := mb.setCmd(removeMachineCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeMachineSession).To(Exit(0))
	})

	It("Basic networking rootful", func() {
		_, session, err := mb.initNowWithName(true)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		networkCmdSession, err := mb.setCmd([]string{"network", "create", "--ipv6", "testnet"}).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(networkCmdSession).To(Exit(0))

		verifyNetworking(mb, true, "testnet", "8082")
	})

	It("machine stop/start cycle", func() {
		// We have seen an issue while stopping and starting machines again
		// and then causing ssh failures on the second start. So test it.
		machineName, session, err := mb.initNowWithName(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		stopMachineCmd := []string{"machine", "stop", machineName}
		stopMachineSession, err := mb.setCmd(stopMachineCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopMachineSession).To(Exit(0))

		startMachineCmd := []string{"machine", "start", machineName}
		startMachineSession, err := mb.setCmd(startMachineCmd).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startMachineSession).To(Exit(0))
	})
})

func verifyNetworking(mb *imageTestBuilder, rootful bool, network string, port string) {
	const containerName = "http"
	// verify networking works
	runCmd := []string{"run", "-d", "-p", port + ":80", "--network", network, "--name", containerName, TESTIMAGE, "/bin/busybox-extras", "httpd", "-f", "-p", "80"}
	runCmdSession, err := mb.setCmd(runCmd).run()
	Expect(err).ToNot(HaveOccurred())
	Expect(runCmdSession).To(Exit(0))

	// Small retry loop in the machine to ensure the http process is up and has bound the port
	// Otherwise the go code below might connect to early.
	for range 5 {
		curlCmd := []string{"machine", "ssh", mb.name, "sudo", "curl", "http://127.0.0.1:" + port}
		curlSession, err := mb.setCmd(curlCmd).run()
		Expect(err).ToNot(HaveOccurred())
		if curlSession.ExitCode() == 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	for _, name := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if rootful && vmTestProvider == WSLVirt && name == "[::1]" {
			// on rootful WSL we do not support forwarding on ::1
			continue
		}
		client := http.Client{
			// USe a custom client with a low timeout to avoid longs hangs on errors
			Timeout: 5 * time.Second,
		}
		// request known file in image
		url := "http://" + name + ":" + port + "/testimage-id"
		resp, err := client.Get(url)
		Expect(err).ToNot(HaveOccurred(), url)
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK), url)
		bytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred(), url)
		_, id, _ := strings.Cut(TESTIMAGE, ":")
		Expect(string(bytes)).To(Equal(id+"\n"), url)
	}

	rmCmdSession, err := mb.setCmd([]string{"rm", "-f", "-t0", containerName}).run()
	Expect(err).ToNot(HaveOccurred())
	Expect(rmCmdSession).To(Exit(0))
}
