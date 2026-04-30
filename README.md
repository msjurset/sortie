# sortie

Intelligent file dispatcher — rule-based file routing for directories like `~/Downloads` and `~/Desktop`.

## 📚 Documentation

This README is a feature/reference overview. The **[User Guide in `docs/guide/`](docs/guide/)** is where to look for learning sortie, working through real-world examples, and debugging issues:

- **[Getting Started](docs/guide/01-getting-started.md)** — install + 10-minute tour
- **[Concepts](docs/guide/02-concepts.md)** — mental model with diagrams
- **[Cookbook](docs/guide/03-cookbook.md)** — ~30 recipes covering every action type plus multi-step chains
- **[Reference](docs/guide/04-reference.md)** — exhaustive schema and grammar reference
- **[Troubleshooting](docs/guide/05-troubleshooting.md)** — symptom-driven fixes with decision trees
- **[Running as a Service](docs/guide/06-running-as-a-service.md)** — launchd / systemd / Task Scheduler walk-throughs
- **[Thinking in sortie](docs/guide/07-thinking-in-sortie.md)** — design patterns, migrations, anti-patterns

## Features

- **Rule-based matching** — match files by extension, glob, regex, size, age, or MIME type
- **22 action types** — move, copy, rename, delete, compress, extract, symlink, chmod, checksum, exec, notify, convert, resize, watermark, ocr, encrypt, decrypt, upload, tag, open, deduplicate, unquarantine
- **Hybrid config** — central `~/.config/sortie/config.yaml` plus per-directory `.sortie.yaml` overrides
- **Watch mode** — real-time file monitoring with fsnotify and configurable debounce
- **Watch existing files** — monitor existing files for changes (e.g., log growth past a size threshold) with `watch_existing: true`
- **Per-directory debounce** — override the global `--debounce` per directory with `debounce: 10s`, useful for slow-syncing cloud-storage mounts
- **Per-directory poll** — `poll: 60s` on a directory entry runs a periodic walk in addition to fsnotify, for cloud-storage mounts (Google Drive, iCloud) where fsnotify events fire unreliably
- **Per-directory concurrency cap** — `concurrency: 4` bounds parallel handler invocations, preventing bulk drops from spawning hundreds of OCR/encode processes simultaneously
- **Dry-run mode** — preview what would happen before committing
- **Undo** — reverse recent actions from the history log
- **Template destinations** — use `{{.Year}}`, `{{.Month}}`, `{{.Name}}`, `{{.Ext}}`, `{{.Path}}` in dest paths and action fields
- **Trash management** — deleted files go to trash, not oblivion
- **Action chaining** — run multiple actions per rule in sequence (e.g., notify then move)
- **Ignore patterns** — `.gitignore`-style exclusions globally and per-directory
- **Content matching** — match files by text content, regex, or byte signatures
- **Config hot-reload** — config changes (rules, ignores, and the watched directory list) are picked up automatically in watch mode
- **Rate limiting** — throttle dispatch throughput per scan or watch cycle
- **Structured logging** — JSON or text log output via `--log-format`
- **Live status** — real-time watcher status with `sortie status --watch`
- **Contextual help** — `sortie actions` describes available action types and their fields
- **First-match-wins** — per-directory rules evaluate before global rules

## Install

### macOS (Homebrew)

```sh
brew install msjurset/tap/sortie
```

The formula installs the binary, man page, and zsh + bash completions. To get updates: `brew upgrade msjurset/tap/sortie`.

### Windows (Scoop)

```powershell
scoop bucket add msjurset https://github.com/msjurset/scoop-bucket
scoop install sortie
```

To get updates: `scoop update sortie`.

### Linux / cross-platform (download a release archive)

Grab the appropriate archive from the [Releases page](https://github.com/msjurset/sortie/releases/latest), extract it, and put `sortie` (or `sortie.exe` on Windows) somewhere on your `PATH`. Per-OS install steps with optional dependencies are in **[Getting Started](docs/guide/01-getting-started.md#install)**.

### From source

```sh
make deploy
```

Builds the binary, installs it to `~/.local/bin/`, installs the man page, and sets up zsh completions. Requires Go 1.26+.

> **First time using sortie?** Once installed, the **[10-minute Getting Started tour](docs/guide/01-getting-started.md)** walks you from `sortie config init` through writing your first rule and watching it move a file.

## Usage

```
sortie [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `scan [path...]` | Scan directories or individual files and apply rules |
| `watch` | Watch directories and dispatch files in real time |
| `history` | Show action history |
| `undo [id]` | Reverse recent dispatch actions |
| `rules [directory...]` | List configured rules |
| `rules test <file>` | Show which rule matches a file |
| `config` | Show resolved configuration |
| `config init` | Create a starter config file |
| `config path` | Print config file path |
| `status` | Show watcher daemon status |
| `trash` | List files in trash |
| `trash purge` | Permanently delete all trashed files |
| `actions [name]` | List action types, or show details for a specific action |
| `validate [directory...]` | Check rules for errors and potential problems |
| `backup snapshot` | Create a state tarball at `~/.config/sortie/backups/sortie-<ts>.tar.gz` |
| `backup list` | List snapshot tarballs, newest first |
| `backup show` | Print the file listing of a snapshot |
| `backup restore` | Restore `config.yaml` from a snapshot (other items need manual `tar -xzf`) |
| `backup diff` | Diff `config.yaml` in a snapshot vs the current config |
| `backup prune` | Delete old snapshots by `--keep N` and/or `--older-than DURATION` |
| `man` | Display manual page |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/sortie/config.yaml` | Config file path |
| `-v, --verbose` | `false` | Verbose output |
| `--log-format` | `text` | Log output format (`text` or `json`) |
| `--version` | — | Show version |

### Scan Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview actions without executing |
| `--rate-limit` | `0` | Max files to dispatch per second (0 = unlimited) |

### Watch Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Log actions without executing |
| `--debounce` | `500ms` | Debounce duration for file events |
| `--rate-limit` | `0` | Max files to dispatch per second (0 = unlimited) |

### History Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --limit` | `20` | Max records to show |

### Undo Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--last` | `1` | Number of recent actions to undo |

### Rules Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--global` | `false` | Include global rules when listing specific directories |

### Status Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--watch` | `false` | Continuously refresh status display |

### Validate Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--global` | `false` | Include global rules when validating specific directories |

### Examples

```bash
# Create a starter config
sortie config init

# Preview what would happen in ~/Downloads
sortie scan ~/Downloads --dry-run

# Scan all configured directories
sortie scan

# Watch directories in real time
sortie watch

# Check which rule matches a specific file
sortie rules test ~/Downloads/report.pdf

# Show recent dispatch history
sortie history

# Undo the last 3 actions
sortie undo --last 3

# List and purge trash
sortie trash
sortie trash purge
```

## Configuration

> **For learning:** the **[Concepts](docs/guide/02-concepts.md)** and **[Cookbook](docs/guide/03-cookbook.md)** pages walk through how rules, matches, and chains compose, with ~30 worked-through recipes. **[Reference appendices](docs/guide/04-reference.md#appendices)** have the complete YAML schema for every action type.

### Central Config (`~/.config/sortie/config.yaml`)

```yaml
log_format: json  # "text" (default) or "json" for structured logging

ignore:
  - .DS_Store
  - .git
  - "*.tmp"
  - node_modules/

directories:
  - path: ~/Downloads
    recursive: false
  - path: ~/Desktop
    recursive: false
  - path: /var/log/myapp
    watch_existing: true    # react to writes on existing files (e.g., growing logs)
  - path: ~/Library/CloudStorage/GoogleDrive-me@example.com/My Drive/Inbox
    debounce: 10s           # override --debounce for this dir (e.g., slow cloud-sync mounts)
    poll: 60s               # also walk the dir every 60s — fsnotify is unreliable on cloud mounts
    concurrency: 2          # cap parallel dispatches; protects against bulk-drop OCR storms

rules:
  - name: images-to-photos
    priority: 10            # lower = higher priority (default 0)
    cooldown: 5s            # skip file if same rule matched it within this window
    match:
      extensions: [.jpg, .jpeg, .png, .heic, .gif, .webp]
    action:
      type: move
      dest: ~/Pictures/Sorted/{{.Year}}/{{.Month}}

  - name: old-downloads
    match:
      min_age: 90d
    action:
      type: delete

  - name: large-files
    match:
      min_size: 500MB
    action:
      type: move
      dest: ~/LargeFiles

  - name: extract-archives
    match:
      extensions: [.zip, .tar.gz, .tgz]
    action:
      type: extract
      dest: ~/Downloads/Extracted/{{.Name}}

  - name: make-scripts-executable
    match:
      extensions: [.sh]
    action:
      type: chmod
      mode: "0755"

  - name: strip-exif
    match:
      extensions: [.jpg, .jpeg]
    action:
      type: exec
      command: "exiftool -all= '{{.Path}}'"

  - name: new-pdf-alert
    match:
      extensions: [.pdf]
    action:
      type: notify
      title: "New PDF"
      message: "{{.Name}}{{.Ext}} arrived"

  - name: encrypt-sensitive
    match:
      glob: "confidential-*"
    action:
      type: encrypt
      recipient: "age1..."
      dest: ~/Encrypted/{{.Name}}{{.Ext}}.age

  - name: backup-to-s3
    match:
      extensions: [.pdf]
    action:
      type: upload
      remote: "s3://my-bucket/docs/{{.Year}}/{{.Name}}{{.Ext}}"
```

### More Action Examples

```yaml
rules:
  - name: symlink-dotfiles
    match:
      glob: "*.conf"
    action:
      type: symlink
      dest: ~/configs/{{.Name}}{{.Ext}}

  - name: hash-large-downloads
    match:
      min_size: 100MB
    action:
      type: checksum
      algorithm: sha256

  - name: convert-videos-to-mp4
    match:
      extensions: [.mov, .avi, .mkv]
    action:
      type: convert
      tool: ffmpeg
      args: "-i {{.Path}} -c:v libx264 -crf 23 {{.Dest}}"
      dest: ~/Videos/Converted/{{.Name}}.mp4

  - name: resize-photos
    match:
      extensions: [.jpg, .png]
      min_size: 5MB
    action:
      type: resize
      width: 1920
      dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}

  - name: watermark-photos
    match:
      extensions: [.jpg, .png]
      glob: "portfolio-*"
    action:
      type: watermark
      overlay: ~/watermark.png
      gravity: southeast
      dest: ~/Pictures/Watermarked/{{.Name}}{{.Ext}}

  - name: ocr-scans
    match:
      extensions: [.png, .tiff]
      glob: "scan-*"
    action:
      type: ocr
      language: eng
      dest: ~/Documents/OCR/{{.Name}}.txt

  - name: decrypt-incoming
    match:
      extensions: [.age]
    action:
      type: decrypt
      key: ~/.age/key.txt
      dest: ~/Decrypted/{{.Name}}

  - name: tag-receipts
    match:
      regex: "(?i)receipt"
      extensions: [.pdf]
    action:
      type: tag
      tags: [Green, Finance]

  - name: open-dmg
    match:
      extensions: [.dmg]
    action:
      type: open

  - name: open-videos-vlc
    match:
      extensions: [.mkv, .avi]
    action:
      type: open
      app: VLC

  - name: dedup-downloads
    match:
      extensions: [.pdf, .zip]
    action:
      type: deduplicate
      dest: ~/Documents/{{.Name}}{{.Ext}}
      on_duplicate: skip

  - name: unquarantine-trusted
    match:
      extensions: [.dmg, .pkg]
      glob: "trusted-*"
    action:
      type: unquarantine

  # Trigger log rotation when a log file grows past 100MB.
  # Requires watch_existing: true on the directory.
  - name: rotate-large-logs
    match:
      extensions: [.log]
      min_size: 100MB
    cooldown: 5m
    action:
      type: exec
      command: "runbook run rotate-logs --file '{{.Path}}'"

  # Use named capture groups in content_regex to extract values from file
  # content and reference them in templates as {{.Match.name}}.
  # PDF text is extracted via pdftotext automatically.
  # Capture groups named "date" are normalized to YYYY-MM-DD.
  - name: sort-invoices
    match:
      extensions: [.pdf]
      content: invoice
      content_regex: '(?P<company>Oracle|Squarespace)?.*?(?P<date>\d{4}-\d{2}-\d{2}|\d{1,2}-[A-Za-z]{3}-\d{4}|[A-Z][a-z]+ \d{1,2}, \d{4})'
    actions:
      - type: notify
        title: "Invoice received"
        message: '{{or .Match.company "Vendor"}} - {{or .Match.date "undated"}}'
      - type: move
        dest: '~/Documents/Invoices/invoice-{{or .Match.company "Vendor"}}-{{or .Match.date "undated"}}{{.Ext}}'
```

### Per-Directory Config (`~/Downloads/.sortie.yaml`)

```yaml
ignore:
  - "*.crdownload"
  - "*.part"

rules:
  - name: pdfs-to-documents
    match:
      extensions: [.pdf]
    action:
      type: move
      dest: ~/Documents/PDFs/{{.Year}}-{{.Month}}
```

Per-directory rules take precedence over global rules. Per-directory ignore patterns are merged with global ignore patterns. All match conditions use AND logic. First matching rule wins (unless `continue: true` is set).

### Rule Fall-Through (`continue`)

By default, sortie stops at the first matching rule. Set `continue: true` to let a rule fire and still allow subsequent rules to match the same file:

```yaml
rules:
  - name: notify-all-downloads
    continue: true                    # keep evaluating after this rule
    match:
      extensions: [.zip, .pdf, .dmg]
    action:
      type: notify
      title: "New download"
      message: "{{.Name}}{{.Ext}} arrived"

  - name: extract-archives
    match:
      extensions: [.zip, .tar.gz, .tgz]
    actions:
      - type: extract
        dest: ~/Downloads/{{.Name}}
      - type: delete
```

Without `continue: true`, the notify rule would consume the match and the extract rule would never fire. With it, a `.zip` file triggers both: notify first, then extract + delete.

### Action Chaining

Rules can specify multiple actions using `actions:` (plural) instead of `action:` (singular). Actions execute in order, and if a move or rename changes the file's location, subsequent actions operate on the file at its new path.

```yaml
rules:
  - name: sort-and-notify
    match:
      extensions: [.pdf]
    actions:
      - type: notify
        title: "New PDF"
        message: "{{.Name}}{{.Ext}} arrived"
      - type: move
        dest: ~/Documents/PDFs/{{.Year}}/{{.Name}}{{.Ext}}

  - name: move-chmod-tag
    match:
      extensions: [.sh]
    actions:
      - type: move
        dest: ~/Scripts/{{.Name}}{{.Ext}}
      - type: chmod
        mode: "0755"
      - type: tag
        tags: [Green, Scripts]
```

If any action in the chain fails, the chain stops. Each action in a chain is recorded separately in history with a shared chain ID, so `sortie undo` reverses all actions in a chain together. The singular `action:` form continues to work for single-action rules.

### Ignore Patterns

Ignore patterns use `.gitignore`-style syntax to exclude files from processing. Patterns can be defined globally in `config.yaml` and per-directory in `.sortie.yaml`. Per-directory patterns are merged with global patterns.

```yaml
ignore:
  - .DS_Store          # exact filename
  - "*.tmp"            # glob pattern
  - "*.crdownload"     # partial downloads
  - node_modules/      # trailing slash = directory only
  - ".*"               # dotfiles
```

A leading `!` negates a pattern (re-includes a previously ignored file). A leading `/` anchors the pattern to the directory root. These follow the same rules as `.gitignore`.

### Match Conditions

| Field | Description | Example |
|-------|-------------|---------|
| `extensions` | File extensions | `[.jpg, .png]` |
| `glob` | Filename glob | `Screenshot*` |
| `regex` | Regex on filename | `(?i)docker\|vscode` |
| `min_size` / `max_size` | Size threshold | `500MB`, `1GB` |
| `min_age` / `max_age` | Age threshold | `30d`, `2h` |
| `mime_type` | MIME type prefix | `image/`, `application/pdf` |
| `content` | Substring in file content (PDF text extracted via `pdftotext`) | `TODO` |
| `content_regex` | Regex against file content; named groups become template vars; `date` groups auto-normalize to YYYY-MM-DD | `(?P<company>\w+).*(?P<date>\d{4}-\d{2}-\d{2})` |
| `content_bytes` | Hex byte signature (magic bytes) | `25504446` (PDF) |

### Action Types

| Type | Description | Undoable | Extra Fields |
|------|-------------|----------|--------------|
| `move` | Move file to dest | Yes | `dest` |
| `copy` | Copy file to dest | Yes | `dest` |
| `rename` | Rename file | Yes | `dest` |
| `delete` | Move to trash | Yes | — |
| `compress` | Gzip and remove original | Yes | `dest` |
| `extract` | Extract archive to dest dir | Yes | `dest` |
| `symlink` | Create symlink at dest | Yes | `dest` |
| `chmod` | Change permissions | Yes | `mode` |
| `checksum` | Write hash sidecar | Yes | `algorithm`, `dest` |
| `exec` | Run shell command | No | `command` |
| `notify` | Desktop notification or webhook | No | `title`, `message` |
| `convert` | Run external converter | Yes | `tool`, `args`, `dest` |
| `resize` | Resize image | Yes | `width`, `height`, `percentage`, `tool`, `dest` |
| `watermark` | Stamp image with overlay | Yes | `overlay`, `gravity`, `tool`, `dest` |
| `ocr` | Extract text (tesseract) | Yes | `language`, `tool`, `dest` |
| `encrypt` | Encrypt file (age/gpg) | Yes | `recipient`, `tool`, `dest` |
| `decrypt` | Decrypt file (age/gpg) | Yes | `key`, `tool`, `dest` |
| `upload` | Upload to cloud storage | No | `remote`, `tool` |
| `tag` | Apply macOS Finder tags | No | `tags` |
| `open` | Open file with default or specified app | No | `app` |
| `deduplicate` | Move to dest if not a duplicate (by hash) | Partial | `dest`, `on_duplicate` |
| `unquarantine` | Remove macOS quarantine xattr | No | — |

### Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Name}}` | Filename without extension | `report` |
| `{{.Ext}}` | Extension with dot | `.pdf` |
| `{{.Path}}` | Full source file path | `/Users/me/Downloads/report.pdf` |
| `{{.Year}}` | 4-digit year | `2026` |
| `{{.Month}}` | 2-digit month | `03` |
| `{{.Day}}` | 2-digit day | `18` |
| `{{.Date}}` | YYYY-MM-DD | `2026-03-18` |
| `{{.Time}}` | HH-MM-SS | `14-30-00` |
| `{{.Match.name}}` | Named capture from `content_regex` | `{{.Match.company}}` |

### External Tool Requirements

Some action types shell out to external tools. Install only the tools you need:

| Action | Default Tool | Install (macOS) | Alternatives |
|--------|-------------|-----------------|--------------|
| `resize` | `sips` | Built-in | `convert` (ImageMagick) |
| `watermark` | `composite` | `brew install imagemagick` | — |
| `convert` | (none, must set `tool`) | `brew install ffmpeg` | `convert`, `pandoc` |
| `ocr` | `tesseract` | `brew install tesseract` | — |
| `encrypt` | `age` | `brew install age` | `gpg` |
| `decrypt` | `age` | `brew install age` | `gpg` |
| `upload` | auto-detect from URI | `brew install awscli` | `gsutil` |
| `tag` | `xattr` | Built-in (macOS) | — |
| `notify` | `osascript` (macOS), `notify-send` (Linux), `BurntToast` (Windows) | Built-in (macOS); `apt install libnotify-bin` (Linux); `Install-Module BurntToast` (Windows) | HTTP webhook |
| `extract` | Go stdlib | Built-in | `tar` for .tar.xz only |
| `open` | `open` | Built-in (macOS) | — |
| `content`/`content_regex` (PDF) | `pdftotext` | `brew install poppler` | — |
| `unquarantine` | `xattr` | Built-in (macOS) | — |

The `extract` action handles `.zip`, `.tar`, `.tar.gz`/`.tgz`, and `.tar.bz2` natively (Go stdlib). Only `.tar.xz` requires the external `tar` command. macOS metadata (`__MACOSX`, `._*` resource forks, `.DS_Store`) is automatically stripped during extraction. For other archive formats (`.rar`, `.7z`, etc.), use `exec`:

```yaml
  - name: extract-rar
    match:
      extensions: [.rar]
    action:
      type: exec
      command: "unrar x '{{.Path}}' ~/Downloads/Extracted/"
```

Actions that require a missing tool will fail with a clear error message indicating which tool to install. Use the `tool` field in your rule to override the default:

```yaml
  - name: encrypt-with-gpg
    match:
      glob: "confidential-*"
    action:
      type: encrypt
      tool: gpg                    # use gpg instead of the default (age)
      recipient: user@example.com
      dest: ~/Encrypted/{{.Name}}{{.Ext}}.gpg

  - name: resize-with-imagemagick
    match:
      extensions: [.jpg, .png]
    action:
      type: resize
      tool: convert                # use ImageMagick instead of the default (sips)
      width: 1920
      dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}
```

## Platform Support

sortie builds and runs on macOS, Linux, and Windows. The watcher itself (`sortie watch`) is cross-platform. A few action types are macOS-only and will error on other platforms:

- `open` — uses macOS `open(1)`
- `tag` — uses macOS Finder tags via `xattr`
- `unquarantine` — macOS-specific extended attribute (no-op on Linux/Windows)

`notify` and `exec` work on all platforms: `exec` runs commands through `sh` on Unix and `cmd.exe` on Windows; `notify` uses `osascript` (macOS), `notify-send` (Linux), and `BurntToast` on Windows with a stderr fallback.

## Running as a Service (macOS)

> Looking for **Linux systemd** or **Windows Task Scheduler** setup, or detailed troubleshooting per platform? See the **[Running as a Service](docs/guide/06-running-as-a-service.md)** guide page.

To run sortie automatically in the background on macOS, install it as a launchd user agent:

```
make install-launchd
```

This creates a plist at `~/Library/LaunchAgents/com.msjurset.sortie.plist` that starts `sortie watch` at login and keeps it running.

The watch command monitors its own binary for changes — after running `make deploy`, the daemon detects the new binary, exits gracefully, and launchd's `KeepAlive` automatically relaunches with the updated version. No manual restart needed for binary updates.

Config changes are picked up automatically in watch mode — sortie detects modifications to `config.yaml` and per-directory `.sortie.yaml` files and reloads rules, ignore patterns, and the watched directory list without restarting.

### Managing the service

```bash
# Check if it's running
sortie status

# View logs
tail -f ~/.config/sortie/logs/sortie.log

# Deploy (daemon auto-restarts when it detects the new binary)
make deploy

# Manual stop
launchctl unload ~/Library/LaunchAgents/com.msjurset.sortie.plist

# Manual start
launchctl load ~/Library/LaunchAgents/com.msjurset.sortie.plist

# Uninstall the service
make uninstall-launchd
```

## Build

```
make build
```

Run tests:

```
make test
```

Cross-compile release binaries:

```
make release VERSION=1.0.0
```

## Need help?

Hit a wall? The **[Troubleshooting guide](docs/guide/05-troubleshooting.md)** has decision trees for the three biggest categories (rule isn't matching, daemon won't stay up, performance) plus 15+ symptom→fix entries. If your problem isn't there, file an issue at https://github.com/msjurset/sortie/issues with the artifacts listed in the [bug-report checklist](docs/guide/05-troubleshooting.md#filing-a-useful-bug-report).

## License

MIT
