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

func TestWithBootstrapEditorsDeclaresFontSizeStep(t *testing.T) {
	catalog := Catalog{Options: []Option{{Key: "font-size", Kind: KindNumber, Edit: EditReadOnlyRepeatable}}}
	updated := catalog.WithBootstrapEditors()
	option := updated.Options[0]
	if option.Edit != EditScalar || option.Step == nil || *option.Step != 0.5 {
		t.Fatalf("font-size editor metadata = %+v", option)
	}
}
