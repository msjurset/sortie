# Concepts

The mental model behind sortie. Read this once; the [Cookbook](03-cookbook.md) will make much more sense afterwards.

## The pipeline

Every file sortie handles flows through the same five stages:

```
file event  →  match  →  action(s)  →  history  →  (optional undo)
    ↑                                      │
    └─ fsnotify (watch) or walk (scan)     └─ ~/.config/sortie/history.json
```

- **Event** — either a `fsnotify` event (`watch`) or a directory walk entry (`scan`).
- **Match** — the file is tested against every rule until one matches.
- **Action** — the matched rule's action (or chain of actions) runs.
- **History** — the outcome is appended as a single JSON record. Real-run only; `--dry-run` skips this.
- **Undo** — reverses a recorded action if it's reversible.

## Rules

A rule has a `name`, a `match` block, an `action` (or `actions` list), and a few optional modifiers. Rules live in two places:

- **Global rules** — the `rules:` list in `~/.config/sortie/config.yaml` (or `%APPDATA%\sortie\config.yaml` on Windows).
- **Per-directory rules** — a `.sortie.yaml` file inside any watched directory. These are merged with globals whenever sortie processes a file in that directory.

### Priority order

Every rule has an optional `priority:` integer (default `0`). Rules are sorted by priority **descending** (highest first), and sortie applies the first one that matches. If two rules have the same priority, YAML order wins as a tiebreaker. Per-directory and global rules are merged into the same sorted list, so a `priority: 10` global rule beats a `priority: 0` per-directory rule even though local usually comes first in practice.

```yaml
- name: urgent-invoices
  priority: 100                # runs before anything at priority 0
  match:
    extensions: [.pdf]
    content: invoice
  action:
    type: move
    dest: ~/Documents/Invoices/
```

### `continue: true`

By default, the first matching rule wins and the file is done. Set `continue: true` on a rule to also let lower-priority rules evaluate — useful when you want (say) a `notify` rule to fire in addition to the `move` rule that follows.

```yaml
- name: ping-me-about-pdfs
  priority: 100
  continue: true               # don't stop here
  match:
    extensions: [.pdf]
  action:
    type: notify
    title: PDF arrived
    message: "{{.Name}}{{.Ext}}"

- name: file-pdfs
  match:
    extensions: [.pdf]
  action:
    type: move
    dest: ~/Documents/
```

Both rules fire: the notify runs first, then the move.

## Matching

Every condition you add to a `match:` block must be satisfied for the rule to match — conditions are **AND**ed, never OR. If you need OR, write two rules.

| Condition | YAML | Matches when |
|-----------|------|--------------|
| Extensions | `extensions: [.pdf, .epub]` | File's extension (case-insensitive) is in the list |
| Glob | `glob: "screencapture-*"` | Filename (not full path) matches the shell glob |
| Regex | `regex: '^IMG_\d+\.(jpg|heic)$'` | Filename matches the Go regex |
| Size | `min_size: 500MB` / `max_size: 1GB` | File size within the given range |
| Age | `min_age: 30d` / `max_age: 2h` | File mtime within the given range. Units: `s`, `m`, `h`, `d` |
| MIME type | `mime_type: image/` | Detected MIME type starts with the given prefix (extension lookup, then content sniff) |
| Content | `content: invoice` | File content contains the given substring (case-insensitive) |
| Content regex | `content_regex: '(?P<total>\$\d+\.\d{2})'` | File content matches the given regex; named captures become template variables |
| Content bytes | `content_bytes: 65536` | How many bytes of file content to read for `content`/`content_regex`. Default 65536 |

For PDFs, `content` and `content_regex` automatically extract text using `pdftotext` (install `poppler`). Content detection uses magic bytes, so a plain text file with a `.pdf` extension won't be treated as a PDF.

### Named capture groups

When you use named groups in `content_regex`, the captures become `{{.Match.<name>}}` variables in templates:

```yaml
- name: tag-invoices
  match:
    extensions: [.pdf]
    content_regex: '(?P<company>Acme|Widget Co).*?(?P<date>\d{4}-\d{2}-\d{2})'
  action:
    type: rename
    dest: '{{.Match.company}}-{{or .Match.date "undated"}}.pdf'
```

A capture named `date` is special: sortie normalizes it to `YYYY-MM-DD` from a handful of common formats (`DD-MON-YYYY`, `January 2, 2026`, `01/02/2026`, etc.) automatically. For any capture, use `{{or .Match.name "fallback"}}` to handle the case where the group didn't match.

## Actions and chains

A rule runs either one action or several in sequence. You can use whichever form is clearer:

```yaml
action:
  type: move
  dest: ~/Documents/

# or

actions:
  - type: notify
    title: Processing
    message: "{{.Name}}"
  - type: deduplicate
    dest: ~/Documents/{{.Name}}{{.Ext}}
  - type: tag
    tags: [imported]
```

A chain runs its steps in order, passing forward the current file path. If any step fails, **the chain stops immediately at that step and the earlier steps are NOT rolled back** — a failed step 3 of 5 leaves steps 1 and 2 in place. All steps in a chain share a `chain_id` in history, so `sortie undo` reverses the whole chain as a unit (to the extent its individual steps are reversible).

Design for this: put reversible structural changes (move, rename, compress) first, and put side-effects (notify, exec, upload) last. That way the common failure mode — "the side effect failed" — leaves your files in a recoverable state.

## Reversibility

Not every action can be undone. The reversible ones produce a file you can put back; the rest have side effects outside the filesystem.

| Reversible | Not reversible |
|------------|----------------|
| move, copy, rename, delete (restored from trash), compress, extract, symlink, chmod, checksum, convert, resize, watermark, ocr, encrypt, decrypt, deduplicate | exec, notify, upload, tag, open, unquarantine |

`sortie undo` will skip non-reversible entries with a clear message; the history record stays but is marked as undone where applicable.

## Dry-run, history, and undo

- **`--dry-run`** evaluates every match condition (including reading file content for `content_regex`) but skips the action and does **not** write to history. Safe to run anywhere, anytime.
- **History** lives at `~/.config/sortie/history.json` (configurable). One JSON record per line. Each record has: `id`, `ts`, `rule`, `action`, `src`, `dest`, `undone`, `error`, and a `chain_id` when the action was part of a chain. Inspect with `sortie history` or `sortie history -n 100`.
- **Undo** reverses a specific record by ID (`sortie undo abc123`) or the most recent N actions (`sortie undo --last 3`). Non-reversible types return an error and are skipped.

## Watch vs. scan

Both drive the same dispatch pipeline, but the event source and defaults differ.

| | `sortie scan` | `sortie watch` |
|---|---|---|
| Event source | Directory walk (one-shot) | fsnotify events (continuous) |
| Default debounce | n/a | 500 ms, configurable with `--debounce` |
| Rate limit | `--rate-limit` flag + per-rule `cooldown:` | Same flag + field |
| Config hot-reload | n/a | Yes — config changes apply to subsequent events without restart |
| Typical use | Cleanup of an existing directory; cron-driven sorting | Persistent background daemon; real-time triage |

### Debounce

When a file is written (download in progress, save-over-save), fsnotify fires many events in rapid succession. The debounce window coalesces them: sortie waits `--debounce` milliseconds of quiet on a path before dispatching. Increase it if large files are being processed before downloads finish; decrease it for snappier response.

### `watch_existing`

By default, `watch` only fires on **Create** and **Rename** events — matching the mental model that "sortie triages new arrivals." Set `watch_existing: true` on a directory entry to also fire on **Write** events, which lets you react to files that grow or are edited in place (a log file crossing a size threshold, a save-over-save on an image). Match conditions still have to hold, so a growing log only dispatches once it crosses `min_size`.

### fsnotify on each platform

The underlying watcher uses `inotify` on Linux, `FSEvents` (via kqueue in Go) on macOS, and `ReadDirectoryChangesW` on Windows. Behavior is consistent enough that the same config works everywhere; edge cases (network filesystems, permission denials) are covered in [Troubleshooting](05-troubleshooting.md).

## Templates

Several action fields — `dest`, `command`, `message`, `title`, `args`, `remote`, `tags` — are template-expanded using Go's `text/template` syntax. The variables come from the source file's metadata and modification time.

| Variable | Produces | Example |
|----------|----------|---------|
| `{{.Name}}` | Filename without extension | `IMG_0123` |
| `{{.Ext}}` | Extension including the dot | `.jpg` |
| `{{.Path}}` | Full source file path | `/Users/you/Downloads/IMG_0123.jpg` |
| `{{.Year}}` | Four-digit year from file mtime | `2026` |
| `{{.Month}}` | Zero-padded month | `04` |
| `{{.Day}}` | Zero-padded day | `24` |
| `{{.Date}}` | Full `YYYY-MM-DD` from mtime | `2026-04-24` |
| `{{.Time}}` | `HH-MM-SS` from mtime | `14-30-00` |
| `{{.Match.<name>}}` | Named capture group from `content_regex` | `{{.Match.company}}` |

Fallback pattern for possibly-empty captures:

```yaml
dest: '{{.Match.company}}/{{or .Match.date "undated"}}.pdf'
```

That's every concept sortie works with. Next up: [the Cookbook](03-cookbook.md) — concrete recipes for every action type and several multi-step workflows.
