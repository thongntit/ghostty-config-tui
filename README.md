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

The project is in the bootstrap phase. Architecture and implementation details
will be recorded as the first technical decision is made.
