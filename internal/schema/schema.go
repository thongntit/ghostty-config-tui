package schema

import (
	"encoding/json"
	"fmt"
	"io"
)

// ValueKind identifies the editor a schema option will eventually use.
type ValueKind string

const (
	KindString     ValueKind = "string"
	KindNumber     ValueKind = "number"
	KindColor      ValueKind = "color"
	KindRepeatable ValueKind = "repeatable"
)

// Option is the deliberately small metadata contract consumed by the first
// TUI screen. More constraints and codecs will be added without changing the
// config document model.
type Option struct {
	Key         string    `json:"key"`
	Category    string    `json:"category"`
	Kind        ValueKind `json:"kind"`
	Description string    `json:"description"`
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

// BootstrapOptions provides a small, known set of options for the shell UI.
// It is intentionally not presented as a complete Ghostty catalog.
func BootstrapOptions() []Option {
	return []Option{
		{Key: "theme", Category: "Appearance", Kind: KindString, Description: "Theme name or path to a custom theme file."},
		{Key: "font-family", Category: "Appearance", Kind: KindString, Description: "Font family used by Ghostty."},
		{Key: "font-size", Category: "Appearance", Kind: KindNumber, Description: "Font size in points."},
		{Key: "background", Category: "Appearance", Kind: KindColor, Description: "Terminal background color."},
		{Key: "foreground", Category: "Appearance", Kind: KindColor, Description: "Terminal foreground color."},
		{Key: "keybind", Category: "Input", Kind: KindRepeatable, Description: "Keyboard shortcut and action pair."},
	}
}
