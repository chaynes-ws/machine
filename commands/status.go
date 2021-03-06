package commands

import (
	"github.com/chaynes-ws/machine/libmachine"
	"github.com/chaynes-ws/machine/libmachine/log"
)

func cmdStatus(c CommandLine, api libmachine.API) error {
	if len(c.Args()) != 1 {
		return ErrExpectedOneMachine
	}

	host, err := api.Load(c.Args().First())
	if err != nil {
		return err
	}

	currentState, err := host.Driver.GetState()
	if err != nil {
		log.Errorf("error getting state for host %s: %s", host.Name, err)
	}

	log.Info(currentState)

	return nil
}
