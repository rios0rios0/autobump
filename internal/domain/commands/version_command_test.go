package commands_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

// TestVersionCommand is deliberately not parallel, at either level. Both cases assign
// the AutobumpVersion package global and swap [os.Stdout], which is process-wide: running
// them alongside anything else makes the version one case asserts depend on when the
// other case happened to write it.
//
// The command reads AutobumpVersion and prints to stdout, so assigning the first and
// swapping the second is the only way to observe what it does — hence the reassign
// suppression.
//
//nolint:reassign // see the paragraph above
func TestVersionCommand(t *testing.T) {
	t.Run("should print the version to stdout when executed", func(t *testing.T) {
		// given
		commands.AutobumpVersion = "1.2.3"
		command := commands.NewVersionCommand()

		r, w, _ := os.Pipe()
		origStdout := os.Stdout
		os.Stdout = w

		// when
		command.Execute()

		// then
		os.Stdout = origStdout
		w.Close()
		out, _ := io.ReadAll(r)
		assert.Equal(t, "autobump version: 1.2.3\n", string(out))
	})

	t.Run("should print dev when version is not set", func(t *testing.T) {
		// given
		commands.AutobumpVersion = "dev"
		command := commands.NewVersionCommand()

		r, w, _ := os.Pipe()
		origStdout := os.Stdout
		os.Stdout = w

		// when
		command.Execute()

		// then
		os.Stdout = origStdout
		w.Close()
		out, _ := io.ReadAll(r)
		assert.Equal(t, "autobump version: dev\n", string(out))
	})
}
