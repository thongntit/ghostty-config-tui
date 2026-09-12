// Package configdoc provides a lossless first-pass representation of a
// Ghostty configuration file.
package configdoc

// NodeKind describes the syntax role of one source line.
type NodeKind uint8

const (
	Blank NodeKind = iota
	Comment
	Assignment
	Unknown
)

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
}

// Document is an ordered config file. Untouched nodes render byte-for-byte
// identically, including comments, unknown lines, line endings, and values
// containing '=' or '#'.
type Document struct {
	Nodes []Node

	newline        []byte
	pendingNewline bool
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	copyOfValue := make([]byte, len(value))
	copy(copyOfValue, value)
	return copyOfValue
}
