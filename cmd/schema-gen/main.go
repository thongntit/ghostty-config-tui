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
			applyReviewedMetadata(&option)
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
	if pathKey(key) {
		return schema.KindPath
	}
	if durationKey(key) {
		return schema.KindDuration
	}
	if colorKey(key) {
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

func durationKey(key string) bool {
	return strings.Contains(key, "duration") || strings.Contains(key, "interval") || strings.Contains(key, "timeout") || key == "notify-on-command-finish-after" || key == "quit-after-last-window-closed-delay"
}

func pathKey(key string) bool {
	return strings.Contains(key, "path") || strings.HasSuffix(key, "-file") || key == "working-directory" || key == "background-image" || key == "macos-custom-icon"
}

func colorKey(key string) bool {
	switch key {
	case "background", "foreground", "selection-foreground", "selection-background", "cursor-text",
		"search-foreground", "search-background", "search-selected-foreground", "search-selected-background",
		"unfocused-split-fill", "window-titlebar-background", "window-titlebar-foreground":
		return true
	default:
		return strings.Contains(key, "color")
	}
}

// These values are deliberately maintained as reviewed metadata instead of
// being inferred from option names. Ghostty's docs are the source of truth for
// the option catalog, but the docs contain prose and examples that are not a
// lossless machine-readable enum format.
var reviewedEnumValues = map[string][]string{
	"alpha-blending":                        {"native", "linear", "linear-corrected"},
	"async-backend":                         {"auto", "epoll", "io_uring"},
	"auto-update":                           {"off", "check", "download"},
	"auto-update-channel":                   {"stable", "tip"},
	"background-image-fit":                  {"contain", "cover", "stretch", "none"},
	"background-image-position":             {"top-left", "top-center", "top-right", "center-left", "center", "center-right", "bottom-left", "bottom-center", "bottom-right"},
	"clipboard-read":                        {"ask", "allow", "deny"},
	"clipboard-write":                       {"ask", "allow", "deny"},
	"copy-on-select":                        {"none", "primary", "clipboard", "both", "true", "false"},
	"cursor-style":                          {"block", "bar", "underline", "block_hollow"},
	"cursor-style-blink":                    {"true", "false"},
	"drag-handle":                           {"always", "auto", "never"},
	"grapheme-width-method":                 {"legacy", "unicode"},
	"gtk-quick-terminal-layer":              {"overlay", "top", "bottom", "background"},
	"gtk-single-instance":                   {"detect", "true", "false", "desktop"},
	"gtk-tabs-location":                     {"top", "bottom", "hidden"},
	"gtk-titlebar-style":                    {"native", "tabs"},
	"gtk-toolbar-style":                     {"flat", "raised", "raised-border"},
	"linux-cgroup":                          {"never", "always", "single-instance"},
	"macos-dock-drop-behavior":              {"new-tab", "new-window"},
	"macos-hidden":                          {"never", "always"},
	"macos-icon":                            {"official", "blueprint", "chalkboard", "microchip", "glass", "holographic", "paper", "retro", "xray", "custom", "custom-style"},
	"macos-icon-frame":                      {"aluminum", "beige", "plastic", "chrome"},
	"macos-non-native-fullscreen":           {"true", "false", "visible-menu", "padded-notch"},
	"macos-option-as-alt":                   {"true", "false", "left", "right"},
	"macos-shortcuts":                       {"ask", "allow", "deny"},
	"macos-titlebar-proxy-icon":             {"visible", "hidden"},
	"macos-titlebar-style":                  {"native", "transparent", "tabs", "hidden"},
	"macos-window-buttons":                  {"visible", "hidden"},
	"middle-click-action":                   {"primary-paste", "clipboard-paste", "ignore"},
	"mouse-shift-capture":                   {"true", "false", "always", "never"},
	"notify-on-command-finish":              {"never", "unfocused", "always"},
	"osc-color-report-format":               {"none", "8-bit", "16-bit"},
	"quick-terminal-keyboard-interactivity": {"none", "on-demand", "exclusive"},
	"quick-terminal-position":               {"top", "bottom", "left", "right", "center"},
	"quick-terminal-screen":                 {"main", "mouse", "macos-menu-bar"},
	"quick-terminal-space-behavior":         {"move", "remain"},
	"resize-overlay":                        {"always", "never", "after-first"},
	"resize-overlay-position":               {"center", "top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right"},
	"right-click-action":                    {"context-menu", "paste", "copy", "copy-or-paste", "ignore"},
	"scrollbar":                             {"system", "never"},
	"shell-integration":                     {"none", "detect", "bash", "elvish", "fish", "nushell", "zsh"},
	"split-preserve-zoom":                   {"navigation", "no-navigation"},
	"window-colorspace":                     {"srgb", "display-p3"},
	"window-decoration":                     {"none", "auto", "client", "server", "true", "false"},
	"window-new-tab-position":               {"current", "end"},
	"window-padding-balance":                {"false", "true", "equal"},
	"window-padding-color":                  {"background", "extend", "extend-always"},
	"window-save-state":                     {"default", "never", "always"},
	"window-show-tab-bar":                   {"always", "auto", "never"},
	"window-subtitle":                       {"false", "working-directory"},
	"window-theme":                          {"auto", "system", "light", "dark", "ghostty"},
}

var reviewedMultipleValues = map[string][]string{
	"app-notifications":               {"clipboard-copy", "config-reload"},
	"bell-features":                   {"system", "audio", "attention", "title", "border"},
	"font-shaping-break":              {"cursor"},
	"font-synthetic-style":            {"bold", "italic", "bold-italic"},
	"freetype-load-flags":             {"hinting", "force-autohint", "monochrome", "autohint", "light"},
	"notify-on-command-finish-action": {"bell", "notify"},
	"scroll-to-bottom":                {"keystroke", "output"},
	"shell-integration-features":      {"cursor", "sudo", "title", "ssh-env", "ssh-terminfo", "path"},
}

func applyReviewedMetadata(option *schema.Option) {
	if values, ok := reviewedEnumValues[option.Key]; ok {
		option.Kind = schema.KindEnum
		option.Values = append([]string(nil), values...)
	}
	if values, ok := reviewedMultipleValues[option.Key]; ok {
		option.Kind = schema.KindString
		option.Multiple = true
		option.Values = append([]string(nil), values...)
	}
	switch option.Key {
	case "font-thicken-strength":
		min, max := 0.0, 255.0
		option.Min, option.Max = &min, &max
	case "minimum-contrast":
		min, max, step := 1.0, 21.0, 0.1
		option.Min, option.Max, option.Step = &min, &max, &step
	case "background-image-opacity", "background-opacity", "cursor-opacity", "bell-audio-volume", "faint-opacity":
		min, max, step := 0.0, 1.0, 0.05
		option.Min, option.Max, option.Step = &min, &max, &step
	case "unfocused-split-opacity":
		min, max, step := 0.15, 1.0, 0.05
		option.Min, option.Max, option.Step = &min, &max, &step
	}
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
