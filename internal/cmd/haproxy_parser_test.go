package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
