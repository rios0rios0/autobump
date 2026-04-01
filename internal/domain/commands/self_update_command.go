package commands

type SelfUpdateCommand struct {
	runnerFn SelfUpdateRunnerFunc
}

func NewSelfUpdateCommand(runnerFn SelfUpdateRunnerFunc) *SelfUpdateCommand {
	return &SelfUpdateCommand{runnerFn: runnerFn}
}

func (c *SelfUpdateCommand) Execute(dryRun, force bool) error {
	return c.runnerFn(dryRun, force)
}
