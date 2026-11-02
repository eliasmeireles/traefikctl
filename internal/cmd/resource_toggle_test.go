package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisableAndEnableHTTPRouter(t *testing.T) {
	dir := t.TempDir()
	disabledDir := filepath.Join(dir, "disabled")
	activeFile := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}

	require.NoError(t, saveDynamicConfig(activeFile, cfg))

	t.Run("given active router when disabled then removed from active config", func(t *testing.T) {
		require.NoError(t, disableRouter("my-app", activeFile, disabledDir))

		restored, err := loadDynamicConfig(activeFile)
		require.NoError(t, err)
		require.Nil(t, restored.HTTP, "HTTP section must be gone after disabling last router")
	})

	t.Run("given disabled router then snapshot file must exist", func(t *testing.T) {
		disabledFile := filepath.Join(disabledDir, "my-app.yaml")
		_, err := os.Stat(disabledFile)
		require.NoError(t, err, "disabled snapshot must exist")
	})

	t.Run("given disabled router when enabled then restored to active config", func(t *testing.T) {
		require.NoError(t, enableRouter("my-app", activeFile, disabledDir))

		reactivated, err := loadDynamicConfig(activeFile)
		require.NoError(t, err)
		require.NotNil(t, reactivated.HTTP)
		require.Contains(t, reactivated.HTTP.Routers, "my-app")
	})
}

func TestDisableAndEnableTCPRouter(t *testing.T) {
	dir := t.TempDir()
	disabledDir := filepath.Join(dir, "disabled")
	activeFile := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		TCP: &TCPConfig{
			Routers: map[string]*TCPRouter{
				"pg": {Rule: "HostSNI(`*`)", EntryPoints: []string{"postgres"}, Service: "pg-svc"},
			},
			Services: map[string]*TCPService{
				"pg-svc": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "127.0.0.1:5432"}}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(activeFile, cfg))

	t.Run("given active TCP router when disabled then removed from active config", func(t *testing.T) {
		require.NoError(t, disableRouter("pg", activeFile, disabledDir))

		restored, err := loadDynamicConfig(activeFile)
		require.NoError(t, err)
		require.Nil(t, restored.TCP)
	})

	t.Run("given disabled TCP router when enabled then restored", func(t *testing.T) {
		require.NoError(t, enableRouter("pg", activeFile, disabledDir))

		reactivated, err := loadDynamicConfig(activeFile)
		require.NoError(t, err)
		require.NotNil(t, reactivated.TCP)
		require.Contains(t, reactivated.TCP.Routers, "pg")
	})
}

func TestDisableTCPRouterPreservesMiddlewareAndSharedService(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "routes.yaml")
	snapshots := filepath.Join(dir, "disabled")
	require.NoError(t, saveDynamicConfig(file, &DynamicConfig{TCP: &TCPConfig{
		Routers:     map[string]*TCPRouter{"admin-vpn": {Rule: "HostSNI(`admin.example.com`)", Service: "shared-https", EntryPoints: []string{"websecure"}, Middlewares: []string{"vpn-only"}, TLS: &TLSConf{Passthrough: true}}},
		Services:    map[string]*TCPService{"shared-https": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}}},
		Middlewares: map[string]*MiddlewareConfig{"vpn-only": {IPAllowList: &IPAllowList{SourceRange: []string{"10.99.0.0/24"}}}},
	}}))
	require.NoError(t, disableRouter("admin-vpn", file, snapshots))
	disabled, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Contains(t, disabled.TCP.Middlewares, "vpn-only")
	require.Contains(t, disabled.TCP.Services, "shared-https")
	require.Empty(t, disabled.TCP.Routers)
	require.NoError(t, enableRouter("admin-vpn", file, snapshots))
	enabled, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Contains(t, enabled.TCP.Routers, "admin-vpn")
	require.Contains(t, enabled.TCP.Middlewares, "vpn-only")
	require.Contains(t, enabled.TCP.Services, "shared-https")
}
