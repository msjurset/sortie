# Getting Started

This is a ten-minute tour. By the end you'll have sortie installed, a starter config in place, and a daemon watching `~/Downloads` that automatically moves files based on rules you control.

If anything feels magical, skip ahead to [Concepts](02-concepts.md) afterwards — it explains the moving parts.

## 1. Install

sortie ships prebuilt binaries for macOS, Linux, and Windows on the [Releases page](https://github.com/msjurset/sortie/releases). Pick the archive for your OS, extract it, and put the binary somewhere on your `PATH`.

### macOS

```sh
# Apple Silicon (M-series)
curl -L -o sortie.tar.gz https://github.com/msjurset/sortie/releases/latest/download/sortie-$(curl -s https://api.github.com/repos/msjurset/sortie/releases/latest | grep tag_name | cut -d'"' -f4 | sed 's/^v//')-darwin-arm64.tar.gz

# Intel
# curl -L -o sortie.tar.gz https://github.com/msjurset/sortie/releases/latest/download/sortie-<version>-darwin-amd64.tar.gz

tar -xzf sortie.tar.gz
mv sortie ~/.local/bin/      # or anywhere on $PATH
sortie --version
```

> **Optional dependencies (macOS)**
> - `brew install poppler` — enables PDF content matching (the `content` / `content_regex` match conditions on `.pdf` files).
> - `brew install imagemagick` — enables the `watermark` and alternative `resize` actions. `sips` (built in) handles basic resize by default.

### Linux

```sh
curl -L -o sortie.tar.gz https://github.com/msjurset/sortie/releases/latest/download/sortie-<version>-linux-amd64.tar.gz
tar -xzf sortie.tar.gz
sudo mv sortie /usr/local/bin/
sortie --version
```

> **Optional dependencies (Linux)**
> - `apt install libnotify-bin` (or distro equivalent) — enables the `notify` action. Without `notify-send` on `PATH`, the action returns a clear error.
> - `apt install poppler-utils` — enables PDF content matching.
> - `apt install imagemagick` — enables `watermark` and `resize`.

### Windows

1. Download `sortie-<version>-windows-amd64.zip` from the [latest release](https://github.com/msjurset/sortie/releases/latest).
2. Extract the zip. It contains `sortie.exe` and the `sortie.1` man page.
3. Move `sortie.exe` to a directory on your `PATH` (e.g. `C:\Users\<you>\bin\`, or add the extract location to `PATH`).
4. Open a new terminal and run `sortie --version` to confirm.

> **Optional dependencies (Windows)**
> - In an elevated PowerShell: `Install-Module -Name BurntToast` — enables toast notifications for the `notify` action. Without BurntToast, notifications fall back to stderr.
> - The `open`, `tag`, and `unquarantine` actions are macOS-only and will return a clear error on Windows.

## 2. Generate a starter config

```sh
sortie config init
```

This writes `~/.config/sortie/config.yaml` on macOS and Linux, or `%APPDATA%\sortie\config.yaml` on Windows, with a handful of sensible starter rules for a `~/Downloads` directory. Open it in your editor:

```sh
sortie config path    # prints the absolute path
$EDITOR "$(sortie config path)"
```

You'll see two top-level sections: `directories:` (what to watch) and `rules:` (what to do). The starter file contains rules for screen captures and installers, plus commented-out examples for images and PDFs.

## 3. Preview before touching anything

**Always** run `--dry-run` the first time you point sortie at a real directory. It walks the directory, matches every file against your rules, and logs what *would* happen — without moving, renaming, or deleting anything.

```sh
sortie scan --dry-run
```

<!-- TODO: embed asciinema cast — assets/01-dry-run.svg -->

Every line shows `[rule-name] /path/to/file → /destination`. If you see files that match the wrong rule, edit the YAML and rerun `--dry-run` until you're happy. Nothing on disk has changed yet.

## 4. Run it for real

Once the preview looks right, run it without `--dry-run`:

```sh
sortie scan
```

sortie walks the directory and executes the matched action for each file. To see what it did, check the history:

```sh
sortie history
```

<!-- TODO: embed asciinema cast — assets/02-scan-history.svg -->

Each record has an ID. If something was moved somewhere you didn't intend, undo the last action:

```sh
sortie undo                 # undo the most recent action
sortie undo --last 3        # undo the last three
```

Not every action is reversible — `exec`, `notify`, and `open` can't be undone because they don't produce a file we can put back. The history still records them, but `undo` will skip with a clear message.

## 5. Add your own rule

Let's file every PDF into `~/Documents/PDFs/`. Open your config:

```sh
$EDITOR "$(sortie config path)"
```

Add a new entry under `rules:` (above the existing rules if you want it to match first — see the [priority note](02-concepts.md#rules) later):

```yaml
  - name: file-pdfs
    match:
      extensions: [.pdf]
    action:
      type: move
      dest: ~/Documents/PDFs/{{.Name}}{{.Ext}}
```

Drop a PDF into `~/Downloads` and run:

```sh
sortie scan --dry-run
```

You should see `[file-pdfs] ~/Downloads/your.pdf → ~/Documents/PDFs/your.pdf`. Remove `--dry-run` to actually move it.

Template variables like `{{.Name}}` and `{{.Ext}}` come from the filename. The full list is in [Concepts › Templates](02-concepts.md#templates).

## 6. Watch in real time

Running `scan` manually is fine, but the real power is the watch daemon: sortie monitors your directories and dispatches files as soon as they arrive (with a short debounce to let in-progress downloads finish).

```sh
sortie watch
```

<!-- TODO: embed asciinema cast — assets/03-watch.svg -->

Leave this running in a terminal, then drag a file into `~/Downloads` from another window. Within a second, you'll see the dispatch line in the watch output, and the file will be gone from Downloads. Press `Ctrl-C` to stop.

For a permanent background service:

- **macOS** — `make install-launchd` installs a user agent that starts at login. (Covered in the root README's "Running as a Service" section.)
- **Linux** — wrap `sortie watch` in a systemd user unit.
- **Windows** — create a Task Scheduler task that runs `sortie.exe watch` at logon.

Detailed service setup per platform lives in the [Cookbook](03-cookbook.md) and [Troubleshooting](05-troubleshooting.md) pages.

## What's next

- [Concepts](02-concepts.md) — the mental model behind rules, matches, actions, chains, and the watch daemon. Read this once; everything else clicks faster afterwards.
- [Cookbook](03-cookbook.md) — at least one recipe per action type, plus real-world multi-step chains.
- [Reference](04-reference.md) — quick-lookup tables for subcommands, match conditions, actions, and template variables.
- [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes for common issues.
