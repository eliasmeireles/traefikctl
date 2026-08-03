package validate

import (
	"encoding/json"
	"regexp"
	"strings"
)

// outcome classifies what a single `traefik healthcheck --configFile=X` call
// told us about the static configuration at X.
type outcome int

const (
	// outcomeParsed means Traefik successfully decoded the static config.
	// healthcheck itself may still have failed for unrelated reasons (e.g.
	// "please enable ping") — that is a decode success from our point of view.
	outcomeParsed outcome = iota
	// outcomeUnknownField means Traefik's paerser rejected a field it
	// doesn't recognize: "field not found, node: X".
	outcomeUnknownField
	// outcomeEmptySection means a section that has to carry at least one
	// field was left empty — most often an artifact of this validator
	// having just stripped its only child key, e.g. "serversTransport:
	// {dialTimeout: 5s}" becomes "serversTransport: {}" once dialTimeout is
	// removed for being unnested. NOT every empty section is invalid
	// (`ping: {}` is a legitimate way to enable a feature with defaults),
	// so this is only inferred from Traefik's own explicit complaint,
	// never assumed.
	outcomeEmptySection
	// outcomeNoConfig means the file has no static config Traefik recognizes
	// at all (e.g. a dynamic-only file was pointed at).
	outcomeNoConfig
	// outcomeBadExtension means the scratch file wasn't named *.yaml/*.yml.
	// This indicates a bug in the caller, not a problem with the user's config.
	outcomeBadExtension
	// outcomeParseError is any other decode failure (bad duration string,
	// type mismatch, ...) that can't be resolved to a single removable key.
	outcomeParseError
	// outcomeRunnerError means traefik itself could not be executed at all
	// (binary missing, context canceled, ...).
	outcomeRunnerError
)

// fieldNotFoundRe extracts the offending key name from Traefik's paerser
// error, e.g. "command healthcheck error: field not found, node: compress".
var fieldNotFoundRe = regexp.MustCompile(`field not found, node: (\S+)`)

// emptySectionRe extracts the section name from Traefik's complaint about a
// now-empty struct-typed section, e.g. "command healthcheck error:
// serversTransport cannot be a standalone element (type
// *static.ServersTransport)". Not anchored — the section name always
// follows a "command ... error: " prefix in the wrapping JSON.
var emptySectionRe = regexp.MustCompile(`(\w+) cannot be a standalone element`)

type classified struct {
	outcome outcome
	// node is the bare key name Traefik blamed, populated for
	// outcomeUnknownField and outcomeEmptySection.
	node string
	// raw is the verbatim message, kept for findings that can't be tied to
	// a specific YAML node.
	raw string
}

// classify interprets the stderr of a `traefik healthcheck` invocation.
//
// Traefik logs a single structured JSON line to stderr on most failures
// (e.g. {"level":"error","error":"command healthcheck error: field not
// found, node: compress",...}), but the "config decoded, health check
// itself refused" case ("Error calling healthcheck: please enable `ping`...")
// is plain text. Both must be handled, and the exit code is 1 in every case
// above — it cannot be used to tell success from failure.
func classify(stderr string, runErr error) classified {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		if runErr == nil {
			return classified{outcome: outcomeParsed}
		}
		return classified{outcome: outcomeRunnerError, raw: runErr.Error()}
	}

	var msg string
	haveJSON := false
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err == nil && entry.Error != "" {
			msg = entry.Error
			haveJSON = true
			continue
		}
		if !haveJSON {
			msg = line
		}
	}

	switch {
	case strings.HasPrefix(msg, "Error calling healthcheck"):
		// The static config decoded fine; healthcheck failed for an
		// unrelated, expected reason (no `ping:` block configured).
		return classified{outcome: outcomeParsed, raw: msg}
	case strings.Contains(msg, "unsupported file extension"):
		return classified{outcome: outcomeBadExtension, raw: msg}
	case strings.Contains(msg, "no valid configuration found"):
		return classified{outcome: outcomeNoConfig, raw: msg}
	default:
		if m := fieldNotFoundRe.FindStringSubmatch(msg); m != nil {
			return classified{outcome: outcomeUnknownField, node: m[1], raw: msg}
		}
		if m := emptySectionRe.FindStringSubmatch(msg); m != nil {
			return classified{outcome: outcomeEmptySection, node: m[1], raw: msg}
		}
		return classified{outcome: outcomeParseError, raw: msg}
	}
}
