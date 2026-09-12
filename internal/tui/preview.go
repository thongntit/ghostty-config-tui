package tui

import (
	"strconv"
	"strings"
)

func (m Model) previewContent() string {
	var output strings.Builder
	output.WriteString("DRY RUN — no file will be written\n")
	output.WriteString("Config: ")
	output.WriteString(m.configPath)
	output.WriteString("\n")

	if len(m.changes) == 0 {
		output.WriteString("\nNo staged changes.\n")
	} else {
		output.WriteString("\nStaged changes:\n")
		for _, option := range m.options {
			change, ok := m.changes[option.Key]
			if !ok {
				continue
			}
			output.WriteString("\n")
			output.WriteString(option.Key)
			if change.Appended {
				output.WriteString(" (append)\n")
				output.WriteString("+ ")
				output.WriteString(previewLine(change.AfterLine))
				output.WriteString("\n")
				continue
			}
			output.WriteString(" (line ")
			output.WriteString(strconv.Itoa(change.Line))
			output.WriteString(")\n- ")
			output.WriteString(previewLine(change.BeforeLine))
			output.WriteString("\n+ ")
			output.WriteString(previewLine(change.AfterLine))
			output.WriteString("\n")
		}
	}

	output.WriteString("\n--- candidate document ---\n")
	candidate := string(m.DraftBytes())
	output.WriteString(candidate)
	if candidate == "" || !strings.HasSuffix(candidate, "\n") {
		output.WriteString("\n")
	}
	return output.String()
}

func previewLine(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}
