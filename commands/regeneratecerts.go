package commands

import (
	"github.com/chaynes-ws/machine/libmachine"
	"github.com/chaynes-ws/machine/libmachine/log"
)

func cmdRegenerateCerts(c CommandLine, api libmachine.API) error {
	if !c.Bool("force") {
		ok, err := confirmInput("Regenerate TLS machine certs?  Warning: this is irreversible.")
		if err != nil {
			return err
		}

		if !ok {
			return nil
		}
	}

	log.Infof("Regenerating TLS certificates")

	return runAction("configureAuth", c, api)
}
