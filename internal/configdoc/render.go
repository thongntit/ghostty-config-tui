package configdoc

import "bytes"

// Render returns the current document bytes.
func (d Document) Render() []byte {
	var output bytes.Buffer
	for i, node := range d.Nodes {
		if i > 0 && d.pendingNewline && i == len(d.Nodes)-1 {
			output.Write(d.newline)
		}

		if node.Kind == Assignment && node.dirty {
			output.Write(node.linePrefix)
			output.Write(node.valueLead)
			output.WriteString(node.Value)
			output.Write(node.valueTail)
			output.Write(node.eol)
			continue
		}
		output.Write(node.Raw)
	}
	return output.Bytes()
}
