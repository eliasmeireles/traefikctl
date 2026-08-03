//go:build integration

package validate

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the real `traefik` binary and only run with
// `go test -tags=integration ./...` on a machine that has it installed —
// they are the ground-truth check that the classification logic in
// errors.go actually matches what the installed Traefik version says.
func TestStaticFileAgainstRealTraefik(t *testing.T) {
	if _, err := exec.LookPath("traefik"); err != nil {
		t.Skip("traefik binary not on PATH")
	}

	t.Run("given a config with rotation fields under accessLog then reports all of them", func(t *testing.T) {
		content := []byte(`log:
  level: INFO
  filePath: /var/log/traefik/traefik.log
  format: common
accessLog:
  filePath: /var/log/traefik/access.log
  format: common
  maxSize: 50
  maxAge: 7
  maxBackups: 3
  compress: true
api:
  dashboard: true
entryPoints:
  web:
    address: ":80"
providers:
  file:
    directory: /etc/traefik/dynamic/
`)
		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{})
		require.NoError(t, err)
		require.False(t, result.OK())

		var keys []string
		for _, f := range result.Findings {
			keys = append(keys, f.Key)
		}
		assert.ElementsMatch(t, []string{"maxSize", "maxAge", "maxBackups", "compress"}, keys)
	})

	t.Run("given serversTransport dialTimeout not nested under forwardingTimeouts then reports it", func(t *testing.T) {
		content := []byte(`log:
  level: INFO
entryPoints:
  web:
    address: ":80"
providers:
  file:
    directory: /etc/traefik/dynamic/
serversTransport:
  dialTimeout: 5s
`)
		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{})
		require.NoError(t, err)
		// serversTransport had only dialTimeout as a child, so removing it
		// leaves an empty section that Traefik separately rejects
		// ("cannot be a standalone element") — that's our own stripping
		// artifact and must be cleaned up silently, not reported.
		require.Len(t, result.Findings, 1)
		assert.Equal(t, "dialTimeout", result.Findings[0].Key)
	})

	t.Run("given a config with tls: options and no rotation problems then reports OK", func(t *testing.T) {
		content := []byte(`log:
  level: INFO
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
providers:
  file:
    directory: /etc/traefik/dynamic/
tls:
  options:
    default:
      minVersion: VersionTLS12
`)
		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{})
		require.NoError(t, err)
		assert.True(t, result.OK(), "unexpected findings: %v", result.Findings)
	})
}
