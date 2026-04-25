# Concepts

> **Status:** scaffolding — full content lands in a follow-up PR.

The mental model behind sortie. Read this once; refer back to it when a recipe confuses you.

## The pipeline

ASCII diagram: file event → match → action → history. One paragraph each on the three stages.

## Rules

Priority ordering, first-match-wins semantics, `continue: true`, global rules (`config.yaml`) vs. per-directory rules (`.sortie.yaml`), the `--global` flag.

## Matching

Walkthrough of every match condition: `extensions`, `glob`, `regex`, `size`, `age`, `mime_type`, `content`, `content_regex`. Includes how content detection handles PDFs and how named capture groups expose `{{.Match.name}}` to templates.

## Actions and chains

Single-action rules vs. `actions: [...]` chains. Failure behavior, partial rollback, which actions are reversible.

## Dry-run, history, undo

Safety net: what `sortie scan --dry-run` does, where history lives, what `sortie undo` can and can't reverse. Reversible actions vs. non-reversible (`open`, `exec`, `notify`, `unquarantine`).

## Watch vs. scan

One-shot (`scan`) vs. daemon (`watch`). Debounce, rate limits, the `watch_existing` option. Brief note on which fsnotify backend is used on each platform.

## Templates

All available template variables with examples: `{{.Name}}`, `{{.Ext}}`, `{{.Path}}`, `{{.Dir}}`, `{{.Dest}}`, `{{.Size}}`, `{{.Date}}`, `{{.Time}}`, `{{.Match.<capture>}}`. Pattern for fallbacks: `{{or .Match.company "Unknown"}}`.
