package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// ValueKind identifies the editor a schema option will eventually use.
type ValueKind string

const (
	KindString     ValueKind = "string"
	KindNumber     ValueKind = "number"
	KindColor      ValueKind = "color"
	KindBoolean    ValueKind = "boolean"
	KindEnum       ValueKind = "enum"
	KindRepeatable ValueKind = "repeatable"
)

// EditMode controls whether the MVP may stage a value for an option.
type EditMode string

const (
	EditScalar             EditMode = "scalar"
	EditReadOnlyRepeatable EditMode = "read-only"
)

// Option is the deliberately small metadata contract consumed by the first
// TUI screen. More constraints and codecs will be added without changing the
// config document model.
type Option struct {
	Key         string    `json:"key"`
	Category    string    `json:"category"`
	Kind        ValueKind `json:"kind"`
	Description string    `json:"description"`
	Edit        EditMode  `json:"edit"`
	Values      []string  `json:"values,omitempty"`
	Min         *float64  `json:"min,omitempty"`
	Max         *float64  `json:"max,omitempty"`
}

// Catalog is the on-disk schema shape used by schema/options.json.
type Catalog struct {
	SchemaVersion  int      `json:"schema_version"`
	GhosttyVersion string   `json:"ghostty_version"`
	Options        []Option `json:"options"`
}

// Load decodes a reviewed schema catalog from JSON.
func Load(r io.Reader) (Catalog, error) {
	var catalog Catalog
	if err := json.NewDecoder(r).Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode schema: %w", err)
	}
	if catalog.SchemaVersion < 1 {
		return Catalog{}, fmt.Errorf("schema_version must be positive")
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

	if len(o.Values) > 0 {
		for _, allowed := range o.Values {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("value must be one of: %s", strings.Join(o.Values, ", "))
	}

	if o.Kind == KindNumber {
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
	}
	return nil
}

// Editable reports whether the option can be changed in this MVP.
func (o Option) Editable() bool {
	return o.Edit != EditReadOnlyRepeatable && o.Kind != KindRepeatable
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
		{Key: "keybind", Category: "Input", Kind: KindRepeatable, Edit: EditReadOnlyRepeatable, Description: "Keyboard shortcut and action pair."},
	}
}
