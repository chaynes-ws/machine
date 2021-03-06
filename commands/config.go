package commands

import (
	"fmt"

	"github.com/chaynes-ws/machine/libmachine"
	"github.com/chaynes-ws/machine/libmachine/check"
	"github.com/chaynes-ws/machine/libmachine/log"
)

func cmdConfig(c CommandLine, api libmachine.API) error {
	// Ensure that log messages always go to stderr when this command is
	// being run (it is intended to be run in a subshell)
	log.RedirectStdOutToStdErr()

	if len(c.Args()) != 1 {
		return ErrExpectedOneMachine
	}

	host, err := api.Load(c.Args().First())
	if err != nil {
		return err
	}

	dockerHost, authOptions, err := check.DefaultConnChecker.Check(host, c.Bool("swarm"))
	if err != nil {
		return fmt.Errorf("Error running connection boilerplate: %s", err)
	}

	log.Debug(dockerHost)

	fmt.Printf("--tlsverify\n--tlscacert=%q\n--tlscert=%q\n--tlskey=%q\n-H=%s\n",
		authOptions.CaCertPath, authOptions.ClientCertPath, authOptions.ClientKeyPath, dockerHost)

	return nil
}
