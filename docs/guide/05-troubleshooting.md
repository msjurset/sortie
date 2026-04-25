# Troubleshooting

> **Status:** scaffolding — full content lands in a follow-up PR.

Symptom-driven fixes. Each entry follows the same shape: **Symptom** / **Likely cause** / **Fix**.

## My rule doesn't match

Priority order, ignore patterns, in-flight temp-file suffixes (`.crdownload`, `.part`, etc.), content detection limits (first 10 PDF pages).

## An action failed silently

Where to look: `sortie history`, log level, re-running with `--dry-run` to isolate.

## `sortie watch` stopped unexpectedly

Log locations per platform: macOS launchd logs, Linux journalctl, Windows Event Viewer / Task Scheduler history.

## Write events aren't triggering my rule

`watch_existing` requirement and the debounce window.

## Windows: notify shows nothing

BurntToast not installed; stderr fallback behavior; how to verify from a terminal.

## PDF content matching returns no hits

`pdftotext` missing (poppler); magic-byte detection; page-count cap.

## `exec` fails on Windows with an odd error

Shell differences: sortie runs commands through `sh` on Unix and `cmd.exe` on Windows. Quoting, path separators, and `{{.Path}}` substitution notes.

## Template errors

Common gotchas with `{{or .Match.x "fallback"}}` and empty captures.

## Rate limits: "rule disabled after N failures"

Where the counter lives, how to inspect it, how to reset.

## Desktop notification reference

Screenshots of the three notification backends side-by-side:

- macOS: `osascript`-driven banner
- Linux: `notify-send` / libnotify bubble
- Windows: `BurntToast` toast (with stderr fallback)
