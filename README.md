# sortie

Intelligent file dispatcher — rule-based file routing for directories like `~/Downloads` and `~/Desktop`.

## Features

- **Rule-based matching** — match files by extension, glob, regex, size, age, or MIME type
- **22 action types** — move, copy, rename, delete, compress, extract, symlink, chmod, checksum, exec, notify, convert, resize, watermark, ocr, encrypt, decrypt, upload, tag, open, deduplicate, unquarantine
- **Hybrid config** — central `~/.config/sortie/config.yaml` plus per-directory `.sortie.yaml` overrides
- **Watch mode** — real-time file monitoring with fsnotify and configurable debounce
- **Dry-run mode** — preview what would happen before committing
- **Undo** — reverse recent actions from the history log
- **Template destinations** — use `{{.Year}}`, `{{.Month}}`, `{{.Name}}`, `{{.Ext}}`, `{{.Path}}` in dest paths and action fields
- **Trash management** — deleted files go to trash, not oblivion
- **Action chaining** — run multiple actions per rule in sequence (e.g., notify then move)
- **First-match-wins** — per-directory rules evaluate before global rules

## Install

```
make deploy
```

This builds the binary, installs it to `~/.local/bin/`, installs the man page, and sets up zsh completions.

## Usage

```
sortie [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `scan [directory...]` | Scan directories and apply rules |
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
| `validate [directory...]` | Check rules for errors and potential problems |
| `man` | Display manual page |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/sortie/config.yaml` | Config file path |
| `-v, --verbose` | `false` | Verbose output |
| `--version` | — | Show version |

### Scan Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview actions without executing |

### Watch Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Log actions without executing |
| `--debounce` | `500ms` | Debounce duration for file events |

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

### Central Config (`~/.config/sortie/config.yaml`)

```yaml
directories:
  - path: ~/Downloads
    recursive: false
  - path: ~/Desktop
    recursive: false

rules:
  - name: images-to-photos
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
```

### Per-Directory Config (`~/Downloads/.sortie.yaml`)

```yaml
rules:
  - name: pdfs-to-documents
    match:
      extensions: [.pdf]
    action:
      type: move
      dest: ~/Documents/PDFs/{{.Year}}-{{.Month}}
```

Per-directory rules take precedence over global rules. All match conditions use AND logic. First matching rule wins.

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

### Match Conditions

| Field | Description | Example |
|-------|-------------|---------|
| `extensions` | File extensions | `[.jpg, .png]` |
| `glob` | Filename glob | `Screenshot*` |
| `regex` | Regex on filename | `(?i)docker\|vscode` |
| `min_size` / `max_size` | Size threshold | `500MB`, `1GB` |
| `min_age` / `max_age` | Age threshold | `30d`, `2h` |
| `mime_type` | MIME type prefix | `image/`, `application/pdf` |

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
| `notify` | `osascript` | Built-in (macOS) | HTTP webhook |
| `extract` | Go stdlib | Built-in | `tar` for .tar.xz only |
| `open` | `open` | Built-in (macOS) | — |
| `unquarantine` | `xattr` | Built-in (macOS) | — |

The `extract` action handles `.zip`, `.tar`, `.tar.gz`/`.tgz`, and `.tar.bz2` natively (Go stdlib). Only `.tar.xz` requires the external `tar` command. For other archive formats (`.rar`, `.7z`, etc.), use `exec`:

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

## Running as a Service (macOS)

To run sortie automatically in the background, install it as a launchd user agent:

```
make install-launchd
```

This creates a plist at `~/Library/LaunchAgents/com.msjurset.sortie.plist` that starts `sortie watch` at login and keeps it running.

The watch command monitors its own binary for changes — after running `make deploy`, the daemon detects the new binary, exits gracefully, and launchd's `KeepAlive` automatically relaunches with the updated version. No manual restart needed for binary updates.

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

# Restart after config changes (binary didn't change, so manual reload needed)
launchctl unload ~/Library/LaunchAgents/com.msjurset.sortie.plist
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

## License

MIT
