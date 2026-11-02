package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyTCPRouter(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.yaml")
	dst := filepath.Join(dir, "destination.yaml")
	require.NoError(t, saveDynamicConfig(src, &DynamicConfig{TCP: &TCPConfig{Routers: map[string]*TCPRouter{
		"original": {Rule: "HostSNI(`original.example.com`)", EntryPoints: []string{"websecure"}, Service: "original-svc", TLS: &TLSConf{Passthrough: true}, Middlewares: []string{"vpn-only"}, Priority: 100},
	}, Services: map[string]*TCPService{"original-svc": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}}}}}))
	t.Run("must_copy_domains_and_preserve_tls_middleware_priority", func(t *testing.T) {
		require.NoError(t, copyRouter("original", "copy", "a.example.com,b.example.com", src, dst))
		result, err := loadDynamicConfig(dst)
		require.NoError(t, err)
		router := result.TCP.Routers["copy"]
		require.Equal(t, "HostSNI(`a.example.com`) || HostSNI(`b.example.com`)", router.Rule)
		require.True(t, router.TLS.Passthrough)
		require.Equal(t, []string{"vpn-only"}, router.Middlewares)
		require.Equal(t, 100, router.Priority)
		require.Equal(t, "copy-svc", router.Service)
		require.Equal(t, "192.0.2.1:443", result.TCP.Services["copy-svc"].LoadBalancer.Servers[0].Address)
		original, err := loadDynamicConfig(src)
		require.NoError(t, err)
		require.Equal(t, "HostSNI(`original.example.com`)", original.TCP.Routers["original"].Rule)
		require.ErrorContains(t, copyRouter("original", "copy", "", src, dst), "already exists")
	})
	t.Run("must_keep_external_service_reference", func(t *testing.T) {
		require.NoError(t, saveDynamicConfig(filepath.Join(dir, "external.yaml"), &DynamicConfig{TCP: &TCPConfig{Routers: map[string]*TCPRouter{
			"external": {Rule: "HostSNI(`*`)", EntryPoints: []string{"websecure"}, Service: "original-svc"},
		}}}))
		require.NoError(t, copyRouter("external", "external-copy", "new.example.com", filepath.Join(dir, "external.yaml"), dst))
		result, err := loadDynamicConfig(dst)
		require.NoError(t, err)
		require.Equal(t, "original-svc", result.TCP.Routers["external-copy"].Service)
		require.NotContains(t, result.TCP.Services, "external-copy-svc")
	})
	t.Run("must_allow_overrides_on_copy", func(t *testing.T) {
		require.NoError(t, copyRouterWithChanges("original", "override", src, dst, routeChanges{service: "original-svc", priority: 200, setPriority: true, passthrough: false, setPassthrough: true}))
		result, err := loadDynamicConfig(dst)
		require.NoError(t, err)
		router := result.TCP.Routers["override"]
		require.Equal(t, "original-svc", router.Service)
		require.Equal(t, 200, router.Priority)
		require.False(t, router.TLS.Passthrough)
	})
}
