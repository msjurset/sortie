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
| `sortie backup snapshot` | Create `~/.config/sortie/backups/sortie-<ts>.tar.gz` (config + history + trash) |
| `sortie backup list` | List snapshot tarballs, newest first |
| `sortie backup show` | Print the file listing of a snapshot |
| `sortie backup restore` | Restore `config.yaml` from a snapshot (other items need manual `tar -xzf`) |
| `sortie backup diff` | Diff `config.yaml` in a snapshot vs the current config |
| `sortie backup prune` | Delete old snapshots by `--keep N` and/or `--older-than DURATION` |
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
| `backup show`, `restore`, `diff` | `--at <prefix>` | (newest) | Match a specific timestamp prefix (e.g. `2026-04-30`) |
| `backup prune` | `--keep <int>` | `10` | Keep newest N snapshots; `0` disables this rule |
| `backup prune` | `--older-than <dur>` | (none) | Delete older than DURATION (`7d`, `30d`, `24h`) |
| `backup prune` | `--dry-run` | `false` | Print what would be deleted |

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

## Debounce, rate limiting, cooldowns, and concurrency

| Mechanism | Where it lives | Default | Reset on restart? |
|-----------|----------------|---------|-------------------|
| `--debounce` (watch) | Flag | 500 ms | n/a — per-event |
| Per-directory `debounce:` | YAML, directory entry | inherits flag | n/a — per-event |
| `--rate-limit` (scan/watch) | Flag | disabled | yes (in-memory) |
| Per-rule `cooldown:` | YAML, rule entry | none | yes (in-memory) |
| Per-directory `poll:` | YAML, directory entry | none | n/a |
| Per-directory `concurrency:` | YAML, directory entry | 0 (unbounded) | n/a |

Rate-limit and cooldown state is **in-memory only** — restart the daemon (`sortie watch`) to clear any rule that's been held back. Per-directory `debounce`, `poll`, and `concurrency` reconcile on hot-reload: changing the value in `config.yaml` resizes the relevant resource (timer delay, poll interval, worker pool) without a restart.

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

---

# Appendices

The rest of this page is exhaustive schema and grammar reference. Skip directly to whichever appendix you need:

- [A. Full configuration schema](#a-full-configuration-schema) — every field of `config.yaml` and `.sortie.yaml`
- [B. Action field reference](#b-action-field-reference) — required and optional fields for each of the 22 action types
- [C. JSON history record schema](#c-json-history-record-schema)
- [D. JSON log format schema](#d-json-log-format-schema)
- [E. Regex flavor notes (RE2)](#e-regex-flavor-notes-re2)
- [F. Template language reference](#f-template-language-reference)
- [G. Size and duration formats](#g-size-and-duration-formats)
- [H. Default values reference](#h-default-values-reference)

---

## A. Full configuration schema

### Top-level `~/.config/sortie/config.yaml`

```yaml
# All fields are optional unless marked required.

log_dir: ~/.config/sortie/logs        # string, where the daemon writes logs
history_file: ~/.config/sortie/history.json   # string, JSON Lines history
trash_dir: ~/.config/sortie/trash     # string, holds files from `delete` action
log_format: text                       # "text" (default) or "json"

ignore:                                # list of glob patterns to skip globally
  - "*.swp"
  - "*~"
  - ".DS_Store"

directories:                           # required; list of watched directories
  - path: ~/Downloads                  # required; absolute or ~-relative path
    recursive: false                   # bool, watch subdirectories too. default false
    watch_existing: false              # bool, fire on Write events for existing files. default false
    debounce: ""                       # duration string, override --debounce for this directory. default inherits global
    poll: ""                           # duration string, run periodic ReadDir in addition to fsnotify. default no polling
    concurrency: 0                     # int, cap parallel dispatches via worker pool. default 0 = unbounded

rules:                                 # list of rules
  - name: example                      # required; unique identifier shown in logs/history
    priority: 0                        # int, higher fires first. default 0
    continue: false                    # bool, fall through to next match. default false
    cooldown: ""                       # duration string, min interval between this rule firing
    match:                             # required; conditions are AND-combined
      # ... see Match schema below ...
    action:                            # single action OR
      # ... see Action schema below ...
    actions:                           # action chain (mutually exclusive with action)
      - # ... ...
```

### `Match` block schema

```yaml
match:
  extensions: [.pdf, .epub]            # list of strings, case-insensitive
  glob: "screencapture-*"              # string, filepath.Match against filename only
  regex: '^IMG_\d+\.(jpg|heic)$'       # string, Go RE2 regex against filename only
  min_size: 500MB                      # size string (see Appendix G)
  max_size: 1GB                        # size string
  min_age: 30d                         # duration string (s/m/h/d)
  max_age: 2h                          # duration string
  mime_type: image/                    # string, prefix match
  content: "invoice"                   # string, case-insensitive substring on file content
  content_regex: '(?P<vendor>Acme)'    # string, RE2 regex on file content
  content_bytes: 65536                 # int, max bytes to read for content. default 65536
```

All `Match` fields are optional. An empty `match: {}` block matches every file. Conditions AND together — see [Concepts › Matching](02-concepts.md#matching).

### Per-directory `.sortie.yaml`

A `.sortie.yaml` file in any watched directory provides directory-scoped rules and ignore patterns. Schema:

```yaml
ignore:                                # list of glob patterns, applied only in this directory
  - "*.partial"
  - "draft-*"

rules:                                 # same Rule schema as the top-level config
  - name: ...
    match: ...
    action: ...
```

There's no `directories:` field at this level — the directory is the file's location. Per-directory rules merge with global rules into one priority-sorted list at evaluation time.

---

## B. Action field reference

Every field on the `Action` struct, organized per action type. Required fields are bold; optional fields have their default in parentheses.

### `move` — relocate a file

```yaml
type: move
dest:        # REQUIRED — string, template-expanded
```

Cross-filesystem moves fall back to copy + delete automatically.

### `copy` — duplicate a file, preserving the original

```yaml
type: copy
dest:        # REQUIRED — string, template-expanded
```

Preserves file mode (Unix permissions). Symlinks are followed and the target is copied, not the link.

### `rename` — relocate within the source directory

```yaml
type: rename
dest:        # REQUIRED — string, template-expanded
```

Implementation calls `os.Rename(src, dest)`. Cross-filesystem renames fail; use `move` for that. There's no `{{.Dir}}` template variable — hardcode the parent directory in the template.

### `delete` — move a file to sortie's trash

```yaml
type: delete
```

No fields. The file moves to `~/.config/sortie/trash/<filename>`; collisions get a numeric suffix. Restore with `sortie undo`; permanent removal with `sortie trash purge`.

### `compress` — gzip a file in place

```yaml
type: compress
dest:        # optional — string, template-expanded; default: <src>.gz
```

Removes the original after successful gzip. **Validator warns if subsequent chain steps need the source.**

### `extract` — unpack an archive into a directory

```yaml
type: extract
dest:        # REQUIRED — string, directory to extract into; template-expanded
```

Supports `.zip`, `.tar`, `.tar.gz` / `.tgz`, `.tar.bz2` (Go stdlib; no external tools). `.tar.xz` requires `tar` on `PATH`. macOS metadata (`__MACOSX`, `._*`, `.DS_Store`) is stripped automatically.

### `symlink` — create a symbolic link to the source

```yaml
type: symlink
dest:        # REQUIRED — string, link path; template-expanded
```

Calls `os.Symlink`. Fails if `dest` already exists. On Windows, requires elevation or developer mode.

### `chmod` — change permission bits

```yaml
type: chmod
mode:        # REQUIRED — string, octal mode e.g. "0755", "644"
```

Original mode is captured in history for undo. POSIX semantics; Windows mapping is partial (read-only bit only).

### `checksum` — write a hash sidecar

```yaml
type: checksum
algorithm:   # optional — "sha256" (default), "md5", "sha1"
dest:        # optional — string, sidecar path; default: <src>.<algorithm>
```

Sidecar format is BSD: `<ALGO> (<filename>) = <hash>`. Compatible with `shasum -c`.

### `compress`, `extract`, `delete`, `move`, `copy`, `rename`, `symlink`, `chmod`, `checksum` are **reversible**.

### `convert` — run an external converter tool

```yaml
type: convert
tool:        # REQUIRED — string, binary name (ffmpeg, pandoc, convert, …)
dest:        # REQUIRED — string, output path; template-expanded
args:        # optional — string, template-expanded args; default: empty
```

`{{.Path}}` and `{{.Dest}}` are available in `args`. Source preserved; reversible (undo deletes the output).

### `resize` — resize an image

```yaml
type: resize
dest:        # REQUIRED — string, output path; template-expanded
width:       # optional — int, pixels (≥ 0)
height:      # optional — int, pixels (≥ 0)
percentage:  # optional — int, scale percentage (e.g. 50)
tool:        # optional — string, "sips" (default on macOS) or "convert" (ImageMagick)
```

At least one of `width`, `height`, or `percentage` is required. Source preserved; reversible.

### `watermark` — composite an overlay image onto a source

```yaml
type: watermark
overlay:     # REQUIRED — string, path to watermark image
dest:        # REQUIRED — string, output path; template-expanded
gravity:     # optional — placement, default "center"; one of: center, north, south, east, west, northeast, northwest, southeast, southwest
tool:        # optional — string, default "composite" (ImageMagick legacy)
```

Source preserved; reversible.

### `ocr` — extract text via tesseract

```yaml
type: ocr
dest:        # optional — string, output .txt path; default: sidecar next to source
language:    # optional — string, tesseract language code; default "eng"
tool:        # optional — string, default "tesseract"
```

Source preserved; reversible (undo deletes the `.txt`).

### `encrypt` — age or gpg encryption

```yaml
type: encrypt
recipient:   # REQUIRED — string, age public key or gpg recipient ID
dest:        # optional — string, output path; default: <src>.age
tool:        # optional — string, "age" (default) or "gpg"
```

Source preserved (chain a `delete` if you want plaintext removed). Reversible (undo deletes the encrypted output).

### `decrypt` — reverse of `encrypt`

```yaml
type: decrypt
dest:        # optional — string, output path; default: source minus extension
key:         # optional — string, identity file (age) or keyring path (gpg)
tool:        # optional — string, "age" (default) or "gpg"
```

Source preserved. Reversible.

### `deduplicate` — hash-compare and conditionally move

```yaml
type: deduplicate
dest:          # REQUIRED — string, candidate destination; template-expanded
on_duplicate:  # optional — "skip" (default) or "delete"
```

SHA-256 over full source contents. Three possible outcomes recorded as `dest:`:

- `moved:<path>` — no duplicate; source moved to dest
- `skip:<path>` — duplicate found; source kept (`on_duplicate: skip`)
- `delete:<path>` — duplicate found; source trashed (`on_duplicate: delete`)

Reversible only for `moved:` and `delete:` outcomes.

### `exec` — run a shell command

```yaml
type: exec
command:     # REQUIRED — string, template-expanded; runs via sh -c (Unix) or cmd /c (Windows)
```

Templates: all standard variables available. **Not reversible.**

### `notify` — desktop notification or webhook

```yaml
type: notify
title:       # optional — string, template-expanded; default "sortie"
message:     # optional — string, template-expanded; if starts with http://, sends webhook POST
```

Backend per platform: `osascript` (macOS), `notify-send` (Linux), `BurntToast` with stderr fallback (Windows). Webhook payload: `{title, file, path, size}` as JSON. **Not reversible.**

### `upload` — push to cloud storage

```yaml
type: upload
remote:      # REQUIRED — string, destination URI; template-expanded; e.g. s3://bucket/key
tool:        # optional — string, override auto-detection (aws, gsutil)
```

Auto-detected from URI scheme: `s3://` → `aws`, `gs://` → `gsutil`. **Not reversible.**

### `tag` — apply macOS Finder tags

```yaml
type: tag
tags:        # REQUIRED — list of strings; e.g. [Red, Finance]
```

Standard color names: `Red`, `Orange`, `Yellow`, `Green`, `Blue`, `Purple`, `Gray`. Custom names work. **macOS only; not reversible.**

### `open` — open a file with macOS `open`

```yaml
type: open
app:         # optional — string, app name e.g. "VLC", "Preview"
```

**macOS only; not reversible.**

### `unquarantine` — strip Gatekeeper quarantine attribute

```yaml
type: unquarantine
```

No fields. **macOS only; not reversible.** No-op on Linux/Windows.

---

## C. JSON history record schema

History lives at `~/.config/sortie/history.json`, one record per line (JSON Lines format). Each record:

```jsonc
{
  "id":        "f3a8c…",          // string, random hex; unique per record
  "ts":        "2026-04-26T14:30:21Z", // ISO 8601 UTC timestamp
  "rule":      "file-pdfs",         // string, the matched rule's `name`
  "action":    "move",              // string, the action type
  "src":       "/Users/you/Downloads/x.pdf",  // string, source file path
  "dest":      "/Users/you/Documents/PDFs/x.pdf",  // string, optional; absent when action has no destination
  "undone":    false,               // bool, optional; true if `sortie undo` reversed this record
  "error":     "...",               // string, optional; non-empty if the action failed
  "chain_id":  "abc123…"            // string, optional; groups records that came from one chain
}
```

Field semantics:

- **`id`** — for use with `sortie undo <id>`. Hex; not a UUID.
- **`ts`** — when the action was attempted (success OR failure).
- **`dest`** — its meaning varies by action: a path for `move`/`copy`/etc., a `moved:`/`skip:`/`delete:` prefixed path for `deduplicate`, empty for actions with no file destination (`exec`, `notify`, `unquarantine`).
- **`undone`** — `true` only when the record itself represents a successful undo. The original record stays in place; `undo` doesn't delete history entries.
- **`error`** — present only when the action errored. Action that succeeded omits this field entirely.
- **`chain_id`** — when set, all records sharing the same `chain_id` came from one rule's `actions:` list and were dispatched in a single pipeline. Use it to undo a chain as a unit: `sortie undo <chain_id>`.

Programmatic queries:

```sh
# Count actions by type
jq -r .action ~/.config/sortie/history.json | sort | uniq -c | sort -rn

# Find every error in the last 100 records
tail -n 100 ~/.config/sortie/history.json | jq 'select(.error)'

# Show all records from a specific chain
jq 'select(.chain_id == "abc123…")' ~/.config/sortie/history.json
```

---

## D. JSON log format schema

When `sortie watch --log-format json` is used, every log line is an object:

```jsonc
{
  "time":  "2026-04-26T14:30:21.123Z",  // RFC 3339 with millisecond precision
  "level": "INFO",                        // one of: DEBUG, INFO, WARN, ERROR
  "msg":   "dispatched",                   // human-readable message string
  // ... arbitrary structured fields per message type ...
}
```

Common message types and the structured fields you'll see attached:

| `msg` | Additional fields |
|-------|-------------------|
| `dispatched` | `rule`, `action`, `src`, `dest` |
| `dispatch failed` | `err` |
| `cooldown` | `rule`, `cooldown` |
| `no match` | `file` |
| `ignored` | `file` |
| `config reloaded` | `rules` (count) |
| `config reload failed` | `err` |
| `binary changed, restarting` | (none) |
| `loading rules` | `dir`, `err` |
| `stat file` | `path`, `err` |
| `cannot watch config dir` | `path`, `err` |
| `cannot watch directory for config changes` | `path`, `err` |
| `watcher error` | `err` |
| `cannot monitor binary` | `err` |
| `cannot resolve binary path` | `err` |
| `cannot stat binary` | `err` |
| `could not write PID file` | `err` |

Filtering examples:

```sh
# Only errors
sortie watch --log-format json | jq 'select(.level == "ERROR")'

# Only dispatches for a specific rule
sortie watch --log-format json | jq 'select(.rule == "invoice-intake")'

# Hide the "no match" noise
sortie watch --log-format json | jq 'select(.msg != "no match")'
```

---

## E. Regex flavor notes (RE2)

sortie uses Go's `regexp` package, which implements **RE2** semantics. Key behaviors and limitations:

### Supported

- Standard character classes: `\d`, `\w`, `\s`, `[...]`, `[^...]`
- Quantifiers: `*`, `+`, `?`, `{n}`, `{n,}`, `{n,m}` (all are greedy by default)
- Lazy quantifiers: `*?`, `+?`, `??` work the same as elsewhere
- Anchors: `^`, `$`, `\b`, `\B`
- Groups: `(...)` (capturing), `(?:...)` (non-capturing), `(?P<name>...)` (named)
- Alternation: `|`
- Inline flags: `(?i)` (case-insensitive), `(?m)` (multi-line), `(?s)` (dot matches newline)

### NOT supported (will fail to compile)

| Construct | What it would do | RE2 stance |
|-----------|------------------|------------|
| `(?<=...)`  | Positive lookbehind | Not supported |
| `(?<!...)`  | Negative lookbehind | Not supported |
| `(?=...)`   | Positive lookahead | Not supported |
| `(?!...)`   | Negative lookahead | Not supported |
| `\1`, `\2`  | Backreferences | Not supported |
| `(?>...)`   | Atomic groups | Not supported |
| `(?(...)...)` | Conditionals | Not supported |

The most common gotcha when porting a regex from Perl, PCRE (Apache, PHP), or .NET is lookaround. RE2's design trades it away in exchange for guaranteed linear-time matching — which is why sortie can run regex matching against millions of files without worrying about ReDoS-style pathological cases.

If you need lookaround:

- **Restructure to avoid it.** Most lookaround uses can be expressed as alternation plus capture-group selection.
- **Match more, post-filter in templates.** Capture too much; use `{{or}}`, `{{if}}` to filter in the dest.
- **Fall back to `exec`** with a tool that has full PCRE (grep, ripgrep with `-P`, perl).

References: [Go regexp/syntax docs](https://pkg.go.dev/regexp/syntax) for the full RE2 specification.

---

## F. Template language reference

sortie expands several `Action` fields with Go's [`text/template`](https://pkg.go.dev/text/template) package: `dest`, `command`, `message`, `title`, `args`, `remote`, `tags`. The variables come from the source file's metadata; the template language adds simple control flow.

### Variables

```
{{.Name}}   {{.Ext}}   {{.Path}}   {{.Dest}}
{{.Year}}   {{.Month}} {{.Day}}    {{.Date}}   {{.Time}}
{{.Match.<capture-name>}}
```

See the [Template variables table](#template-variables) above for what each produces.

### Operators that work

```
{{or .A .B "fallback"}}             # first non-empty
{{eq .Match.kind "invoice"}}        # equality (true/false)
{{if .Match.vendor}}…{{else}}…{{end}}  # branching
{{if and .A .B}}…{{end}}            # boolean AND
{{if or .A .B}}…{{end}}             # boolean OR
{{with .Match.vendor}}{{.}}-{{end}} # run body if non-empty; . is bound to the value
{{- ... -}}                          # whitespace trim left/right
```

### Functions sortie does NOT register

The following are common in third-party Go template usage but **are not available** in sortie:

- `lower`, `upper`, `title`, `replace`, `trim`, `split`, `join` (string transforms)
- `printf`, `print`, `println`
- `slice`, `index`, `len` (slice/string operations)
- Custom date formatting (`{{.Time | date "2006-01-02"}}` won't work)

If you need string transforms, use `exec` with `sed`/`awk`/PowerShell to do the work and pass the result back via filename conventions.

### Common idioms

```yaml
# Optional capture with fallback
dest: '~/Invoices/{{or .Match.vendor "Unknown"}}/{{.Name}}{{.Ext}}'

# Year + month partition
dest: '~/Documents/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}'

# Conditional prefix based on match capture
dest: '{{if eq .Match.kind "urgent"}}URGENT_{{end}}{{.Name}}{{.Ext}}'

# Date + time for collision-free naming
dest: '~/Archives/{{.Date}}_{{.Time}}_{{.Name}}{{.Ext}}'
```

---

## G. Size and duration formats

### Size strings

Used in `min_size` and `max_size`:

| Suffix | Value |
|--------|-------|
| (none) | bytes |
| `KB` or `K` | × 1024 |
| `MB` or `M` | × 1024² |
| `GB` or `G` | × 1024³ |
| `TB` or `T` | × 1024⁴ |

Examples: `500`, `42KB`, `5MB`, `1.5GB`, `1TB`. Parsing is case-insensitive.

### Duration strings

Used in `min_age`, `max_age`, `cooldown`, `--debounce`, `--rate-limit`:

| Suffix | Unit |
|--------|------|
| `ns` | nanoseconds |
| `us` / `µs` | microseconds |
| `ms` | milliseconds |
| `s` | seconds |
| `m` | minutes |
| `h` | hours |
| `d` | days (sortie-specific extension to Go's standard duration) |

Examples: `500ms`, `2s`, `5m`, `2h30m`, `7d`, `30d`. Compound durations like `1h30m` work.

---

## H. Default values reference

If you specify nothing, here's what sortie uses:

### Config-level

| Field | Default |
|-------|---------|
| `log_dir` | `~/.config/sortie/logs` |
| `history_file` | `~/.config/sortie/history.json` |
| `trash_dir` | `~/.config/sortie/trash` |
| `log_format` | `text` |
| `ignore` | empty list |
| `directories` | empty list (you must configure at least one for `watch`) |
| `rules` | empty list |

### Directory-level

| Field | Default |
|-------|---------|
| `recursive` | `false` |
| `watch_existing` | `false` |
| `debounce` | empty (inherits the global `--debounce` flag) |
| `poll` | empty (no polling — fsnotify only) |
| `concurrency` | `0` (unbounded — every dispatch spawns a goroutine) |

### Rule-level

| Field | Default |
|-------|---------|
| `priority` | `0` |
| `continue` | `false` |
| `cooldown` | empty (no cooldown) |

### Match-level

| Field | Default |
|-------|---------|
| `content_bytes` | `65536` (64 KB) |
| All other fields | unset; condition not enforced |

### Watch flags

| Flag | Default |
|------|---------|
| `--debounce` | `500ms` |
| `--rate-limit` | `0` (disabled) |

### Action-specific defaults

Most action fields are required when the action is specified. Notable defaults:

| Action | Field | Default |
|--------|-------|---------|
| `checksum` | `algorithm` | `sha256` |
| `checksum` | `dest` | `<src>.<algorithm>` |
| `compress` | `dest` | `<src>.gz` |
| `notify` | `title` | `"sortie"` |
| `deduplicate` | `on_duplicate` | `skip` |
| `resize` | `tool` | `sips` |
| `watermark` | `tool` | `composite` |
| `watermark` | `gravity` | `center` |
| `ocr` | `language` | `eng` |
| `ocr` | `tool` | `tesseract` |
| `ocr` | `dest` | sidecar `<src>.txt` |
| `encrypt` | `tool` | `age` |
| `encrypt` | `dest` | `<src>.age` |
| `decrypt` | `tool` | `age` |
