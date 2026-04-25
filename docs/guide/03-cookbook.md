# Cookbook

> **Status:** scaffolding — full content lands in a follow-up PR.

Recipes for every action type, plus real-world multi-step chains. Each recipe follows the same template: **Goal** / **YAML** / **Notes** (with platform caveats where relevant).

## Basics

- Move all PDFs into `~/Documents/`
- Copy screenshots to a shared folder (keep the original)
- Rename dated files to ISO format
- Auto-delete files older than 30 days
- Compress daily backups into `.tar.gz`

## Archives

- Auto-extract downloaded zips, then delete the archive
- Extract only if the archive contains a specific file pattern

## Media

- Resize incoming photos to 1920px (`sips` on macOS, `convert` elsewhere)
- Watermark blog images with a logo PNG (ImageMagick)
- OCR scanned PDFs into a searchable archive
- Convert `.mov` → `.mp4` via `ffmpeg`

## Security

- Auto-encrypt exports from a work directory with `age`
- Decrypt-on-arrival in an inbox directory
- Remove macOS quarantine from installers *(macOS only)*

## Dedup and cleanup

- Deduplicate `~/Downloads` against an archive folder (skip if dupe, delete source)
- Trash duplicates automatically

## Integrations

- `exec` — rsync new files to a NAS (cross-platform note on `sh` vs `cmd`)
- `notify` — toast when an invoice arrives, with vendor + date captured from filename
- `upload` — push to S3 with the `aws` CLI
- `tag` — apply Finder tags to receipts *(macOS only)*
- `open` — auto-open `.dmg` installers *(macOS only)*

## Chains (multi-step workflows)

- **Download triage** — `deduplicate` → `extract` → `tag` → `notify`
- **Invoice intake** — PDF `content_regex` capture → `rename` with template → `move` to vendor folder → `notify`
- **Photo import** — EXIF date via tool override → `rename` → `resize` sidecar → `move` to dated folder
- **Screenshot tidy** — `rename` to ISO → weekly `compress` into an archive → `notify`
- **Log rotator** — `watch_existing` + size threshold → `compress` → `delete` original → `upload` to S3

Each chain recipe includes a short "what happens if step N fails?" paragraph linking back to the [Concepts](02-concepts.md) page.
