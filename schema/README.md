# Option schema

This directory contains the versioned option metadata used by the TUI.

The runtime schema must remain conservative: it can improve labels, grouping,
and editors, but the installed Ghostty binary remains authoritative when it is
available. Unknown configuration keys must stay editable and must survive a
round trip even when they are not present in this schema.

`cmd/schema-gen` imports documentation from a pinned Ghostty executable using
`ghostty +show-config --default --docs`. The generated catalog is reviewed and
copied into `internal/schema/options.json`, where it is embedded into release
builds. Every generated option is exposed to the editor: scalar options use
typed validation plus a raw-value input, while repeatable and keybinding
options use an occurrence list editor. Friendly controls are layered on top
when the catalog has enough metadata or Ghostty can provide an inventory;
unknown keys remain editable as raw one-line values.

Regenerate both catalog copies on a machine with Ghostty installed:

```sh
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out schema/options.json
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out internal/schema/options.json
# After generation, check a pinned output without overwriting it:
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out schema/options.json --check
```

Do not redirect the full `+show-config --default --docs` output into a user
config file. It is documentation for the catalog, not a minimal config.
