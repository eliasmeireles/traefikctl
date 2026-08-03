package validate

import (
	"regexp"
	"strings"
)

// Schema is a prefix trie of static-config paths derived from
// `traefik --help`, e.g. "--log.compress" registers ["log","compress"].
// Map-valued config nodes (entry point names, TLS option names, resolver
// names, ...) appear in --help as a literal "<name>" segment.
//
// Schema is NEVER a source of truth about validity by itself. Some legal
// static-config sections have no CLI flag at all — "tls:" is a documented
// example (it has zero --tls.* flags on 3.3.5 yet is accepted by Traefik,
// including in this project's own DefaultStaticConfig). The only authority
// on whether a key is valid is Traefik's own healthcheck decode. Schema
// exists solely to disambiguate WHICH of several same-named occurrences a
// "field not found" error is blaming, once healthcheck has already said
// something is wrong. A schema gap can at worst downgrade a finding to
// Certain=false; it must never invent a finding on its own.
type Schema struct {
	root *schemaNode
	// paths holds every leaf path (dotted, lowercase) for suggestion lookups.
	paths []string
}

type schemaNode struct {
	children map[string]*schemaNode
}

func newSchemaNode() *schemaNode {
	return &schemaNode{children: map[string]*schemaNode{}}
}

// helpFlagRe matches a `traefik --help` flag line, e.g.:
//
//	--log.compress  (Default: "false")
var helpFlagRe = regexp.MustCompile(`^\s+--([a-zA-Z0-9._<>-]+)`)

// ParseHelp builds a Schema from the raw output of `traefik --help`.
func ParseHelp(help string) *Schema {
	root := newSchemaNode()
	var paths []string

	for _, line := range strings.Split(help, "\n") {
		m := helpFlagRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		dotted := strings.ToLower(m[1])
		paths = append(paths, dotted)
		insert(root, strings.Split(dotted, "."))
	}

	return &Schema{root: root, paths: paths}
}

func insert(root *schemaNode, segs []string) {
	n := root
	for _, s := range segs {
		child, ok := n.children[s]
		if !ok {
			child = newSchemaNode()
			n.children[s] = child
		}
		n = child
	}
}

// Known reports whether path (case-insensitive) resolves to a real node in
// the schema — either an exact flag or a valid intermediate prefix of one.
// A nil Schema, or a nil/empty path, is never "known".
func (s *Schema) Known(path []string) bool {
	if s == nil || s.root == nil || len(path) == 0 {
		return false
	}
	lower := make([]string, len(path))
	for i, p := range path {
		lower[i] = strings.ToLower(p)
	}
	return knownAt(s.root, lower)
}

func knownAt(n *schemaNode, path []string) bool {
	if len(path) == 0 {
		return true
	}
	seg := path[0]

	if child, ok := n.children[seg]; ok {
		if knownAt(child, path[1:]) {
			return true
		}
	}

	// seg didn't match literally (or the literal branch didn't pan out) —
	// try treating it as a user-chosen name under a map-valued node
	// (an entry point name, a TLS options profile, a certResolver name...).
	if wild, ok := n.children["<name>"]; ok {
		if len(path) == 1 {
			return true
		}
		return knownAt(wild, path[1:])
	}

	return false
}

// SuggestPaths returns every known dotted path whose final segment matches
// leaf (case-insensitive) — used to build "did you mean ...?" hints.
func (s *Schema) SuggestPaths(leaf string) []string {
	if s == nil {
		return nil
	}
	leaf = strings.ToLower(leaf)
	var out []string
	for _, p := range s.paths {
		segs := strings.Split(p, ".")
		if strings.EqualFold(segs[len(segs)-1], leaf) {
			out = append(out, p)
		}
	}
	return out
}
