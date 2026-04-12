package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleHAProxyConfig = `
global
    log /dev/log    local0
    user haproxy
    daemon

defaults
    log    global
    mode    http
    timeout connect 5000

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

func TestParseHAProxyConfig(t *testing.T) {
	t.Run("must parse one frontend with ACLs and default_backend", func(t *testing.T) {
		cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
		require.NoError(t, err)
		require.Len(t, cfg.Frontends, 1)

		fe := cfg.Frontends[0]
		require.Equal(t, "hapctl-traefik-http", fe.Name)
		require.Equal(t, []string{"*:80"}, fe.Binds)
		require.Equal(t, "http", fe.Mode)
		require.Len(t, fe.ACLs, 1)
		require.Equal(t, "host_fileserver", fe.ACLs[0].Name)
		require.Equal(t, "hdr(host) -i fileserver.solutionstk.com", fe.ACLs[0].Condition)
		require.Len(t, fe.UseBackends, 1)
		require.Equal(t, "hapctl-traefik-http-fileserver-backend", fe.UseBackends[0].Backend)
		require.Equal(t, "host_fileserver", fe.UseBackends[0].ACLName)
		require.Equal(t, "hapctl-traefik-http-default-backend", fe.DefaultBackend)
	})

	t.Run("must parse two backends with servers", func(t *testing.T) {
		cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
		require.NoError(t, err)
		require.Len(t, cfg.Backends, 2)

		be := cfg.Backends[0]
		require.Equal(t, "hapctl-traefik-http-fileserver-backend", be.Name)
		require.Equal(t, "http", be.Mode)
		require.Equal(t, "roundrobin", be.Balance)
		require.Len(t, be.Servers, 1)
		require.Equal(t, "hapctl-fileserver", be.Servers[0].Name)
		require.Equal(t, "10.99.0.142:80", be.Servers[0].Address)
		require.Equal(t, "check", be.Servers[0].Options)
	})

	t.Run("must parse one listen block with TCP mode", func(t *testing.T) {
		cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
		require.NoError(t, err)
		require.Len(t, cfg.Listens, 1)

		ls := cfg.Listens[0]
		require.Equal(t, "hapctl-game-server", ls.Name)
		require.Equal(t, []string{"*:7777"}, ls.Binds)
		require.Equal(t, "tcp", ls.Mode)
		require.Equal(t, "roundrobin", ls.Balance)
		require.Len(t, ls.Servers, 1)
		require.Equal(t, "127.0.0.1:30777", ls.Servers[0].Address)
	})

	t.Run("must ignore global and defaults sections", func(t *testing.T) {
		cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
		require.NoError(t, err)
		require.Len(t, cfg.Frontends, 1)
		require.Len(t, cfg.Backends, 2)
		require.Len(t, cfg.Listens, 1)
	})

	t.Run("when empty config then returns empty HAProxyConfig", func(t *testing.T) {
		cfg, err := ParseHAProxyConfig("")
		require.NoError(t, err)
		require.Empty(t, cfg.Frontends)
		require.Empty(t, cfg.Backends)
		require.Empty(t, cfg.Listens)
	})
}

func TestHAProxyDataStructures(t *testing.T) {
	t.Run("must create Frontend with expected fields", func(t *testing.T) {
		f := HAProxyFrontend{
			Name:           "hapctl-traefik-http",
			Binds:          []string{"*:80"},
			Mode:           "http",
			ACLs:           []HAProxyACL{{Name: "host_fs", Condition: `hdr(host) -i fs.example.com`}},
			UseBackends:    []HAProxyUseBackend{{Backend: "be-fs", ACLName: "host_fs"}},
			DefaultBackend: "be-default",
		}
		require.Equal(t, "hapctl-traefik-http", f.Name)
		require.Equal(t, "http", f.Mode)
		require.Len(t, f.ACLs, 1)
	})

	t.Run("must create Backend with expected fields", func(t *testing.T) {
		b := HAProxyBackend{
			Name:    "be-default",
			Mode:    "http",
			Balance: "roundrobin",
			Servers: []HAProxyServer{{Name: "s1", Address: "127.0.0.1:8080", Options: "check"}},
		}
		require.Equal(t, "be-default", b.Name)
		require.Len(t, b.Servers, 1)
		require.Equal(t, "127.0.0.1:8080", b.Servers[0].Address)
	})

	t.Run("must create Listen with expected fields", func(t *testing.T) {
		l := HAProxyListen{
			Name:    "hapctl-game",
			Binds:   []string{"*:7777"},
			Mode:    "tcp",
			Balance: "roundrobin",
			Servers: []HAProxyServer{{Name: "srv", Address: "127.0.0.1:30777", Options: "check"}},
		}
		require.Equal(t, "hapctl-game", l.Name)
		require.Equal(t, "tcp", l.Mode)
	})
}
