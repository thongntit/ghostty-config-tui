# Option schema

This directory will contain the versioned option metadata used by the TUI.

The runtime schema must remain conservative: it can improve labels, grouping,
and editors, but the installed Ghostty binary remains authoritative when it is
available. Unknown configuration keys must stay editable and must survive a
round trip even when they are not present in this schema.

The future `schema-gen` command may import documentation from
`ghostty +show-config --default --docs`; generated data must be reviewed before
it becomes a runtime schema.
