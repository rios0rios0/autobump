//go:build unit

package commands_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestSelfUpdateCommand(t *testing.T) {
	t.Parallel()

	t.Run("should delegate to runner when executed successfully", func(t *testing.T) {
		// given
		var calledDryRun, calledForce bool
		runnerFn := commands.SelfUpdateRunnerFunc(func(dryRun, force bool) error {
			calledDryRun = dryRun
			calledForce = force
			return nil
		})
		command := commands.NewSelfUpdateCommand(runnerFn)

		// when
		err := command.Execute(true, false)

		// then
		assert.NoError(t, err)
		assert.True(t, calledDryRun)
		assert.False(t, calledForce)
	})

	t.Run("should propagate runner error when update fails", func(t *testing.T) {
		// given
		expectedErr := errors.New("network error")
		var calledDryRun, calledForce bool
		runnerFn := commands.SelfUpdateRunnerFunc(func(dryRun, force bool) error {
			calledDryRun = dryRun
			calledForce = force
			return expectedErr
		})
		command := commands.NewSelfUpdateCommand(runnerFn)

		// when
		err := command.Execute(false, true)

		// then
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, calledDryRun)
		assert.True(t, calledForce)
	})
}
