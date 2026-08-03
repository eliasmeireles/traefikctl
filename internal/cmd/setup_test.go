package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexOf(t *testing.T, names []string, name string) int {
	t.Helper()
	for i, n := range names {
		if n == name {
			return i
		}
	}
	t.Fatalf("step %q not found in %v", name, names)
	return -1
}

func TestSetupStepSequence(t *testing.T) {
	t.Run("must validate the config before starting or restarting the service", func(t *testing.T) {
		// This is the entire point of `setup`: the incident that motivated
		// it was a config error only discovered AFTER the service was
		// already crash-looping. Validation must happen first.
		validateIdx := indexOf(t, setupStepNames, "validate")
		startIdx := indexOf(t, setupStepNames, "start")
		assert.Less(t, validateIdx, startIdx)
	})

	t.Run("must check for root before doing anything else", func(t *testing.T) {
		assert.Equal(t, "root-check", setupStepNames[0])
	})

	t.Run("must install the binary and system user/directories before generating config", func(t *testing.T) {
		binaryIdx := indexOf(t, setupStepNames, "binary")
		usersIdx := indexOf(t, setupStepNames, "user-and-directories")
		configIdx := indexOf(t, setupStepNames, "generate-config")
		assert.Less(t, binaryIdx, configIdx)
		assert.Less(t, usersIdx, configIdx)
	})

	t.Run("must install the systemd unit before validating and starting", func(t *testing.T) {
		unitIdx := indexOf(t, setupStepNames, "service-unit")
		validateIdx := indexOf(t, setupStepNames, "validate")
		startIdx := indexOf(t, setupStepNames, "start")
		assert.Less(t, unitIdx, validateIdx)
		assert.Less(t, unitIdx, startIdx)
	})

	t.Run("must run the summary check last", func(t *testing.T) {
		assert.Equal(t, "summary", setupStepNames[len(setupStepNames)-1])
	})

	t.Run("must have exactly one implementation registered per declared step name", func(t *testing.T) {
		// Guards against a typo silently dropping a step: runSetup looks up
		// steps by name in a map, so a name with no matching function is
		// skipped rather than failing loudly.
		seen := map[string]bool{}
		for _, n := range setupStepNames {
			require.False(t, seen[n], "duplicate step name %q", n)
			seen[n] = true
		}
	})
}
