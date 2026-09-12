package configdoc

// Lookup returns the last value for key in the document. The last assignment
// wins for a single file, matching the useful local editing behavior while
// include precedence remains the responsibility of the resolver.
func (d Document) Lookup(key string) (string, bool) {
	for i := len(d.Nodes) - 1; i >= 0; i-- {
		node := d.Nodes[i]
		if node.Kind == Assignment && node.Key == key {
			return node.Value, true
		}
	}
	return "", false
}

// Set updates the last assignment for key or appends a new assignment. The
// existing spacing around '=' and the line ending are retained when editing.
func (d *Document) Set(key, value string) {
	for i := len(d.Nodes) - 1; i >= 0; i-- {
		node := &d.Nodes[i]
		if node.Kind != Assignment || node.Key != key {
			continue
		}
		node.Value = value
		node.dirty = true
		return
	}

	if len(d.Nodes) > 0 && len(d.Nodes[len(d.Nodes)-1].eol) == 0 {
		d.pendingNewline = true
	}

	d.Nodes = append(d.Nodes, Node{
		Kind:       Assignment,
		Key:        key,
		Value:      value,
		linePrefix: []byte(key + " ="),
		valueLead:  []byte(" "),
		eol:        cloneBytes(d.newline),
		dirty:      true,
	})
}
