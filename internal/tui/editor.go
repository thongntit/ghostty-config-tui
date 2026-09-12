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
	case option.Kind == schema.KindColor && option.Key != "palette" && len(m.colorChoices) > 0:
		return "color chooser"
	case option.Kind == schema.KindNumber:
		return "numeric stepper"
	case option.Kind == schema.KindBoolean:
		return "on/off toggle"
	case option.Kind == schema.KindKeybind:
		return "keybinding list"
	case option.Repeatable():
		return "repeatable list"
	case option.Kind == schema.KindCommand:
		return "command input"
	default:
		return "text input"
	}
}

func (m Model) optionStatus(option schema.Option) string {
	assignments := m.optionAssignments(option.Key)
	if len(assignments) == 0 {
		return "not set"
	}
	if option.Repeatable() {
		return fmt.Sprintf("%d value(s)", len(assignments))
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

func (m Model) draftOptionStatus(option schema.Option) string {
	return m.optionStatus(option)
}

func (m Model) optionSource(option schema.Option) string {
	if m.graph == nil {
		return ""
	}
	assignments := m.optionAssignments(option.Key)
	if len(assignments) == 0 {
		return "No assignment in loaded config graph"
	}
	effective := assignments[len(assignments)-1]
	return fmt.Sprintf("Effective source: %s:%d", effective.Path, effective.Line)
}
