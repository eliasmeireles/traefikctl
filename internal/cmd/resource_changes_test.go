package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRouteChanges(t *testing.T) {
	t.Run("must_bind_and_detect_explicit_zero_values", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		changes := routeChanges{}
		bindRouteChanges(cmd, &changes)
		require.NoError(t, cmd.Flags().Parse([]string{"--priority=0", "--passthrough=false", "--middleware="}))
		changes.readChangedFlags(cmd)
		require.True(t, changes.setPriority)
		require.True(t, changes.setPassthrough)
		require.True(t, changes.setMiddlewares)
		router := &TCPRouter{Rule: "HostSNI(`*`)", Priority: 10, Middlewares: []string{"vpn-only"}, TLS: &TLSConf{Passthrough: true}}
		require.NoError(t, changes.applyTCP(router))
		require.Zero(t, router.Priority)
		require.Empty(t, router.Middlewares)
		require.False(t, router.TLS.Passthrough)
	})
	t.Run("must_reject_http_passthrough_and_type_mismatch", func(t *testing.T) {
		require.ErrorContains(t, (routeChanges{passthrough: true}).applyHTTP(&Router{}), "only for TCP")
		require.ErrorContains(t, (routeChanges{protocol: "tcp"}).validate("http", "unused"), "router type")
		require.ErrorContains(t, (routeChanges{address: "192.0.2.1:443", service: "shared"}).validate("tcp", "unused"), "mutually exclusive")
	})
	t.Run("must_apply_http_domain_and_metadata", func(t *testing.T) {
		router := &Router{}
		changes := routeChanges{domains: "new.example.com", entrypoint: "web,websecure", setMiddlewares: true, middlewares: []string{"one,two"}, setPriority: true, priority: 100}
		require.NoError(t, changes.applyHTTP(router))
		require.Equal(t, "Host(`new.example.com`)", router.Rule)
		require.Equal(t, []string{"web", "websecure"}, router.EntryPoints)
		require.Equal(t, []string{"one", "two"}, router.Middlewares)
		require.Equal(t, 100, router.Priority)
	})
}

func TestUpdateTCPRouter(t *testing.T) {
	file := filepath.Join(t.TempDir(), "routes.yaml")
	cfg := &DynamicConfig{TCP: &TCPConfig{Routers: map[string]*TCPRouter{"route": {Rule: "HostSNI(`*`)", Service: "route-svc", EntryPoints: []string{"websecure"}}}, Services: map[string]*TCPService{
		"route-svc": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}},
		"shared":    {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.2:443"}}}},
	}}}
	require.NoError(t, saveDynamicConfig(file, cfg))
	t.Run("must_update_sni_tls_middleware_priority_and_address", func(t *testing.T) {
		require.NoError(t, updateRouter("route", file, routeChanges{domains: "a.example.com,b.example.com", address: "192.0.2.3:443", passthrough: true, setPassthrough: true, middlewares: []string{"vpn-only"}, setMiddlewares: true, priority: 100, setPriority: true}))
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		router := result.TCP.Routers["route"]
		require.Equal(t, "HostSNI(`a.example.com`) || HostSNI(`b.example.com`)", router.Rule)
		require.True(t, router.TLS.Passthrough)
		require.Equal(t, []string{"vpn-only"}, router.Middlewares)
		require.Equal(t, 100, router.Priority)
		require.Equal(t, "192.0.2.3:443", result.TCP.Services["route-svc"].LoadBalancer.Servers[0].Address)
	})
	t.Run("must_switch_to_existing_service", func(t *testing.T) {
		require.NoError(t, updateRouter("route", file, routeChanges{service: "shared"}))
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Equal(t, "shared", result.TCP.Routers["route"].Service)
		require.ErrorContains(t, updateRouter("route", file, routeChanges{service: "missing"}), "not found")
	})
	t.Run("must_leave_file_unchanged_for_invalid_domain", func(t *testing.T) {
		require.ErrorContains(t, updateRouter("route", file, routeChanges{domains: "https://example.com"}), "invalid domain")
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Equal(t, "HostSNI(`a.example.com`) || HostSNI(`b.example.com`)", result.TCP.Routers["route"].Rule)
	})
	require.ErrorContains(t, updateRouter("missing", file, routeChanges{domains: "example.com"}), "not found")
}

func TestTCPProtocolDisambiguation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "routes.yaml")
	require.NoError(t, saveDynamicConfig(file, &DynamicConfig{
		HTTP: &HTTPConfig{Routers: map[string]*Router{"shared-name": {Rule: "Host(`http.example.com`)", Service: "http-service"}}},
		TCP:  &TCPConfig{Routers: map[string]*TCPRouter{"shared-name": {Rule: "HostSNI(`*`)", Service: "tcp-service", EntryPoints: []string{"websecure"}}}, Services: map[string]*TCPService{"tcp-service": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}}}},
	}))
	require.NoError(t, updateRouter("shared-name", file, routeChanges{protocol: "tcp", domains: "tcp.example.com"}))
	require.NoError(t, copyRouterWithChanges("shared-name", "tcp-copy", file, file, routeChanges{protocol: "tcp", domains: "copy.example.com"}))
	result, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Equal(t, "Host(`http.example.com`)", result.HTTP.Routers["shared-name"].Rule)
	require.Equal(t, "HostSNI(`tcp.example.com`)", result.TCP.Routers["shared-name"].Rule)
	require.Equal(t, "HostSNI(`copy.example.com`)", result.TCP.Routers["tcp-copy"].Rule)
	require.NotContains(t, result.HTTP.Routers, "tcp-copy")
}
