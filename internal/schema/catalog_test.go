package schema

import "testing"

func TestEmbeddedCatalogContainsFullGhosttySurface(t *testing.T) {
	catalog, err := EmbeddedCatalog()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	if len(catalog.Options) < 100 {
		t.Fatalf("embedded catalog has %d options; expected the full generated surface", len(catalog.Options))
	}
	if catalog.GhosttyVersion == "" || catalog.Source == "" {
		t.Fatalf("catalog is missing provenance: %+v", catalog)
	}
	keys := make(map[string]struct{}, len(catalog.Options))
	for _, option := range catalog.Options {
		keys[option.Key] = struct{}{}
	}
	for _, key := range []string{"theme", "font-family", "font-size", "background", "foreground", "keybind", "config-file"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("catalog is missing %q", key)
		}
	}
}
