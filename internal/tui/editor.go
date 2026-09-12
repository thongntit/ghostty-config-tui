package tui

import (
	"fmt"

	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func (m Model) optionStatus(option schema.Option) string {
	if m.graph != nil {
		if option.Key == "config-file" {
			if len(m.graph.Includes) == 0 {
				return "not set"
			}
			return fmt.Sprintf("%d included files", len(m.graph.Includes))
		}
		assignments := m.graph.AssignmentsFor(option.Key)
		if len(assignments) == 0 {
			return "not set"
		}
		effective := assignments[len(assignments)-1]
		if effective.Empty {
			return "default"
		}
		if len(assignments) > 1 {
			return fmt.Sprintf("%s (%d assignments)", effective.Value, len(assignments))
		}
		return effective.Value
	}

	assignments := m.draft.Assignments(option.Key)
	switch len(assignments) {
	case 0:
		return "not set"
	case 1:
		if assignments[0].Empty {
			return "default"
		}
		return assignments[0].Value
	default:
		return "duplicate (read-only)"
	}
}

func (m Model) optionSource(option schema.Option) string {
	if m.graph == nil {
		return ""
	}
	assignments := m.graph.AssignmentsFor(option.Key)
	if len(assignments) == 0 {
		return "No assignment in loaded config graph"
	}
	effective := assignments[len(assignments)-1]
	return fmt.Sprintf("Effective source: %s:%d", effective.Path, effective.Line)
}
