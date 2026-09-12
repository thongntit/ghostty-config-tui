package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ValueKind identifies the editor a schema option will eventually use.
type ValueKind string

const (
	KindString     ValueKind = "string"
	KindNumber     ValueKind = "number"
	KindColor      ValueKind = "color"
	KindBoolean    ValueKind = "boolean"
	KindEnum       ValueKind = "enum"
	KindPath       ValueKind = "path"
	KindDuration   ValueKind = "duration"
	KindRepeatable ValueKind = "repeatable"
	KindKeybind    ValueKind = "keybind"
	KindCommand    ValueKind = "command"
)

// EditMode describes the shape of the edit session exposed for an option.
type EditMode string

const (
	EditScalar             EditMode = "scalar"
	EditRepeatable         EditMode = "repeatable"
	EditReadOnlyRepeatable EditMode = "read-only"
)

// Option is the metadata contract consumed by the TUI. Raw values remain
// authoritative in configdoc; these fields describe how a value may be shown
// or edited when a codec is available.
type Option struct {
	Key          string    `json:"key"`
	Category     string    `json:"category"`
	Kind         ValueKind `json:"kind"`
	Description  string    `json:"description"`
	Edit         EditMode  `json:"edit"`
	Default      string    `json:"default,omitempty"`
	Introduced   string    `json:"introduced,omitempty"`
	Reload       string    `json:"reload,omitempty"`
	Context      []string  `json:"context,omitempty"`
	Availability []string  `json:"availability,omitempty"`
	Docs         string    `json:"docs,omitempty"`
	Values       []string  `json:"values,omitempty"`
	Multiple     bool      `json:"multiple,omitempty"`
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	Step         *float64  `json:"step,omitempty"`
}

// Catalog is the on-disk schema shape used by schema/options.json.
type Catalog struct {
	SchemaVersion  int      `json:"schema_version"`
	GhosttyVersion string   `json:"ghostty_version"`
	Source         string   `json:"source,omitempty"`
	SourceRevision string   `json:"source_revision,omitempty"`
	Options        []Option `json:"options"`
}

var durationComponentPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:ns|us|µs|ms|s|m|h|d|w|y)$`)
var durationSequencePattern = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]+)?(?:ns|us|µs|ms|s|m|h|d|w|y))+$`)

// Load decodes a reviewed schema catalog from JSON.
func Load(r io.Reader) (Catalog, error) {
	var catalog Catalog
	if err := json.NewDecoder(r).Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode schema: %w", err)
	}
	if catalog.SchemaVersion < 1 {
		return Catalog{}, fmt.Errorf("schema_version must be positive")
	}
	seen := make(map[string]struct{}, len(catalog.Options))
	for index, option := range catalog.Options {
		if strings.TrimSpace(option.Key) == "" {
			return Catalog{}, fmt.Errorf("option %d has an empty key", index)
		}
		if _, exists := seen[option.Key]; exists {
			return Catalog{}, fmt.Errorf("duplicate option key %q", option.Key)
		}
		seen[option.Key] = struct{}{}
	}
	return catalog, nil
}

// Validate checks a newly entered value. Existing values are deliberately not
// validated here: the editor must keep showing legacy or version-specific
// values even when it cannot interpret them.
func (o Option) Validate(value string) error {
	if value == "" {
		return fmt.Errorf("empty value: use the reset command to stage an explicit default")
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("value must not contain a line break or NUL")
	}

	if o.Multiple {
		return validateMultiple(value, o.Values)
	}
	if len(o.Values) > 0 {
		for _, allowed := range o.Values {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("value must be one of: %s", strings.Join(o.Values, ", "))
	}

	trimmed := strings.TrimSpace(value)
	switch o.Kind {
	case KindBoolean:
		if trimmed != "true" && trimmed != "false" {
			return fmt.Errorf("value must be true or false")
		}
	case KindNumber:
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("value must be a finite number")
		}
		if o.Min != nil && number < *o.Min {
			return fmt.Errorf("value must be at least %g", *o.Min)
		}
		if o.Max != nil && number > *o.Max {
			return fmt.Errorf("value must be at most %g", *o.Max)
		}
	case KindColor:
		if !validColor(trimmed) {
			return fmt.Errorf("value must be a hex color or a named color")
		}
	case KindPath:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("path must not be blank")
		}
	case KindDuration:
		if !validDuration(trimmed) {
			return fmt.Errorf("value must be a non-negative duration such as 250ms or 1s 200ms")
		}
	}
	return nil
}

func validateMultiple(value string, allowed []string) error {
	values := strings.Split(value, ",")
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			return fmt.Errorf("list values must not contain an empty item")
		}
		if candidate == "true" || candidate == "false" {
			continue
		}
		base := strings.TrimPrefix(candidate, "no-")
		found := false
		for _, allowedValue := range allowed {
			if base == allowedValue {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value %q must be one of: %s (optionally prefixed with no-)", candidate, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func validColor(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		if len(hex) != 6 {
			return false
		}
		for _, character := range hex {
			if !isHexDigit(character) {
				return false
			}
		}
		return true
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || unicode.IsSpace(character) || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func isHexDigit(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F'
}

func validDuration(value string) bool {
	if value == "0" {
		return true
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number >= 0 && !math.IsNaN(number) && !math.IsInf(number, 0)
	}
	if strings.ContainsAny(value, "\r\n") {
		return false
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return false
	}
	if len(parts) == 1 {
		return durationSequencePattern.MatchString(parts[0])
	}
	for _, part := range parts {
		if !durationComponentPattern.MatchString(part) {
			return false
		}
	}
	return true
}

// Editable reports whether the option can be changed by the current editor.
func (o Option) Editable() bool {
	return o.Edit != EditReadOnlyRepeatable
}

// Repeatable reports whether an option owns an ordered list of assignments.
// Keybindings use the same source representation even though they have a
// dedicated grammar.
func (o Option) Repeatable() bool {
	return o.Edit == EditRepeatable || o.Kind == KindRepeatable || o.Kind == KindKeybind
}

// WithAllEditors enables the catalog's generic editor for every known option.
// Scalar values use typed validation and repeatable/keybind values use an
// occurrence editor. Grammar-specific choices are still optional; users can
// always replace a value through the raw input field.
func (c Catalog) WithAllEditors() Catalog {
	clone := c
	clone.Options = append([]Option(nil), c.Options...)
	for index := range clone.Options {
		option := &clone.Options[index]
		if option.Kind == KindRepeatable || option.Kind == KindKeybind {
			option.Edit = EditRepeatable
		} else {
			option.Edit = EditScalar
		}
		if option.Key == "font-size" && option.Step == nil {
			step := 0.5
			option.Step = &step
		}
	}
	return clone
}

// WithBootstrapEditors returns the original four-option catalog overlay. It
// is retained for compatibility with the first vertical-slice tests; new
// application code should use WithAllEditors.
func (c Catalog) WithBootstrapEditors() Catalog {
	clone := c
	clone.Options = append([]Option(nil), c.Options...)
	for index := range clone.Options {
		option := &clone.Options[index]
		switch option.Key {
		case "theme":
			option.Kind = KindString
			option.Edit = EditScalar
		case "font-size":
			option.Kind = KindNumber
			option.Edit = EditScalar
			step := 0.5
			option.Step = &step
		case "background", "foreground":
			option.Kind = KindColor
			option.Edit = EditScalar
		}
	}
	return clone
}

// BootstrapOptions provides a small, known set of options for the shell UI.
// It is intentionally not presented as a complete Ghostty catalog.
func BootstrapOptions() []Option {
	return []Option{
		{Key: "theme", Category: "Appearance", Kind: KindString, Edit: EditScalar, Description: "Theme name or path to a custom theme file."},
		{Key: "font-family", Category: "Appearance", Kind: KindString, Edit: EditScalar, Description: "Font family used by Ghostty."},
		{Key: "font-size", Category: "Appearance", Kind: KindNumber, Edit: EditScalar, Description: "Font size in points."},
		{Key: "background", Category: "Appearance", Kind: KindColor, Edit: EditScalar, Description: "Terminal background color."},
		{Key: "foreground", Category: "Appearance", Kind: KindColor, Edit: EditScalar, Description: "Terminal foreground color."},
		{Key: "keybind", Category: "Input", Kind: KindKeybind, Edit: EditRepeatable, Description: "Keyboard shortcut and action pair."},
	}
}
