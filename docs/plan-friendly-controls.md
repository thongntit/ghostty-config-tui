# Plan: friendly Ghostty option controls

Status: phase 1 implemented in the single-file dry-run boundary
Date: 2026-09-12

## Product decision

Users should choose values through controls that match the option grammar. The
raw Ghostty value remains visible and available as an explicit escape hatch,
but it is not the default interaction for supported options.

Editors must be selected from explicit metadata or an authoritative provider.
The option name and description are not enough evidence to invent a control,
range, unit, choice list, or serializer.

## Current slice

Single-file configs without `config-file` includes expose these dry-run
controls:

- `font-size`: a metadata-declared half-point numeric stepper; `ctrl+r` opens
  raw numeric input.
- `background` and `foreground`: searchable named-color chooser with RGB
  values and a terminal swatch. Ghostty's `+list-colors` action supplies the
  inventory when available; a small fallback palette remains usable without
  Ghostty.
- `theme`: searchable chooser backed by Ghostty's `+list-themes` action when
  the installed binary is available. Unknown or custom values can be replaced
  through `ctrl+r` raw input.

Browse labels use human-readable names while retaining the raw key in the
details pane. The existing draft/preview boundary remains unchanged: controls
stage changes in memory and never write or reload Ghostty.

## Keyboard contract

- Browse: `↑/k`, `↓/j`, `/` search, `enter/e` edit, `r` reset, `u` revert,
  `p` preview, `q` quit.
- Choice editor: type to filter, `↑/↓` or `←/→` choose, `enter` select,
  `ctrl+r` raw value, `esc` cancel.
- Numeric editor: `←/→` adjust by the declared step, `ctrl+r` raw value,
  `enter` stage, `esc` cancel.
- The footer describes the valid keys for the active control.

## Next phases

1. Add a control-neutral typed edit contract that separates effective value,
   target-local value, documented default, unset-local, validation, and exact
   serialization.
2. Add reviewed grammar metadata and safe scalar controls for booleans,
   closed enums, durations, sizes, paths, and free strings. Unknown existing
   values stay byte-preserved until the user explicitly replaces them.
3. Add explicit source-target selection for multi-file graphs. Show whether a
   change edits the owning assignment or creates a root override, and simulate
   the resulting effective value before review.
4. Add repeatable/list editors, keybinding forms, map/structured editors, and
   an advanced raw editor only where occurrence and serialization semantics are
   known.
5. Add guarded persistence only after preview can report target files,
   fingerprints, exact patches, and rollback behavior.

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
