package tui

import (
	"fmt"
	"strconv"
	"strings"
)

func (m Model) previewContent() string {
	var output strings.Builder
	output.WriteString("DRY RUN — no file will be written while previewing\n")
	output.WriteString("Candidate configuration\n\n")

	snapshots := m.changedFileSnapshots()
	if len(snapshots) == 0 {
		output.WriteString("No staged changes.\n")
	} else {
		output.WriteString("Staged file changes:\n")
		for _, snapshot := range snapshots {
			output.WriteString("\n--- ")
			output.WriteString(snapshot.Path)
			output.WriteString(" ---\n")
			output.WriteString(documentDiff(snapshot.Original, snapshot.Candidate))
		}
	}

	if m.graph != nil {
		output.WriteString("\n")
		output.WriteString(graphSummary(m))
	} else {
		output.WriteString("\n--- candidate document ---\n")
		candidate := string(m.DraftBytes())
		output.WriteString(candidate)
		if candidate == "" || !strings.HasSuffix(candidate, "\n") {
			output.WriteString("\n")
		}
	}
	return output.String()
}

func documentDiff(original, candidate []byte) string {
	oldLines := splitPreviewLines(string(original))
	newLines := splitPreviewLines(string(candidate))
	limit := len(oldLines)
	if len(newLines) > limit {
		limit = len(newLines)
	}
	var output strings.Builder
	changed := false
	for index := 0; index < limit; index++ {
		oldLine, newLine := "", ""
		if index < len(oldLines) {
			oldLine = oldLines[index]
		}
		if index < len(newLines) {
			newLine = newLines[index]
		}
		if oldLine == newLine {
			continue
		}
		changed = true
		if oldLine != "" || index < len(oldLines) {
			output.WriteString("- ")
			output.WriteString(oldLine)
			output.WriteString("\n")
		}
		if newLine != "" || index < len(newLines) {
			output.WriteString("+ ")
			output.WriteString(newLine)
			output.WriteString("\n")
		}
	}
	if !changed {
		return "No line changes.\n"
	}
	return output.String()
}

func splitPreviewLines(value string) []string {
	if value == "" {
		return nil
	}
	lines := strings.Split(value, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func graphSummary(m Model) string {
	draftGraph := m.draftGraph()
	var output strings.Builder
	output.WriteString("Effective Ghostty configuration graph\n\n")
	output.WriteString("Roots:\n")
	for _, root := range draftGraph.Roots {
		output.WriteString("- ")
		output.WriteString(root)
		output.WriteString("\n")
	}

	output.WriteString("\nLoaded source files:\n")
	for index, file := range draftGraph.Files {
		output.WriteString(fmt.Sprintf("%d. %s\n", index+1, file.Path))
	}

	if len(draftGraph.Includes) > 0 {
		output.WriteString("\nIncludes (reloaded from saved files):\n")
		for _, include := range draftGraph.Includes {
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

	if len(draftGraph.Diagnostics) > 0 {
		output.WriteString("\nDiagnostics:\n")
		for _, diagnostic := range draftGraph.Diagnostics {
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

	output.WriteString(fmt.Sprintf("\nEffective assignments: %d\n", len(draftGraph.Assignments)))
	for _, assignment := range draftGraph.Assignments {
		output.WriteString(fmt.Sprintf("%s:%d  %s = %s\n", assignment.Path, assignment.Line, assignment.Key, assignment.Value))
	}

	output.WriteString("\n--- candidate source documents ---\n")
	for _, snapshot := range m.changedFileSnapshots() {
		output.WriteString("\n# ")
		output.WriteString(snapshot.Path)
		output.WriteString("\n")
		output.Write(snapshot.Candidate)
		if len(snapshot.Candidate) == 0 || snapshot.Candidate[len(snapshot.Candidate)-1] != '\n' {
			output.WriteString("\n")
		}
	}
	return output.String()
}
