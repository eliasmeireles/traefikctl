package cmd

import (
	"fmt"
	"os"
	"strings"
)

// requireRoot fails fast with the ACTUAL command line the caller ran
// (prefixed with sudo) rather than a hardcoded guess. Every command that
// mutates system state calls this first.
func requireRoot() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("requires root — run: sudo %s", commandLine())
}

// commandLine renders the process's actual invocation, e.g. "traefikctl
// update --version latest" — used both by requireRoot and by
// permissionHint so a mid-flight EACCES always suggests the command that
// was actually running, not an unrelated hardcoded one.
func commandLine() string {
	return strings.Join(os.Args, " ")
}
