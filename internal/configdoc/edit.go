package configdoc

import "strings"

// Assignments returns all assignments for key in source order. It intentionally
// does not collapse duplicates, because duplicate scalar keys need explicit
// handling by the editor.
func (d Document) Assignments(key string) []Assignment {
	assignments := make([]Assignment, 0)
	for index, node := range d.Nodes {
		if node.Kind != AssignmentNode || node.Key != key {
			continue
		}
		assignments = append(assignments, Assignment{
			Key:   node.Key,
			Value: node.Value,
			Empty: node.Value == "",
			Line:  index + 1,
		})
	}
	return assignments
}

// Lookup returns the last value for key in the document. The last assignment
// wins for a single file, matching the useful local editing behavior while
// include precedence remains the responsibility of the resolver.
func (d Document) Lookup(key string) (string, bool) {
	for i := len(d.Nodes) - 1; i >= 0; i-- {
		node := d.Nodes[i]
		if node.Kind == AssignmentNode && node.Key == key {
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
		if node.Kind != AssignmentNode || node.Key != key {
			continue
		}
		node.Value = value
		node.emptyReset = value == ""
		node.dirty = true
		return
	}

	if len(d.Nodes) > 0 && len(d.Nodes[len(d.Nodes)-1].eol) == 0 && d.pendingNewlineAt < 0 {
		d.pendingNewlineAt = len(d.Nodes)
	}

	d.Nodes = append(d.Nodes, Node{
		Kind:       AssignmentNode,
		Key:        key,
		Value:      value,
		linePrefix: []byte(key + " ="),
		valueLead:  []byte(" "),
		eol:        cloneBytes(d.newline),
		dirty:      true,
		emptyReset: value == "",
	})
}

// SetScalar edits exactly one assignment or appends a new one. Duplicate keys
// are rejected so a user never changes an ambiguous effective value by
// accident.
func (d *Document) SetScalar(key, value string) (Change, error) {
	if strings.TrimSpace(key) == "" {
		return Change{}, ErrInvalidKey
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return Change{}, ErrUnsafeValue
	}

	assignments := d.Assignments(key)
	if len(assignments) > 1 {
		return Change{}, ErrAmbiguousAssignment
	}
	if len(assignments) == 1 {
		index := assignments[0].Line - 1
		before := string(d.renderNode(d.Nodes[index]))
		setNodeValue(&d.Nodes[index], value)
		after := string(d.renderNode(d.Nodes[index]))
		return Change{
			Key:        key,
			Line:       assignments[0].Line,
			BeforeLine: before,
			AfterLine:  after,
		}, nil
	}

	if len(d.Nodes) > 0 && len(d.Nodes[len(d.Nodes)-1].eol) == 0 && d.pendingNewlineAt < 0 {
		d.pendingNewlineAt = len(d.Nodes)
	}

	node := Node{
		Kind:       AssignmentNode,
		Key:        key,
		Value:      value,
		linePrefix: []byte(key + " ="),
		valueLead:  []byte(" "),
		eol:        cloneBytes(d.newline),
		dirty:      true,
		emptyReset: value == "",
	}
	d.Nodes = append(d.Nodes, node)
	return Change{
		Key:        key,
		BeforeLine: "",
		AfterLine:  string(d.renderNode(node)),
		Appended:   true,
	}, nil
}

func setNodeValue(node *Node, value string) {
	node.Value = value
	node.emptyReset = value == ""
	node.dirty = true
}
