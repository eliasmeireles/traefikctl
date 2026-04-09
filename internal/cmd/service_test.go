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

	assert.True(t, cmds["restart"], "restart subcommand must be registered")
	assert.True(t, cmds["reload"], "reload subcommand must be registered")
}
