# Plan: integrate the full Ghostty configuration surface

Status: implemented for the current pinned catalog
Date: 2026-09-12

Current milestone: the pinned catalog, graph-aware source editing, repeatable
editors, Ghostty validation for single-file candidates, and guarded persistence
are implemented. Future work is limited to richer grammar-specific controls
and broader candidate validation for graphs with external includes.

## 1. Goal and scope

“Full config” means three things shipped together, in stages:

1. Every configuration key documented by a pinned Ghostty version is visible
   in the catalog, including platform/version availability, defaults,
   deprecations, aliases, multiplicity, and reload behavior.
2. The app can explain the effective configuration across default files and
   `config-file` includes, while retaining each source file and line.
3. Each value gets an editor appropriate to its grammar: scalar values,
   lists, maps, paths, durations, colors, commands, keybindings, and future
   complex types.

The app must remain useful when Ghostty is not installed. A reviewed embedded
catalog is the fallback; an installed Ghostty binary is the authority for
version detection and optional validation. Unknown or newer keys must remain
visible and round-trip safely.

Browsing and preview remain non-mutating. Persistence happens only after an
explicit save command or quit confirmation; it creates a `.bak` backup,
checks source fingerprints, and atomically replaces changed source files.
The app still does not create missing config roots or automatically reload
Ghostty.

## 2. Facts to preserve from Ghostty

These are external contracts, not guesses to encode in the UI:

- Config keys are case-sensitive, lower-case, and also map to Ghostty CLI
  flags. The official reference is the complete option list.
- The syntax is line-oriented `key = value`; comments are whole lines, empty
  values reset an option, and values may be quoted or unquoted.
- Default config files are loaded in order. On macOS, XDG files are followed
  by Application Support files, and later values override earlier values.
- `config-file` is repeatable and recursive. Its paths are relative to the
  containing file, optional paths begin with `?`, and included files are
  processed after the containing file. Cycles are ignored with a warning.
- `keybind` is not an ordinary string: it has trigger/action syntax, prefixes,
  sequences, key tables, duplicate-trigger replacement, and a separate action
  vocabulary.
- `+show-config --default --docs` is a documentation source, not a config
  template to paste into a user's file. Some options are intentionally
  platform-specific or only take effect for new windows/restarts.

Sources:

- [Ghostty configuration overview](https://ghostty.org/docs/config)
- [Ghostty option reference](https://ghostty.org/docs/config/reference)
- [Ghostty keybinding guide](https://ghostty.org/docs/config/keybind)
- [Ghostty Config.zig](https://github.com/ghostty-org/ghostty/blob/main/src/config/Config.zig)
- [Ghostty config template warning](https://github.com/ghostty-org/ghostty/blob/main/src/config/config-template)

## 3. Target architecture

### `internal/schema`

Replace the six-option bootstrap list with a versioned catalog and typed
value codecs. Keep the existing lossless document model independent from the
catalog.

Each option should carry at least:

- key, display name, category, description, examples, and documentation link;
- value type and multiplicity (`scalar`, `repeatable`, list/map, or special
  grammar);
- default/unset representation and reset behavior;
- constraints and a parser/formatter codec;
- aliases, deprecated status, and the Ghostty version that introduced it;
- platform/build availability and whether it is file-only, CLI-only, or
  valid in a theme file;
- reload scope: live, new surface/window, or restart required;
- provenance: Ghostty version, source revision, generator, and overlay
  revision.

Keep raw text and typed values separate. A value the current catalog cannot
interpret must still be displayed and preserved, but editing it should require
an explicit raw-value path.

### `schema-gen` and catalog provenance

Use a two-layer catalog:

1. A generated base extracted from an explicit Ghostty version. The generator
   should support an installed binary through `+version` and
   `+show-config --default --docs`, with checked-in command-output fixtures for
   reproducibility.
2. A reviewed overlay for information that cannot be reliably inferred from
   text output, such as grouping, nuanced constraints, aliases, platform
   notes, reload behavior, and custom codecs.

Do not scrape the website at runtime and do not make application startup
depend on network access. Store generated catalogs under a versioned path,
for example `schema/generated/<version>.json`, and embed a reviewed catalog
with `go:embed`.

`schema-gen` should provide:

- `generate`: emit a candidate catalog and provenance report;
- `check`: fail when generated output is stale, duplicated, malformed, or
  missing an option from the source fixture;
- `diff`: compare two Ghostty versions and classify additions, removals,
  aliases, and changed constraints.

The generator must fail closed on an unrecognized output shape. A human review
is required before a new catalog becomes the runtime default. CI should pin
the Ghostty source/output fixture so a moving `main` branch cannot silently
change the application.

### `internal/ghostty` and `internal/configgraph`

Keep path discovery and binary probing in `internal/ghostty`. Add a graph
package rather than flattening files into one synthetic document:

```text
default root 1 ─┐
default root 2 ─┼─> ConfigGraph ─> Effective assignments
include A ──────┘             └─> diagnostics and source locations
```

The graph should contain one lossless `configdoc.Document` per file, include
edges, canonical identity for cycle detection, original path spelling, and
file/line spans for every assignment. The effective stream must follow
Ghostty's ordering rules: local assignments first, then that file's includes
in declaration order, recursively.

Discovery behavior should change from selecting only the last default file to
loading every existing default root in documented order. Explicit `--config`
still selects one root and its recursive includes. The UI must show which
roots and includes contributed to the result.

Graph safety limits:

- reject or diagnose unreadable/non-regular files;
- detect cycles using canonical paths while preserving the displayed path;
- distinguish required missing includes from optional `?` includes;
- resolve relative includes against their containing file;
- bound include depth, number of files, and total bytes;
- never execute commands or evaluate config values while inspecting a graph.

Extend `configdoc` only where needed for Ghostty semantics: quoted values,
include-path decoding, source spans, and exact round-trip rendering. Keep
untouched lines byte-for-byte stable.

### `internal/validation`

Split validation into three layers:

1. Structural checks: syntax, unsafe bytes, include graph, duplicate rules,
   and path diagnostics.
2. Catalog checks: typed codecs, enum/range constraints, platform/version
   warnings, and cross-option constraints.
3. Optional Ghostty-authoritative checks: render a candidate graph in a
   temporary directory and invoke the installed binary's validation action
   through `exec.Command` (never a shell). If the binary is absent or its
   version is unsupported, report “not validated by Ghostty” instead of
   claiming success.

### `internal/storage`

The existing hashing/storage seam becomes the final write boundary. It should
own conflict detection, backup, permissions, symlink policy, and atomic
replacement. A graph edit that changes multiple files must be an all-or-none
transaction, or remain preview-only until a safe rollback strategy exists.

### Implementation map

| Area | Existing surface | Planned additions |
| --- | --- | --- |
| Catalog | `internal/schema`, `schema/`, `cmd/schema-gen` | generated versioned catalogs, overlays, codecs, embed/load and freshness checks |
| Files | `internal/ghostty/paths.go`, `internal/configdoc` | all default roots, quoted/include decoding, source spans and graph loading |
| Effective state | none | `internal/configgraph` with precedence, diagnostics and assignment provenance |
| Validation | `internal/ghostty/validator.go` | structural, catalog and optional binary-backed validation pipeline |
| UI | `internal/tui` | search, source/effective views, typed editors, keybind/repeatable editors and multi-file preview |
| Persistence | `internal/storage` | guarded save transaction, backup, atomic replacement and recovery tests |

Keep each stage compilable and usable. The catalog and graph stages should
land before changing the save boundary; typed editors can then be added one
codec at a time.

## 4. TUI roadmap

### Phase A — complete catalog (complete)

- Replace the bootstrap options with the embedded catalog.
- Add category navigation, fuzzy search, and a details pane.
- Show configured value, default/unset state, codec type, source file/line,
  availability, and reload scope.
- Put unknown keys and duplicate assignments in an explicit “custom / needs
  review” section.

Acceptance: every key in the pinned catalog is searchable; no existing key,
comment, duplicate, or unknown line is lost; a user can identify the source
file for every displayed value.

### Phase B — effective configuration graph (complete)

- Load all default roots and recursive includes.
- Add a file graph/precedence view with required/optional/cycle diagnostics.
- Let the user switch between “effective value” and “source file” views.
- Keep preview non-mutating while allowing the same graph to provide save
  targets for the explicit persistence boundary.

Acceptance: fixtures demonstrate root precedence, include ordering, relative
paths, optional missing files, cycles, and source locations exactly.

### Phase C — typed scalar editors (complete)

Implement codecs and editors in increasing risk order:

1. booleans, enums, colors, numbers, opacity, durations, and sizes;
2. strings, paths, fonts, themes, and shell commands;
3. dependent settings such as paired dimensions or related window behavior.

Every editor must support reset-to-default, show raw/current/effective values,
validate before staging, and preserve the original source line until the
candidate is rendered.

Acceptance: editing a typed option changes only its intended assignment in the
candidate preview, including comments, spacing, line endings, and unrelated
files.

### Phase D — repeatable and special grammars (base complete)

- Build a dedicated keybinding table editor for trigger, prefixes, key table,
  action, and action arguments.
- Source available actions and default bindings from the installed Ghostty
  binary when possible; keep a versioned fallback catalog.
- Add repeatable font families, palettes, links, notifications, environment
  entries, and other list/map options using option-specific codecs.
- Provide a raw-value escape hatch for complex or newly introduced syntax.

Acceptance: duplicate-trigger replacement, unbind/ignore actions, key tables,
repeatable values, quoted values, and unknown action arguments have golden
fixtures and safe round trips.

### Phase E — graph-aware editing and preview (complete)

- When a key exists in several files, show the effective source and edit that
  source; repeatable options expose every occurrence and its source line.
- Preview a per-file diff plus an effective-value summary and diagnostics.
- Make generated defaults read-only; never suggest copying the full
  `+show-config` output into a user config.

Acceptance: a preview makes it impossible to mistake a source-file edit for an
effective-value change, and no include is silently flattened.

### Phase F — guarded persistence (complete)

- Add an explicit save mode separate from preview mode.
- Re-read and hash every target immediately before writing; abort on conflict.
- Create a recoverable backup, preserve mode/permissions, and atomically
  replace files in their original directories.
- Refuse symlink writes by default or require a clearly displayed target
  confirmation; never replace an unexpected target.
- For multi-file saves, stage all outputs, validate them, then commit with a
  rollback path. Do not reload Ghostty automatically.

Acceptance: crash/interruption, concurrent edit, permission, symlink, and
partial multi-file failure tests prove that the original configuration can be
recovered.

## 5. Version and platform behavior

- Detect the installed Ghostty binary and version without requiring it for
  startup.
- Select the newest catalog known to be compatible with that version; mark
  newer options as “catalog newer than installed Ghostty” and older removed
  options as legacy rather than deleting them.
- Filter platform-only options in the default view, but allow a user to
  inspect/edit them in a cross-platform config with a visible warning.
- Treat CLI-only settings such as default-file control as a separate context;
  do not pretend that every CLI flag belongs in a config file.
- Add a “catalog source” indicator so users know whether validation is
  embedded, binary-backed, or unavailable.

## 6. Tests and CI

### Parser and graph

- Golden round trips for LF/CRLF, no final newline, spacing, quotes, empty
  resets, comments, unknown keys, repeated keys, and values containing `=` or
  `#`.
- Include fixtures for relative/absolute paths, optional paths, nested
  includes, cycles, duplicate roots, symlinks, missing files, and limits.
- Property/fuzz tests must guarantee no panic and stable rendering for
  arbitrary UTF-8 input within the size limit.

### Schema

- Generator snapshots for at least two pinned Ghostty versions.
- Coverage check: every source option appears once in the generated catalog;
  overlay keys must refer to a known option or an intentional compatibility
  entry.
- Codec tables for valid, invalid, reset, boundary, platform, and deprecated
  values.

### TUI and integration

- Model tests for search, category navigation, source selection, effective
  values, raw fallback, staged reset, graph diagnostics, and preview.
- Terminal snapshot tests for narrow and wide layouts.
- Optional integration tests run only when a Ghostty binary is available;
  otherwise use captured command output and deterministic fixtures.
- CI continues to run `go test -race ./...`, `go vet ./...`, `go build ./...`,
  `git diff --check`, and catalog freshness checks.

## 7. Risks and non-goals

Risks:

- Ghostty's source and option grammar evolve faster than a hand-maintained
  catalog.
- Documentation text is rich but not a stable machine-readable schema.
- Some values have platform/runtime behavior that cannot be validated from a
  static config file.
- Multi-file writes and symlinked dotfiles are substantially riskier than the
  current single-file preview.
- A full keybind editor can accidentally change semantics if it normalizes
  triggers or escape sequences too aggressively.

Non-goals for the first full-config release:

- Implementing a complete Ghostty runtime or reproducing every OS/window
  manager behavior in Go.
- Fetching remote docs during startup.
- Executing configured commands, opening arbitrary paths, or auto-reloading
  Ghostty.
- Generating a giant default config as a replacement for Ghostty's built-in
  defaults.
- Supporting save before graph/source attribution and authoritative
  validation are reliable.

## 8. Delivered implementation slice

The initial vertical slice has now landed:

1. Make `schema-gen` produce a reviewed, versioned catalog from a pinned
   Ghostty command-output fixture, with an overlay and provenance.
2. Embed that catalog and replace `BootstrapOptions()` with the complete
   editable catalog.
3. Add `internal/configgraph` for default roots, recursive includes, source
   locations, precedence, and diagnostics.
4. Upgrade the TUI to search/browse all options, edit graph sources and
   repeatable occurrences, preview per-file changes, and save through the
   guarded storage boundary.

The remaining work can now be added incrementally without changing the lossless
file model or the guarded storage boundary: richer grammar-specific forms,
explicit non-effective source selection, and authoritative validation for
staged graphs with external includes.
