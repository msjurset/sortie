# sortie

Intelligent file dispatcher — rule-based file routing for directories like `~/Downloads` and `~/Desktop`.

## Features

- **Rule-based matching** — match files by extension, glob, regex, size, age, or MIME type
- **Multiple actions** — move, copy, rename, delete (to trash), compress (gzip)
- **Hybrid config** — central `~/.config/sortie/config.yaml` plus per-directory `.sortie.yaml` overrides
- **Watch mode** — real-time file monitoring with fsnotify and configurable debounce
- **Dry-run mode** — preview what would happen before committing
- **Undo** — reverse recent actions from the history log
- **Template destinations** — use `{{.Year}}`, `{{.Month}}`, `{{.Name}}`, `{{.Ext}}` in dest paths
- **Trash management** — deleted files go to trash, not oblivion
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
| `rules` | List configured rules |
| `rules test <file>` | Show which rule matches a file |
| `config` | Show resolved configuration |
| `config init` | Create a starter config file |
| `config path` | Print config file path |
| `status` | Show watcher daemon status |
| `trash` | List files in trash |
| `trash purge` | Permanently delete all trashed files |
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

### Match Conditions

| Field | Description | Example |
|-------|-------------|---------|
| `extensions` | File extensions | `[.jpg, .png]` |
| `glob` | Filename glob | `Screenshot*` |
| `regex` | Regex on filename | `(?i)docker\|vscode` |
| `min_size` / `max_size` | Size threshold | `500MB`, `1GB` |
| `min_age` / `max_age` | Age threshold | `30d`, `2h` |
| `mime_type` | MIME type prefix | `image/`, `application/pdf` |

### Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Name}}` | Filename without extension | `report` |
| `{{.Ext}}` | Extension with dot | `.pdf` |
| `{{.Year}}` | 4-digit year | `2026` |
| `{{.Month}}` | 2-digit month | `03` |
| `{{.Day}}` | 2-digit day | `18` |
| `{{.Date}}` | YYYY-MM-DD | `2026-03-18` |
| `{{.Time}}` | HH-MM-SS | `14-30-00` |

## Running as a Service (macOS)

To run sortie automatically in the background, install it as a launchd user agent:

```
make install-launchd
```

This creates a plist at `~/Library/LaunchAgents/com.msjurset.sortie.plist` that starts `sortie watch` at login and keeps it running.

### Managing the service

```bash
# Check if it's running
sortie status

# View logs
tail -f ~/.config/sortie/logs/sortie.log

# Stop the service
launchctl unload ~/Library/LaunchAgents/com.msjurset.sortie.plist

# Start the service
launchctl load ~/Library/LaunchAgents/com.msjurset.sortie.plist

# Restart after config changes
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
