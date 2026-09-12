package schema

import (
	"bytes"
	_ "embed"
)

// optionsJSON is generated from a pinned Ghostty release and reviewed before
// it becomes the runtime catalog.
//
//go:embed options.json
var optionsJSON []byte

// EmbeddedCatalog loads the reviewed catalog shipped with the application.
func EmbeddedCatalog() (Catalog, error) {
	return Load(bytes.NewReader(optionsJSON))
}
