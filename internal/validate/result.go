package validate

import (
	"fmt"
	"strings"
)

// Severity classifies how serious a Finding is.
type Severity int

const (
	// SeverityError means Traefik will refuse to start with this config as-is.
	SeverityError Severity = iota
	// SeverityWarning means the config is syntactically fine but suspect.
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "WARN"
	}
	return "ERROR"
}

// Finding is a single problem located in a static Traefik configuration file.
type Finding struct {
	// Path is the lowercase dotted schema path, e.g. "accesslog.compress".
	Path string
	// Key is the literal YAML key name that was rejected, e.g. "compress".
	Key string
	// Message describes the problem in human terms.
	Message string
	// Hint suggests a fix, when one is known.
	Hint string
	// Line and Column are 1-based positions in the original file. Zero when
	// the finding could not be tied to a specific YAML node (e.g. a raw
	// parse error Traefik reported that isn't a removable key).
	Line, Column int
	Severity     Severity
	// Certain is false when the schema could not tell which of several
	// same-named occurrences Traefik actually rejected, so all of them were
	// removed and reported together as a best-effort guess.
	Certain bool
}

// Result is the outcome of validating one static configuration file.
type Result struct {
	ConfigPath string
	Findings   []Finding
	// Iterations is how many times the validator re-ran Traefik.
	Iterations int
	// Truncated is true if MaxIterations was hit before Traefik accepted
	// the (possibly stripped-down) config.
	Truncated bool
	// Skipped is true when no runner was available (Traefik not installed)
	// and only a YAML syntax check could be performed.
	Skipped    bool
	SkipReason string
}

// OK reports whether the configuration has no findings at all.
func (r *Result) OK() bool {
	return r != nil && len(r.Findings) == 0
}

// Errors returns only the error-severity findings.
func (r *Result) Errors() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			out = append(out, f)
		}
	}
	return out
}

// String renders a human-readable report, one finding per line(s).
func (r *Result) String() string {
	if r == nil {
		return ""
	}

	var b strings.Builder
	if r.Skipped {
		fmt.Fprintf(&b, "skipped: %s\n", r.SkipReason)
		return b.String()
	}
	if r.OK() {
		fmt.Fprintf(&b, "%s: OK\n", r.ConfigPath)
		return b.String()
	}

	for _, f := range r.Findings {
		loc := r.ConfigPath
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", r.ConfigPath, f.Line)
		}
		suffix := ""
		if !f.Certain {
			suffix = " (best guess)"
		}
		fmt.Fprintf(&b, "[%s] %s: %s%s\n", f.Severity, loc, f.Message, suffix)
		if f.Hint != "" {
			fmt.Fprintf(&b, "       Fix: %s\n", f.Hint)
		}
	}

	if r.Truncated {
		fmt.Fprintf(&b, "... stopped after %d iterations, more problems may remain\n", r.Iterations)
	}

	return b.String()
}
