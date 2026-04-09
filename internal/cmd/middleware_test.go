package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddRedirectHTTPSMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("when redirect-https middleware added then config has correct scheme", func(t *testing.T) {
		require.NoError(t, addMiddleware("redirect-https", "redirect-https", map[string]string{
			"scheme":    "https",
			"permanent": "true",
		}, file))

		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.NotNil(t, result.HTTP.Middlewares["redirect-https"])
		require.Equal(t, "https", result.HTTP.Middlewares["redirect-https"].RedirectScheme.Scheme)
		require.True(t, result.HTTP.Middlewares["redirect-https"].RedirectScheme.Permanent)
	})
}

func TestAddRateLimitMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("when rate-limit middleware added then config has correct values", func(t *testing.T) {
		require.NoError(t, addMiddleware("my-limit", "rate-limit", map[string]string{
			"average": "100",
			"burst":   "50",
		}, file))

		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.Equal(t, 100, result.HTTP.Middlewares["my-limit"].RateLimit.Average)
		require.Equal(t, 50, result.HTTP.Middlewares["my-limit"].RateLimit.Burst)
	})
}

func TestRemoveMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
		Middlewares: map[string]*MiddlewareConfig{
			"my-mw": {RedirectScheme: &RedirectScheme{Scheme: "https"}},
		},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("when middleware removed then it no longer exists in config", func(t *testing.T) {
		require.NoError(t, removeMiddleware("my-mw", file))

		result, err := loadDynamicConfig(file)
		require.NoError(t, err)
		require.NotContains(t, result.HTTP.Middlewares, "my-mw")
	})
}
