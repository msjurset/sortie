# Reference

Quick-lookup tables. For exhaustive flag documentation, use `sortie <subcommand> --help` or read the [man page](../../sortie.1).

## Subcommands

| Subcommand | Purpose |
|------------|---------|
| `sortie config` | Show resolved configuration |
| `sortie config init` | Write a starter `config.yaml` |
| `sortie config path` | Print the absolute config file path |
| `sortie actions` | List every action type with a short description |
| `sortie actions <name>` | Detailed help for one action type |
| `sortie rules [dir...]` | List configured rules (optionally filtered by directory) |
| `sortie rules test <file>` | Show which rule matches a given file |
| `sortie scan [path...]` | One-shot scan of directories or files |
| `sortie watch` | Run the watch daemon |
| `sortie status` | Show daemon status and recent activity |
| `sortie validate [dir...]` | Check rule YAML for errors and suspicious patterns |
| `sortie history` | Show the most recent dispatch records |
| `sortie undo [id]` | Reverse one or more recorded actions |
| `sortie trash` | Show trashed files |
| `sortie trash purge` | Empty the trash directory |
| `sortie man` | Print the roff-formatted man page to stdout |

## Global flags

These are accepted by every subcommand.

| Flag | Default | Purpose |
|------|---------|---------|
| `--config <path>` | `~/.config/sortie/config.yaml` | Use an alternate config file |
| `--log-format <fmt>` | `text` | Log output format: `text` or `json` |
| `-v`, `--verbose` | — | Include debug-level detail in logs |
| `-h`, `--help` | — | Per-subcommand help |

sortie does **not** read any environment variables — all configuration happens via the YAML file or CLI flags.

## Per-subcommand flags

Only the ones you're likely to reach for. See `sortie <cmd> --help` for the rest.

| Subcommand | Flag | Default | Purpose |
|------------|------|---------|---------|
| `scan` | `--dry-run` | `false` | Evaluate matches without executing actions |
| `scan`, `watch` | `--rate-limit <dur>` | `0` | Minimum interval between dispatches (e.g. `500ms`, `1s`) |
| `watch` | `--dry-run` | `false` | Log dispatches without executing them |
| `watch` | `--debounce <dur>` | `500ms` | Coalesce rapid events on the same path |
| `history` | `-n`, `--limit <int>` | `20` | Max records to print |
| `undo` | `--last <int>` | `1` | Reverse the N most recent actions |

## Match conditions

Every condition in a rule's `match:` block must be satisfied for the rule to fire — they AND together.

| YAML field | Type | Matches when |
|------------|------|--------------|
| `extensions` | list | File extension (case-insensitive) is in the list |
| `glob` | string | Filename (not path) matches the shell glob |
| `regex` | string | Filename matches the Go regex |
| `min_size` / `max_size` | size | File size within range (`500MB`, `1GB`, `42KB`) |
| `min_age` / `max_age` | duration | File mtime within range (`30d`, `2h`, `5m`, `10s`) |
| `mime_type` | string | Detected MIME type starts with the given prefix |
| `content` | string | File content contains the substring (case-insensitive) |
| `content_regex` | string | File content matches the regex; named captures become `{{.Match.<name>}}` |
| `content_bytes` | integer | Max bytes to read for content matching. Default `65536` |

For full semantics see [Concepts › Matching](02-concepts.md#matching).

## Actions

| Action | Reversible | Platform | Required external tool |
|--------|:----------:|----------|------------------------|
| `move` | ✓ | all | — |
| `copy` | ✓ | all | — |
| `rename` | ✓ | all | — |
| `delete` | ✓ (restore from trash) | all | — |
| `symlink` | ✓ | all | — |
| `chmod` | ✓ | all (Unix semantics) | — |
| `checksum` | ✓ | all | — |
| `compress` | ✓ | all | — |
| `extract` | ✓ | all | `tar` for `.tar.xz` only |
| `deduplicate` | ✓ | all | — |
| `encrypt` | ✓ | all | `age` (default) or `gpg` |
| `decrypt` | ✓ | all | `age` (default) or `gpg` |
| `convert` | ✓ | all | user-specified (`ffmpeg`, `pandoc`, …) |
| `resize` | ✓ | all | `sips` (built into macOS) or `convert` (ImageMagick) |
| `watermark` | ✓ | all | `composite` (ImageMagick) |
| `ocr` | ✓ | all | `tesseract` |
| `exec` | ✗ | all | — |
| `notify` | ✗ | all | `osascript` (macOS), `notify-send` (Linux), `BurntToast` (Windows, optional) |
| `upload` | ✗ | all | `aws` (for `s3://`), `gsutil` (for `gs://`) |
| `tag` | ✗ | macOS only | `xattr` (built-in) |
| `open` | ✗ | macOS only | `open` (built-in) |
| `unquarantine` | ✗ | macOS only | `xattr` (built-in) |

For field-level detail on each action, run `sortie actions <name>` or see the [Cookbook](03-cookbook.md).

## Template variables

Available in `dest`, `command`, `message`, `title`, `args`, `remote`, and `tags`.

| Variable | Produces | Example |
|----------|----------|---------|
| `{{.Name}}` | Filename without extension | `IMG_0123` |
| `{{.Ext}}` | Extension including the dot | `.jpg` |
| `{{.Path}}` | Full source file path | `/Users/you/Downloads/IMG_0123.jpg` |
| `{{.Year}}` | Four-digit year from mtime | `2026` |
| `{{.Month}}` | Zero-padded month | `04` |
| `{{.Day}}` | Zero-padded day | `24` |
| `{{.Date}}` | Full `YYYY-MM-DD` from mtime | `2026-04-24` |
| `{{.Time}}` | `HH-MM-SS` from mtime | `14-30-00` |
| `{{.Dest}}` | Resolved destination path (usable inside `convert` / `exec` args) | depends on the rule |
| `{{.Match.<name>}}` | Named capture from `content_regex` | `{{.Match.company}}` |

For possibly-empty captures, use `{{or .Match.field "fallback"}}`.

There is no `{{.Dir}}` or `{{.Size}}` — if you need the source directory for a `rename` or `move`, hardcode it in the template (see the [rename recipe](03-cookbook.md#rename-with-an-iso-date-prefix)).

## Config file locations

**All platforms (macOS / Linux / Windows)**: `~/.config/sortie/config.yaml`.

On Windows this resolves to `C:\Users\<you>\.config\sortie\config.yaml` — sortie uses the Unix-style layout even on Windows rather than `%APPDATA%`. This keeps config portable and predictable across OSes.

Per-directory overrides live as `.sortie.yaml` in any watched directory and are merged with globals via rule priority.

Other default paths (all configurable via `config.yaml`):

| Purpose | YAML key | Default |
|---------|----------|---------|
| History file | `history_file` | `~/.config/sortie/history.json` |
| Trash directory | `trash_dir` | `~/.config/sortie/trash/` |
| Log directory | `log_dir` | `~/.config/sortie/logs/` |

## File naming conventions

These influence matching:

- **Ignored temp suffixes** — the watcher skips events for files ending in `.crdownload`, `.part`, `.partial`, `.download`, and `.tmp`, treating them as in-progress downloads. Rules never see these files until they're renamed to their final name.
- **Dotfiles** — files starting with `.` are scanned/watched but rules usually skip them via their own `glob` or `extensions` filter.
- **macOS metadata** — `.DS_Store`, `._*` resource forks, and `__MACOSX` folders inside archives are stripped automatically by `extract`.

## Debounce, rate limiting, and cooldowns

| Mechanism | Where it lives | Default | Reset on restart? |
|-----------|----------------|---------|-------------------|
| `--debounce` (watch) | Flag | 500 ms | n/a — per-event |
| `--rate-limit` (scan/watch) | Flag | disabled | yes (in-memory) |
| Per-rule `cooldown` | YAML `cooldown:` field | none | yes (in-memory) |

Rate-limit and cooldown state is **in-memory only** — restart the daemon (`sortie watch`) to clear any rule that's been held back.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Runtime error (action failed, config invalid, etc.) |
| `2` | CLI misuse (unknown flag, missing required arg) |

## See also

- [Getting Started](01-getting-started.md) — install and first-run walkthrough
- [Concepts](02-concepts.md) — mental model
- [Cookbook](03-cookbook.md) — recipes for every action type
- [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes
- [`sortie.1`](../../sortie.1) — the man page (also via `sortie man | less`)
