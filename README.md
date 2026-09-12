# Ghostty Config TUI

A terminal user interface for configuring [Ghostty](https://ghostty.org/)
without requiring users to memorize the raw configuration syntax.

## Project goal

Make Ghostty configuration discoverable, safe, and approachable while keeping
the user's existing config intact.

## Current behavior

- Load the user's Ghostty config from its standard location.
- Browse and search settings by category.
- Edit values with controls appropriate to their type.
- Validate values before saving.
- Preview the resulting diff.
- Preserve comments and options the application does not yet understand.
- Create a backup before writing changes.

## Run the editor

With no arguments, the app reads all existing default Ghostty config files,
follows their `config-file` includes, and shows the effective values with
source locations:

```sh
go run ./cmd/ghostty-config-tui
```

Use `--config PATH` when you want to select a specific existing file instead:

```sh
go run ./cmd/ghostty-config-tui --config testdata/configs/editor-basic.ghostty
```

The default root lookup order follows [Ghostty's configuration
docs](https://ghostty.org/docs/config):

1. `$XDG_CONFIG_HOME/ghostty/config.ghostty`
2. `$XDG_CONFIG_HOME/ghostty/config`
3. On macOS, `~/Library/Application Support/com.mitchellh.ghostty/config.ghostty`
4. On macOS, `~/Library/Application Support/com.mitchellh.ghostty/config`

When `XDG_CONFIG_HOME` is unset, `$HOME/.config` is used. The legacy
`config` filename remains supported. The graph preserves root and include
precedence, reports missing optional/required files and cycles, and never
flattens source documents into a file to write. If no root file exists, it
reports the checked paths and suggests `--config PATH`; it does not create a
config file.

Browse with `↑`/`k` and `↓`/`j`. Press `enter` or `e` to edit any catalog option,
`r` to stage a default reset, `u` to revert a staged option, `p` to preview,
and `ctrl+s` to save. Press `q`, then `y`, to save and quit; `d` discards the
staged changes and quits.

Scalar options use typed validation, boolean toggles, numeric steppers,
documented enum pickers, multi-select lists, duration/unit pickers, path
browsers, and friendly theme/color/font choosers where an inventory is
available. Repeatable options such as `font-family`, `palette`, `env`,
`keybind`, and `config-file` open a list editor where each occurrence can be
added, changed, or removed. Pair settings use forms for keys and values;
keybindings and command-palette entries expose trigger/action fields. Press
`ctrl+r` inside any friendly control when you need the raw Ghostty value for a
custom or version-specific case.
For duplicate scalar assignments, the editor targets the effective source
line and displays its file and line number. New values are added to the
selected root when no source assignment exists.

The editor keeps one draft per root/include file. Preview shows the exact
per-file candidate changes and the resulting effective graph. Saving checks
that every source is unchanged, writes a `.bak` backup, and atomically replaces
all changed files; Ghostty's own validator is also run for a single-file
candidate when the Ghostty binary is available. Loading never creates or
writes a file.

Architecture and implementation decisions are recorded in
[`docs/tech-stack.md`](docs/tech-stack.md).

The friendly-control roadmap is in
[`docs/plan-friendly-controls.md`](docs/plan-friendly-controls.md).
