package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUnitState(t *testing.T) {
	t.Run("given a real systemctl show fixture with an active healthy unit then parses every field", func(t *testing.T) {
		out := "ActiveState=active\nSubState=running\nNRestarts=0\nResult=success\nExecMainStatus=0\n"
		s := parseUnitState(out)
		assert.Equal(t, "active", s.ActiveState)
		assert.Equal(t, "running", s.SubState)
		assert.Equal(t, 0, s.NRestarts)
		assert.Equal(t, "success", s.Result)
		assert.Equal(t, 0, s.ExecMainStatus)
	})

	t.Run("given a crash-looping unit fixture then parses the high restart count", func(t *testing.T) {
		out := "ActiveState=activating\nSubState=auto-restart\nNRestarts=622\nResult=exit-code\nExecMainStatus=1\n"
		s := parseUnitState(out)
		assert.Equal(t, "activating", s.ActiveState)
		assert.Equal(t, "auto-restart", s.SubState)
		assert.Equal(t, 622, s.NRestarts)
		assert.Equal(t, 1, s.ExecMainStatus)
	})

	t.Run("given malformed or unknown lines then ignores them without erroring", func(t *testing.T) {
		out := "not a key value line\nActiveState=active\n\nSomeUnknownField=whatever\n"
		s := parseUnitState(out)
		assert.Equal(t, "active", s.ActiveState)
	})

	t.Run("given an unparsable NRestarts value then leaves it at zero", func(t *testing.T) {
		out := "NRestarts=not-a-number\n"
		s := parseUnitState(out)
		assert.Equal(t, 0, s.NRestarts)
	})
}

func TestIsCrashLooping(t *testing.T) {
	t.Run("given active and running with restarts then is not crash-looping", func(t *testing.T) {
		s := unitState{ActiveState: "active", SubState: "running", NRestarts: 3}
		assert.False(t, isCrashLooping(s))
	})

	t.Run("given activating and auto-restart then is crash-looping", func(t *testing.T) {
		s := unitState{ActiveState: "activating", SubState: "auto-restart", NRestarts: 622}
		assert.True(t, isCrashLooping(s))
	})

	t.Run("given failed active state then is crash-looping", func(t *testing.T) {
		s := unitState{ActiveState: "failed", SubState: "failed"}
		assert.True(t, isCrashLooping(s))
	})

	t.Run("given activating and start with zero restarts then is not crash-looping because it is still booting for the first time", func(t *testing.T) {
		s := unitState{ActiveState: "activating", SubState: "start", NRestarts: 0}
		assert.False(t, isCrashLooping(s))
	})
}
