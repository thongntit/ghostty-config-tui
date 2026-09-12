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
	return d.SetAssignment(key, 0, value)
}

// SetAssignment edits the assignment at line. A line of zero appends a new
// assignment. The key is checked at the requested line so callers cannot
// accidentally overwrite a different source line after a draft changed.
func (d *Document) SetAssignment(key string, line int, value string) (Change, error) {
	if strings.TrimSpace(key) == "" {
		return Change{}, ErrInvalidKey
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return Change{}, ErrUnsafeValue
	}

	if line > 0 {
		index := line - 1
		if index < 0 || index >= len(d.Nodes) || d.Nodes[index].Kind != AssignmentNode || d.Nodes[index].Key != key {
			return Change{}, ErrAssignmentTarget
		}
		before := string(d.renderNode(d.Nodes[index]))
		setNodeValue(&d.Nodes[index], value)
		after := string(d.renderNode(d.Nodes[index]))
		return Change{
			Key:        key,
			Line:       line,
			BeforeLine: before,
			AfterLine:  after,
		}, nil
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

// AppendAssignment always appends a new assignment, even when the key already
// exists. It is the operation used by repeatable and keybinding editors.
func (d *Document) AppendAssignment(key, value string) (Change, error) {
	if strings.TrimSpace(key) == "" {
		return Change{}, ErrInvalidKey
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return Change{}, ErrUnsafeValue
	}
	return d.appendAssignment(key, value)
}

func (d *Document) appendAssignment(key, value string) (Change, error) {
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

// RemoveAssignment removes the assignment at the one-based source line. The
// surrounding nodes and their original bytes remain untouched.
func (d *Document) RemoveAssignment(key string, line int) error {
	index := line - 1
	if line <= 0 || index < 0 || index >= len(d.Nodes) || d.Nodes[index].Kind != AssignmentNode || d.Nodes[index].Key != key {
		return ErrAssignmentTarget
	}
	d.Nodes = append(d.Nodes[:index], d.Nodes[index+1:]...)
	if d.pendingNewlineAt > index {
		d.pendingNewlineAt--
	} else if d.pendingNewlineAt == index {
		d.pendingNewlineAt = -1
	}
	return nil
}

func setNodeValue(node *Node, value string) {
	node.Value = value
	node.emptyReset = value == ""
	node.dirty = true
}
