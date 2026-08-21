package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

var originalHomeDir = os.Getenv("HOME")

const (
	defaultTimeout         = 10 * time.Minute
	podmanBinaryName       = "podman"
	podmanRemoteBinaryName = "podman-remote"
)

type ImageTestBuilder interface {
	setName(name string) *ImageTestBuilder
	setCmd(mc []string) *ImageTestBuilder
	setTimeout(duration time.Duration) *ImageTestBuilder
	run() (*machineSession, error)
	withDiskSize() (*machineSession, error)
}
type machineSession struct {
	*Session
}

type imageTestBuilder struct {
	cmd       []string
	imagePath string
	name      string
	timeout   time.Duration
	session   *machineSession
	diskSize  uint
}

// waitWithTimeout waits for a command to complete for a given
// number of seconds
func (ms *machineSession) waitWithTimeout(timeout time.Duration) {
	Eventually(ms, timeout).Should(Exit(), func() string {
		// Note eventually does not kill the command as such the command is leaked forever without killing it
		// Also let's use SIGABRT to create a go stack trace so in case there is a deadlock we see it.
		ms.Signal(syscall.SIGABRT)
		// Give some time to let the command print the output so it is not printed much later
		// in the log at the wrong place.
		time.Sleep(1 * time.Second)
		return fmt.Sprintf("command timed out after %fs: %v",
			timeout.Seconds(), ms.Command.Args)
	})
}

func (ms *machineSession) Bytes() []byte {
	return []byte(ms.outputToString())
}

func (ms *machineSession) outputToStringSlice() []string {
	var results []string
	output := string(ms.Out.Contents())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// outputToString returns the output from a session in string form
func (ms *machineSession) outputToString() string {
	if ms == nil || ms.Out == nil || ms.Out.Contents() == nil {
		return ""
	}

	fields := strings.Fields(string(ms.Out.Contents()))
	return strings.Join(fields, " ")
}

// errorToString returns the error output from a session in string form
func (ms *machineSession) errorToString() string {
	if ms == nil || ms.Err == nil || ms.Err.Contents() == nil {
		return ""
	}
	return string(ms.Err.Contents())
}

// newMB constructor for machine test builders
func newMB() (*imageTestBuilder, error) {
	mb := imageTestBuilder{
		timeout: defaultTimeout,
	}
	if os.Getenv("MACHINE_TEST_TIMEOUT") != "" {
		seconds, err := strconv.Atoi(os.Getenv("MACHINE_TEST_TIMEOUT"))
		if err != nil {
			return nil, err
		}
		mb.timeout = time.Duration(seconds) * time.Second
	}
	mb.imagePath = imagePath
	return &mb, nil
}

// setName sets the name of the virtuaql machine for the command
func (m *imageTestBuilder) setName(name string) *imageTestBuilder {
	m.name = name
	return m
}

// setCmd takes a machineCommand struct and assembles a cmd line
// representation of the podman machine command
func (m *imageTestBuilder) setCmd(c []string) *imageTestBuilder {
	m.cmd = c
	return m
}

func (m *imageTestBuilder) withDiskSize(s uint) *imageTestBuilder {
	m.diskSize = s
	return m
}

func (m *imageTestBuilder) setTimeout(timeout time.Duration) *imageTestBuilder { //nolint: unparam
	m.timeout = timeout
	return m
}

func (m *imageTestBuilder) runWithStdin(stdin io.Reader) (*machineSession, error) {
	return runWrapper(podmanBinary, m.cmd, m.timeout, stdin)
}

func (m *imageTestBuilder) run() (*machineSession, error) {
	return runWrapper(podmanBinary, m.cmd, m.timeout, nil)
}

func (m *imageTestBuilder) initNowWithName(rootful bool) (string, *machineSession, error) {
	machineName := randomString()
	diskSize := defaultDiskSize
	if m.diskSize > 0 {
		diskSize = m.diskSize
	}
	optRootful := "--rootful=false"
	if rootful {
		optRootful = "--rootful"
	}

	cmdLine := []string{"machine", "init", "--now", "--image", m.imagePath, "--disk-size", strconv.Itoa(int(diskSize)), optRootful, machineName}
	session, err := m.setName(machineName).setCmd(cmdLine).run()
	return machineName, session, err
}

func runWrapper(podmanBinary string, cmdArgs []string, timeout time.Duration, stdin io.Reader) (*machineSession, error) {
	if len(os.Getenv("DEBUG")) > 0 {
		cmdArgs = append([]string{"--log-level=debug"}, cmdArgs...)
	}
	GinkgoWriter.Println(podmanBinary + " " + strings.Join(cmdArgs, " "))
	c := exec.Command(podmanBinary, cmdArgs...)
	c.Stdin = stdin
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("Unable to start session: %q", err))
		return nil, err
	}
	ms := machineSession{session}
	ms.waitWithTimeout(timeout)
	return &ms, nil
}

// randomString returns a string of given length composed of random characters
func randomString() string {
	length := 12
	b := make([]byte, length)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

type ValidJSONMatcher struct {
	types.GomegaMatcher
}

func BeValidJSON() *ValidJSONMatcher {
	return &ValidJSONMatcher{}
}

func (matcher *ValidJSONMatcher) Match(actual interface{}) (success bool, err error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ValidJSONMatcher expects a string, not %q", actual)
	}

	var i interface{}
	if err := json.Unmarshal([]byte(s), &i); err != nil {
		return false, err
	}
	return true, nil
}

func (matcher *ValidJSONMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be valid JSON")
}

func (matcher *ValidJSONMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to _not_ be valid JSON")
}

// Only used on Windows
//
//nolint:unparam,unused
func runSystemCommand(binary string, cmdArgs []string, timeout time.Duration, wait bool) (*machineSession, error) {
	GinkgoWriter.Println(binary + " " + strings.Join(cmdArgs, " "))
	c := exec.Command(binary, cmdArgs...)
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("Unable to start session: %q", err))
		return nil, err
	}
	ms := machineSession{session}
	if wait {
		ms.waitWithTimeout(timeout)
	}
	return &ms, nil
}

// Note: this will need to be moved Linux and everything else
// to distinguish between podman and podman-remote
func getPodmanBinary() (string, error) {
	if env := os.Getenv("PODMAN_BINARY"); env != "" {
		return env, nil
	}
	podmanBinary := getPodmanBinaryName()
	return exec.LookPath(podmanBinary)
}
