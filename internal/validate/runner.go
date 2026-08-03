package validate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner is the boundary between the validator and the real `traefik`
// binary, so tests can substitute a scripted fake without a real install.
type Runner interface {
	// Healthcheck runs `traefik healthcheck --configFile=configPath` and
	// returns its stderr (which is where Traefik reports config decode
	// errors as structured JSON) plus the exec error, if any.
	Healthcheck(ctx context.Context, configPath string) (stderr string, err error)
	// Help returns the output of `traefik --help`, used to build a Schema.
	Help(ctx context.Context) (stdout string, err error)
	// Available reports whether the underlying binary can be run at all.
	Available() bool
}

// ExecRunner shells out to a real traefik binary.
type ExecRunner struct {
	// Binary is the executable name or path. Defaults to "traefik".
	Binary string
	// Timeout bounds each invocation. Defaults to 10s.
	Timeout time.Duration
}

// NewExecRunner returns an ExecRunner for the given binary (or "traefik" if
// binary is empty).
func NewExecRunner(binary string) *ExecRunner {
	if binary == "" {
		binary = "traefik"
	}
	return &ExecRunner{Binary: binary}
}

func (r *ExecRunner) binary() string {
	if r.Binary == "" {
		return "traefik"
	}
	return r.Binary
}

func (r *ExecRunner) timeout() time.Duration {
	if r.Timeout <= 0 {
		return 10 * time.Second
	}
	return r.Timeout
}

func (r *ExecRunner) Available() bool {
	_, err := exec.LookPath(r.binary())
	return err == nil
}

func (r *ExecRunner) Healthcheck(ctx context.Context, configPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	// --configFile with a nonexistent path silently falls back to
	// /etc/traefik/traefik.yaml (verified empirically) — refuse to run
	// against a path we can't confirm exists, so a broken caller never
	// ends up validating the live system config by accident.
	if _, err := os.Stat(configPath); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, r.binary(), "healthcheck", "--configFile="+configPath)
	cmd.Dir = scratchDirOf(configPath)
	cmd.Env = filterEnv(os.Environ())

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	return stderr.String(), runErrOrNil(runErr)
}

func (r *ExecRunner) Help(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, r.binary(), "--help")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runErrOrNil discards *exec.ExitError — an exit code of 1 is the normal,
// expected outcome for every healthcheck classification (Traefik never
// returns 0 from this subcommand), so callers must classify by stderr
// content, not by err.
func runErrOrNil(err error) error {
	var exitErr *exec.ExitError
	if err == nil || errors.As(err, &exitErr) {
		return nil
	}
	return err
}

func scratchDirOf(path string) string {
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		return path[:idx]
	}
	return "."
}

// filterEnv strips TRAEFIK_* variables so Traefik's env-based config loader
// can't inject fields the scratch file doesn't have, which would make
// validation results depend on the caller's shell environment.
func filterEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "TRAEFIK_") {
			continue
		}
		out = append(out, e)
	}
	return out
}
