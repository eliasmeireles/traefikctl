package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddHTTPResourceWithMiddleware(t *testing.T) {
	dir := t.TempDir()
	setupAddFlags("my-app", "127.0.0.1:8080", "app.example.com", "web")
	addMiddlewares = []string{"redirect-to-https"}

	t.Run("when middleware flag set then router has middleware attached", func(t *testing.T) {
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, filepath.Join(dir, "services.yaml")))

		require.Contains(t, cfg.HTTP.Routers["my-app"].Middlewares, "redirect-to-https")
	})
}

func TestAddHTTPResourceWithTLS(t *testing.T) {
	dir := t.TempDir()
	setupAddFlags("tls-app", "127.0.0.1:8443", "secure.example.com", "websecure")
	addTLS = true
	addCertResolver = "letsencrypt"

	t.Run("when tls and cert-resolver set then router has TLS config", func(t *testing.T) {
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, filepath.Join(dir, "services.yaml")))

		router := cfg.HTTP.Routers["tls-app"]
		require.NotNil(t, router.TLS)
		require.Equal(t, "letsencrypt", router.TLS.CertResolver)
	})
}

func TestAddHTTPResourceWithRedirectHTTPS(t *testing.T) {
	dir := t.TempDir()
	setupAddFlags("web-app", "127.0.0.1:8080", "app.example.com", "web")
	addRedirectHTTPS = true

	t.Run("when redirect-https set then main router is on websecure", func(t *testing.T) {
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, filepath.Join(dir, "services.yaml")))

		require.Equal(t, []string{"websecure"}, cfg.HTTP.Routers["web-app"].EntryPoints)
	})

	t.Run("when redirect-https set then HTTP redirect router is created on web", func(t *testing.T) {
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, filepath.Join(dir, "services.yaml")))

		httpRouter, ok := cfg.HTTP.Routers["web-app-http"]
		require.True(t, ok, "HTTP redirect router must exist")
		require.Equal(t, []string{"web"}, httpRouter.EntryPoints)
		require.Contains(t, httpRouter.Middlewares, "redirect-to-https")
	})

	t.Run("when redirect-https set then redirect middleware is configured", func(t *testing.T) {
		cfg := &DynamicConfig{}
		require.NoError(t, addHTTPResource(cfg, filepath.Join(dir, "services.yaml")))

		mw := cfg.HTTP.Middlewares["redirect-to-https"]
		require.NotNil(t, mw)
		require.Equal(t, "https", mw.RedirectScheme.Scheme)
		require.True(t, mw.RedirectScheme.Permanent)
	})
}

// setupAddFlags resets the package-level add vars to known values for testing.
func setupAddFlags(name, address, domain, entrypoint string) {
	addName = name
	addAddress = address
	addDomain = domain
	addEntrypoint = entrypoint
	addType = "http"
	addMiddlewares = nil
	addRedirectHTTPS = false
	addTLS = false
	addCertResolver = ""
}
