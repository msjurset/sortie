# Cookbook

Copy-pasteable recipes. Each one follows the same shape:

- **Goal** — what the recipe accomplishes
- **YAML** — drop under `rules:` in your config and edit paths
- **Notes** — platform caveats, gotchas, and when to use the `cooldown:` or `continue:` modifiers

All recipes assume you've read [Getting Started](01-getting-started.md) and [Concepts](02-concepts.md). When in doubt, run with `--dry-run` first.

---

## Basic filing

### Route downloads by extension

**Goal:** move images, archives, and PDFs into type-specific folders.

```yaml
- name: route-images
  match:
    extensions: [.jpg, .jpeg, .png, .heic, .gif, .webp]
  action:
    type: move
    dest: ~/Pictures/Inbox/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}

- name: route-archives
  match:
    extensions: [.zip, .tar, .tar.gz, .tgz, .tar.bz2]
  action:
    type: move
    dest: ~/Downloads/Archives/{{.Name}}{{.Ext}}

- name: route-pdfs
  match:
    extensions: [.pdf]
  action:
    type: move
    dest: ~/Documents/PDFs/{{.Year}}/{{.Name}}{{.Ext}}
```

**Notes:** Parent directories are created automatically. Cross-filesystem moves fall back to copy+delete, which is transparent but slightly slower.

### Mirror a folder without moving originals

**Goal:** keep a synchronized backup of every invoice PDF that lands in Downloads.

```yaml
- name: backup-invoices
  match:
    extensions: [.pdf]
    content: "invoice"
  action:
    type: copy
    dest: ~/Backups/Invoices/{{.Name}}{{.Ext}}
```

**Notes:** `content: "invoice"` matches case-insensitively and reads the first 64 KB by default. For PDFs, `pdftotext` is used automatically if installed — without it, the match is against the raw PDF bytes and won't find the word.

### Rename with an ISO date prefix

**Goal:** turn `IMG_1234.jpg` into `2026-04-24_IMG_1234.jpg` so files sort chronologically in any file manager.

```yaml
- name: date-prefix-images
  match:
    extensions: [.jpg, .jpeg, .png]
  action:
    type: rename
    dest: ~/Downloads/{{.Date}}_{{.Name}}{{.Ext}}
```

**Notes:** `rename` uses `os.Rename` with the literal destination you give it, so hardcode the parent directory (there's no `{{.Dir}}` template variable). If you want to move across directories too, use `move` instead — it works identically but doesn't imply "same directory."

### Link config files into a central directory

**Goal:** keep `.conf` files where they live on disk, but surface them all under `~/configs/` for easy review.

```yaml
- name: link-configs
  match:
    glob: "*.conf"
  action:
    type: symlink
    dest: ~/configs/{{.Name}}{{.Ext}}
```

**Notes:** Undoing removes the symlink, not the original file.

---

## Cleanup & space management

### Expire old temp files

**Goal:** auto-trash `.tmp` and `.bak` files older than 30 days.

```yaml
- name: expire-temp-files
  match:
    extensions: [.tmp, .bak, .partial]
    min_age: 30d
  action:
    type: delete
```

**Notes:** Deletes go to `~/.config/sortie/trash/`, not `/dev/null`. Restore with `sortie undo` or `sortie trash list` + manual move. Purge everything with `sortie trash purge`.

### Gzip-compress old logs

**Goal:** compress `.log` files that haven't been touched in a week, then forget about them.

```yaml
- name: compress-old-logs
  match:
    extensions: [.log]
    min_age: 7d
  action:
    type: compress
    dest: ~/Archives/Logs/{{.Name}}{{.Ext}}.gz
```

**Notes:** `compress` gzips and removes the original in one step. The output has a `.gz` extension; undo decompresses and restores.

### Deduplicate incoming downloads

**Goal:** if an identical PDF already lives in `~/Documents/`, leave the new copy alone; otherwise move it in.

```yaml
- name: dedup-downloads
  match:
    extensions: [.pdf, .zip]
  action:
    type: deduplicate
    dest: ~/Documents/{{.Name}}{{.Ext}}
    on_duplicate: skip
```

**Notes:** Identity is by SHA-256 over full file contents. `on_duplicate: delete` removes the source instead of leaving it — use only when you're sure you want the aggressive behavior.

---

## Archives

### Auto-extract and clean up

**Goal:** extract zips into a per-name subdirectory, then delete the archive.

```yaml
- name: extract-zips
  match:
    extensions: [.zip, .tar.gz, .tgz, .tar.bz2]
  continue: true
  action:
    type: extract
    dest: ~/Downloads/Extracted/{{.Name}}

- name: delete-extracted-archives
  match:
    extensions: [.zip, .tar.gz, .tgz, .tar.bz2]
  action:
    type: delete
```

**Notes:** The first rule extracts and sets `continue: true` so the second rule also fires and trashes the archive. macOS resource forks (`__MACOSX`, `._*`, `.DS_Store`) are stripped automatically. `.tar.xz` needs the `tar` command installed.

---

## Media processing

### Resize big photos

**Goal:** create a 1920-pixel-wide copy of any photo larger than 5 MB, leaving the original untouched.

```yaml
- name: resize-photos
  match:
    extensions: [.jpg, .jpeg, .png]
    min_size: 5MB
  action:
    type: resize
    width: 1920
    dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}
```

**Notes:** Defaults to `sips` (built into macOS). On Linux/Windows, set `tool: convert` to use ImageMagick, or install `sips` via MacPorts. Specify `width`, `height`, or `percentage` — at least one is required.

### Watermark portfolio images

**Goal:** stamp every image matching `portfolio-*` with a logo in the bottom-right corner.

```yaml
- name: watermark-portfolio
  match:
    extensions: [.jpg, .png]
    glob: "portfolio-*"
  action:
    type: watermark
    overlay: ~/brand/watermark.png
    gravity: southeast
    dest: ~/Pictures/Watermarked/{{.Name}}{{.Ext}}
```

**Notes:** Requires ImageMagick (`brew install imagemagick` / `apt install imagemagick`). Use a transparent PNG for the overlay; `gravity` accepts `center`, `north`, `south`, `east`, `west`, and the four corners.

### Transcode videos with ffmpeg

**Goal:** convert `.mov` / `.avi` / `.mkv` to web-friendly H.264 MP4.

```yaml
- name: transcode-videos
  match:
    extensions: [.mov, .avi, .mkv]
  action:
    type: convert
    tool: ffmpeg
    args: "-i {{.Path}} -c:v libx264 -crf 23 -c:a aac {{.Dest}}"
    dest: ~/Videos/Converted/{{.Name}}.mp4
```

**Notes:** `{{.Dest}}` refers to the template-expanded output path. The source is preserved — safe to tune `-crf` and rerun.

### OCR scanned images

**Goal:** extract text from scanned images into a `.txt` sidecar.

```yaml
- name: ocr-scans
  match:
    extensions: [.png, .tiff]
    glob: "scan-*"
  action:
    type: ocr
    language: eng
    dest: ~/Documents/OCR/{{.Name}}.txt
```

**Notes:** Requires `tesseract`. For non-English content, install a language pack (`brew install tesseract-lang` or `apt install tesseract-ocr-<lang>`) and set `language:` accordingly.

---

## Permissions & integrity

### Make scripts executable

**Goal:** every `.sh` dropped into `~/Downloads` becomes `0755`.

```yaml
- name: make-scripts-executable
  match:
    extensions: [.sh]
  action:
    type: chmod
    mode: "0755"
```

**Notes:** The original mode is stored in history; undo restores it. On Windows, POSIX permissions are limited — the action runs but effect is minimal.

### Write a sidecar hash for large downloads

**Goal:** drop a `file.sha256` next to every download over 100 MB.

```yaml
- name: hash-big-downloads
  match:
    min_size: 100MB
  action:
    type: checksum
    algorithm: sha256
```

**Notes:** The sidecar is in BSD format (`sha256 (file) = <hash>`), readable by `shasum -c`. Supports `sha256` (default), `md5`, and `sha1`.

---

## Security

### Auto-encrypt outgoing files

**Goal:** anything matching `confidential-*` in `~/Outbox/` gets age-encrypted before being uploaded anywhere.

```yaml
- name: encrypt-outbox
  match:
    glob: "confidential-*"
  action:
    type: encrypt
    recipient: "age1abcdefg..."    # your recipient's public key
    dest: ~/Outbox/Encrypted/{{.Name}}{{.Ext}}.age
```

**Notes:** `age` is the default tool (`brew install age`). Set `tool: gpg` and use an email address or key ID for GPG instead. The source file is preserved — chain with `delete` if you want to remove the plaintext after encryption.

### Auto-decrypt an inbox

**Goal:** anything dropping into `~/EncryptedInbox/` with `.age` extension is decrypted next to it.

```yaml
- name: decrypt-inbox
  match:
    extensions: [.age]
  action:
    type: decrypt
    key: ~/.age/identity.txt
    dest: ~/Inbox/{{.Name}}
```

**Notes:** The plaintext lands at `dest`; the encrypted source is preserved. Chain with `delete` or `move` if you want to archive or trash the `.age` file afterwards.

### Unquarantine trusted installers *(macOS)*

**Goal:** strip Gatekeeper's quarantine bit from installers you explicitly trust (e.g., ones you build yourself).

```yaml
- name: unquarantine-internal-builds
  match:
    extensions: [.dmg, .pkg]
    glob: "internal-build-*"
  action:
    type: unquarantine
```

**Notes:** No-op on Linux/Windows. Only use this for files from sources you fully trust — quarantine is a meaningful security signal. Non-reversible.

---

## Integrations & side effects

### rsync new files to a NAS

**Goal:** push every new photo to a network share.

```yaml
- name: rsync-to-nas
  match:
    extensions: [.jpg, .raw, .heic]
  action:
    type: exec
    command: "rsync -a '{{.Path}}' nas.local:/volume1/photos/"
```

**Notes:** `exec` runs through `sh -c` on Unix and `cmd /c` on Windows. Quote `{{.Path}}` with single quotes to handle filenames with spaces on Unix; on Windows, use double quotes and be aware that `cmd.exe` has different escaping rules. Not reversible.

### Toast when an invoice arrives

**Goal:** desktop notification including the captured vendor and date.

```yaml
- name: notify-on-invoice
  match:
    extensions: [.pdf]
    content_regex: '(?P<company>Acme|WidgetCo).*?(?P<date>\d{4}-\d{2}-\d{2})'
  action:
    type: notify
    title: "Invoice received"
    message: '{{.Match.company}} — {{or .Match.date "undated"}}'
```

**Notes:** On macOS uses `osascript`; on Linux requires `notify-send` (`libnotify-bin`); on Windows prefers `BurntToast` and falls back to writing to stderr if the module isn't installed.

### POST new-file events to a webhook

**Goal:** send JSON metadata to a pipeline URL on every new PDF.

```yaml
- name: webhook-on-pdf
  match:
    extensions: [.pdf]
  action:
    type: notify
    title: "sortie"
    message: "https://automate.example.com/sortie/pdf-arrived"
```

**Notes:** If `message` starts with `http://` or `https://`, sortie does an HTTP POST with `{title, file, path, size}` as JSON instead of showing a desktop notification. Cross-platform.

### Upload nightly reports to S3

**Goal:** push `.pdf` reports under a year-partitioned S3 prefix.

```yaml
- name: backup-reports-to-s3
  match:
    extensions: [.pdf]
    glob: "report-*"
  cooldown: 2s
  action:
    type: upload
    remote: "s3://my-bucket/reports/{{.Year}}/{{.Name}}{{.Ext}}"
```

**Notes:** Auto-detects the tool from the URI scheme (`s3://` → `aws`, `gs://` → `gsutil`). Configure credentials first (`aws configure`). `cooldown:` prevents runaway uploads if the watcher is flooded. Not reversible.

### Apply Finder tags to receipts *(macOS)*

**Goal:** color-code PDF receipts red in Finder.

```yaml
- name: tag-receipts
  match:
    extensions: [.pdf]
    regex: "(?i)receipt"
  action:
    type: tag
    tags: [Red, Finance]
```

**Notes:** macOS-only (uses `xattr` with the Finder tag plist). Standard tag colors: `Red`, `Orange`, `Yellow`, `Green`, `Blue`, `Purple`, `Gray`. Custom names work and appear in Finder. Not reversible.

### Auto-open disk images in Preview *(macOS)*

**Goal:** when a `.dmg` appears, mount it by opening it in Finder.

```yaml
- name: open-dmgs
  match:
    extensions: [.dmg]
  action:
    type: open
```

**Notes:** Uses macOS `open(1)`. Set `app: VLC` to force a specific application. macOS-only; returns a clear error on Linux/Windows. Not reversible.

---

## Multi-step chains

Chains combine several actions into a single rule. They run in order and share a `chain_id` in history, so `sortie undo <chain_id>` reverses the whole thing (to the extent each step is reversible). **Remember the rule from [Concepts](02-concepts.md#actions-and-chains):** chains don't roll back on partial failure. Put reversible, structural work first and side effects last.

### Download triage

**Goal:** when an archive lands, extract it, tag with "imported," and notify — all in one pass.

```yaml
- name: download-triage
  match:
    extensions: [.zip, .tar.gz, .tgz]
  actions:
    - type: extract
      dest: ~/Downloads/Extracted/{{.Name}}
    - type: tag
      tags: [Blue, imported]
    - type: notify
      title: "Archive imported"
      message: "{{.Name}}{{.Ext}}"
```

**Notes:** `tag` is macOS-only; on Linux/Windows, it errors and the chain stops before `notify`. If `extract` fails on a corrupt archive, `tag` and `notify` do not run but the archive stays where it is. `deduplicate` intentionally isn't in this chain — it has three possible outcomes (`skip`, `move`, `delete`) and always consumes the source file on a "move," which would prevent the rest of the chain from finding anything to operate on. If you want dedup too, use it as a standalone rule with a higher priority and accept that non-duplicates will be moved to the dedup archive (and the triage rule won't fire on them).

### Invoice intake with capture-group filing

**Goal:** parse the company and date out of invoice PDFs, rename by that, move to a vendor folder, and notify.

```yaml
- name: invoice-intake
  match:
    extensions: [.pdf]
    content_regex: '(?P<company>Acme|WidgetCo|GlobalTech).*?(?P<date>\d{4}-\d{2}-\d{2}|[A-Z][a-z]+ \d{1,2}, \d{4}|\d{1,2}/\d{1,2}/\d{4})'
  actions:
    - type: move
      dest: '~/Documents/Invoices/{{.Match.company}}/{{or .Match.date "undated"}}-{{.Name}}{{.Ext}}'
    - type: notify
      title: "Invoice filed"
      message: '{{.Match.company}} · {{or .Match.date "undated"}}'
```

**Notes:** The `date` capture is auto-normalized to `YYYY-MM-DD` when it matches common formats. `{{or .Match.field "fallback"}}` handles missing optional captures. If `move` fails (permission error, full disk), `notify` does not run.

### Photo import pipeline

**Goal:** rename photos to ISO-prefixed, generate a resized web version, then move the original to a year/month folder.

```yaml
- name: photo-import
  match:
    extensions: [.jpg, .jpeg, .heic]
    min_size: 2MB
  actions:
    - type: resize
      width: 1920
      dest: ~/Pictures/Web/{{.Date}}_{{.Name}}{{.Ext}}
    - type: move
      dest: ~/Pictures/Originals/{{.Year}}/{{.Month}}/{{.Date}}_{{.Name}}{{.Ext}}
```

**Notes:** The resize step runs against the source path, so it must come before the move. If resize fails (unsupported format, `sips` missing), move doesn't run and the photo stays in its inbox.

### Screenshot tidy

**Goal:** file screenshots into a year/month archive and toast you about it.

```yaml
- name: screenshot-tidy
  match:
    glob: "Screenshot*"
    extensions: [.png, .jpg]
  actions:
    - type: move
      dest: ~/Pictures/Screenshots/{{.Year}}/{{.Month}}/{{.Date}}_{{.Time}}{{.Ext}}
    - type: notify
      title: "Screenshot saved"
      message: "{{.Year}}/{{.Month}} · {{.Time}}"
```

**Notes:** `{{.Date}}_{{.Time}}` gives a unique filename for multiple screenshots in the same day. If the move fails, the notify step doesn't run and the screenshot stays where it landed.

### Log rotator

**Goal:** when a log crosses 100 MB, archive it locally, ship it to S3, and clear out the original.

```yaml
# In config.yaml:
directories:
  - path: /var/log/myapp
    watch_existing: true

rules:
  - name: rotate-large-logs
    match:
      extensions: [.log]
      min_size: 100MB
    cooldown: 1m
    actions:
      - type: copy
        dest: ~/Archives/Logs/{{.Name}}-{{.Date}}{{.Ext}}
      - type: upload
        remote: "s3://logs-bucket/{{.Year}}/{{.Month}}/{{.Name}}-{{.Date}}.log"
      - type: delete
```

**Notes:** `watch_existing: true` is the key — without it, the rule only fires on newly-created files, not on writes to existing ones. `cooldown: 1m` prevents the rule from re-firing while the log is still being written to. The chain is ordered safely: `copy` runs first so a local archive exists, `upload` ships the original (not the archive), and `delete` only runs if both earlier steps succeeded. If `upload` fails, the chain stops and the archive copy is still on disk — `sortie validate` will warn you if you try to chain `compress` before `upload`, since compress removes the source out from under later steps.

---

## Where to go next

- [Reference](04-reference.md) — quick-lookup tables for flags, match conditions, actions, and template variables.
- [Troubleshooting](05-troubleshooting.md) — when a rule doesn't match, an action fails silently, or the daemon won't stay up.
- Stuck? Open an issue at https://github.com/msjurset/sortie/issues with your YAML and a dry-run output.
