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

Browse with `↑`/`k` and `↓`/`j`. Press `enter` or `e` to edit a supported scalar,
`r` to stage an explicit default reset, `u` to revert a staged option, and `p`
to preview the candidate document. `q` asks for confirmation when changes are
staged. Repeatable values such as `keybind` are visible but read-only for now.

The full discovered catalog supports read-only browsing/search and
effective-value preview. A single discovered or explicit root with no
`config-file` includes also exposes friendly dry-run editors for the initial
safe scalar options: `font-size` uses a half-point stepper, `background` and
`foreground` use a searchable color chooser with swatches, and `theme` uses
Ghostty's installed theme inventory when available. `ctrl+r` keeps an
explicit raw-value escape hatch; unknown existing values are preserved.
Multiple roots, includes, repeatable values, and special grammars remain
read-only until their source target can be selected safely. All modes keep
original and draft documents separate and do not invoke Ghostty or write any
file.

Architecture and implementation decisions are recorded in
[`docs/tech-stack.md`](docs/tech-stack.md).

The friendly-control roadmap is in
[`docs/plan-friendly-controls.md`](docs/plan-friendly-controls.md).
