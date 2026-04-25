# Reference

> **Status:** scaffolding — full content lands in a follow-up PR.

Quick-lookup tables. For exhaustive detail see the [man page](../../sortie.1).

## Subcommands and flags

One-line summary per subcommand (`init`, `scan`, `watch`, `rules`, `history`, `undo`, `validate`, `status`, `trash`). Link to man page for full flag detail.

## Match conditions

Condensed table: condition → YAML key → value type → one-line description. Link into [Concepts › Matching](02-concepts.md#matching) for detail.

## Actions

Condensed table: action → reversible? → platform support → required external tool.

## Template variables

Cheat sheet: variable → produces → example.

## Config file locations

- **macOS / Linux:** `~/.config/sortie/config.yaml`
- **Windows:** `%APPDATA%\sortie\config.yaml`

Per-directory overrides: `.sortie.yaml` in any watched directory.

## Environment variables

If any are supported, listed here.
