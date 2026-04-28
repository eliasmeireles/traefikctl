package traefik

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
