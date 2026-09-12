# Plan: friendly Ghostty option controls

Status: full catalog editing and picker-based controls implemented
Date: 2026-09-12

## Product decision

Users should choose values through controls that match the option grammar. The
raw Ghostty value remains visible and available as an explicit escape hatch,
but it is not the default interaction for supported options.

Editors must be selected from explicit metadata or an authoritative provider.
The option name and description are not enough evidence to invent a control,
range, unit, choice list, or serializer.

## Current slice

The editor now exposes controls for every known catalog option across all
loaded roots and `config-file` includes:

- `font-size`: a metadata-declared half-point numeric stepper; `ctrl+r` opens
  raw numeric input.
- `background` and `foreground`: searchable named-color chooser with RGB
  values and a terminal swatch. Ghostty's `+list-colors` action supplies the
  inventory when available; a small fallback palette remains usable without
  Ghostty.
- `theme`: searchable chooser backed by Ghostty's `+list-themes` action when
  the installed binary is available. Unknown or custom values can be replaced
  through `ctrl+r` raw input.
- documented closed enums: searchable pickers generated from reviewed
  Ghostty metadata rather than inferred from option names.
- multi-value flags such as shell integration, bell features, font shaping,
  and notifications: each token has `default`, `enabled`, and `disabled`
  states, with `space` or `←/→` cycling and a raw fallback.
- durations: an amount field plus a unit picker for the supported Ghostty
  units; raw compound durations remain available through `ctrl+r`.
- paths: a directory browser for working directories and file paths, with a
  raw fallback for paths that are not present locally.
- fonts and actions: Ghostty's `+list-fonts` and `+list-actions` inventories
  power font-family, keybind, and command-palette pickers.
- structured repeatable forms: environment variables, font variations,
  codepoint maps, palette entries, keybindings, and command-palette entries
  are edited as fields instead of one opaque line.
- repeatable values and keybindings: occurrence list editors with add, edit,
  delete, clear, and source-file/line provenance.
- every other scalar grammar: a validated raw-value editor remains available
  for inherently open-ended values such as shell commands, titles, font
  feature expressions, and custom shader details.

Browse labels use human-readable names while retaining the raw key in the
details pane. Controls stage changes in memory. `ctrl+s` or quit confirmation
persists the candidate through conflict detection, backup, and atomic
replacement.

## Keyboard contract

- Browse: `↑/k`, `↓/j`, `/` search, `enter/e` edit, `r` reset, `u` revert,
  `p` preview, `q` quit.
- Choice editor: type to filter, `↑/↓` or `←/→` choose, `enter` select,
  `ctrl+r` raw value, `esc` cancel.
- Multi-select editor: `↑/↓` select a token, `space` or `←/→` cycle
  `default/enabled/disabled`, `enter` stage, `ctrl+r` raw value.
- Duration editor: type an amount, `←/→` choose a unit, `↑/↓` adjust the
  amount, `enter` stage, `ctrl+r` raw duration.
- Path editor: `↑/↓` browse, `enter` open/select, `s` select the current
  folder, `ctrl+r` raw path.
- Repeatable editor: `a` add, `e` edit, `d` delete, `r` clear, `enter` stage,
  `esc` cancel. Supported pairs and keybindings open field forms; `tab` or
  `enter` advances fields and action fields use a picker.
- Numeric editor: `←/→` adjust by the declared step, `ctrl+r` raw value,
  `enter` stage, `esc` cancel.
- The footer describes the valid keys for the active control.

## Next phases

1. Add a control-neutral typed edit contract that separates effective value,
   target-local value, documented default, unset-local, validation, and exact
   serialization.
2. Add explicit source-target selection for multi-file graphs when users want
   to edit a non-effective duplicate instead of the effective assignment.
3. Extend authoritative Ghostty validation to staged graphs with includes.

## Acceptance criteria

- A supported single-file edit does not require Ghostty assignment syntax.
- Effective, source, target-local, and documented-default values remain
  distinct in the UI.
- Invalid existing values remain byte-preserved and can be explicitly
  replaced.
- Choice inventories are authoritative or clearly labeled as fallback
  presets; no option list is fabricated from a key name.
- Controls work with keyboard input at 80x24 and larger, without relying on
  color alone for meaning.
- Preview shows the exact candidate change and still performs no file write.
