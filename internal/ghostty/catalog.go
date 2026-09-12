package ghostty

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const actionTimeout = 3 * time.Second

// ColorChoice is one color name and its canonical RGB value as reported by
// Ghostty. The name is the value users can select in a color editor.
type ColorChoice struct {
	Name  string
	Value string
}

// ListThemes asks the installed Ghostty binary for its available theme names.
// The command includes both user-configured themes and bundled resources; an
// empty result is an error so callers can fall back to raw input explicitly.
func ListThemes(binary string) ([]string, error) {
	output, err := actionOutput(binary, "+list-themes", "--plain")
	if err != nil {
		return nil, fmt.Errorf("list Ghostty themes: %w", err)
	}
	return parseThemeList(output), nil
}

// ListColors asks the installed Ghostty binary for its named X11 colors.
func ListColors(binary string) ([]ColorChoice, error) {
	output, err := actionOutput(binary, "+list-colors", "--plain")
	if err != nil {
		return nil, fmt.Errorf("list Ghostty colors: %w", err)
	}
	return parseColorList(output), nil
}

func actionOutput(binary string, args ...string) ([]byte, error) {
	if strings.TrimSpace(binary) == "" {
		return nil, fmt.Errorf("Ghostty executable is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return output, nil
}

func parseThemeList(output []byte) []string {
	choices := make([]string, 0)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		// `+list-themes` annotates each row with `(config)` or `(resources)`.
		// Strip only that trailing source annotation, preserving spaces in the
		// actual theme name.
		if marker := strings.LastIndex(line, " ("); marker >= 0 && strings.HasSuffix(line, ")") {
			line = strings.TrimSpace(line[:marker])
		}
		if line == "" {
			continue
		}
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		choices = append(choices, line)
	}
	return choices
}

func parseColorList(output []byte) []ColorChoice {
	choices := make([]ColorChoice, 0)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(rawLine)
		separator := strings.Index(line, "=")
		if separator <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:separator])
		value := strings.TrimSpace(line[separator+1:])
		if name == "" || value == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		choices = append(choices, ColorChoice{Name: name, Value: value})
	}
	return choices
}
