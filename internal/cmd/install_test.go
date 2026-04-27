package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/traefikctl/internal/traefik"
)

func TestResolveTraefikVersion(t *testing.T) {
	installer := traefik.NewInstaller()

	t.Run("given explicit version then returns it verbatim", func(t *testing.T) {
		v, err := resolveTraefikVersion(installer, "v3.4.0")
		require.NoError(t, err)
		require.Equal(t, "v3.4.0", v)
	})

	t.Run("given default version then returns it verbatim", func(t *testing.T) {
		v, err := resolveTraefikVersion(installer, traefik.DefaultVersion)
		require.NoError(t, err)
		require.Equal(t, traefik.DefaultVersion, v)
	})

	t.Run("given empty value then returns empty (cobra default applies upstream)", func(t *testing.T) {
		v, err := resolveTraefikVersion(installer, "")
		require.NoError(t, err)
		require.Equal(t, "", v)
	})
}
