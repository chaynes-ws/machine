package provision

import (
	"github.com/chaynes-ws/machine/libmachine/auth"
	"github.com/chaynes-ws/machine/libmachine/engine"
)

type EngineConfigContext struct {
	DockerPort       int
	AuthOptions      auth.Options
	EngineOptions    engine.Options
	DockerOptionsDir string
}
