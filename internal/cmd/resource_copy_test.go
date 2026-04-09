package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyHTTPRouter(t *testing.T) {
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

	t.Run("when copied then both routers exist", func(t *testing.T) {
		require.NoError(t, copyRouter("my-app", "my-app-staging", "staging.example.com", file, file))

		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Contains(t, result.HTTP.Routers, "my-app")
		require.Contains(t, result.HTTP.Routers, "my-app-staging")
	})

	t.Run("when copied then new router has correct domain", func(t *testing.T) {
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Equal(t, "Host(`staging.example.com`)", result.HTTP.Routers["my-app-staging"].Rule)
	})

	t.Run("when copied then new service exists", func(t *testing.T) {
		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Contains(t, result.HTTP.Services, "my-app-staging-svc")
	})
}

func TestCopyRouterFailsIfDestinationExists(t *testing.T) {
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

	t.Run("must return error when destination name already exists", func(t *testing.T) {
		err := copyRouter("my-app", "my-app", "other.example.com", file, file)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists")
	})
}
