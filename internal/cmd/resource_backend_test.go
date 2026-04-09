package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddBackendServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

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
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("when backend added then service has two servers", func(t *testing.T) {
		require.NoError(t, addBackendServer("my-app", "127.0.0.1:8081", file))

		updated, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Len(t, updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers, 2)
		require.Equal(t, "http://127.0.0.1:8081", updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers[1].URL)
	})

	t.Run("when duplicate backend added then error returned", func(t *testing.T) {
		err := addBackendServer("my-app", "127.0.0.1:8081", file)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})
}

func TestRemoveBackendServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{
					{URL: "http://127.0.0.1:8080"},
					{URL: "http://127.0.0.1:8081"},
				}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("when backend removed then one server remains", func(t *testing.T) {
		require.NoError(t, removeBackendServer("my-app", "127.0.0.1:8081", file))

		updated, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Len(t, updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers, 1)
		require.Equal(t, "http://127.0.0.1:8080", updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers[0].URL)
	})
}

func TestRemoveLastBackendServerReturnsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

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
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("must return error when removing the last server", func(t *testing.T) {
		err := removeBackendServer("my-app", "127.0.0.1:8080", file)
		require.Error(t, err)
		require.Contains(t, err.Error(), "last server")
	})
}
