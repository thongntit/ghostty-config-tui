# Technical direction

## Decision

Use Go 1.25+ with Bubble Tea v2, Bubbles v2, and Lip Gloss v2.

This gives the project a native, portable binary and a mature terminal UI
model while keeping the contributor toolchain small. The first release target
is macOS and Linux; Windows remains a build target until platform-specific
Ghostty behavior is available to test.

The application will use the Go standard library for parsing, metadata loading,
validation, hashing, backups, and atomic file writes. Additional frameworks
are intentionally deferred until the MVP proves they are needed.

## Architectural boundaries

- `internal/configdoc` owns a lossless ordered document model. It must preserve
  comments, unknown lines, repeated keys, empty values, line endings, and the
  original bytes of untouched nodes.
- `internal/schema` owns typed option metadata and value codecs. The embedded
  schema is versioned and is not treated as a replacement for Ghostty's own
  validator.
- `internal/ghostty` owns binary discovery, version detection, default-root
  discovery, dynamic catalog generation, and optional authoritative validation
  through the installed Ghostty binary.
- `internal/configgraph` owns recursive `config-file` loading, root/include
  precedence, cycle and missing-file diagnostics, and assignment provenance.
- `internal/storage` owns conflict detection, backups, permissions, symlink-safe
  writes, and atomic replacement.
- `internal/tui` owns interaction and rendering; it talks to the other packages
  through small interfaces so tests do not need a live Ghostty installation.

## Important product constraints

Ghostty configuration is not a simple key/value map. A key can repeat, values
can be empty, `#` is only a comment marker on its own line, and `config-file`
entries are resolved after the containing file. The graph editor loads all
default roots and includes while retaining each source document, rather than
silently flattening everything.

Saving is a guarded operation: render a candidate, validate it, detect
concurrent changes, create a backup, and atomically replace the target. The
application will not reload Ghostty automatically in the MVP.

## Research sources

- [Ghostty configuration](https://ghostty.org/docs/config)
- [Ghostty configuration reference](https://ghostty.org/docs/config/reference)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Bubbles v2 upgrade guide](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- [GoReleaser](https://www.goreleaser.com/getting-started/quick-start/)
