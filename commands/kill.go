package commands

import "github.com/chaynes-ws/machine/libmachine"

func cmdKill(c CommandLine, api libmachine.API) error {
	return runAction("kill", c, api)
}
