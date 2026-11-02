package cmd

import (
	"os"
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
	addService = ""
	addPriority = 0
	addPassthrough = false
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

func TestAddTCPResource(t *testing.T) {
	t.Cleanup(func() { setupAddFlags("", "", "", "web") })
	for _, tc := range []struct{ title, domain, rule string }{
		{"must_keep_default_wildcard_without_tls", "", "HostSNI(`*`)"},
		{"must_match_one_domain", "a.example.com", "HostSNI(`a.example.com`)"},
		{"must_match_many_domains", "a.example.com,b.example.com", "HostSNI(`a.example.com`) || HostSNI(`b.example.com`)"},
	} {
		t.Run(tc.title, func(t *testing.T) {
			setupAddFlags("database", "192.0.2.1:5432", tc.domain, "database")
			cfg := &DynamicConfig{}
			require.NoError(t, addTCPResource(cfg, filepath.Join(t.TempDir(), "routes.yaml")))
			require.Equal(t, tc.rule, cfg.TCP.Routers["database"].Rule)
			require.Equal(t, tc.domain != "", cfg.TCP.Routers["database"].TLS != nil)
		})
	}
}

func TestTCPVPNGolden(t *testing.T) {
	t.Cleanup(func() { setupAddFlags("", "", "", "web") })
	dir := t.TempDir()
	file := filepath.Join(dir, "vpn.yaml")
	require.NoError(t, saveDynamicConfig(filepath.Join(dir, "services.yaml"), &DynamicConfig{TCP: &TCPConfig{Services: map[string]*TCPService{
		"shared-https": {LoadBalancer: &TCPLoadBalancer{Servers: []ServerAddress{{Address: "192.0.2.1:443"}}}},
	}}}))
	require.NoError(t, addMiddlewareForProtocol("vpn-only", "ip-allowlist", "tcp", map[string]string{"sourcerange": "10.99.0.0/24"}, file))
	cfg, err := loadDynamicConfig(file)
	require.NoError(t, err)
	setupAddFlags("admin-vpn", "", "admin.example.com", "websecure")
	addPassthrough = true
	addPriority = 100
	addService = "shared-https"
	addMiddlewares = []string{"vpn-only"}
	require.NoError(t, addTCPResource(cfg, file))
	actual, err := os.ReadFile(file)
	require.NoError(t, err)
	golden, err := os.ReadFile("testdata/tcp-vpn.yaml")
	require.NoError(t, err)
	require.YAMLEq(t, string(golden), string(actual))
	require.Empty(t, cfg.TCP.Services)
	require.ErrorContains(t, addTCPResource(cfg, file), "already exists")
}
