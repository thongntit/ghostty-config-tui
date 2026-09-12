package configdoc

import "bytes"

func (d Document) renderNode(node Node) []byte {
	if node.Kind != AssignmentNode || !node.dirty {
		return cloneBytes(node.Raw)
	}

	var output bytes.Buffer
	output.Write(node.linePrefix)
	if !node.emptyReset {
		output.Write(node.valueLead)
	}
	output.WriteString(node.Value)
	if !node.emptyReset {
		output.Write(node.valueTail)
	}
	output.Write(node.eol)
	return output.Bytes()
}

// Render returns the current document bytes.
func (d Document) Render() []byte {
	var output bytes.Buffer
	for i, node := range d.Nodes {
		if i == d.pendingNewlineAt {
			output.Write(d.newline)
		}

		output.Write(d.renderNode(node))
	}
	return output.Bytes()
}

// Bytes is an explicit alias for Render at call sites that deal with file
// content rather than a textual view.
func (d Document) Bytes() []byte {
	return d.Render()
}
