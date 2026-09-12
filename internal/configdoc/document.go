// Package configdoc provides a lossless first-pass representation of a
// Ghostty configuration file.
package configdoc

import "errors"

// NodeKind describes the syntax role of one source line.
type NodeKind uint8

const (
	Blank NodeKind = iota
	Comment
	AssignmentNode
	Unknown
)

var (
	ErrAmbiguousAssignment = errors.New("configuration key has multiple assignments")
	ErrInvalidKey          = errors.New("configuration key must not be empty")
	ErrUnsafeValue         = errors.New("configuration value contains a line break or NUL")
)

// Assignment is the public, ordered view of one assignment node.
type Assignment struct {
	Key   string
	Value string
	Empty bool
	Line  int
}

// Change describes one staged source-line change. Line numbers are one-based;
// an appended assignment has Line set to zero.
type Change struct {
	Key        string
	Line       int
	BeforeLine string
	AfterLine  string
	Appended   bool
}

// Node retains the original source line. Assignment fields are populated for
// lines containing a non-empty key and the first '=' character.
type Node struct {
	Kind  NodeKind
	Raw   []byte
	Key   string
	Value string

	linePrefix []byte
	valueLead  []byte
	valueTail  []byte
	eol        []byte
	dirty      bool
	emptyReset bool
}

// Document is an ordered config file. Untouched nodes render byte-for-byte
// identically, including comments, unknown lines, line endings, and values
// containing '=' or '#'.
type Document struct {
	Nodes []Node

	newline          []byte
	pendingNewlineAt int
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	copyOfValue := make([]byte, len(value))
	copy(copyOfValue, value)
	return copyOfValue
}

// Clone returns a deep copy suitable for independent draft editing.
func (d Document) Clone() *Document {
	clone := &Document{
		Nodes:            make([]Node, len(d.Nodes)),
		newline:          cloneBytes(d.newline),
		pendingNewlineAt: d.pendingNewlineAt,
	}
	for index, node := range d.Nodes {
		clone.Nodes[index] = node
		clone.Nodes[index].Raw = cloneBytes(node.Raw)
		clone.Nodes[index].linePrefix = cloneBytes(node.linePrefix)
		clone.Nodes[index].valueLead = cloneBytes(node.valueLead)
		clone.Nodes[index].valueTail = cloneBytes(node.valueTail)
		clone.Nodes[index].eol = cloneBytes(node.eol)
	}
	return clone
}
