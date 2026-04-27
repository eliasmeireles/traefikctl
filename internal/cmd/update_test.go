package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadURLPattern(t *testing.T) {
	t.Run("must build asset URLs that match the release workflow naming (hyphens, not underscores)", func(t *testing.T) {
		got := fmt.Sprintf(downloadURLPattern, "v0.0.4-rc-02", "linux", "amd64")
		want := "https://github.com/eliasmeireles/traefikctl/releases/download/v0.0.4-rc-02/traefikctl-linux-amd64"
		require.Equal(t, want, got)
	})
}

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

		path, err := downloadToTemp(srv.URL+"/binary", t.TempDir())
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

		_, err := downloadToTemp(srv.URL+"/binary", t.TempDir())
		require.Error(t, err)
	})

	t.Run("given unwritable target dir then falls back to default temp dir", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("hello"))
		}))
		defer srv.Close()

		path, err := downloadToTemp(srv.URL+"/binary", "/nonexistent-dir-for-traefikctl-test")
		require.NoError(t, err)
		require.NotEmpty(t, path)
		require.NoError(t, os.Remove(path))
	})
}

func TestReplaceBinary(t *testing.T) {
	t.Run("when src and dst on same filesystem then renames atomically", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		dst := filepath.Join(dir, "dst")

		require.NoError(t, os.WriteFile(src, []byte("payload"), 0644))
		require.NoError(t, replaceBinary(src, dst))

		_, err := os.Stat(src)
		require.True(t, os.IsNotExist(err), "src should be moved")

		content, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, "payload", string(content))
	})

	t.Run("when dst dir does not exist then returns error", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		require.NoError(t, os.WriteFile(src, []byte("payload"), 0644))

		err := replaceBinary(src, "/nonexistent-dir-for-traefikctl-test/dst")
		require.Error(t, err)
	})
}
