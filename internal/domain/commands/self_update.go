package commands

type SelfUpdate interface {
	Execute(dryRun, force bool) error
}
