package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListAndCheckExternalTCPService(t *testing.T) {
	file := filepath.Join(t.TempDir(), "routes.yaml")
	require.NoError(t, saveDynamicConfig(file, &DynamicConfig{TCP: &TCPConfig{Routers: map[string]*TCPRouter{
		"admin-vpn": {Rule: "HostSNI(`admin.example.com`)", EntryPoints: []string{"websecure"}, Service: "shared-https", TLS: &TLSConf{Passthrough: true}, Middlewares: []string{"vpn-only"}},
	}}}))
	resourceListFile = file
	checkFile = file
	t.Cleanup(func() { resourceListFile = ""; checkFile = "" })
	t.Run("must_show_sni_passthrough_middlewares_and_service_reference", func(t *testing.T) {
		output := captureCommandOutput(t, func() { require.NoError(t, runResourceList(resourceListCmd, nil)) })
		require.Contains(t, output, "HostSNI(`admin.example.com`)")
		require.Contains(t, output, "passthrough=true")
		require.Contains(t, output, "middlewares=[vpn-only]")
		require.Contains(t, output, "service:shared-https")
	})
	t.Run("must_skip_external_backends_without_error", func(t *testing.T) {
		output := captureCommandOutput(t, func() { require.NoError(t, runResourceCheck(resourceCheckCmd, nil)) })
		require.Contains(t, output, "[SKIP]")
		require.Contains(t, output, "another dynamic file")
	})
}
