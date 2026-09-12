package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func valueHint(option schema.Option) string {
	if len(option.Values) > 0 {
		return strings.Join(option.Values, " | ")
	}
	switch option.Kind {
	case schema.KindBoolean:
		return "true or false"
	case schema.KindNumber:
		return "finite number"
	case schema.KindColor:
		return "#RRGGBB or named color"
	case schema.KindPath:
		return "path"
	case schema.KindDuration:
		return "0, 250ms, or 1s 200ms"
	default:
		return "enter a value"
	}
}

func defaultColorChoices() []ColorChoice {
	return []ColorChoice{
		{Name: "Black", Value: "#000000"},
		{Name: "White", Value: "#ffffff"},
		{Name: "Red", Value: "#f38ba8"},
		{Name: "Orange", Value: "#fab387"},
		{Name: "Yellow", Value: "#f9e2af"},
		{Name: "Green", Value: "#a6e3a1"},
		{Name: "Cyan", Value: "#94e2d5"},
		{Name: "Blue", Value: "#89b4fa"},
		{Name: "Purple", Value: "#cba6f7"},
		{Name: "Ghostty dark", Value: "#282c34"},
	}
}

func friendlyOptionName(key string) string {
	initialisms := map[string]string{
		"gtk":   "GTK",
		"macos": "macOS",
		"ssh":   "SSH",
		"url":   "URL",
		"vt":    "VT",
		"x11":   "X11",
	}
	parts := strings.Split(key, "-")
	for index, part := range parts {
		if replacement, ok := initialisms[part]; ok {
			parts[index] = replacement
			continue
		}
		runes := []rune(part)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		parts[index] = string(runes)
	}
	return strings.Join(parts, " ")
}

func numberStep(option schema.Option) float64 {
	if option.Step != nil && *option.Step > 0 && !math.IsNaN(*option.Step) && !math.IsInf(*option.Step, 0) {
		return *option.Step
	}
	return 1
}

func numberStepLabel(option schema.Option) string {
	return strconv.FormatFloat(numberStep(option), 'f', -1, 64)
}

func (m Model) editorLabel(option schema.Option) string {
	switch {
	case option.Key == "theme" && len(m.themeChoices) > 0:
		return "theme chooser"
	case (option.Key == "background" || option.Key == "foreground") && len(m.colorChoices) > 0:
		return "color chooser"
	case option.Key == "font-size" && option.Step != nil:
		return "numeric stepper"
	case option.Kind == schema.KindBoolean:
		return "on/off toggle"
	default:
		return string(option.Kind)
	}
}

func (m Model) optionStatus(option schema.Option) string {
	if m.graph != nil && !m.readOnly {
		return m.draftOptionStatus(option)
	}
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

	return m.draftOptionStatus(option)
}

func (m Model) draftOptionStatus(option schema.Option) string {
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
