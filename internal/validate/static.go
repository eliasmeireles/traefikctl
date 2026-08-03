// Package validate checks a Traefik static configuration file the way
// Traefik itself would parse it, instead of trusting a bare os.Stat.
//
// Traefik's own config decoder (paerser) reports only the FIRST unknown
// field it encounters, then exits — so fixing a broken traefik.yaml by
// hand becomes a one-field-per-restart loop. StaticFile runs
// `traefik healthcheck --configFile=X` (a full config decode that binds no
// ports and needs no root) repeatedly, stripping out each rejected field
// from a scratch copy until the config decodes cleanly or nothing more can
// be removed, and reports every rejected field it found in one pass.
package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultMaxIterations bounds how many times the validator will re-run
// Traefik against a shrinking scratch copy before giving up.
const DefaultMaxIterations = 25

// Options configures StaticFile / StaticBytes. All fields are optional.
type Options struct {
	// Runner executes `traefik`. Defaults to NewExecRunner("").
	Runner Runner
	// Schema disambiguates which occurrence of a rejected key is the
	// invalid one. Built lazily from Runner.Help if nil and Runner is
	// available.
	Schema *Schema
	// MaxIterations caps the strip-and-retry loop. Defaults to
	// DefaultMaxIterations.
	MaxIterations int
	// WorkDir is where the scratch config copy is written. A temp dir is
	// created and cleaned up automatically if empty.
	WorkDir string
}

func (o Options) withDefaults() Options {
	if o.Runner == nil {
		o.Runner = NewExecRunner("")
	}
	if o.MaxIterations <= 0 {
		o.MaxIterations = DefaultMaxIterations
	}
	return o
}

// StaticFile validates the static Traefik configuration at path. It never
// modifies path — all mutation happens on an in-memory copy and a scratch
// file under Options.WorkDir.
func StaticFile(ctx context.Context, path string, opts Options) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				ConfigPath: path,
				Findings: []Finding{{
					Message:  fmt.Sprintf("config file not found: %s", path),
					Severity: SeverityError,
				}},
			}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return StaticBytes(ctx, path, data, opts)
}

// StaticBytes validates content as if it were the static config at name
// (used only for reporting; content need not exist on disk).
func StaticBytes(ctx context.Context, name string, content []byte, opts Options) (*Result, error) {
	opts = opts.withDefaults()
	result := &Result{ConfigPath: name}

	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		result.Findings = append(result.Findings, Finding{
			Message:  fmt.Sprintf("YAML syntax error: %v", err),
			Severity: SeverityError,
		})
		return result, nil
	}

	if !opts.Runner.Available() {
		result.Skipped = true
		result.SkipReason = "traefik binary not available — only YAML syntax was checked"
		return result, nil
	}

	if opts.Schema == nil {
		if help, err := opts.Runner.Help(ctx); err == nil {
			opts.Schema = ParseHelp(help)
		}
		// A Help failure just means every rejected field is reported with
		// Certain=false — schema is an aid, never a requirement.
	}

	scratchDir := opts.WorkDir
	if scratchDir == "" {
		dir, err := os.MkdirTemp("", "traefikctl-validate-*")
		if err != nil {
			return nil, fmt.Errorf("create scratch dir: %w", err)
		}
		defer os.RemoveAll(dir)
		scratchDir = dir
	}
	scratchPath := filepath.Join(scratchDir, "traefik.yaml")

	for iter := 1; iter <= opts.MaxIterations; iter++ {
		result.Iterations = iter

		out, err := yaml.Marshal(&doc)
		if err != nil {
			return nil, fmt.Errorf("re-marshal config: %w", err)
		}
		if err := os.WriteFile(scratchPath, out, 0o600); err != nil {
			return nil, fmt.Errorf("write scratch config: %w", err)
		}

		stderr, runErr := opts.Runner.Healthcheck(ctx, scratchPath)
		c := classify(stderr, runErr)

		switch c.outcome {
		case outcomeParsed:
			return result, nil

		case outcomeUnknownField:
			if !stripUnknownField(&doc, c, opts.Schema, result) {
				return result, nil
			}
			continue

		case outcomeEmptySection:
			// This section was valid in the user's original file; it only
			// went empty because we just stripped its one child key. Drop
			// it silently — it's our own artifact, not something to report.
			occs := findKey(&doc, c.node)
			if len(occs) == 0 {
				result.Findings = append(result.Findings, Finding{
					Message:  fmt.Sprintf("%s (and the now-empty section could not be located to remove it)", c.raw),
					Severity: SeverityError,
				})
				return result, nil
			}
			removeOccurrences(occs)
			continue

		case outcomeNoConfig, outcomeParseError:
			result.Findings = append(result.Findings, Finding{
				Message:  c.raw,
				Severity: SeverityError,
			})
			return result, nil

		case outcomeBadExtension:
			return nil, fmt.Errorf("internal error: scratch file rejected on extension: %s", c.raw)

		default: // outcomeRunnerError
			return nil, fmt.Errorf("failed to run traefik healthcheck: %s", c.raw)
		}
	}

	result.Truncated = true
	return result, nil
}

// stripUnknownField locates every occurrence of the field Traefik just
// rejected, records a Finding for the ones it removes, and mutates doc in
// place. It returns false when the field couldn't be located at all (a
// literal-key search came up empty), in which case the loop cannot make
// progress and must stop.
func stripUnknownField(doc *yaml.Node, c classified, schema *Schema, result *Result) bool {
	occs := findKey(doc, c.node)
	if len(occs) == 0 {
		result.Findings = append(result.Findings, Finding{
			Message:  fmt.Sprintf("Traefik rejected unknown field %q but it could not be located in the YAML (%s)", c.node, c.raw),
			Severity: SeverityError,
		})
		return false
	}

	var known, unknown []occurrence
	for _, o := range occs {
		if schema.Known(o.path) {
			known = append(known, o)
		} else {
			unknown = append(unknown, o)
		}
	}

	toRemove := unknown
	certain := true
	if len(occs) > 1 && len(known) == 0 {
		// More than one place has this key name, and the schema couldn't
		// clear any of them — we don't actually know which one Traefik
		// meant, only that at least one of them is wrong.
		certain = false
	}
	if len(toRemove) == 0 {
		// Schema insists every occurrence is valid, yet Traefik rejected
		// the name — remove all of them and say so rather than loop
		// forever on a field we can't otherwise resolve.
		toRemove = occs
		certain = false
	}

	for _, o := range toRemove {
		dotted := strings.ToLower(strings.Join(o.path, "."))
		parentPath := strings.Join(o.path[:len(o.path)-1], ".")
		result.Findings = append(result.Findings, Finding{
			Path:     dotted,
			Key:      o.path[len(o.path)-1],
			Message:  fmt.Sprintf("unknown field %q under %s", o.path[len(o.path)-1], parentPath),
			Hint:     hint(o.path, schema),
			Line:     o.line,
			Column:   o.column,
			Severity: SeverityError,
			Certain:  certain,
		})
	}

	removeOccurrences(toRemove)
	return true
}
