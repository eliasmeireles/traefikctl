package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const minimalHAProxyCfg = `
frontend test-http
    bind *:9090
    mode http
    default_backend test-backend

backend test-backend
    mode http
    balance roundrobin
    server srv1 127.0.0.1:8080 check
`

func TestReadHAProxyInput(t *testing.T) {
	t.Run("given file path then returns file contents", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "haproxy.cfg")
		require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))
		got, err := readHAProxyInput(p, "")
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("given base64 string then returns decoded contents", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte(minimalHAProxyCfg))
		got, err := readHAProxyInput("", enc)
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("when both provided then file path takes precedence", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "haproxy.cfg")
		require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))
		enc := base64.StdEncoding.EncodeToString([]byte("different content"))
		got, err := readHAProxyInput(p, enc)
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("when neither provided then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("", "")
		require.Error(t, err)
	})

	t.Run("when file path does not exist then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("/nonexistent/haproxy.cfg", "")
		require.Error(t, err)
	})

	t.Run("when base64 is invalid then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("", "!!!not-base64!!!")
		require.Error(t, err)
	})
}
