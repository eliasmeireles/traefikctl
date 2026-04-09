package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountRoutes(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.yaml")
	file2 := filepath.Join(dir, "b.yaml")

	cfg1 := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{"r1": {}, "r2": {}},
		Services: map[string]*Service{},
	}}
	cfg2 := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{"r3": {}},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file1, cfg1))
	require.NoError(t, saveDynamicConfig(file2, cfg2))

	t.Run("must count HTTP routes across multiple files", func(t *testing.T) {
		http, tcp := countRoutes([]string{file1, file2})
		assert.Equal(t, 3, http)
		assert.Equal(t, 0, tcp)
	})
}

func TestCountRoutesMixedTypes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "mixed.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers:  map[string]*Router{"web": {}},
			Services: map[string]*Service{},
		},
		TCP: &TCPConfig{
			Routers:  map[string]*TCPRouter{"db": {}},
			Services: map[string]*TCPService{},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	t.Run("must count HTTP and TCP routes separately", func(t *testing.T) {
		http, tcp := countRoutes([]string{file})
		assert.Equal(t, 1, http)
		assert.Equal(t, 1, tcp)
	})
}
