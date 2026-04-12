package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const minimalHAProxyCfg = `
frontend test-http
    bind *:9090
    mode http
    default_backend test-backend

backend test-backend
    mode http
    balance roundrobin
    server srv1 127.0.0.1:8080 check
`

func TestReadHAProxyInput(t *testing.T) {
	t.Run("given file path then returns file contents", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "haproxy.cfg")
		require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))
		got, err := readHAProxyInput(p, "")
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("given base64 string then returns decoded contents", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte(minimalHAProxyCfg))
		got, err := readHAProxyInput("", enc)
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("when both provided then file path takes precedence", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "haproxy.cfg")
		require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))
		enc := base64.StdEncoding.EncodeToString([]byte("different content"))
		got, err := readHAProxyInput(p, enc)
		require.NoError(t, err)
		require.Contains(t, got, "frontend test-http")
	})

	t.Run("when neither provided then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("", "")
		require.Error(t, err)
	})

	t.Run("when file path does not exist then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("/nonexistent/haproxy.cfg", "")
		require.Error(t, err)
	})

	t.Run("when base64 is invalid then returns error", func(t *testing.T) {
		_, err := readHAProxyInput("", "!!!not-base64!!!")
		require.Error(t, err)
	})
}

func TestExtractPort(t *testing.T) {
	t.Run("given *:80 then returns 80", func(t *testing.T) {
		p, err := extractPort("*:80")
		require.NoError(t, err)
		require.Equal(t, "80", p)
	})

	t.Run("given 10.99.0.168:3306 then returns 3306", func(t *testing.T) {
		p, err := extractPort("10.99.0.168:3306")
		require.NoError(t, err)
		require.Equal(t, "3306", p)
	})

	t.Run("given bind without colon then returns error", func(t *testing.T) {
		_, err := extractPort("noport")
		require.Error(t, err)
	})
}

func TestCheckPortConflict(t *testing.T) {
	t.Run("when port not in used set then returns false", func(t *testing.T) {
		used := map[string]struct{}{"80": {}}
		require.False(t, checkPortConflict("443", used))
	})

	t.Run("when port already in used set then returns true", func(t *testing.T) {
		used := map[string]struct{}{"80": {}}
		require.True(t, checkPortConflict("80", used))
	})
}

func TestConvertHTTPFrontend(t *testing.T) {
	fe := HAProxyFrontend{
		Name:  "hapctl-traefik-http",
		Binds: []string{"*:80"},
		Mode:  "http",
		ACLs: []HAProxyACL{
			{Name: "host_fileserver", Condition: "hdr(host) -i fileserver.solutionstk.com"},
		},
		UseBackends: []HAProxyUseBackend{
			{Backend: "hapctl-traefik-http-fileserver-backend", ACLName: "host_fileserver"},
		},
		DefaultBackend: "hapctl-traefik-http-default-backend",
	}
	backends := map[string]HAProxyBackend{
		"hapctl-traefik-http-fileserver-backend": {
			Name:    "hapctl-traefik-http-fileserver-backend",
			Mode:    "http",
			Balance: "roundrobin",
			Servers: []HAProxyServer{{Name: "srv1", Address: "10.99.0.142:80", Options: "check"}},
		},
		"hapctl-traefik-http-default-backend": {
			Name:    "hapctl-traefik-http-default-backend",
			Mode:    "http",
			Balance: "roundrobin",
			Servers: []HAProxyServer{{Name: "srv2", Address: "127.0.0.1:32080", Options: "check"}},
		},
	}

	t.Run("must create HTTP router for ACL-based backend with Host rule", func(t *testing.T) {
		cfg := convertHTTPFrontend(fe, backends, "web")
		require.NotNil(t, cfg.HTTP)
		router, ok := cfg.HTTP.Routers["hapctl-traefik-http-fileserver-backend"]
		require.True(t, ok)
		require.Equal(t, "Host(`fileserver.solutionstk.com`)", router.Rule)
		require.Equal(t, []string{"web"}, router.EntryPoints)
		require.Equal(t, "hapctl-traefik-http-fileserver-backend", router.Service)
	})

	t.Run("must create HTTP service with correct server URL for ACL backend", func(t *testing.T) {
		cfg := convertHTTPFrontend(fe, backends, "web")
		svc, ok := cfg.HTTP.Services["hapctl-traefik-http-fileserver-backend"]
		require.True(t, ok)
		require.Len(t, svc.LoadBalancer.Servers, 1)
		require.Equal(t, "http://10.99.0.142:80", svc.LoadBalancer.Servers[0].URL)
	})

	t.Run("must create default_backend router with PathPrefix rule and lower priority", func(t *testing.T) {
		cfg := convertHTTPFrontend(fe, backends, "web")
		router, ok := cfg.HTTP.Routers["hapctl-traefik-http-default"]
		require.True(t, ok)
		require.Equal(t, "PathPrefix(`/`)", router.Rule)
		require.Equal(t, 1, router.Priority)
	})

	t.Run("must create service for default_backend with correct server URL", func(t *testing.T) {
		cfg := convertHTTPFrontend(fe, backends, "web")
		svc, ok := cfg.HTTP.Services["hapctl-traefik-http-default-backend"]
		require.True(t, ok)
		require.Len(t, svc.LoadBalancer.Servers, 1)
		require.Equal(t, "http://127.0.0.1:32080", svc.LoadBalancer.Servers[0].URL)
	})
}

func TestConvertHTTPFrontendNoRouterKeyCollision(t *testing.T) {
	t.Run("when same backend used in use_backend and default_backend then ACL router is preserved", func(t *testing.T) {
		sameBeFE := HAProxyFrontend{
			Name:  "my-frontend",
			Binds: []string{"*:80"},
			Mode:  "http",
			ACLs: []HAProxyACL{
				{Name: "host_app", Condition: "hdr(host) -i app.example.com"},
			},
			UseBackends:    []HAProxyUseBackend{{Backend: "my-backend", ACLName: "host_app"}},
			DefaultBackend: "my-backend",
		}
		be := map[string]HAProxyBackend{
			"my-backend": {Name: "my-backend", Mode: "http", Servers: []HAProxyServer{{Address: "127.0.0.1:8080"}}},
		}
		cfg := convertHTTPFrontend(sameBeFE, be, "web")

		aclRouter, ok := cfg.HTTP.Routers["my-backend"]
		require.True(t, ok)
		require.Equal(t, "Host(`app.example.com`)", aclRouter.Rule)

		defaultRouter, ok := cfg.HTTP.Routers["my-frontend-default"]
		require.True(t, ok)
		require.Equal(t, "PathPrefix(`/`)", defaultRouter.Rule)
	})
}

func TestEntrypointNameForPort(t *testing.T) {
	t.Run("port 80 returns web", func(t *testing.T) {
		require.Equal(t, "web", entrypointNameForPort("80", "any-frontend"))
	})
	t.Run("port 443 returns websecure", func(t *testing.T) {
		require.Equal(t, "websecure", entrypointNameForPort("443", "any-frontend"))
	})
	t.Run("other port returns frontend name", func(t *testing.T) {
		require.Equal(t, "hapctl-vpn-http", entrypointNameForPort("8080", "hapctl-vpn-http"))
	})
}

func TestConvertTCPListen(t *testing.T) {
	ls := HAProxyListen{
		Name:    "hapctl-game-server",
		Binds:   []string{"*:7777"},
		Mode:    "tcp",
		Balance: "roundrobin",
		Servers: []HAProxyServer{
			{Name: "hapctl-game-server", Address: "127.0.0.1:30777", Options: "check"},
		},
	}

	t.Run("must create TCP router with HostSNI wildcard rule", func(t *testing.T) {
		cfg := convertTCPListen(ls, "hapctl-game-server")
		require.NotNil(t, cfg.TCP)
		router, ok := cfg.TCP.Routers["hapctl-game-server"]
		require.True(t, ok)
		require.Equal(t, "HostSNI(`*`)", router.Rule)
		require.Equal(t, []string{"hapctl-game-server"}, router.EntryPoints)
		require.Equal(t, "hapctl-game-server", router.Service)
	})

	t.Run("must create TCP service with server address", func(t *testing.T) {
		cfg := convertTCPListen(ls, "hapctl-game-server")
		svc, ok := cfg.TCP.Services["hapctl-game-server"]
		require.True(t, ok)
		require.Len(t, svc.LoadBalancer.Servers, 1)
		require.Equal(t, "127.0.0.1:30777", svc.LoadBalancer.Servers[0].Address)
	})

	t.Run("must NOT include TLS passthrough for non-443 TCP", func(t *testing.T) {
		cfg := convertTCPListen(ls, "hapctl-game-server")
		router := cfg.TCP.Routers["hapctl-game-server"]
		require.Nil(t, router.TLS)
	})

	t.Run("must include TLS passthrough for port 443 (websecure entrypoint)", func(t *testing.T) {
		tlsListen := HAProxyListen{
			Name:    "hapctl-game-server",
			Binds:   []string{"*:443"},
			Mode:    "tcp",
			Balance: "roundrobin",
			Servers: []HAProxyServer{
				{Name: "hapctl-game-server", Address: "127.0.0.1:30443", Options: "check"},
			},
		}
		cfg := convertTCPListen(tlsListen, "websecure")
		router := cfg.TCP.Routers["hapctl-game-server"]
		require.NotNil(t, router.TLS)
		require.True(t, router.TLS.Passthrough)
	})
}

const fullHAProxyConfig = `
frontend hapctl-traefik-http
    bind *:80
    mode http
    acl host_fileserver hdr(host) -i fileserver.solutionstk.com
    use_backend hapctl-traefik-http-fileserver-backend if host_fileserver
    default_backend hapctl-traefik-http-default-backend

backend hapctl-traefik-http-fileserver-backend
    mode http
    balance roundrobin
    server hapctl-fileserver 10.99.0.142:80 check

backend hapctl-traefik-http-default-backend
    mode http
    balance roundrobin
    server hapctl-traefik-http 127.0.0.1:32080 check

listen hapctl-game-server
    bind *:7777
    mode tcp
    balance roundrobin
    server hapctl-game-server 127.0.0.1:30777 check
`

func TestExportHAProxyToDir(t *testing.T) {
	t.Run("must create one yaml file per frontend", func(t *testing.T) {
		dir := t.TempDir()
		warnings, err := exportHAProxyToDir(fullHAProxyConfig, dir)
		require.NoError(t, err)
		require.Empty(t, warnings)
		_, err = os.Stat(filepath.Join(dir, "hapctl-traefik-http.yaml"))
		require.NoError(t, err)
	})

	t.Run("must create one yaml file per listen block", func(t *testing.T) {
		dir := t.TempDir()
		_, err := exportHAProxyToDir(fullHAProxyConfig, dir)
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(dir, "hapctl-game-server.yaml"))
		require.NoError(t, err)
	})

	t.Run("when same port used twice then second block is skipped with warning", func(t *testing.T) {
		conflictCfg := `
frontend first-http
    bind *:80
    mode http
    default_backend first-backend

backend first-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8001 check

frontend second-http
    bind *:80
    mode http
    default_backend second-backend

backend second-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8002 check
`
		dir := t.TempDir()
		warnings, err := exportHAProxyToDir(conflictCfg, dir)
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "80")

		_, err = os.Stat(filepath.Join(dir, "first-http.yaml"))
		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(dir, "second-http.yaml"))
		require.True(t, os.IsNotExist(statErr))
	})

	t.Run("when listen block has mode http then creates HTTP yaml", func(t *testing.T) {
		httpListenCfg := `
listen hapctl-vpn-http
    bind 10.99.0.168:80
    mode http
    balance roundrobin
    server hapctl-vault 127.0.0.1:31201 check
`
		dir := t.TempDir()
		_, err := exportHAProxyToDir(httpListenCfg, dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, "hapctl-vpn-http.yaml"))
		require.NoError(t, err)
		content := string(data)
		require.Contains(t, content, "http:")
		require.NotContains(t, content, "tcp:")
	})
}

func TestResolveBindPort(t *testing.T) {
	t.Run("when no binds then returns skipped with warning", func(t *testing.T) {
		used := map[string]struct{}{}
		var warnings []string
		port, skipped := resolveBindPort(nil, "test-block", used, &warnings)
		require.True(t, skipped)
		require.Empty(t, port)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "test-block")
	})

	t.Run("when bind has no port then returns skipped with warning", func(t *testing.T) {
		used := map[string]struct{}{}
		var warnings []string
		port, skipped := resolveBindPort([]string{"nocolon"}, "test-block", used, &warnings)
		require.True(t, skipped)
		require.Empty(t, port)
		require.Len(t, warnings, 1)
	})

	t.Run("when port already used then returns skipped with warning containing port", func(t *testing.T) {
		used := map[string]struct{}{"80": {}}
		var warnings []string
		port, skipped := resolveBindPort([]string{"*:80"}, "second-http", used, &warnings)
		require.True(t, skipped)
		require.Empty(t, port)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "80")
		require.Contains(t, warnings[0], "second-http")
	})

	t.Run("when port is available then returns port and not skipped", func(t *testing.T) {
		used := map[string]struct{}{}
		var warnings []string
		port, skipped := resolveBindPort([]string{"*:8080"}, "my-service", used, &warnings)
		require.False(t, skipped)
		require.Equal(t, "8080", port)
		require.Empty(t, warnings)
	})
}

func TestHAProxyExportCommandFlags(t *testing.T) {
	t.Run("must have --file flag", func(t *testing.T) {
		f := haproxyExportCmd.Flags().Lookup("file")
		require.NotNil(t, f)
	})
	t.Run("must have --base64 flag", func(t *testing.T) {
		f := haproxyExportCmd.Flags().Lookup("base64")
		require.NotNil(t, f)
	})
	t.Run("must have --output-dir flag", func(t *testing.T) {
		f := haproxyExportCmd.Flags().Lookup("output-dir")
		require.NotNil(t, f)
	})
}
