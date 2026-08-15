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

	t.Run("given an older tag then returns it verbatim", func(t *testing.T) {
		v, err := resolveTraefikVersion(installer, "v3.3.5")
		require.NoError(t, err)
		require.Equal(t, "v3.3.5", v)
	})
}

func TestInstallWithoutVersion(t *testing.T) {
	t.Run("must refuse to install when no version was resolved", func(t *testing.T) {
		err := traefik.NewInstaller().Install("")
		require.Error(t, err)
	})
}

func TestInstallVersionFlagDefault(t *testing.T) {
	t.Run("must default install --version to latest so no stale release is pinned", func(t *testing.T) {
		require.Equal(t, latestVersionAlias, installCmd.Flags().Lookup("version").DefValue)
	})

	t.Run("must default setup --version to latest so no stale release is pinned", func(t *testing.T) {
		require.Equal(t, latestVersionAlias, setupCmd.Flags().Lookup("version").DefValue)
	})
}
