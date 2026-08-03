package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeHelp = `NAME:
   traefik

GLOBAL OPTIONS:
   --log.level (Default: "ERROR")
   --log.filepath (Default: "")
   --log.compress (Default: "false")
   --log.maxsize (Default: "0")
   --accesslog.filepath (Default: "")
   --accesslog.format (Default: "common")
   --accesslog.fields.names.<name> (Default: "")
   --entrypoints.<name>.address (Default: "")
   --certificatesresolvers.<name>.acme.email (Default: "")
   --serverstransport.forwardingtimeouts.dialtimeout (Default: "0")
`

func TestParseHelp(t *testing.T) {
	t.Run("given help output then registers leaf flags as known", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		require.NotNil(t, s)
		assert.True(t, s.Known([]string{"log", "compress"}))
		assert.True(t, s.Known([]string{"log", "maxsize"}))
	})

	t.Run("given a key with no matching flag then reports unknown", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		assert.False(t, s.Known([]string{"accessLog", "compress"}))
		assert.False(t, s.Known([]string{"accessLog", "maxSize"}))
	})

	t.Run("given an intermediate valid prefix then reports known", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		assert.True(t, s.Known([]string{"log"}))
		assert.True(t, s.Known([]string{"accessLog"}))
	})

	t.Run("given a path under a user-named wildcard then reports known", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		assert.True(t, s.Known([]string{"entryPoints", "mongoexpress", "address"}))
		assert.True(t, s.Known([]string{"certificatesResolvers", "letsencrypt", "acme", "email"}))
	})

	t.Run("given a section with zero flags then reports unknown even though the file section is legal", func(t *testing.T) {
		// tls: is legal in the static file but has no --tls.* flags at all —
		// Schema must say "unknown" here; only the healthcheck-driven
		// caller is the actual authority, Schema is just an aid.
		s := ParseHelp(fakeHelp)
		assert.False(t, s.Known([]string{"tls"}))
	})

	t.Run("given a nil schema then Known returns false", func(t *testing.T) {
		var s *Schema
		assert.False(t, s.Known([]string{"log", "compress"}))
	})

	t.Run("given an empty path then Known returns false", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		assert.False(t, s.Known(nil))
	})
}

func TestSchemaSuggestPaths(t *testing.T) {
	t.Run("given a leaf name that exists once then returns that one path", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		got := s.SuggestPaths("dialtimeout")
		require.Len(t, got, 1)
		assert.Equal(t, "serverstransport.forwardingtimeouts.dialtimeout", got[0])
	})

	t.Run("given a leaf name that does not exist then returns nothing", func(t *testing.T) {
		s := ParseHelp(fakeHelp)
		assert.Empty(t, s.SuggestPaths("nonexistentleaf"))
	})

	t.Run("given a nil schema then returns nothing", func(t *testing.T) {
		var s *Schema
		assert.Empty(t, s.SuggestPaths("compress"))
	})
}
