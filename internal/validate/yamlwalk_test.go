package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseDoc(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &doc))
	return &doc
}

func TestFindKey(t *testing.T) {
	t.Run("given a key present under two different parents then returns both occurrences with distinct paths", func(t *testing.T) {
		doc := parseDoc(t, `
log:
  compress: true
accessLog:
  compress: true
`)
		occs := findKey(doc, "compress")
		require.Len(t, occs, 2)
		assert.Equal(t, []string{"log", "compress"}, occs[0].path)
		assert.Equal(t, []string{"accessLog", "compress"}, occs[1].path)
	})

	t.Run("given a key that does not appear then returns no occurrences", func(t *testing.T) {
		doc := parseDoc(t, "log:\n  level: INFO\n")
		assert.Empty(t, findKey(doc, "compress"))
	})

	t.Run("given case-differing key name then still matches case-insensitively", func(t *testing.T) {
		doc := parseDoc(t, "log:\n  Compress: true\n")
		occs := findKey(doc, "compress")
		require.Len(t, occs, 1)
		assert.Equal(t, "Compress", occs[0].path[1])
	})

	t.Run("given a mapping key inside a sequence of maps then still finds it", func(t *testing.T) {
		doc := parseDoc(t, `
tls:
  options:
    default:
      cipherSuites:
        - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
      compress: true
`)
		occs := findKey(doc, "compress")
		require.Len(t, occs, 1)
		assert.Equal(t, []string{"tls", "options", "default", "compress"}, occs[0].path)
	})

	t.Run("given a key then records its original line number", func(t *testing.T) {
		doc := parseDoc(t, "log:\n  level: INFO\n  compress: true\n")
		occs := findKey(doc, "compress")
		require.Len(t, occs, 1)
		assert.Equal(t, 3, occs[0].line)
	})
}

func TestRemoveOccurrences(t *testing.T) {
	t.Run("given one occurrence then removes only its key and value", func(t *testing.T) {
		doc := parseDoc(t, "log:\n  level: INFO\n  compress: true\n")
		occs := findKey(doc, "compress")
		require.Len(t, occs, 1)

		removeOccurrences(occs)

		out, err := yaml.Marshal(doc)
		require.NoError(t, err)
		assert.Contains(t, string(out), "level: INFO")
		assert.NotContains(t, string(out), "compress")
	})

	t.Run("given occurrences under different parents then removes both and leaves siblings intact", func(t *testing.T) {
		doc := parseDoc(t, `
log:
  level: INFO
  compress: true
accessLog:
  format: common
  compress: true
`)
		occs := findKey(doc, "compress")
		require.Len(t, occs, 2)

		removeOccurrences(occs)

		out, err := yaml.Marshal(doc)
		require.NoError(t, err)
		s := string(out)
		assert.Contains(t, s, "level: INFO")
		assert.Contains(t, s, "format: common")
		assert.NotContains(t, s, "compress")
	})

	t.Run("given two occurrences under the same parent then removes both without corrupting indices", func(t *testing.T) {
		doc := parseDoc(t, "log:\n  a: dup\n  keep: 1\n  a: dup2\n")
		occs := findKey(doc, "a")
		require.Len(t, occs, 2)

		removeOccurrences(occs)

		out, err := yaml.Marshal(doc)
		require.NoError(t, err)
		s := string(out)
		assert.Contains(t, s, "keep: 1")
		assert.NotContains(t, s, "a:")
	})
}
