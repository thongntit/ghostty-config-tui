package tui

import (
	"fmt"
	"strconv"
	"strings"
)

func (m Model) previewContent() string {
	if m.graph != nil && m.readOnly {
		return m.graphPreviewContent()
	}

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

func (m Model) graphPreviewContent() string {
	var output strings.Builder
	output.WriteString("DRY RUN — no file will be written\n")
	output.WriteString("Effective Ghostty configuration graph\n\n")
	output.WriteString("Roots:\n")
	for _, root := range m.graph.Roots {
		output.WriteString("- ")
		output.WriteString(root)
		output.WriteString("\n")
	}

	output.WriteString("\nLoaded source files:\n")
	for index, file := range m.graph.Files {
		output.WriteString(fmt.Sprintf("%d. %s\n", index+1, file.Path))
	}

	if len(m.graph.Includes) > 0 {
		output.WriteString("\nIncludes:\n")
		for _, include := range m.graph.Includes {
			output.WriteString(fmt.Sprintf("- %s:%d -> %s", include.SourcePath, include.Line, include.ResolvedPath))
			if include.Optional {
				output.WriteString(" (optional)")
			}
			if !include.Loaded {
				output.WriteString(" (not loaded)")
			}
			output.WriteString("\n")
		}
	}

	if len(m.graph.Diagnostics) > 0 {
		output.WriteString("\nDiagnostics:\n")
		for _, diagnostic := range m.graph.Diagnostics {
			output.WriteString("- ")
			output.WriteString(string(diagnostic.Severity))
			if diagnostic.Path != "" {
				output.WriteString(" ")
				output.WriteString(diagnostic.Path)
				if diagnostic.Line > 0 {
					output.WriteString(":" + strconv.Itoa(diagnostic.Line))
				}
			}
			output.WriteString(" — ")
			output.WriteString(diagnostic.Message)
			output.WriteString("\n")
		}
	}

	output.WriteString(fmt.Sprintf("\nEffective assignments: %d\n", len(m.graph.Assignments)))
	for _, assignment := range m.graph.Assignments {
		output.WriteString(fmt.Sprintf("%s:%d  %s = %s\n", assignment.Path, assignment.Line, assignment.Key, assignment.Value))
	}

	output.WriteString("\n--- selected source document ---\n")
	output.Write(m.DraftBytes())
	if candidate := m.DraftBytes(); len(candidate) == 0 || !strings.HasSuffix(string(candidate), "\n") {
		output.WriteString("\n")
	}
	return output.String()
}

func previewLine(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}
