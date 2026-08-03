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

func TestServiceRestartFlags(t *testing.T) {
	t.Run("must expose skip-validate, no-wait and wait flags so an operator can bypass safety in an emergency", func(t *testing.T) {
		assert.NotNil(t, serviceRestartCmd.Flags().Lookup("skip-validate"))
		assert.NotNil(t, serviceRestartCmd.Flags().Lookup("no-wait"))
		assert.NotNil(t, serviceRestartCmd.Flags().Lookup("wait"))
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

	t.Run("must keep restarting when an entry point binds to an address that appears late at boot", func(t *testing.T) {
		// Traefik exits instead of retrying when a bind fails, which happens
		// when an entry point targets a VPN/overlay address configured after
		// network-online.target. on-failure alone is not enough: systemd's
		// default start limit stops the unit for good after 5 tries in 10s.
		assert.Contains(t, defaultServiceTemplate, "Restart=always")
		assert.Contains(t, defaultServiceTemplate, "StartLimitIntervalSec=0")
		assert.NotContains(t, defaultServiceTemplate, "Restart=on-failure")
	})

	t.Run("must declare StartLimitIntervalSec in [Unit], where systemd expects it", func(t *testing.T) {
		unitSection := strings.SplitN(defaultServiceTemplate, "[Service]", 2)[0]
		assert.Contains(t, unitSection, "StartLimitIntervalSec=0",
			"systemd >= 229 parses StartLimitIntervalSec from [Unit]; in [Service] it is silently ignored on current releases")
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
