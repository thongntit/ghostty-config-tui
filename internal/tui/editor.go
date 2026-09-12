package tui

import "github.com/thongntit/ghostty-config-tui/internal/schema"

func (m Model) optionStatus(option schema.Option) string {
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
