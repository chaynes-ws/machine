package commands

import (
	"errors"
	"fmt"

	"github.com/chaynes-ws/machine/libmachine"
	"github.com/chaynes-ws/machine/libmachine/persist"
)

var (
	errNoActiveHost = errors.New("No active host found")
)

func cmdActive(c CommandLine, api libmachine.API) error {
	if len(c.Args()) > 0 {
		return ErrTooManyArguments
	}

	hosts, hostsInError, err := persist.LoadAllHosts(api)
	if err != nil {
		return fmt.Errorf("Error getting active host: %s", err)
	}

	items := getHostListItems(hosts, hostsInError)

	for _, item := range items {
		if item.Active {
			fmt.Println(item.Name)
			return nil
		}
	}

	return errNoActiveHost
}
