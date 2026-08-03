package validate

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// occurrence is one place a given key name appears as a mapping key
// somewhere in a YAML document.
type occurrence struct {
	parent *yaml.Node // the enclosing mapping node
	index  int        // index of the key node within parent.Content (key, value pairs)
	// path is the chain of keys from the document root to this key,
	// original casing preserved for reporting.
	path []string
	// line/column are captured at find time, before any mutation, so they
	// stay anchored to the original file regardless of later edits.
	line, column int
}

// findKey returns every mapping-key occurrence of name (case-insensitive)
// anywhere in doc. Traefik's "field not found, node: X" error gives only
// the bare key name, not its full path, so ambiguity (the same key nested
// under several parents) is expected and left to the caller to resolve.
func findKey(doc *yaml.Node, name string) []occurrence {
	root := doc
	if root != nil && root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}

	var out []occurrence
	var walk func(n *yaml.Node, path []string)
	walk = func(n *yaml.Node, path []string) {
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				keyNode := n.Content[i]
				valNode := n.Content[i+1]

				childPath := make([]string, len(path)+1)
				copy(childPath, path)
				childPath[len(path)] = keyNode.Value

				if strings.EqualFold(keyNode.Value, name) {
					out = append(out, occurrence{
						parent: n,
						index:  i,
						path:   childPath,
						line:   keyNode.Line,
						column: keyNode.Column,
					})
				}
				walk(valNode, childPath)
			}
		case yaml.SequenceNode:
			for _, item := range n.Content {
				walk(item, path)
			}
		}
	}
	walk(root, nil)
	return out
}

// removeOccurrences deletes each occurrence's key/value pair from its
// parent mapping. Occurrences sharing the same parent are removed in
// descending index order so earlier removals don't invalidate later
// indices within that parent.
func removeOccurrences(occs []occurrence) {
	byParent := map[*yaml.Node][]int{}
	for _, o := range occs {
		byParent[o.parent] = append(byParent[o.parent], o.index)
	}
	for parent, idxs := range byParent {
		sort.Sort(sort.Reverse(sort.IntSlice(idxs)))
		for _, idx := range idxs {
			parent.Content = append(parent.Content[:idx], parent.Content[idx+2:]...)
		}
	}
}
