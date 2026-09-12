package configdoc

import (
	"bytes"
	"strings"
)

// Parse creates a lossless document from Ghostty config bytes. Ghostty's
// syntax is intentionally simple, so malformed non-empty lines are retained
// as Unknown nodes instead of being discarded.
func Parse(input []byte) Document {
	document := Document{newline: []byte("\n")}
	if bytes.Contains(input, []byte("\r\n")) {
		document.newline = []byte("\r\n")
	}

	remaining := input
	for len(remaining) > 0 {
		line, rest := takeLine(remaining)
		document.Nodes = append(document.Nodes, parseLine(line))
		remaining = rest
	}

	return document
}

func takeLine(input []byte) (line, rest []byte) {
	newline := bytes.IndexByte(input, '\n')
	if newline < 0 {
		return cloneBytes(input), nil
	}
	return cloneBytes(input[:newline+1]), input[newline+1:]
}

func parseLine(line []byte) Node {
	content, eol := splitEOL(line)
	rawContent := cloneBytes(content)
	trimmed := bytes.TrimSpace(content)

	base := Node{Raw: cloneBytes(line), eol: cloneBytes(eol)}
	if len(trimmed) == 0 {
		base.Kind = Blank
		return base
	}

	if bytes.HasPrefix(bytes.TrimLeft(content, " \t"), []byte("#")) {
		base.Kind = Comment
		return base
	}

	equals := bytes.IndexByte(content, '=')
	if equals < 0 {
		base.Kind = Unknown
		return base
	}

	key := strings.TrimSpace(string(content[:equals]))
	if key == "" {
		base.Kind = Unknown
		return base
	}

	rawValue := content[equals+1:]
	valueStart := len(rawValue) - len(bytes.TrimLeft(rawValue, " \t"))
	valueEnd := len(bytes.TrimRight(rawValue, " \t"))

	base.Kind = Assignment
	base.Key = key
	base.Value = string(rawValue[valueStart:valueEnd])
	base.linePrefix = cloneBytes(rawContent[:equals+1])
	base.valueLead = cloneBytes(rawValue[:valueStart])
	base.valueTail = cloneBytes(rawValue[valueEnd:])
	return base
}

func splitEOL(line []byte) (content, eol []byte) {
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		return line[:len(line)-2], []byte("\r\n")
	case bytes.HasSuffix(line, []byte("\n")):
		return line[:len(line)-1], []byte("\n")
	default:
		return line, nil
	}
}
