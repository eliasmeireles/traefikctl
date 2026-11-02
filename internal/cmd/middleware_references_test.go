package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTCPMiddlewareLifecycle(t *testing.T) {
	file := filepath.Join(t.TempDir(), "middlewares.yaml")
	require.NoError(t, addMiddlewareForProtocol("vpn-only", "ip-allowlist", "tcp", map[string]string{"sourcerange": "10.99.0.0/24,192.0.2.1,2001:db8::/32"}, file))
	cfg, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Nil(t, cfg.HTTP)
	require.Equal(t, []string{"10.99.0.0/24", "192.0.2.1", "2001:db8::/32"}, cfg.TCP.Middlewares["vpn-only"].IPAllowList.SourceRange)
	t.Run("must_list_tcp_middleware_and_protocol", func(t *testing.T) {
		mwListFile = file
		mwListProtocol = "tcp"
		t.Cleanup(func() { mwListFile = ""; mwListProtocol = "" })
		output := captureCommandOutput(t, func() { require.NoError(t, runMiddlewareList(middlewareListCmd, nil)) })
		require.Contains(t, output, "[TCP]")
		require.Contains(t, output, "vpn-only")
		require.Contains(t, output, "ip-allowlist")
		require.NotContains(t, output, "[HTTP]")
	})
	t.Run("must_remove_unreferenced_tcp_middleware", func(t *testing.T) {
		require.NoError(t, removeMiddleware("vpn-only", file))
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.NotContains(t, result.TCP.Middlewares, "vpn-only")
	})
}

func TestMiddlewareReferences(t *testing.T) {
	for _, protocol := range []string{"http", "tcp"} {
		t.Run("must_reject_removal_when_"+protocol+"_routers_reference_middleware", func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "middlewares.yaml")
			require.NoError(t, addMiddlewareForProtocol("vpn-only", "ip-allowlist", protocol, map[string]string{"sourcerange": "10.99.0.0/24"}, file))
			routes := &DynamicConfig{
				HTTP: &HTTPConfig{Routers: map[string]*Router{"http-router": {Middlewares: []string{"vpn-only@file"}}}},
				TCP:  &TCPConfig{Routers: map[string]*TCPRouter{"tcp-router": {Middlewares: []string{"vpn-only"}}}},
			}
			require.NoError(t, saveDynamicConfig(filepath.Join(dir, "routes.yaml"), routes))
			require.ErrorContains(t, removeMiddlewareForProtocol("vpn-only", protocol, file), protocol+"-router")
			result, err := loadDynamicConfig(file)
			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
	t.Run("must_require_protocol_for_ambiguous_name", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "middlewares.yaml")
		require.NoError(t, addMiddlewareForProtocol("shared", "ip-allowlist", "http", map[string]string{"sourcerange": "192.0.2.1"}, file))
		require.NoError(t, addMiddlewareForProtocol("shared", "ip-allowlist", "tcp", map[string]string{"sourcerange": "192.0.2.1"}, file))
		require.ErrorContains(t, removeMiddleware("shared", file), "specify --protocol")
		require.NoError(t, removeMiddlewareForProtocol("shared", "tcp", file))
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Contains(t, result.HTTP.Middlewares, "shared")
	})
	require.False(t, referencesMiddleware([]string{"vpn-only@other", "other"}, "vpn-only"))
}

func TestAllowlistValidation(t *testing.T) {
	for _, protocol := range []string{"http", "tcp"} {
		t.Run("must_validate_"+protocol+"_source_ranges", func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "middlewares.yaml")
			for _, source := range []string{"", "invalid", "10.0.0.0/33", "192.0.2.999", "example.com"} {
				require.Error(t, addMiddlewareForProtocol("vpn-only", "ip-allowlist", protocol, map[string]string{"sourcerange": source}, file))
			}
		})
	}
	for _, mwType := range []string{"redirect-https", "rate-limit", "basic-auth", "strip-prefix", "ip-whitelist", "unknown"} {
		t.Run("must_reject_tcp_"+mwType, func(t *testing.T) {
			require.ErrorContains(t, addMiddlewareForProtocol("vpn-only", mwType, "tcp", nil, filepath.Join(t.TempDir(), "middlewares.yaml")), "TCP supports only ip-allowlist")
		})
	}
	t.Run("must_preserve_http_legacy_ip_whitelist", func(t *testing.T) {
		mw, err := buildMiddleware("ip-whitelist", map[string]string{"sourcerange": "192.0.2.1"})
		require.NoError(t, err)
		require.NotNil(t, mw.IPWhiteList)
		require.Nil(t, mw.IPAllowList)
	})
	require.Error(t, addMiddlewareForProtocol("bad/name", "ip-allowlist", "tcp", nil, "unused"))
	require.ErrorContains(t, addMiddlewareForProtocol("valid", "ip-allowlist", "udp", nil, "unused"), "invalid --protocol")
}

// captureCommandOutput captures short command listings written to standard output.
func captureCommandOutput(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()
	run()
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(output)
}
