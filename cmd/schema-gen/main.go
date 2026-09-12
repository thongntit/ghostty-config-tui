// Command schema-gen creates a reviewed Ghostty option catalog from a pinned
// Ghostty binary. Generated output is a candidate until it is reviewed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/ghostty"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "schema-gen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("schema-gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ghosttyPath := flags.String("ghostty", ghostty.Find(), "path to the Ghostty executable")
	outputPath := flags.String("out", "", "output JSON catalog path")
	check := flags.Bool("check", false, "fail if the generated catalog differs from --out")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *outputPath == "" {
		return fmt.Errorf("--out PATH is required")
	}
	if *ghosttyPath == "" {
		return fmt.Errorf("Ghostty executable not found; pass --ghostty PATH")
	}

	versionOutput, err := commandOutput(*ghosttyPath, "+version")
	if err != nil {
		return fmt.Errorf("read Ghostty version: %w", err)
	}
	ghosttyVersion, err := parseVersion(versionOutput)
	if err != nil {
		return err
	}

	docs, err := commandOutput(*ghosttyPath, "+show-config", "--default", "--docs")
	if err != nil {
		return fmt.Errorf("read Ghostty config documentation: %w", err)
	}
	catalog, err := catalogFromDocs(docs, ghosttyVersion)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	encoded = append(encoded, '\n')
	if *check {
		existing, err := os.ReadFile(*outputPath)
		if err != nil {
			return fmt.Errorf("read catalog for check %q: %w", *outputPath, err)
		}
		if !bytes.Equal(existing, encoded) {
			return fmt.Errorf("catalog %q is stale; regenerate it with schema-gen", *outputPath)
		}
		fmt.Fprintf(os.Stdout, "catalog is current: %s\n", *outputPath)
		return nil
	}
	if err := os.WriteFile(*outputPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write catalog %q: %w", *outputPath, err)
	}
	fmt.Fprintf(os.Stdout, "generated %d options for Ghostty %s at %s\n", len(catalog.Options), ghosttyVersion, *outputPath)
	return nil
}

func commandOutput(path string, args ...string) ([]byte, error) {
	command := exec.Command(path, args...)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return output, nil
}

func parseVersion(output []byte) (string, error) {
	fields := strings.Fields(string(output))
	for index, field := range fields {
		if field == "Ghostty" && index+1 < len(fields) {
			return fields[index+1], nil
		}
	}
	return "", fmt.Errorf("Ghostty version output did not contain a Ghostty version")
}

func catalogFromDocs(docs []byte, ghosttyVersion string) (schema.Catalog, error) {
	document := configdoc.Parse(docs)
	options := make([]schema.Option, 0, len(document.Nodes))
	seen := make(map[string]struct{})
	comments := make([]string, 0, 8)
	for _, node := range document.Nodes {
		switch node.Kind {
		case configdoc.Comment:
			comments = append(comments, commentLine(node.Raw))
		case configdoc.Blank:
			comments = nil
		case configdoc.AssignmentNode:
			if _, exists := seen[node.Key]; exists {
				comments = nil
				continue
			}
			seen[node.Key] = struct{}{}
			context := strings.TrimSpace(strings.Join(comments, " "))
			description := firstSentence(context)
			if description == "" {
				description = "Ghostty configuration option."
			}
			option := schema.Option{
				Key:          node.Key,
				Category:     categoryFor(node.Key),
				Kind:         kindFor(node.Key, node.Value, context),
				Description:  description,
				Edit:         editFor(node.Key, node.Value, context),
				Default:      node.Value,
				Availability: availabilityFor(context),
				Docs:         "https://ghostty.org/docs/config/reference#" + node.Key,
			}
			if node.Key == "font-size" {
				step := 0.5
				option.Step = &step
			}
			options = append(options, option)
			comments = nil
		default:
			comments = nil
		}
	}
	if len(options) == 0 {
		return schema.Catalog{}, fmt.Errorf("Ghostty documentation contained no configuration options")
	}
	return schema.Catalog{
		SchemaVersion:  2,
		GhosttyVersion: ghosttyVersion,
		Source:         "ghostty +show-config --default --docs",
		SourceRevision: sourceRevision(ghosttyVersion),
		Options:        options,
	}, nil
}

func sourceRevision(version string) string {
	if index := strings.LastIndex(version, "+"); index >= 0 && index+1 < len(version) {
		return version[index+1:]
	}
	return ""
}

func commentLine(raw []byte) string {
	line := strings.TrimSpace(string(raw))
	line = strings.TrimPrefix(line, "#")
	return strings.TrimSpace(line)
}

func firstSentence(context string) string {
	context = strings.Join(strings.Fields(context), " ")
	if context == "" {
		return ""
	}
	if index := strings.Index(context, ". "); index >= 0 {
		return strings.TrimSpace(context[:index+1])
	}
	return context
}

func categoryFor(key string) string {
	groups := []struct {
		name     string
		prefixes []string
	}{
		{name: "Appearance", prefixes: []string{"background", "foreground", "font", "palette", "theme", "selection", "cursor-color", "bold-color", "minimum-contrast"}},
		{name: "Input", prefixes: []string{"keybind", "mouse", "copy", "paste", "link"}},
		{name: "Window", prefixes: []string{"window", "tab", "split", "fullscreen", "maximize", "quick-terminal", "gtk-", "macos-"}},
		{name: "Shell", prefixes: []string{"command", "initial-command", "working-directory", "shell-integration", "term", "env"}},
		{name: "Terminal", prefixes: []string{"scroll", "cursor", "bell", "clipboard", "image", "vt-", "unicode", "mouse-reporting", "adjust-cell-width"}},
	}
	for _, group := range groups {
		for _, prefix := range group.prefixes {
			if strings.HasPrefix(key, prefix) {
				return group.name
			}
		}
	}
	return "General"
}

func kindFor(key, defaultValue, context string) schema.ValueKind {
	if key == "keybind" {
		return schema.KindKeybind
	}
	if repeatableKey(key) {
		return schema.KindRepeatable
	}
	if key == "command" || key == "initial-command" {
		return schema.KindCommand
	}
	if strings.Contains(key, "path") || strings.HasSuffix(key, "-file") || key == "working-directory" {
		return schema.KindPath
	}
	if strings.Contains(key, "duration") || strings.Contains(key, "interval") || strings.Contains(key, "timeout") {
		return schema.KindDuration
	}
	if strings.Contains(key, "color") || key == "background" || key == "foreground" {
		return schema.KindColor
	}
	if defaultValue == "true" || defaultValue == "false" {
		return schema.KindBoolean
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(defaultValue), 64); err == nil && strings.TrimSpace(defaultValue) != "" {
		return schema.KindNumber
	}
	return schema.KindString
}

func editFor(key, defaultValue, context string) schema.EditMode {
	if key == "keybind" || repeatableKey(key) {
		return schema.EditRepeatable
	}
	return schema.EditScalar
}

func repeatableKey(key string) bool {
	switch key {
	case "font-family", "font-feature", "font-variation", "font-codepoint-map", "clipboard-codepoint-map",
		"palette", "env", "input", "key-remap", "command-palette-entry", "config-file", "custom-shader", "gtk-custom-css":
		return true
	default:
		return false
	}
}

func availabilityFor(context string) []string {
	text := strings.ToLower(context)
	availability := make([]string, 0, 2)
	for _, platform := range []string{"macOS", "GTK", "Linux", "Windows"} {
		if strings.Contains(text, strings.ToLower(platform)+" only") || strings.Contains(text, "only supported on "+strings.ToLower(platform)) {
			availability = append(availability, platform)
		}
	}
	return availability
}
