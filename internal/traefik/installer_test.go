package traefik

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchLatestVersion(t *testing.T) {
	t.Run("when valid response then returns tag_name", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name": "v3.4.0"}`))
		}))
		defer srv.Close()

		v, err := fetchLatestVersion(srv.URL)
		require.NoError(t, err)
		require.Equal(t, "v3.4.0", v)
	})

	t.Run("when server returns non-200 then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL)
		require.Error(t, err)
	})

	t.Run("when tag_name is empty then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name": ""}`))
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL)
		require.Error(t, err)
	})

	t.Run("when payload is invalid JSON then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL)
		require.Error(t, err)
	})
}

func TestEnsureACMEStore(t *testing.T) {
	i := NewInstaller()

	t.Run("must create the store 0600 so Traefik does not skip the resolver", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "acme.json")

		require.NoError(t, i.ensureACMEStore(path))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
			"Traefik refuses an ACME store that is readable by anyone else")
	})

	t.Run("must never truncate an existing store", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "acme.json")
		existing := []byte(`{"letsencrypt":{"Account":{"Email":"admin@example.com"}}}`)
		require.NoError(t, os.WriteFile(path, existing, 0644))

		require.NoError(t, i.ensureACMEStore(path))

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, existing, got, "the store holds issued certificates and account keys")

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "loose permissions must still be tightened")
	})
}
