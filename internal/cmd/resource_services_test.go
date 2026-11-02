package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExistingServiceRouting(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "routes.yaml")
	require.NoError(t, saveDynamicConfig(filepath.Join(dir, "services.yml"), &DynamicConfig{
		TCP:  &TCPConfig{Services: map[string]*TCPService{"shared-tcp": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}}}},
		HTTP: &HTTPConfig{Services: map[string]*Service{"shared-http": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://192.0.2.1:8080"}}}}}},
	}))
	t.Run("must_find_service_in_another_file_and_matching_protocol", func(t *testing.T) {
		require.NoError(t, validateExistingService("shared-tcp", "tcp", file))
		require.NoError(t, validateExistingService("shared-http", "http", file))
		require.ErrorContains(t, validateExistingService("shared-http", "tcp", file), "not found")
		require.ErrorContains(t, validateExistingService("missing", "tcp", file), "not found")
	})
	t.Run("must_require_exactly_one_backend_source", func(t *testing.T) {
		_, err := resourceServiceName("route", "", "", "tcp", file)
		require.ErrorContains(t, err, "exactly one")
		_, err = resourceServiceName("route", "192.0.2.1:443", "shared-tcp", "tcp", file)
		require.ErrorContains(t, err, "mutually exclusive")
		name, err := resourceServiceName("route", "192.0.2.1:443", "", "tcp", file)
		require.NoError(t, err)
		require.Equal(t, "route-svc", name)
	})
	t.Run("must_reject_missing_service_without_writing", func(t *testing.T) {
		setupAddFlags("missing-route", "", "admin.example.com", "websecure")
		t.Cleanup(func() { setupAddFlags("", "", "", "web") })
		addService = "missing"
		require.ErrorContains(t, addTCPResource(&DynamicConfig{}, file), "not found")
		_, err := os.Stat(file)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("must_attach_http_existing_service_without_creating_service", func(t *testing.T) {
		setupAddFlags("http-route", "", "app.example.com", "web")
		t.Cleanup(func() { setupAddFlags("", "", "", "web") })
		addService = "shared-http"
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, file))
		require.Equal(t, "shared-http", cfg.HTTP.Routers["http-route"].Service)
		require.Empty(t, cfg.HTTP.Services)
	})
}

func TestNeighboringConfigs(t *testing.T) {
	t.Run("must_report_invalid_yaml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("tcp: ["), 0600))
		_, err := neighboringConfigs(filepath.Join(dir, "routes.yaml"))
		require.ErrorContains(t, err, "bad.yaml")
	})
	t.Run("must_skip_non_yaml_files_and_directories", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0600))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "nested.yaml"), 0700))
		cfgs, err := neighboringConfigs(filepath.Join(dir, "routes.yaml"))
		require.NoError(t, err)
		require.Empty(t, cfgs)
	})
}
