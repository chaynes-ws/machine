package localbinary

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chaynes-ws/machine/libmachine/log"
)

var (
	// Timeout where we will bail if we're not able to properly contact the
	// plugin server.
	defaultTimeout = 10 * time.Second
	CoreDrivers    = [...]string{"amazonec2", "azure", "digitalocean",
		"exoscale", "generic", "google", "hyperv", "none", "openstack",
		"rackspace", "softlayer", "virtualbox", "vmwarefusion",
		"vmwarevcloudair", "vmwarevsphere"}
)

const (
	pluginOut           = "(%s) %s"
	pluginErr           = "(%s) DBG | %s"
	PluginEnvKey        = "MACHINE_PLUGIN_TOKEN"
	PluginEnvVal        = "42"
	PluginEnvDriverName = "MACHINE_PLUGIN_DRIVER_NAME"
)

type PluginStreamer interface {
	// Return a channel for receiving the output of the stream line by
	// line, and a channel for stopping the stream when we are finished
	// reading from it.
	//
	// It happens to be the case that we do this all inside of the main
	// plugin struct today, but that may not be the case forever.
	AttachStream(*bufio.Scanner) (<-chan string, chan<- bool)
}

type PluginServer interface {
	// Get the address where the plugin server is listening.
	Address() (string, error)

	// Serve kicks off the plugin server.
	Serve() error

	// Close shuts down the initialized server.
	Close() error
}

type McnBinaryExecutor interface {
	// Execute the driver plugin.  Returns scanners for plugin binary
	// stdout and stderr.
	Start() (*bufio.Scanner, *bufio.Scanner, error)

	// Stop reading from the plugins in question.
	Close() error
}

// DriverPlugin interface wraps the underlying mechanics of starting a driver
// plugin server and then figuring out where it can be dialed.
type DriverPlugin interface {
	PluginServer
	PluginStreamer
}

type Plugin struct {
	Executor    McnBinaryExecutor
	Addr        string
	MachineName string
	addrCh      chan string
	stopCh      chan bool
}

type Executor struct {
	pluginStdout, pluginStderr io.ReadCloser
	DriverName                 string
	binaryPath                 string
}

type ErrPluginBinaryNotFound struct {
	driverName string
}

func (e ErrPluginBinaryNotFound) Error() string {
	return fmt.Sprintf("Driver %q not found. Do you have the plugin binary accessible in your PATH?", e.driverName)
}

func driverPath(driverName string) string {
	for _, coreDriver := range CoreDrivers {
		if coreDriver == driverName {
			return os.Args[0] // "docker-machine"
		}
	}

	return fmt.Sprintf("docker-machine-driver-%s", driverName)
}

func NewPlugin(driverName string) (*Plugin, error) {
	driverPath := driverPath(driverName)
	binaryPath, err := exec.LookPath(driverPath)
	if err != nil {
		return nil, ErrPluginBinaryNotFound{driverName}
	}

	log.Debugf("Found binary path at %s", binaryPath)

	return &Plugin{
		stopCh: make(chan bool),
		addrCh: make(chan string, 1),
		Executor: &Executor{
			DriverName: driverName,
			binaryPath: binaryPath,
		},
	}, nil
}

func (lbe *Executor) Start() (*bufio.Scanner, *bufio.Scanner, error) {
	var err error

	log.Debugf("Launching plugin server for driver %s", lbe.DriverName)

	cmd := exec.Command(lbe.binaryPath)

	lbe.pluginStdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("Error getting cmd stdout pipe: %s", err)
	}

	lbe.pluginStderr, err = cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("Error getting cmd stderr pipe: %s", err)
	}

	outScanner := bufio.NewScanner(lbe.pluginStdout)
	errScanner := bufio.NewScanner(lbe.pluginStderr)

	os.Setenv(PluginEnvKey, PluginEnvVal)
	os.Setenv(PluginEnvDriverName, lbe.DriverName)

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("Error starting plugin binary: %s", err)
	}

	return outScanner, errScanner, nil
}

func (lbe *Executor) Close() error {
	if err := lbe.pluginStdout.Close(); err != nil {
		return err
	}

	if err := lbe.pluginStderr.Close(); err != nil {
		return err
	}

	return nil
}

func stream(scanner *bufio.Scanner, streamOutCh chan<- string, stopCh <-chan bool) {
	lines := make(chan string)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	for {
		select {
		case <-stopCh:
			close(streamOutCh)
			return
		case line := <-lines:
			streamOutCh <- strings.Trim(line, "\n")
			if err := scanner.Err(); err != nil {
				log.Warnf("Scanning stream: %s", err)
			}
		}
	}
}

func (lbp *Plugin) AttachStream(scanner *bufio.Scanner) (<-chan string, chan<- bool) {
	streamOutCh := make(chan string)
	stopCh := make(chan bool)
	go stream(scanner, streamOutCh, stopCh)
	return streamOutCh, stopCh
}

func (lbp *Plugin) execServer() error {
	outScanner, errScanner, err := lbp.Executor.Start()
	if err != nil {
		return err
	}

	// Scan just one line to get the address, then send it to the relevant
	// channel.
	outScanner.Scan()
	addr := outScanner.Text()
	if err := outScanner.Err(); err != nil {
		return fmt.Errorf("Reading plugin address failed: %s", err)
	}

	lbp.addrCh <- strings.TrimSpace(addr)

	stdOutCh, stopStdoutCh := lbp.AttachStream(outScanner)
	stdErrCh, stopStderrCh := lbp.AttachStream(errScanner)

	for {
		select {
		case out := <-stdOutCh:
			log.Infof(pluginOut, lbp.MachineName, out)
		case err := <-stdErrCh:
			log.Debugf(pluginErr, lbp.MachineName, err)
		case _ = <-lbp.stopCh:
			stopStdoutCh <- true
			stopStderrCh <- true
			if err := lbp.Executor.Close(); err != nil {
				return fmt.Errorf("Error closing local plugin binary: %s", err)
			}
			return nil
		}
	}
}

func (lbp *Plugin) Serve() error {
	return lbp.execServer()
}

func (lbp *Plugin) Address() (string, error) {
	if lbp.Addr == "" {
		select {
		case lbp.Addr = <-lbp.addrCh:
			log.Debugf("Plugin server listening at address %s", lbp.Addr)
			close(lbp.addrCh)
			return lbp.Addr, nil
		case <-time.After(defaultTimeout):
			return "", fmt.Errorf("Failed to dial the plugin server in %s", defaultTimeout)
		}
	}
	return lbp.Addr, nil
}

func (lbp *Plugin) Close() error {
	lbp.stopCh <- true
	return nil
}
