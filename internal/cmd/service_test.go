package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceCommandsRegistered(t *testing.T) {
	cmds := map[string]bool{}
	for _, sub := range serviceCmd.Commands() {
		cmds[sub.Use] = true
	}

	t.Run("must have restart subcommand registered", func(t *testing.T) {
		assert.True(t, cmds["restart"])
	})

	t.Run("must have reload subcommand registered", func(t *testing.T) {
		assert.True(t, cmds["reload"])
	})
}
