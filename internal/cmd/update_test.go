package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchLatestVersion(t *testing.T) {
	t.Run("when valid response then returns tag_name", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name": "v9.9.9"}`))
		}))
		defer srv.Close()

		version, err := fetchLatestVersion(srv.URL + "/repos/eliasmeireles/traefikctl/releases/latest")
		require.NoError(t, err)
		require.Equal(t, "v9.9.9", version)
	})

	t.Run("when server returns non-200 status then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL + "/repos/eliasmeireles/traefikctl/releases/latest")
		require.Error(t, err)
	})

	t.Run("when response has empty tag_name then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name": ""}`))
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL + "/repos/eliasmeireles/traefikctl/releases/latest")
		require.Error(t, err)
	})

	t.Run("when response is invalid JSON then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		_, err := fetchLatestVersion(srv.URL + "/repos/eliasmeireles/traefikctl/releases/latest")
		require.Error(t, err)
	})
}

func TestDownloadToTemp(t *testing.T) {
	t.Run("given 200 response then returns temp file containing body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("hello"))
		}))
		defer srv.Close()

		path, err := downloadToTemp(srv.URL + "/binary")
		require.NoError(t, err)
		require.NotEmpty(t, path)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "hello", string(content))

		require.NoError(t, os.Remove(path))
	})

	t.Run("given non-200 response then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := downloadToTemp(srv.URL + "/binary")
		require.Error(t, err)
	})
}
