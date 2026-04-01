//go:build unit

package commands_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/autobump/internal/domain/commands"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()

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
