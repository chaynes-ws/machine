package commands

import (
	"testing"

	"github.com/chaynes-ws/machine/commands/commandstest"
	"github.com/chaynes-ws/machine/drivers/fakedriver"
	"github.com/chaynes-ws/machine/libmachine/host"
	"github.com/chaynes-ws/machine/libmachine/libmachinetest"
	"github.com/chaynes-ws/machine/libmachine/state"
	"github.com/stretchr/testify/assert"
)

func TestCmdIPMissingMachineName(t *testing.T) {
	commandLine := &commandstest.FakeCommandLine{}
	api := &libmachinetest.FakeAPI{}

	err := cmdURL(commandLine, api)

	assert.EqualError(t, err, "Error: Expected one machine name as an argument")
}

func TestCmdIP(t *testing.T) {
	commandLine := &commandstest.FakeCommandLine{
		CliArgs: []string{"machine"},
	}
	api := &libmachinetest.FakeAPI{
		Hosts: []*host.Host{
			{
				Name: "machine",
				Driver: &fakedriver.Driver{
					MockState: state.Running,
					MockIP:    "1.2.3.4",
				},
			},
		},
	}

	stdoutGetter := commandstest.NewStdoutGetter()
	defer stdoutGetter.Stop()

	err := cmdIP(commandLine, api)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4\n", stdoutGetter.Output())
}
