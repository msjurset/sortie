# Cookbook

Working recipes for every action type, plus multi-step chains for real workflows. Each recipe follows a consistent shape:

- **When to reach for this** — the situation it solves
- **YAML** — the rule, ready to paste
- **What happens** — concrete walk-through of an example file flowing through it
- **Variations** — adaptations for related cases
- **Gotchas** — failure modes specific to this action
- **Notes** — platform caveats, undo behavior, performance considerations

Every recipe was checked with `sortie validate` against current sortie. Run that command after pasting and editing — it will catch chain ordering bugs (e.g. an action that consumes the source before a later action needs it) before the daemon does.

---

## Table of contents

- **Basic filing** — [route by extension](#route-downloads-by-extension), [mirror without moving](#mirror-a-folder-without-moving-originals), [date-prefix rename](#rename-with-an-iso-date-prefix), [symlink farm](#link-config-files-into-a-central-directory)
- **Cleanup & space** — [expire old temp files](#expire-old-temp-files), [compress old logs](#gzip-compress-old-logs), [deduplicate](#deduplicate-incoming-downloads)
- **Archives** — [auto-extract](#auto-extract-and-clean-up)
- **Media** — [resize](#resize-big-photos), [watermark](#watermark-portfolio-images), [transcode video](#transcode-videos-with-ffmpeg), [OCR](#ocr-scanned-images)
- **Permissions & integrity** — [chmod](#make-scripts-executable), [checksum](#write-a-sidecar-hash-for-large-downloads)
- **Security** — [encrypt](#auto-encrypt-outgoing-files), [decrypt](#auto-decrypt-an-inbox), [unquarantine](#unquarantine-trusted-installers-macos)
- **Integrations** — [exec / rsync](#rsync-new-files-to-a-nas), [desktop notify](#toast-when-an-invoice-arrives), [webhook](#post-new-file-events-to-a-webhook), [upload](#upload-nightly-reports-to-s3), [Finder tag](#apply-finder-tags-to-receipts-macos), [auto-open](#auto-open-disk-images-macos)
- **Multi-step chains** — [download triage](#download-triage), [invoice intake](#invoice-intake-with-capture-group-filing), [photo import](#photo-import-pipeline), [screenshot tidy](#screenshot-tidy), [log rotator](#log-rotator), [searchable file cabinet](#searchable-file-cabinet)

---

## Basic filing

### Route downloads by extension

**When to reach for this:** the bread-and-butter rule. You've got a `~/Downloads` folder that's a junk drawer, and you want each file routed to a sensible permanent home as soon as it arrives.

**YAML:**

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

**What happens:** drop `IMG_4521.heic` (saved 2026-04-25) into `~/Downloads/`. The watcher fires, the `route-images` rule matches first because its extension is in the list. The destination template expands to `~/Pictures/Inbox/2026/04/IMG_4521.heic`. Parent directories `~/Pictures/Inbox/2026/04/` are created automatically. The file moves; `sortie history` records `move` with `src=~/Downloads/IMG_4521.heic` and `dest=~/Pictures/Inbox/2026/04/IMG_4521.heic`. Total elapsed time: a few milliseconds.

If a file with the same name already lives at the destination, sortie appends a `_001`, `_002`, … counter to avoid clobbering — `IMG_4521_001.heic`, `IMG_4521_002.heic`, and so on, up to 999.

**Variations:**

- **Year-only partitioning:** drop `/{{.Month}}` from the dest if a single folder per year is enough.
- **By media kind, not by date:** `~/Pictures/Inbox/RAW/` for `.cr3`/`.arw`/`.dng` extensions; combine with a separate rule for processed JPGs.
- **Cross-volume routing:** the same template works for an external drive (`/Volumes/Photos/...` on macOS, `D:\Photos\...` on Windows) — sortie falls back to copy+delete for cross-filesystem moves automatically.
- **Default catch-all:** add a final rule with `glob: "*"` and a low priority that routes everything else into `~/Downloads/Misc/` so nothing slips past.

**Gotchas:**

- The watcher ignores in-progress download suffixes (`.crdownload`, `.part`, `.partial`, `.download`, `.tmp`), so a 5 GB download won't trigger this rule until the browser does its final rename.
- If your `dest:` path is a directory (no trailing filename or extension), sortie appends the original filename. `dest: ~/Pictures/Inbox/` works the same as `dest: ~/Pictures/Inbox/{{.Name}}{{.Ext}}`.
- Match conditions AND together. `extensions: [.pdf]` plus `min_size: 1MB` on the same rule rejects all small PDFs — make sure that's what you want.

**Notes:** undo restores the file to its original location. `sortie undo --last 1` reverses the most recent move; `sortie undo <id>` reverses a specific record from `sortie history`.

---

### Mirror a folder without moving originals

**When to reach for this:** you want a separate backup copy of certain files (invoices, contracts, source-of-truth documents) but the originals must stay where they live for an external app to find them.

**YAML:**

```yaml
- name: backup-invoices
  match:
    extensions: [.pdf]
    content: "invoice"
  action:
    type: copy
    dest: ~/Backups/Invoices/{{.Year}}/{{.Name}}{{.Ext}}
```

**What happens:** sortie matches PDFs whose extracted text contains the word "invoice" (case-insensitive). For each match, it copies the file to the date-partitioned backup folder. The original is untouched — useful when an accounting app expects the file in a specific inbox. File permissions are preserved on the copy.

**Variations:**

- **Dated backups:** `dest: ~/Backups/Invoices/{{.Date}}_{{.Name}}{{.Ext}}` keeps every version including the date the copy was taken.
- **Multiple destinations:** chain two rules, each copying to a different target, using `continue: true` on the first to ensure both run. Or use a single chain of two `copy` actions.
- **By content regex with capture:** swap `content: "invoice"` for `content_regex: '(?P<vendor>Acme|Globex)'` and use `{{.Match.vendor}}` in the dest path.

**Gotchas:**

- `copy` runs every time the file is seen, so under `sortie watch` with `watch_existing: true` the destination will keep getting overwritten as the file is edited. Add a `cooldown:` if that's not what you want.
- For PDFs, `content` matching needs `pdftotext` (poppler) installed. Without it the substring match runs against raw PDF bytes and almost never finds words.

**Notes:** undo deletes the copy; it does not touch the original. Reversible.

---

### Rename with an ISO date prefix

**When to reach for this:** you have files with arbitrary names (`Screenshot 2026-04-25 at 14.30.21.png`, `IMG_4521.jpg`) and you want them sortable chronologically by name in any file manager.

**YAML:**

```yaml
- name: date-prefix-images
  match:
    extensions: [.jpg, .jpeg, .png]
  action:
    type: rename
    dest: ~/Downloads/{{.Date}}_{{.Name}}{{.Ext}}
```

**What happens:** `IMG_4521.jpg` becomes `2026-04-25_IMG_4521.jpg` in the same directory. The date comes from the file's mtime, not the current wall clock — so files saved yesterday get yesterday's prefix, even if you only run sortie today.

**Variations:**

- **Prefix with capture data:** for invoices with a known vendor, use `dest: ~/Documents/{{.Match.vendor}}-{{.Name}}{{.Ext}}`. Requires `content_regex` with a `vendor` capture in the match block.
- **Date + time for collision-free per-second naming:** `dest: ~/Downloads/{{.Date}}_{{.Time}}_{{.Name}}{{.Ext}}`.
- **Strip an existing prefix:** `rename` doesn't transform `{{.Name}}` — it uses it literally. Use `exec` with `mv` and a regex if you need to strip and re-apply prefixes.

**Gotchas:**

- `rename` calls `os.Rename` with the literal expanded dest. There's no `{{.Dir}}` template variable, so you must hardcode the parent directory. If you need to move across directories too, use `move` instead — it's identical in semantics but doesn't imply "same directory."
- If the file already has the date prefix from a previous run, you'll get `2026-04-25_2026-04-25_IMG_4521.jpg`. Either guard with a `regex` match condition (`regex: '^(?!\d{4}-\d{2}-\d{2}_)'`) or accept the idempotent-but-ugly form.

**Notes:** undo restores the original name. Reversible.

---

### Link config files into a central directory

**When to reach for this:** your dotfiles live with their respective tools (`~/.gitconfig`, `~/.config/nvim/init.vim`, etc.) but you want them surfaced in a central `~/configs/` for review, version control, or syncing.

**YAML:**

```yaml
- name: link-configs
  match:
    glob: "*.conf"
  action:
    type: symlink
    dest: ~/configs/{{.Name}}{{.Ext}}
```

**What happens:** for every `*.conf` file in a watched directory, sortie creates a symlink at `~/configs/<name>.conf` pointing back at the original. Edit either the original or the symlink — both reflect the change because they're the same inode.

**Variations:**

- **Per-application directories:** `dest: ~/configs/{{.Name | toLower}}/...` — except sortie doesn't have `toLower` in the template func map, so just choose a uniform layout.
- **Inverted pattern:** put the file under `~/configs/` and link **back** to where the tool expects it. Easier with regular file management; sortie isn't ideal for that direction.
- **Read-only tracking:** combine with `chmod 0444` on the original via a chain to make the linked source read-only.

**Gotchas:**

- Symlinks across filesystems work fine on Unix but don't have a meaningful equivalent on Windows for non-admin users — `mklink` requires elevation. Run `sortie watch` as Administrator on Windows or stick to copy/move actions.
- Undo removes the symlink, not the original file.

**Notes:** the symlink is created with `os.Symlink`, which fails if the destination already exists. If the link already exists from a previous run, the action errors. Pair with `delete` first if you need to refresh.

---

## Cleanup & space management

### Expire old temp files

**When to reach for this:** scratch directories accumulate `.tmp`, `.bak`, and `.partial` files that nothing cleans up. You want a recurring "trash anything stale" rule.

**YAML:**

```yaml
- name: expire-temp-files
  match:
    extensions: [.tmp, .bak, .partial]
    min_age: 30d
  action:
    type: delete
```

**What happens:** every `.tmp`/`.bak`/`.partial` file with mtime older than 30 days is moved to `~/.config/sortie/trash/<filename>`. Restorable with `sortie undo`. Inspect the trash with `sortie trash`; empty it with `sortie trash purge`.

**Variations:**

- **By directory:** define this as a per-directory rule in `.sortie.yaml` inside the scratch dir, so it doesn't apply globally.
- **Aggressive:** drop `min_age` to `7d` for a working directory you actively manage.
- **By size:** `max_size: 100KB` plus `min_age: 7d` — only trash small stale files (assumes large stale files are intentional caches you want to keep).
- **Pair with `find`-style cleanup:** for files older than a year, chain `compress` first to keep them but reclaim space, then `delete` after another year.

**Gotchas:**

- `min_age` looks at mtime, not atime. A file you opened yesterday but haven't modified for 60 days still ages out by mtime. If you need atime semantics, `exec` with `find` is more flexible.
- The trash directory grows until you `sortie trash purge`. If disk space is the goal, set up a periodic purge (cron or another sortie rule on `~/.config/sortie/trash/` with a higher `min_age`).

**Notes:** reversible until purge. Each `delete` records the original location in history.

---

### Gzip-compress old logs

**When to reach for this:** application logs that you want to keep but don't actively read. Compress to ~10% of original size and forget about them.

**YAML:**

```yaml
- name: compress-old-logs
  match:
    extensions: [.log]
    min_age: 7d
  action:
    type: compress
    dest: ~/Archives/Logs/{{.Name}}{{.Ext}}.gz
```

**What happens:** `app-2026-04-18.log` (with mtime older than 7 days) becomes `~/Archives/Logs/app-2026-04-18.log.gz`. The original is removed. `sortie undo` decompresses and restores it.

**Variations:**

- **By size, not age:** swap `min_age: 7d` for `min_size: 100MB` when you want to compress as soon as a log gets big rather than waiting for it to age.
- **Include the date in the archive name:** `dest: ~/Archives/Logs/{{.Date}}_{{.Name}}{{.Ext}}.gz`. Useful when logs rotate with the same name (`app.log` written daily, archived).
- **Per-directory archive:** instead of a flat `~/Archives/Logs/`, partition by app: `dest: ~/Archives/{{.Name}}/{{.Date}}{{.Ext}}.gz`.

**Gotchas:**

- `compress` removes the source after successful gzip. If you chain another action after `compress`, the source path is gone — see [the validator warning](#log-rotator) for an example. Use `copy + compress + upload` or upload first, then compress.
- The output is plain gzip (`.gz`), not gzipped-tar (`.tar.gz`). It compresses a single file. For multi-file archives, use `exec` with `tar czf`.
- If `dest` already exists, the conflict resolver appends `_001`, etc.

**Notes:** reversible — undo decompresses to the original location.

---

### Deduplicate incoming downloads

**When to reach for this:** you sometimes save the same PDF or zip twice (different browsers, different mailing lists). Catch the duplicate before it adds to clutter.

**YAML:**

```yaml
- name: dedup-downloads
  match:
    extensions: [.pdf, .zip]
  action:
    type: deduplicate
    dest: ~/Documents/{{.Name}}{{.Ext}}
    on_duplicate: skip
```

**What happens:** sortie computes the SHA-256 of the source file. If a file at the dest path exists with the same hash, the source stays put (`on_duplicate: skip`) or is trashed (`on_duplicate: delete`). If the dest is missing or has different content, the source is moved to dest as if it were a regular `move`.

The history record's `dest` field encodes the outcome: `moved:/path` (no duplicate), `skip:/path` (duplicate, source kept), or `delete:/path` (duplicate, source trashed).

**Variations:**

- **Strict skip with no auto-move:** if you don't want sortie to move non-duplicates either, use a regular `move` rule first to put files where they belong, then a separate dedupe rule on the destination directory.
- **Aggressive dedup with delete:** `on_duplicate: delete` removes the new copy. Use this in append-only archives where the canonical version is already in place.
- **Content-aware dedup with renaming:** combine with `content_regex` to extract a vendor or date, and use that in the dest. Two PDFs from the same vendor on the same date are likely the same invoice; non-matching content paths produce different dest paths and bypass the hash check.

**Gotchas:**

- Hash is over the entire file contents — two PDFs that differ by a single timestamp metadata byte are NOT duplicates from sortie's perspective.
- Don't put `deduplicate` in a chain. It has three possible outcomes and on `moved` it consumes the source, breaking subsequent steps. Validator will warn you.
- For very large files, the SHA-256 is computed twice (source and dest) — there's no caching, so a 10 GB ISO check costs ~20 GB of disk reads on a system with cold cache.

**Notes:** reversible only when the outcome is `moved:` or `delete:`. `skip:` is a no-op so undo silently succeeds.

---

## Archives

### Auto-extract and clean up

**When to reach for this:** archives you download are a means to an end — you want the contents, not the `.zip`. Have sortie unpack into a sensible folder and remove the original automatically.

**YAML:**

```yaml
- name: extract-archives
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

**What happens:** when `project-1.2.3.zip` lands in Downloads, the first rule extracts it into `~/Downloads/Extracted/project-1.2.3/`. Because that rule has `continue: true`, evaluation continues; the second rule then matches the same archive and trashes it. The end state: a folder of contents, no archive littering Downloads.

**Variations:**

- **Single chain instead of two rules:**

  ```yaml
  - name: extract-and-clean
    match:
      extensions: [.zip, .tar.gz, .tgz]
    actions:
      - type: extract
        dest: ~/Downloads/Extracted/{{.Name}}
      - type: delete
  ```

  Same effect, slightly more compact. The `delete` runs only if `extract` succeeded — if the archive is corrupt, the `.zip` stays for you to investigate.

- **Inspect before extracting:** add `min_size: 1KB` to skip tiny zero-byte placeholder archives that sometimes appear during stalled downloads.
- **Per-format extraction targets:** define separate rules with narrower `extensions:` lists if you want code archives in `~/code/`, photo archives in `~/Pictures/Imports/`, etc.

**Gotchas:**

- macOS metadata (`__MACOSX/`, `._*` resource forks, `.DS_Store`) is automatically stripped. You don't need to filter it manually.
- `.zip`, `.tar`, `.tar.gz`, `.tgz`, and `.tar.bz2` work without any external tools (Go stdlib). Only `.tar.xz` requires the `tar` command on PATH.
- For `.rar`, `.7z`, and other non-standard formats, use `exec` with the appropriate tool:

  ```yaml
  action:
    type: exec
    command: "unrar x -y '{{.Path}}' '{{.Dir}}/extracted/'"
  ```

  (No `{{.Dir}}` exists — substitute the actual path.)

**Notes:** `extract` is reversible — undo deletes the extracted directory and restores the archive.

---

## Media processing

### Resize big photos

**When to reach for this:** you want a web-friendly version of every photo over some size threshold, without touching the originals.

**YAML:**

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

**What happens:** when a JPEG larger than 5 MB lands, sortie shells out to `sips` (macOS built-in) and writes a 1920-pixel-wide copy to `~/Pictures/Resized/`. Aspect ratio is preserved — height is computed automatically. The source is untouched, so you can rerun with different dimensions until you're happy.

**Variations:**

- **Use ImageMagick on Linux/Windows:**

  ```yaml
  action:
    type: resize
    width: 1920
    tool: convert
    dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}
  ```

- **Percentage instead of pixels:** `percentage: 50` halves the dimensions.
- **Hard-cap height too:** `height: 1080` plus `width: 1920` produces a 16:9 letterbox-fit (depends on tool semantics — sips fits within the box).
- **Generate both a thumbnail and a web copy:**

  ```yaml
  actions:
    - type: resize
      width: 400
      dest: ~/Pictures/Thumbs/{{.Name}}{{.Ext}}
    - type: resize
      width: 1920
      dest: ~/Pictures/Web/{{.Name}}{{.Ext}}
  ```

  Both run on the source path, which is preserved across resize calls.

**Gotchas:**

- `sips` is macOS-only. If you don't override `tool:`, this rule fails on Linux/Windows. Use `tool: convert` (ImageMagick) for portability.
- HEIC support varies by tool: macOS `sips` handles HEIC natively; ImageMagick needs `libheif` linked in. For HEIC sources on Linux, you may need to convert to JPEG first via `convert` action.
- `resize` does not strip EXIF. For a privacy-aware pipeline, chain `resize` then `exec` with `exiftool -all=` on the resized output.

**Notes:** reversible — undo deletes the resized copy. Source is preserved either way.

---

### Watermark portfolio images

**When to reach for this:** you publish photos on a portfolio site or share them externally and want a logo stamped on each before they leave the source folder.

**YAML:**

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

**What happens:** every image starting with `portfolio-` gets the contents of `~/brand/watermark.png` composited onto its bottom-right corner. The source is preserved. Output goes to a separate folder so you can publish the watermarked version while keeping the clean original.

**Variations:**

- **Center watermark:** `gravity: center` for full-frame branding.
- **Different watermarks per project:** several rules, each with a `glob:` matching a project prefix and pointing at a different overlay.
- **Variable opacity:** ImageMagick's composite operator doesn't expose opacity directly through sortie's watermark action. For fine-grained control, use `exec` with a full ImageMagick command.

**Gotchas:**

- Requires ImageMagick. Install with `brew install imagemagick` (macOS) or your distro's package manager.
- Use a transparent PNG for the overlay, not a JPEG — JPEGs don't have alpha and will paint a white box.
- `composite` is the ImageMagick legacy tool. On newer ImageMagick installs, `magick composite` works equivalently — set `tool: magick` if your install renamed the binaries.

**Notes:** reversible — undo deletes the watermarked copy.

---

### Transcode videos with ffmpeg

**When to reach for this:** consistent output formats from heterogeneous inputs. Camera produces `.mov`, screen recorder produces `.mkv`, you want H.264 MP4 for everything.

**YAML:**

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

**What happens:** sortie expands the args template (substituting `{{.Path}}` with the source and `{{.Dest}}` with the resolved output path), then invokes ffmpeg with those arguments. The source is preserved — safe to tweak `-crf` and rerun until the output looks right.

**Variations:**

- **Different codec / container:** swap `libx264` for `libx265` (HEVC) or `libvpx-vp9`. Update the dest extension to `.webm` for VP9.
- **Audio passthrough:** `-c:a copy` skips re-encoding the audio (faster, no quality loss) when the source is already AAC.
- **Multiple outputs:** chain two `convert` actions with different presets — one for fast preview, one for final.
- **Use HandBrakeCLI:** `tool: HandBrakeCLI` plus `args: "-i {{.Path}} -o {{.Dest}} --preset 'Fast 1080p30'"`.

**Gotchas:**

- The args field is a template. Anything containing `{{` or `}}` that isn't a sortie variable will confuse the template parser.
- `convert` is for file-to-file transforms. For streaming or piping, use `exec` with the full `ffmpeg | …` chain in a shell command.
- If ffmpeg fails (missing codec, malformed input), sortie returns the exit code and stderr in the history `error` field. Inspect with `sortie history -n 5`.

**Notes:** reversible — undo deletes the converted output. Source is always preserved.

---

### OCR scanned images

**When to reach for this:** you scan paper documents to PNG/TIFF and want a searchable text sidecar without manually feeding each one to an OCR tool.

**YAML:**

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

**What happens:** for every `scan-*.png` or `scan-*.tiff`, sortie runs `tesseract <source> <dest> -l eng` (with the `.txt` extension stripped because tesseract appends it itself). The result is plain text in the dest. The source image is untouched.

**Variations:**

- **Multilingual:** `language: eng+fra+deu` (requires those tesseract language packs installed).
- **PDF input:** tesseract can OCR PDFs directly if compiled with the relevant features. Otherwise, convert to TIFF first via a chain (`convert` → `ocr`).
- **Sidecar in the same directory:** drop `dest` to default to a sidecar next to the source.
- **Search-friendly text:** pipe the resulting `.txt` into Spotlight (macOS) or your text indexer of choice via a follow-up `exec` action.

**Gotchas:**

- Requires `tesseract` on PATH. Install with `brew install tesseract` plus `brew install tesseract-lang` for non-English.
- Quality depends heavily on input: low-DPI scans and skewed pages produce poor results. Pre-process with `exec` calling `convert -deskew 40% -threshold 50%` for messy scans.
- Sortie passes the source as the first arg and dest as the second; you can't currently inject custom tesseract flags. Use `exec` for `--psm` or `--oem` tuning.

**Notes:** reversible — undo deletes the `.txt` output. For a complete searchable-archive workflow (scan → OCR → searchable PDF or `.txt` sidecar in a date-organized cabinet), see the [Searchable file cabinet](#searchable-file-cabinet) chain below — it includes an Apple Vision variant that's noticeably more accurate than vanilla tesseract on phone-scan input.

---

## Permissions & integrity

### Make scripts executable

**When to reach for this:** download a `.sh` from somewhere and want to run it without remembering `chmod +x` every time.

**YAML:**

```yaml
- name: make-scripts-executable
  match:
    extensions: [.sh]
  action:
    type: chmod
    mode: "0755"
```

**What happens:** when `install-foo.sh` lands, sortie reads its current mode (probably `0644` from the browser's umask), records it in history, and changes it to `0755`. `sortie undo` restores the original mode.

**Variations:**

- **Read-only artifacts:** `mode: "0444"` for files you never want to edit accidentally.
- **Group writable:** `mode: "0664"` for shared project files in a group-owned directory.
- **Per-glob targeting:** narrow with `glob: "trusted-*"` so untrusted downloads keep their default safe mode.

**Gotchas:**

- The `mode:` value must be a quoted string in YAML, not a bare number. `mode: 0755` is parsed as octal-755 = decimal 493 by some YAML libraries, but the validator catches the type mismatch.
- POSIX permissions are limited on Windows. The action runs but only the read/write bits map meaningfully; execute bits are determined by file extension on Windows, not the mode bit.
- `chmod` doesn't change ownership. For chown semantics, use `exec`.

**Notes:** reversible — undo restores the previous mode (recorded in history).

---

### Write a sidecar hash for large downloads

**When to reach for this:** you want integrity verification on big downloads (ISOs, model weights, datasets) without piping every download through `shasum` manually.

**YAML:**

```yaml
- name: hash-big-downloads
  match:
    min_size: 100MB
  action:
    type: checksum
    algorithm: sha256
```

**What happens:** when a 4 GB ISO arrives, sortie computes its SHA-256 and writes a sidecar at `<source>.sha256`. The format is BSD: `SHA256 (filename) = <hash>`, which is what `shasum -c` expects.

```
$ cat ubuntu-24.04.iso.sha256
SHA256 (ubuntu-24.04.iso) = 9c3a0... (full 64 hex chars)
$ shasum -c ubuntu-24.04.iso.sha256
ubuntu-24.04.iso: OK
```

**Variations:**

- **MD5 for legacy compatibility:** `algorithm: md5` if you're verifying against an old upstream that publishes MD5 sums.
- **SHA-1:** `algorithm: sha1` (still in use for git but not recommended for security).
- **Custom dest:** `dest: ~/Hashes/{{.Name}}{{.Ext}}.sha256` to keep all hashes in one folder.

**Gotchas:**

- `checksum` reads the entire file once. For a 100 GB dataset, expect proportional disk read time.
- The sidecar contains the bare filename (no path), so moving the source file invalidates `shasum -c` from a different directory unless you `cd` first or rename the sidecar.
- There's no built-in verify mode in sortie. To verify, run `shasum -c <file>.sha256` manually.

**Notes:** reversible — undo deletes the sidecar.

---

## Security

### Auto-encrypt outgoing files

**When to reach for this:** sensitive files headed for cloud storage, email, or another machine. You want them encrypted at rest in your outbox so a leaked clone of the directory doesn't leak content.

**YAML:**

```yaml
- name: encrypt-outbox
  match:
    glob: "confidential-*"
  action:
    type: encrypt
    recipient: "age1abcdefg..."     # paste your recipient's age public key
    dest: ~/Outbox/Encrypted/{{.Name}}{{.Ext}}.age
```

**What happens:** any `confidential-*` file gets encrypted to the named recipient using `age` and written to the Encrypted folder. The source is preserved (so you still have the plaintext locally). To remove the plaintext after encryption, chain a `delete`:

```yaml
actions:
  - type: encrypt
    recipient: "age1abcdefg..."
    dest: ~/Outbox/Encrypted/{{.Name}}{{.Ext}}.age
  - type: delete
```

**Variations:**

- **GPG instead of age:** `tool: gpg` plus `recipient: "user@example.com"` (or a key ID). Requires the recipient's public key in your GPG keyring.
- **Multiple recipients:** age supports `-r recip1 -r recip2` natively, but sortie's `recipient:` is a single string. For multi-recipient, use `exec` calling `age` directly with multiple `-r` flags.
- **Key from a file:** if your recipients are listed in a file, set `tool: gpg` plus `recipient: "$(< ~/keys/recipient.txt)"` — except sortie doesn't shell out for the recipient field. Use `exec` for that case too.

**Gotchas:**

- The recipient public key is stored in your config in plaintext. That's fine for public keys (they're public) but easy to confuse with private identity files. Don't paste a private key into `recipient:`.
- `age` is the default tool. Install with `brew install age` or `apt install age`. GPG must be initialized with `gpg --gen-key` before first use.
- Encryption is one-way. To decrypt later you need the matching private identity (see the next recipe).

**Notes:** reversible — undo deletes the `.age` output. The plaintext source is preserved unless you chain a `delete`.

---

### Auto-decrypt an inbox

**When to reach for this:** counterpart to encrypt. You receive `.age` files in an inbox folder and want them decrypted as soon as they arrive.

**YAML:**

```yaml
- name: decrypt-inbox
  match:
    extensions: [.age]
  action:
    type: decrypt
    key: ~/.age/identity.txt
    dest: ~/Inbox/{{.Name}}
```

**What happens:** when `report.pdf.age` arrives, sortie runs `age --decrypt -i ~/.age/identity.txt -o ~/Inbox/report.pdf report.pdf.age`. The plaintext is written to the dest. The encrypted source is preserved — chain a `delete` if you want to remove it after successful decryption.

**Variations:**

- **GPG decryption:** `tool: gpg` plus `key:` pointing at the keyring (often `~/.gnupg/`). GPG decryption uses the keyring rather than a single identity file by default; `key:` is treated as a hint.
- **Filename inheritance:** the `.age` extension is stripped from `{{.Name}}` automatically because `{{.Name}}` is the basename without the last extension. So `report.pdf.age` → `{{.Name}}` is `report.pdf`. The dest becomes `~/Inbox/report.pdf`.

**Gotchas:**

- The `key:` file must be readable. On a fresh machine, copying a backup of `identity.txt` over but forgetting to restore mode `0600` causes age to refuse with "key file is too permissive."
- If the source is malformed (corrupt, wrong recipient), age exits with an error. The history record's `error` field captures the message.

**Notes:** reversible — undo deletes the decrypted plaintext.

---

### Unquarantine trusted installers *(macOS)*

**When to reach for this:** Gatekeeper attaches the `com.apple.quarantine` attribute to anything downloaded by a browser. For installers you build yourself or get from sources you fully trust, you can skip the "downloaded from internet" warning by stripping the attribute.

**YAML:**

```yaml
- name: unquarantine-internal-builds
  match:
    extensions: [.dmg, .pkg]
    glob: "internal-build-*"
  action:
    type: unquarantine
```

**What happens:** sortie runs `xattr -d com.apple.quarantine <path>`. If the attribute isn't present, it's a no-op. macOS will no longer show the "internet-downloaded" warning when you open the file.

**Variations:**

- **Per-source pattern:** match by directory or domain naming convention if your build pipeline drops files with predictable names.
- **Combined with verification:** chain `checksum` first, then `unquarantine` only if the hash matches an expected value. (Sortie doesn't have a built-in hash compare — use `exec` for that step.)

**Gotchas:**

- This is a security-relevant action. Quarantine is meaningful — only strip it from sources you trust. Don't run this on a globbed `~/Downloads/*.dmg`.
- macOS-only. Returns nil (no-op) on Linux and Windows because there's no quarantine attribute to remove.
- Not reversible. Once stripped, the attribute can be re-added manually but sortie can't restore it because there's nothing in history that captures the previous state cleanly.

**Notes:** non-reversible. History records the attempt for audit purposes.

---

## Integrations & side effects

### rsync new files to a NAS

**When to reach for this:** every new photo (or build artifact, or backup file) should be replicated to a network share without you thinking about it.

**YAML:**

```yaml
- name: rsync-to-nas
  match:
    extensions: [.jpg, .raw, .heic]
  action:
    type: exec
    command: "rsync -a '{{.Path}}' nas.local:/volume1/photos/"
```

**What happens:** sortie expands `{{.Path}}` to the absolute source path, wraps it in single quotes (so spaces in the filename don't break the command), and runs `rsync` via `sh -c` on Unix or `cmd /c` on Windows. rsync handles the network transfer; sortie just blocks until it returns.

**Variations:**

- **Authenticated SSH:** rely on the user's `~/.ssh/config` for ProxyJump, ports, identity files. Don't put credentials in the command.
- **Continuous mirror:** use `--delete` only if you're confident the source is the source of truth.
- **Bandwidth-limited:** `--bwlimit=2M` to cap throughput when you're on a metered network.
- **Multiple destinations:** chain two `exec` actions, each rsyncing to a different host.

**Gotchas:**

- `exec` blocks the dispatch. If rsync stalls (network out), the watcher won't process other files until it completes or sortie is killed. Add a timeout with `timeout 60 rsync ...` (Unix) or set up a separate dedicated rule with rate-limiting.
- Quoting: single quotes work on Unix (`sh -c`). On Windows (`cmd /c`), double quotes are safer:

  ```yaml
  command: 'rsync -a "{{.Path}}" nas.local:/volume1/photos/'
  ```

  Use the form that works on your daemon's primary OS.

- Not reversible. The history record captures that the exec ran; it can't roll back the rsync.

**Notes:** for cross-platform setups consider a wrapper script invoked by sortie, so the YAML stays simple and the platform-specific quoting lives in the script.

---

### Toast when an invoice arrives

**When to reach for this:** non-urgent but worth-knowing events. You want a desktop notification when a specific class of file shows up — invoices, alerts, deliveries.

**YAML:**

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

**What happens:** sortie matches PDFs whose extracted text mentions Acme or WidgetCo and contains an ISO-format date. The capture groups become template variables. The notification reads "Acme — 2026-04-25" or "WidgetCo — undated" depending on what was captured. macOS shows a banner; Linux a `notify-send` bubble; Windows a BurntToast toast (if installed) or a stderr line otherwise.

**Variations:**

- **Webhook instead of desktop:** swap the `message:` for a URL starting with `http://` or `https://`. Sortie POSTs JSON metadata instead of showing a desktop notification (see the next recipe).
- **Sound on macOS:** prepend the message with `sound name "Glass"` — actually, sortie's notify uses `display notification`, which doesn't expose sound. For sound, use `exec` with `afplay`.
- **Combine with file action:** put `notify` first in a chain with `move` so you see what's happening before files are filed away.

**Gotchas:**

- The `content_regex` must match somewhere in the first 64 KB of the file (configurable via `content_bytes:`). For a multi-page PDF, only the first 10 pages are extracted.
- On Linux, `notify-send` requires `libnotify-bin`. Without it, the action errors with a clear message about installing it.
- On Windows, BurntToast is optional. Without it, sortie falls back to writing `[sortie notify] Title: Message` to stderr and the action returns success. If you're not seeing toasts and not seeing stderr output, check that BurntToast is installed (`Get-Module -ListAvailable BurntToast`).

**Notes:** non-reversible. Notifications can't be unsent.

---

### POST new-file events to a webhook

**When to reach for this:** you want sortie to feed a separate automation pipeline (Zapier, n8n, internal API) that does something more sophisticated than what sortie can express directly.

**YAML:**

```yaml
- name: webhook-on-pdf
  match:
    extensions: [.pdf]
  action:
    type: notify
    title: "sortie"
    message: "https://automate.example.com/sortie/pdf-arrived"
```

**What happens:** because the message starts with `https://`, sortie does an HTTP POST to that URL with a JSON body:

```json
{
  "title": "sortie",
  "file": "report.pdf",
  "path": "/Users/you/Downloads/report.pdf",
  "size": "8421337"
}
```

The remote pipeline can read the path, fetch the file, do whatever logic it needs, and return any HTTP status. Sortie treats anything in the `2xx` range as success and anything `4xx`/`5xx` as failure (recorded in history).

**Variations:**

- **Bearer token:** sortie doesn't currently support custom HTTP headers. For authenticated webhooks, point the URL at a small local proxy that adds the header and forwards.
- **Body customization:** the body fields are fixed. For richer payloads, use `exec` calling `curl` with a custom JSON body built from template variables.
- **Multiple endpoints:** chain two `notify` actions, each with a different URL. Useful for a primary pipeline plus a separate audit log.

**Gotchas:**

- The URL is template-expanded, so you can include `{{.Match.foo}}` captures. If the URL contains a literal `{` or `}`, escape it or you'll confuse the template parser.
- The webhook call blocks the dispatch. If the endpoint is slow, the watcher backs up. Use `cooldown:` on the rule.
- Cross-platform — works the same on macOS, Linux, and Windows.

**Notes:** non-reversible.

---

### Upload nightly reports to S3

**When to reach for this:** generated PDFs, logs, or backups land in a directory and you want them shipped to object storage automatically.

**YAML:**

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

**What happens:** sortie auto-detects the tool from the URI scheme: `s3://` → `aws`, `gs://` → `gsutil`. It runs `aws s3 cp <source> <remote>`. The source is preserved locally — pair with `delete` in a chain to clean up after upload.

**Variations:**

- **Google Cloud Storage:** `remote: "gs://my-bucket/reports/{{.Year}}/{{.Name}}{{.Ext}}"`. Auto-uses `gsutil`.
- **Override the tool:** `tool: aws` if the auto-detection is wrong (e.g. you've aliased `aws` to a wrapper script).
- **Per-vendor partitioning:** with capture groups, push to vendor-specific prefixes: `remote: "s3://my-bucket/{{.Match.vendor}}/{{.Year}}/{{.Name}}{{.Ext}}"`.

**Gotchas:**

- Configure credentials before the rule fires the first time. `aws configure` for AWS, `gcloud auth application-default login` for GCS.
- The `cooldown: 2s` is important under `sortie watch` — without it, a flood of generated reports would fire concurrent uploads. 2 s is a conservative spacing for a small bucket on a fast network; tune based on your throughput needs.
- Failure modes are S3-specific: 403 Forbidden (credentials), 503 Slow Down (rate limit). Sortie surfaces the tool's stderr in the history `error` field.

**Notes:** non-reversible. The remote object can be deleted manually if needed.

---

### Apply Finder tags to receipts *(macOS)*

**When to reach for this:** you use Finder's color tags or smart folders for organization, and want sortie to apply tags automatically based on filename or content.

**YAML:**

```yaml
- name: tag-receipts
  match:
    extensions: [.pdf]
    regex: "(?i)receipt"
  action:
    type: tag
    tags: [Red, Finance]
```

**What happens:** sortie writes the macOS Finder tag plist to `com.apple.metadata:_kMDItemUserTags` via `xattr`. The PDF appears with a red color label in Finder, and shows up in any smart folder that filters by `tag:Finance`.

**Variations:**

- **Color + custom name:** `tags: [Blue, Q2-2026, Reviewed]`. Custom names work alongside color names.
- **Multiple tag rules:** several rules each adding different tags via `continue: true`. Tags don't replace each other — they accumulate on the file.
- **Conditional on content:** combine with `content_regex` to tag based on extracted content rather than just filename.

**Gotchas:**

- macOS-only. Errors clearly on Linux and Windows.
- Standard color names: `Red`, `Orange`, `Yellow`, `Green`, `Blue`, `Purple`, `Gray`. Anything else is a custom tag name (still works).
- Not reversible. Tags can be removed manually in Finder, but sortie doesn't track the previous tag state.

**Notes:** non-reversible.

---

### Auto-open disk images *(macOS)*

**When to reach for this:** every `.dmg` you download you immediately double-click anyway. Skip the click.

**YAML:**

```yaml
- name: open-dmgs
  match:
    extensions: [.dmg]
  action:
    type: open
```

**What happens:** sortie runs `open <path>`, which on macOS triggers the default handler. For `.dmg`, that's the disk mounter — the volume mounts and opens in Finder.

**Variations:**

- **Force a specific app:** `app: VLC` opens video files in VLC instead of QuickTime. `app: Preview` for `.dmg` would mount and then preview the contents (not particularly useful).
- **Per-extension targeting:** different rules for `.dmg`, `.pkg`, `.iso` if you want different behavior.

**Gotchas:**

- macOS-only. Returns a clear error on Linux/Windows. The cross-platform stub doesn't try to invoke `xdg-open` or `cmd /c start`.
- Auto-opening anything from `~/Downloads` is a security smell. Pair with `unquarantine` cautiously, and only for sources you trust.

**Notes:** non-reversible. Once the DMG is mounted, sortie doesn't track it — Finder owns the lifecycle from there.

---

## Multi-step chains

Chains combine several actions into a single rule. They share a `chain_id` in history so `sortie undo <chain_id>` reverses the whole thing (to the extent each step is reversible). Three rules apply to chains:

1. **No automatic rollback on failure.** If step 3 of 5 fails, steps 1 and 2 stay in place. The chain stops at the failing step.
2. **Reversible operations first; side effects last.** A failed `notify` shouldn't leave a half-moved file in limbo, but a failed `move` should leave you ungroped by the rest of the chain.
3. **Some actions consume the source.** `compress` removes the original, `move` relocates it. Anything after these in the chain runs against a missing source path and fails. `sortie validate` warns about this.

Each chain below includes a "Failure modes" subsection walking through what happens if each step breaks.

---

### Download triage

**When to reach for this:** you want every archive that hits Downloads unpacked, tagged for follow-up, and announced via notification.

**YAML:**

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

**What happens:** drop `project-1.0.tar.gz` into Downloads. The chain runs in order:

1. `extract` unpacks it into `~/Downloads/Extracted/project-1.0/`. The original `.tar.gz` stays in Downloads.
2. `tag` writes Finder tag metadata onto the original archive (still at the source path). The blue label appears in Finder, and `tag:imported` smart folders pick it up.
3. `notify` shows a desktop notification.

**Sequencing rationale:** `extract` is reversible and structural — needs to run first because if it fails (corrupt archive), you don't want a "imported" tag claiming success. `tag` is structural-ish (modifies metadata, macOS-only). `notify` is the side effect — runs last so the user only sees a notification when the actual import worked.

**Failure modes:**

- **`extract` fails (corrupt archive):** chain stops. The `.tar.gz` stays in Downloads with no tags or notifications. You can investigate manually via `sortie history -n 5`, see the error, and re-download or fix the archive.
- **`tag` fails (running on Linux/Windows):** the archive is extracted but not tagged, and `notify` doesn't run. On macOS this almost never fails.
- **`notify` fails (Linux without `notify-send`, Windows without BurntToast and no terminal):** the file is extracted, tagged, but no desktop notification. The history record shows the notify error.

**Variations:**

- **Add cleanup:** append a final `delete` step to trash the archive after successful extraction. Adds risk — if `tag` or `notify` fails, the original is gone.
- **Cross-platform replacement for `tag`:** swap `tag` for an `exec` that writes to a logfile, which works on every OS.
- **Pre-filter with dedupe:** see the [deduplicate gotcha](#deduplicate-incoming-downloads) — `deduplicate` doesn't compose well in chains. Use it as a separate rule with a higher priority instead, accepting that non-duplicates will be moved (preventing the triage chain from firing on them).

**Notes:** chain undo (`sortie undo <chain_id>`) reverses `extract` (deletes the unpacked directory) but can't undo `tag` or `notify`. Mixed reversibility is the norm.

---

### Invoice intake with capture-group filing

**When to reach for this:** vendors send invoices as PDFs to a shared inbox. You want them filed by vendor and date in a single pass, with a notification confirming the result.

**YAML:**

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

**What happens:** drop `inv_4521.pdf` from Acme dated 2026-04-15 into the watched directory:

1. `pdftotext` extracts the first 10 pages.
2. The regex finds `Acme` (captured as `company`) and `2026-04-15` (captured as `date`).
3. The dest template expands to `~/Documents/Invoices/Acme/2026-04-15-inv_4521.pdf`.
4. `move` relocates the file. Parent directories are created as needed.
5. `notify` shows "Invoice filed: Acme · 2026-04-15".

If the PDF says "April 15, 2026" instead of "2026-04-15", the second alternative in the date regex matches and sortie auto-normalizes to `2026-04-15` because the capture group is named `date`.

**Sequencing rationale:** the move is reversible and is the primary action — needs to succeed for the notification to make sense. The notify is a side effect that doesn't affect file state.

**Failure modes:**

- **Match doesn't fire:** the rule simply doesn't apply. The PDF stays in the inbox. Lower-priority rules may pick it up.
- **`move` fails (read-only filesystem, full disk):** chain stops. The PDF stays in the source location, no notification. History records the move error.
- **`notify` fails:** the file is filed but no notification. Rare on macOS; possible on Linux without `notify-send`.

**Variations:**

- **Add a `copy` for legal retention:** insert a `copy` action before `move`, sending a duplicate to a long-term archive that uses a different naming convention. Failures in `copy` block the move.
- **Add OCR for image-only PDFs:** `pdftotext` returns empty text for scanned images. Chain `ocr` before the match rule, or have a separate rule that catches PDFs with no extractable text and OCRs them via `exec` calling `tesseract`.
- **Aggressive fallbacks:** `{{or .Match.company "Unknown"}}` for unknown vendors, `{{or .Match.date "undated"}}` already shown.

**Notes:** undo `<chain_id>` restores the original location and removes the destination. The notify step is non-reversible but a no-op for undo purposes.

---

### Photo import pipeline

**When to reach for this:** a workflow that imports photos from a SD card or shared folder, generates a web-resolution version, and files the original by year/month for archival.

**YAML:**

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

**What happens:** for each photo over 2 MB:

1. `resize` reads the source, writes a 1920px-wide copy to `~/Pictures/Web/2026-04-25_IMG_4521.heic`. Source is preserved.
2. `move` relocates the original (still at the source path) to `~/Pictures/Originals/2026/04/2026-04-25_IMG_4521.heic`.

Both ops use the same date prefix because they share the file's mtime via `{{.Date}}`.

**Sequencing rationale:** `resize` operates on the source path and preserves it; `move` then consumes the source. The order is correct — reversing it would move the file before resize could find it. `sortie validate` would catch that swap.

**Failure modes:**

- **`resize` fails (unsupported format on Linux without ImageMagick HEIC support):** chain stops. The original stays in the inbox; no web copy is created. Common pitfall on Linux for HEIC sources.
- **`move` fails (permissions, full disk):** the web copy already exists at `~/Pictures/Web/...` but the original is still in the source. State is recoverable: re-running picks up where it left off (the resize will overwrite the web copy).

**Variations:**

- **Add a thumbnail pass:** insert a smaller `resize` (`width: 400`) before the larger one, writing to `~/Pictures/Thumbs/`.
- **Add metadata stripping:** chain an `exec` calling `exiftool -all=` on the resized output to strip EXIF before publication.
- **RAW workflow:** match `.cr3`/`.arw`/`.dng` and route to a different folder; convert to JPEG via `convert` action before resizing.
- **iCloud upload:** chain `upload` after `move` to push the original to S3 or GCS.

**Notes:** undo reverses `move` (back to source) and `resize` (deletes the web copy). The chain is fully reversible.

---

### Screenshot tidy

**When to reach for this:** you take a lot of screenshots. Macs put them on the desktop with awkward filenames; you want them filed by year and month with a stable naming convention.

**YAML:**

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

**What happens:** `Screenshot 2026-04-25 at 14.30.21.png` on the Desktop becomes `~/Pictures/Screenshots/2026/04/2026-04-25_14-30-21.png`. The notification confirms the save with the date and time.

**Sequencing rationale:** trivial — `move` first, `notify` second. The file is structurally relocated before any user-visible feedback fires.

**Failure modes:**

- **`move` fails:** screenshot stays on the Desktop. No notification.
- **`notify` fails:** screenshot is filed; you just don't get a toast.

**Variations:**

- **OCR the screenshot:** chain `ocr` before `move` for text-rich screenshots (terminal output, code reviews) so you get a `.txt` sidecar searchable in Spotlight or grep.
- **Per-app categorization:** `glob: "Screenshot*Slack*"` (or a regex) routes screenshots from specific apps to specific folders.
- **Annotate first, file later:** `exec` calling a markup tool (e.g. shottr's CLI) to crop or annotate, then move.

**Notes:** fully reversible until `notify`.

---

### Log rotator

**When to reach for this:** application logs that grow unbounded. You want a size-triggered rotation that keeps a local archive copy and ships the original to S3 before deleting it.

**YAML:**

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

**What happens:** the directory entry has `watch_existing: true`, so `sortie watch` reacts to Write events on existing log files (in addition to Create). When `myapp.log` crosses 100 MB, the rule fires:

1. `copy` writes a date-stamped copy to `~/Archives/Logs/myapp-2026-04-25.log`. The original stays in `/var/log/myapp/`.
2. `upload` ships the original (still at the source path) to `s3://logs-bucket/2026/04/myapp-2026-04-25.log`.
3. `delete` trashes the original. The archive copy and the S3 object remain.

The `cooldown: 1m` prevents the rule from re-firing while the log is still being written to during the upload. After 1 minute of silence, sortie can fire again — by which time the log has either been deleted (so no match) or the application has rotated to a new log file.

**Sequencing rationale:** copy first creates a safety net (local archive). Upload is the primary side effect (long-term storage). Delete is the last step — only fires if both earlier steps succeeded. If you reversed copy and compress (an earlier draft of this recipe did), `compress` would consume the source before `upload` runs and the chain would fail. `sortie validate` warns about that pattern.

**Failure modes:**

- **`copy` fails (out of disk):** chain stops. The log keeps growing, no rotation happens. Free up space and the next watch event will retry.
- **`upload` fails (S3 credentials, network):** the local archive exists, but the original isn't deleted. State is recoverable — the next event can retry the upload of a now-stale source. Long-term, monitor history for repeated upload errors.
- **`delete` fails (read-only filesystem):** archive and S3 object exist; original log is still in place. Mostly benign — manual cleanup.

**Variations:**

- **Compress before archiving:** insert a `compress` step at the end (after `delete`) — but only on a separate rule matching the archive directory, since the chain's source is already gone. Alternatively, configure rsync's `--compress` instead.
- **Pre-rotate notification:** add `notify` at the start (with `continue: true` semantics) to alert ops about the rotation.
- **Per-app retention:** split into multiple rules with different `min_size` thresholds for different apps' logs.

**Notes:** the chain is mostly reversible — undo can restore the original log from the trash and delete the archive copy, but it cannot delete the S3 object.

---

### Searchable file cabinet

**When to reach for this:** you scan paper documents (mail, receipts, contracts) with your phone and want a searchable archive on your computer rather than dropping the images into a folder you'll never grep through. Two flavors are shown: an **Apple Vision** variant (best quality on macOS) and a **searchable PDF** variant (one self-contained file per document, indexable by Spotlight, Preview, and most PDF viewers).

**Prerequisites:**

- Use a real document scanning app on your phone (iOS Notes "Scan Documents," Files app's scan, or the Google Drive app's `+` → Scan). These auto-rectify perspective, threshold to clean B&W, and produce dramatically better OCR input than casual photos. The single biggest accuracy win available is at the capture step, not at the OCR step.
- Pick one of the OCR backends below and install it.

**Choose your OCR backend:**

| Backend | Install | Quality on phone scans | Where it works |
|---------|---------|------------------------|----------------|
| Apple Vision via `ocrit` | Download the signed `.pkg` from [ocrit releases](https://github.com/insidegui/ocrit/releases/latest), or `swift build -c release` from source | Excellent — same engine as Live Text | macOS only |
| `tesseract` to searchable PDF | `brew install tesseract` (or apt/scoop) | Good on clean scans, mediocre on photos | macOS / Linux / Windows |
| `ocrmypdf` | `brew install ocrmypdf` (wraps tesseract + preprocessing) | Better than raw tesseract; PDF-input only | macOS / Linux / Windows |

#### Variant 1: Apple Vision → `.txt` sidecar (macOS)

```yaml
- name: scan-cabinet-vision
  match:
    extensions: [.jpg, .jpeg, .heic, .png]
    glob: "Scan_*"               # adjust to your scanner's naming convention
  cooldown: 5s                    # let cloud sync settle if the source is in iCloud / Drive
  actions:
    - type: exec
      command: 'ocrit "{{.Path}}" -o "{{.Path}}.txt"'
    - type: move
      dest: ~/Documents/FileCabinet/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}
```

**What happens:** `ocrit` calls Apple's Vision framework on the scan image and writes the recognized text to `<source>.txt` in the same directory. Then `move` files both the image and... wait — only the image moves. The `.txt` sidecar gets left behind in the source directory. To keep them together, swap the order:

```yaml
actions:
  - type: move
    dest: ~/Documents/FileCabinet/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}
  - type: exec
    command: 'ocrit "{{.Dest}}" -o "{{.Dest}}.txt"'
```

After the move, `{{.Dest}}` resolves to the new location, so `ocrit` runs on the image at its archived path and writes the sidecar next to it. Final layout:

```
~/Documents/FileCabinet/2026/04/Scan_2026-04-28_Anthem-bill.heic
~/Documents/FileCabinet/2026/04/Scan_2026-04-28_Anthem-bill.heic.txt
```

Spotlight indexes the `.txt` automatically; searching for "anthem" in Finder will surface both files.

#### Variant 2: tesseract → searchable PDF (cross-platform)

A searchable PDF is a single file containing the original image as the visual layer plus an invisible text layer for search. Self-contained, portable, indexable.

```yaml
- name: scan-cabinet-pdf
  match:
    extensions: [.jpg, .jpeg, .png]
    glob: "Scan_*"
  cooldown: 5s
  actions:
    - type: exec
      command: 'tesseract "{{.Path}}" "{{.Path}}.searchable" pdf'
    - type: move
      dest: ~/Documents/FileCabinet/{{.Year}}/{{.Month}}/{{.Name}}.pdf
```

The first step produces `<source>.searchable.pdf` (tesseract appends `.pdf` automatically). The second step moves it into the dated cabinet, dropping the `.searchable` infix in the dest. After both steps, the original image and the searchable PDF coexist — adjust as you prefer (delete the original after, or keep it as backup).

#### Variant 3: PDF input → searchable PDF via `ocrmypdf`

If your scans are already PDFs (some scanner apps export directly to PDF), `ocrmypdf` is dramatically better than raw tesseract because it handles deskew, despeckle, and orientation detection automatically:

```yaml
- name: scan-cabinet-ocrmypdf
  match:
    extensions: [.pdf]
    glob: "Scan_*"
  cooldown: 5s
  actions:
    - type: exec
      command: 'ocrmypdf --skip-text --deskew "{{.Path}}" "{{.Path}}.searchable.pdf"'
    - type: move
      dest: ~/Documents/FileCabinet/{{.Year}}/{{.Month}}/{{.Name}}.pdf
```

`--skip-text` avoids re-OCR'ing PDFs that already have a text layer; `--deskew` is the single most useful preprocessing step for handheld scans.

**Sequencing rationale:**

OCR before move is fine in Variant 2/3 because the OCR step writes the output to a new file; the source is preserved. The order in Variant 1 is reversed because `ocrit` writes the `.txt` *next to* its input, so we move the input first and OCR at its destination — keeping the image and sidecar together.

**Failure modes:**

- **OCR step fails (tool missing, bad image):** chain stops. The original scan stays where it is. Investigate via `sortie history -n 5`.
- **Move step fails (permissions, disk full):** the `.txt` sidecar (Variant 1) or `.searchable.pdf` (Variant 2/3) exists at the source location but the original isn't moved. Re-running the rule on the next event picks up where it left off.
- **Apple Vision occasionally hangs on corrupt images.** `ocrit` will return an error; the move doesn't run. The history record's `error` field captures the message.

**Variations:**

- **Skip the `move` step and OCR in place** if you'd rather keep scans where they land (Drive folder, iCloud, etc.) and let those services handle their own sync — sortie just produces sidecars.
- **Combine date capture with content-based filing.** Add a `content_regex` step (`(?P<vendor>...)`) on the OCR'd `.txt` to file scans by vendor as well as date. Two-pass workflow: chain 1 OCRs and writes the sidecar; chain 2 reads the sidecar's content via a separate rule with `content_regex` matching, and re-files based on captures.
- **Multi-page PDF from a multi-photo capture.** If you take three photos for a three-page document, combine before OCR'ing: `convert page1.jpg page2.jpg page3.jpg combined.pdf` (ImageMagick) followed by `ocrmypdf` on the combined PDF.

**Notes for the workflow you described (phone → Drive → file cabinet):**

- Google Drive runs its own server-side OCR on every uploaded image. If your only search surface is Drive's web/app interface, you may not need any of this — Drive's search will find scanned text automatically. Sortie OCR's value is producing **local, portable, greppable, Spotlight-indexable** text.
- If you sync Google Drive to a local folder, point the rule at the synced location (e.g. `~/Library/CloudStorage/GoogleDrive-you@example.com/My Drive/Scans/`). When sortie writes the `.txt` or `.searchable.pdf`, Drive will sync it back up — the searchable form is then available on your phone too.
- For the cleanest archive, consider piping new scans through this chain into a **non-Drive** location (`~/Documents/FileCabinet/`). That gives you sortie's local-first archive plus Drive's online OCR — belt and suspenders.

---

## Where to go next

- [Reference](04-reference.md) — quick-lookup tables for flags, match conditions, actions, and template variables.
- [Troubleshooting](05-troubleshooting.md) — when a rule doesn't match, an action fails silently, or the daemon won't stay up.
- File an issue at https://github.com/msjurset/sortie/issues with your YAML and a `--dry-run` trace if a recipe doesn't behave as documented — that's almost always a doc bug, not user error.
