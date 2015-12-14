package commands

import "github.com/chaynes-ws/machine/libmachine"

func cmdUpgrade(c CommandLine, api libmachine.API) error {
	return runAction("upgrade", c, api)
}
