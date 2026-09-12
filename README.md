# Ghostty Config TUI

A terminal user interface for configuring [Ghostty](https://ghostty.org/)
without requiring users to memorize the raw configuration syntax.

## Project goal

Make Ghostty configuration discoverable, safe, and approachable while keeping
the user's existing config intact.

## MVP direction

- Load the user's Ghostty config from its standard location.
- Browse and search settings by category.
- Edit values with controls appropriate to their type.
- Validate values before saving.
- Preview the resulting diff.
- Preserve comments and options the application does not yet understand.
- Create a backup before writing changes.

## Try the dry-run editor

The current MVP requires an existing config path and never writes to it:

```sh
go run ./cmd/ghostty-config-tui --config testdata/configs/editor-basic.ghostty
```

Browse with `↑`/`k` and `↓`/`j`. Press `enter` or `e` to edit a supported scalar,
`r` to stage an explicit default reset, `u` to revert a staged option, and `p`
to preview the candidate document. `q` asks for confirmation when changes are
staged. Repeatable values such as `keybind` are visible but read-only for now.

The editor loads one explicit file, keeps the original and draft documents
separate, and previews exact candidate bytes without invoking Ghostty or
writing any file.

Architecture and implementation decisions are recorded in
[`docs/tech-stack.md`](docs/tech-stack.md).
