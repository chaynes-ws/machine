package commands

import "github.com/chaynes-ws/machine/libmachine"

func cmdIP(c CommandLine, api libmachine.API) error {
	return runAction("ip", c, api)
}
