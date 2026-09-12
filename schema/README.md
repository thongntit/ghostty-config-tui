# Option schema

This directory contains the versioned option metadata used by the TUI.

The runtime schema must remain conservative: it can improve labels, grouping,
and editors, but the installed Ghostty binary remains authoritative when it is
available. Unknown configuration keys must stay editable and must survive a
round trip even when they are not present in this schema.

`cmd/schema-gen` imports documentation from a pinned Ghostty executable using
`ghostty +show-config --default --docs`. The generated catalog is reviewed and
copied into `internal/schema/options.json`, where it is embedded into release
builds. The generator intentionally emits read-only options until a dedicated
typed codec has been reviewed; catalog metadata is not a promise that every
Ghostty grammar is editable yet.

Regenerate both catalog copies on a machine with Ghostty installed:

```sh
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out schema/options.json
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out internal/schema/options.json
# After generation, check a pinned output without overwriting it:
go run ./cmd/schema-gen --ghostty /path/to/ghostty --out schema/options.json --check
```

Do not redirect the full `+show-config --default --docs` output into a user
config file. It is documentation for the catalog, not a minimal config.
