package libmachine

import (
	"fmt"
	"path/filepath"

	"github.com/chaynes-ws/machine/libmachine/auth"
	"github.com/chaynes-ws/machine/libmachine/cert"
	"github.com/chaynes-ws/machine/libmachine/check"
	"github.com/chaynes-ws/machine/libmachine/crashreport"
	"github.com/chaynes-ws/machine/libmachine/drivers"
	"github.com/chaynes-ws/machine/libmachine/engine"
	"github.com/chaynes-ws/machine/libmachine/host"
	"github.com/chaynes-ws/machine/libmachine/log"
	"github.com/chaynes-ws/machine/libmachine/mcnutils"
	"github.com/chaynes-ws/machine/libmachine/persist"
	"github.com/chaynes-ws/machine/libmachine/provision"
	"github.com/chaynes-ws/machine/libmachine/ssh"
	"github.com/chaynes-ws/machine/libmachine/state"
	"github.com/chaynes-ws/machine/libmachine/swarm"
	"github.com/chaynes-ws/machine/libmachine/version"
)

type API interface {
	persist.Store
	persist.PluginDriverFactory
	NewHost(drivers.Driver) (*host.Host, error)
	Create(h *host.Host) error
}

type Client struct {
	*persist.PluginStore
	IsDebug        bool
	SSHClientType  ssh.ClientType
	GithubAPIToken string
}

func NewClient(storePath string) *Client {
	certsDir := filepath.Join(storePath, ".docker", "machine", "certs")
	return &Client{
		IsDebug:       false,
		SSHClientType: ssh.External,
		PluginStore:   persist.NewPluginStore(storePath, certsDir, certsDir),
	}
}

func (api *Client) NewHost(driver drivers.Driver) (*host.Host, error) {
	certDir := filepath.Join(api.Path, "certs")

	hostOptions := &host.Options{
		AuthOptions: &auth.Options{
			CertDir:          certDir,
			CaCertPath:       filepath.Join(certDir, "ca.pem"),
			CaPrivateKeyPath: filepath.Join(certDir, "ca-key.pem"),
			ClientCertPath:   filepath.Join(certDir, "cert.pem"),
			ClientKeyPath:    filepath.Join(certDir, "key.pem"),
			ServerCertPath:   filepath.Join(api.GetMachinesDir(), "server.pem"),
			ServerKeyPath:    filepath.Join(api.GetMachinesDir(), "server-key.pem"),
		},
		EngineOptions: &engine.Options{
			InstallURL:    "https://get.docker.com",
			StorageDriver: "aufs",
			TLSVerify:     true,
		},
		SwarmOptions: &swarm.Options{
			Host:     "tcp://0.0.0.0:3376",
			Image:    "swarm:latest",
			Strategy: "spread",
		},
	}

	return &host.Host{
		ConfigVersion: version.ConfigVersion,
		Name:          driver.GetMachineName(),
		Driver:        driver,
		DriverName:    driver.DriverName(),
		HostOptions:   hostOptions,
	}, nil
}

// Create is the wrapper method which covers all of the boilerplate around
// actually creating, provisioning, and persisting an instance in the store.
func (api *Client) Create(h *host.Host) error {
	if err := cert.BootstrapCertificates(h.HostOptions.AuthOptions); err != nil {
		return fmt.Errorf("Error generating certificates: %s", err)
	}

	log.Info("Running pre-create checks...")

	if err := h.Driver.PreCreateCheck(); err != nil {
		return fmt.Errorf("Error with pre-create check: %s", err)
	}

	if err := api.Save(h); err != nil {
		return fmt.Errorf("Error saving host to store before attempting creation: %s", err)
	}

	log.Info("Creating machine...")

	if err := api.performCreate(h); err != nil {
		crashreport.Send(err, "api.performCreate", h.DriverName, "Create")
		return err
	}

	log.Debug("Reticulating splines...")

	return nil
}

func (api *Client) performCreate(h *host.Host) error {

	if err := h.Driver.Create(); err != nil {
		return fmt.Errorf("Error in driver during machine creation: %s", err)
	}

	if err := api.Save(h); err != nil {
		return fmt.Errorf("Error saving host to store after attempting creation: %s", err)
	}

	// TODO: Not really a fan of just checking "none" here.
	if h.Driver.DriverName() != "none" {
		log.Info("Waiting for machine to be running, this may take a few minutes...")
		if err := mcnutils.WaitFor(drivers.MachineInState(h.Driver, state.Running)); err != nil {
			return fmt.Errorf("Error waiting for machine to be running: %s", err)
		}

		log.Info("Machine is running, waiting for SSH to be available...")
		if err := drivers.WaitForSSH(h.Driver); err != nil {
			return fmt.Errorf("Error waiting for SSH: %s", err)
		}

		log.Info("Detecting operating system of created instance...")
		provisioner, err := provision.DetectProvisioner(h.Driver)
		if err != nil {
			return fmt.Errorf("Error detecting OS: %s", err)
		}

		log.Infof("Provisioning with %s...", provisioner.String())
		if err := provisioner.Provision(*h.HostOptions.SwarmOptions, *h.HostOptions.AuthOptions, *h.HostOptions.EngineOptions); err != nil {
			return fmt.Errorf("Error running provisioning: %s", err)
		}

		// We should check the connection to docker here
		log.Info("Checking connection to Docker...")
		if _, _, err = check.DefaultConnChecker.Check(h, false); err != nil {
			return fmt.Errorf("Error checking the host: %s", err)
		}

		log.Info("Docker is up and running!")
	}

	return nil

}
