package commands

// SelfUpdate is the domain contract for the self-update use case.
type SelfUpdate interface {
	Execute(dryRun, force bool) error
}

// SelfUpdateRunnerFunc abstracts the underlying binary update mechanism for testability.
type SelfUpdateRunnerFunc func(dryRun, force bool) error
