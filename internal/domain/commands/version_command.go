package commands

import logger "github.com/sirupsen/logrus"

// AutobumpVersion is set at build time via ldflags through the main package bridge.
// During development (`go run`), it defaults to "dev".
//
//nolint:gochecknoglobals // Version set at build time via ldflags
var AutobumpVersion = "dev"

type VersionCommand struct{}

func NewVersionCommand() *VersionCommand {
	return &VersionCommand{}
}

func (c *VersionCommand) Execute() {
	logger.Infof("autobump version: %s", AutobumpVersion)
}
