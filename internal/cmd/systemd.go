package cmd

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultRestartWait bounds how long waitHealthy polls after a restart
// before giving up and reporting the service never stabilized.
const DefaultRestartWait = 12 * time.Second

// DefaultSettle is how long a unit must hold active/running with a stable
// restart count before it's considered healthy, not just alive between two
// crashes.
const DefaultSettle = 5 * time.Second

// unitState is the subset of `systemctl show` fields needed to tell a
// genuinely healthy restart apart from a config-error crash-loop. A single
// `systemctl is-active` poll can't do this: the unit's Restart=always plus
// StartLimitIntervalSec=0 (see defaultServiceTemplate) means a permanently
// broken config crash-loops in "activating" forever and is briefly
// "active" between each crash and the next — is-active can report "active"
// at exactly the wrong instant.
type unitState struct {
	ActiveState    string
	SubState       string
	Result         string
	NRestarts      int
	ExecMainStatus int
}

// showUnit queries systemd for the current state of the named unit.
func showUnit(name string) (unitState, error) {
	out, err := exec.Command("systemctl", "show", name,
		"-p", "ActiveState", "-p", "SubState", "-p", "NRestarts",
		"-p", "Result", "-p", "ExecMainStatus",
	).Output()
	if err != nil {
		return unitState{}, fmt.Errorf("systemctl show %s: %w", name, err)
	}
	return parseUnitState(string(out)), nil
}

// parseUnitState parses the KEY=VALUE lines from `systemctl show`. It never
// errors — unrecognized or malformed lines are simply ignored, leaving
// their field at its zero value.
func parseUnitState(out string) unitState {
	var s unitState
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			s.ActiveState = value
		case "SubState":
			s.SubState = value
		case "Result":
			s.Result = value
		case "NRestarts":
			if n, err := strconv.Atoi(value); err == nil {
				s.NRestarts = n
			}
		case "ExecMainStatus":
			if n, err := strconv.Atoi(value); err == nil {
				s.ExecMainStatus = n
			}
		}
	}
	return s
}

// isCrashLooping reports whether s describes a unit that systemd is
// actively cycling through restarts, as opposed to a normal one-shot start
// still in progress (ActiveState=activating, SubState=start is a healthy
// transient the very first time a unit starts).
func isCrashLooping(s unitState) bool {
	return s.SubState == "auto-restart" || s.ActiveState == "failed"
}

// waitHealthy polls name's unit state until it has held active/running
// with a stable restart count for settle, or until timeout elapses, or
// until a crash-loop or an unexpected restart is detected — whichever
// comes first. A single healthy poll is not enough evidence on a unit with
// Restart=always: the process can be alive at the instant of any one poll
// while still dying every few seconds.
func waitHealthy(name string, timeout, settle time.Duration) (unitState, error) {
	base, err := showUnit(name)
	if err != nil {
		return unitState{}, err
	}
	baseRestarts := base.NRestarts

	deadline := time.Now().Add(timeout)
	var healthySince time.Time

	for {
		s, err := showUnit(name)
		if err != nil {
			return s, err
		}

		if isCrashLooping(s) {
			return s, fmt.Errorf("crash-looping (state=%s/%s, restarts=%d, last exit=%d)", s.ActiveState, s.SubState, s.NRestarts, s.ExecMainStatus)
		}
		if s.NRestarts > baseRestarts {
			return s, fmt.Errorf("restarted unexpectedly while stabilizing (restarts %d -> %d)", baseRestarts, s.NRestarts)
		}

		if s.ActiveState == "active" && s.SubState == "running" {
			if healthySince.IsZero() {
				healthySince = time.Now()
			}
			if time.Since(healthySince) >= settle {
				return s, nil
			}
		} else {
			healthySince = time.Time{}
		}

		if time.Now().After(deadline) {
			return s, fmt.Errorf("did not stabilize within %s (state=%s/%s)", timeout, s.ActiveState, s.SubState)
		}

		time.Sleep(250 * time.Millisecond)
	}
}
