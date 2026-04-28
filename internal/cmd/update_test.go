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

func TestFetchLatestStableVersion(t *testing.T) {
	t.Run("given a list with prereleases first then returns the first stable tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[
				{"tag_name": "v0.0.4-rc-05", "prerelease": true,  "draft": false},
				{"tag_name": "v0.0.4-rc-04", "prerelease": true,  "draft": false},
				{"tag_name": "v0.0.3",       "prerelease": false, "draft": false},
				{"tag_name": "v0.0.2",       "prerelease": false, "draft": false}
			]`))
		}))
		defer srv.Close()

		version, err := fetchLatestStableVersion(srv.URL)
		require.NoError(t, err)
		require.Equal(t, "v0.0.3", version)
	})

	t.Run("must skip drafts even when tag matches the stable pattern", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[
				{"tag_name": "v9.9.9", "prerelease": false, "draft": true},
				{"tag_name": "v1.2.3", "prerelease": false, "draft": false}
			]`))
		}))
		defer srv.Close()

		version, err := fetchLatestStableVersion(srv.URL)
		require.NoError(t, err)
		require.Equal(t, "v1.2.3", version)
	})

	t.Run("must skip tags that don't match v\\d+.\\d+.\\d+", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[
				{"tag_name": "v1.2.3-beta", "prerelease": false, "draft": false},
				{"tag_name": "release-2024", "prerelease": false, "draft": false},
				{"tag_name": "v2.0.0",      "prerelease": false, "draft": false}
			]`))
		}))
		defer srv.Close()

		version, err := fetchLatestStableVersion(srv.URL)
		require.NoError(t, err)
		require.Equal(t, "v2.0.0", version)
	})

	t.Run("when no stable release exists then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[
				{"tag_name": "v0.0.4-rc-05", "prerelease": true, "draft": false},
				{"tag_name": "v0.0.4-rc-04", "prerelease": true, "draft": false}
			]`))
		}))
		defer srv.Close()

		_, err := fetchLatestStableVersion(srv.URL)
		require.Error(t, err)
	})

	t.Run("when server returns non-200 status then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := fetchLatestStableVersion(srv.URL)
		require.Error(t, err)
	})

	t.Run("when response is invalid JSON then returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer srv.Close()

		_, err := fetchLatestStableVersion(srv.URL)
		require.Error(t, err)
	})
}

func TestStableTagRegex(t *testing.T) {
	t.Run("must match plain semver tags", func(t *testing.T) {
		cases := []string{"v0.0.1", "v1.2.3", "v10.20.30", "v0.0.0"}
		for _, c := range cases {
			require.True(t, stableTagRegex.MatchString(c), "expected match: %s", c)
		}
	})

	t.Run("must reject pre-release and non-semver tags", func(t *testing.T) {
		cases := []string{
			"v0.0.4-rc-05",
			"v1.2.3-beta",
			"1.2.3",
			"v1.2",
			"v1.2.3.4",
			"release-2024",
			"",
		}
		for _, c := range cases {
			require.False(t, stableTagRegex.MatchString(c), "expected no match: %q", c)
		}
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
