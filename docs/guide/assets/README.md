# Guide Assets

Screenshots and [asciinema](https://asciinema.org) casts referenced by the user guide. Everything here is optional: the prose doesn't depend on the media to make sense, but the casts give a much better feel for how sortie behaves in real time.

## Casts to record

Each cast below is referenced as a TODO placeholder in the guide pages. Record them with `asciinema rec FILE.cast`, then convert to SVG for embedding with [`svg-term-cli`](https://github.com/marionebl/svg-term-cli) (`npm i -g svg-term-cli`):

```sh
svg-term --in 01-dry-run.cast --out 01-dry-run.svg --window --width 100 --height 24
```

| File | Shows | Referenced in |
|------|-------|---------------|
| `01-dry-run.svg` | `sortie scan --dry-run` against a populated `~/Downloads`, showing matched and unmatched files | [01-getting-started.md](../01-getting-started.md) |
| `02-scan-history.svg` | `sortie scan` executing, followed by `sortie history` listing the dispatched records | [01-getting-started.md](../01-getting-started.md) |
| `03-watch.svg` | `sortie watch` running; a file dropped into the watched directory triggers a visible dispatch line | [01-getting-started.md](../01-getting-started.md) |

## Screenshots to capture

| File | Shows | Referenced in |
|------|-------|---------------|
| `notify-macos.png` | A sortie `notify` action triggering a macOS banner via `osascript` | [05-troubleshooting.md](../05-troubleshooting.md) |
| `notify-linux.png` | A sortie `notify` action triggering a `notify-send` bubble on GNOME or KDE | [05-troubleshooting.md](../05-troubleshooting.md) |
| `notify-windows.png` | A sortie `notify` action triggering a BurntToast toast on Windows 10/11 | [05-troubleshooting.md](../05-troubleshooting.md) |

## Recording tips

- Use a clean shell prompt and 100-column wide terminal so the output renders well at embed size.
- Keep casts under ~30 seconds — longer is hard to watch and inflates the SVG.
- Pre-stage a sandbox `~/Downloads` with a few representative files (a screenshot, an installer, a PDF) so each rule in the starter config has something to match.
- After recording, preview the SVG locally before committing — `open 01-dry-run.svg` (macOS) or dragging into a browser.
