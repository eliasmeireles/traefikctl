package validate

import (
	"fmt"
	"strings"
)

// knownHints are fixes for specific bad paths we've actually seen in the
// wild, keyed by lowercase dotted path. These take priority over the
// generic schema-suggestion fallback because they can explain *why*, not
// just *where else*.
var knownHints = map[string]string{
	"accesslog.maxsize":    "Traefik 3 only supports log rotation under `log:`, not `accessLog:`. Use `traefikctl logs rotation install` to rotate access.log instead.",
	"accesslog.maxage":     "Traefik 3 only supports log rotation under `log:`, not `accessLog:`. Use `traefikctl logs rotation install` to rotate access.log instead.",
	"accesslog.maxbackups": "Traefik 3 only supports log rotation under `log:`, not `accessLog:`. Use `traefikctl logs rotation install` to rotate access.log instead.",
	"accesslog.compress":   "Traefik 3 only supports log rotation under `log:`, not `accessLog:`. Use `traefikctl logs rotation install` to rotate access.log instead.",
}

// hint suggests a fix for a rejected key at path (original casing, root to
// leaf). It checks the known-issues table first, then falls back to
// suggesting other schema locations that share the same leaf name.
func hint(path []string, schema *Schema) string {
	if len(path) == 0 {
		return ""
	}
	dotted := strings.ToLower(strings.Join(path, "."))
	if h, ok := knownHints[dotted]; ok {
		return h
	}

	leaf := path[len(path)-1]
	suggestions := schema.SuggestPaths(leaf)
	switch len(suggestions) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("did you mean `%s`?", suggestions[0])
	default:
		return fmt.Sprintf("did you mean one of: %s?", strings.Join(suggestions, ", "))
	}
}
