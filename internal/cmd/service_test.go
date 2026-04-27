package cmd

import (
	"strings"
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

func TestDefaultServiceTemplate(t *testing.T) {
	t.Run("must declare LogsDirectory so /var/log/traefik is recreated with traefik:traefik on every start", func(t *testing.T) {
		assert.Contains(t, defaultServiceTemplate, "LogsDirectory=traefik")
		assert.Contains(t, defaultServiceTemplate, "LogsDirectoryMode=0755")
	})

	t.Run("must run as the traefik user and group", func(t *testing.T) {
		assert.Contains(t, defaultServiceTemplate, "User=traefik")
		assert.Contains(t, defaultServiceTemplate, "Group=traefik")
	})

	t.Run("must keep ReadWritePaths for the configuration directory", func(t *testing.T) {
		assert.Contains(t, defaultServiceTemplate, "ReadWritePaths=/etc/traefik")
	})

	t.Run("must not redundantly grant ReadWritePaths on /var/log/traefik when LogsDirectory handles it", func(t *testing.T) {
		for _, line := range strings.Split(defaultServiceTemplate, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "ReadWritePaths=") {
				continue
			}
			assert.NotContains(t, line, "/var/log/traefik",
				"LogsDirectory=traefik already provides write access; listing it here is redundant and reintroduces the chown footgun if removed")
		}
	})
}
