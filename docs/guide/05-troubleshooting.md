# Troubleshooting

Symptom-driven fixes for the problems you're most likely to run into. Each entry has three parts: what you observe, why it happens, and how to fix it.

If none of these match what you're seeing, file an issue at https://github.com/msjurset/sortie/issues with your YAML, the relevant `sortie history` output, and a `--dry-run` trace.

---

## A rule doesn't match a file I expected it to

**Likely causes:**

1. **Another rule with higher priority matched first.** Without `continue: true`, only the first match fires.
2. **A match condition silently excludes the file.** Conditions AND together — a `min_size: 5MB` next to `extensions: [.pdf]` rejects all PDFs smaller than 5 MB.
3. **The file is still being written.** The watcher skips files ending in `.crdownload`, `.part`, `.partial`, `.download`, and `.tmp`, which means a long download won't trigger any rule until the browser renames it to the final name.

**Fix:**

```sh
sortie rules test ~/Downloads/your-file.pdf
```

That command walks your rules in priority order and tells you exactly which one would fire (or why none did). If it shows the wrong rule, either adjust priorities or narrow the match on the higher-priority rule. If it shows no match at all, recheck the extension, size, and age conditions.

Also check the priority order explicitly:

```sh
sortie rules
```

Output is sorted by effective priority. Per-directory `.sortie.yaml` rules appear alongside globals.

---

## An action failed silently during `sortie watch`

**Likely causes:**

1. The action ran but the output looks like a no-op (e.g. `deduplicate` with `skip` when an identical file already exists at `dest`).
2. An external tool is missing — `notify` without `notify-send` on Linux, `resize` without `sips` on non-macOS, `pdftotext` for PDF content matching, etc.

**Fix:** pull the record from history and read the `error` field.

```sh
sortie history -n 10
```

Every record includes the rule name, action type, and either the destination path or an `error` field. For a deeper look — including records without errors — inspect the raw JSON:

```sh
jq . ~/.config/sortie/history.json | less
```

For notifications specifically, the action returns `nil` on the Windows stderr fallback (see [the Windows notify entry below](#windows-notifications-dont-show-up)), so an action that "succeeds" without producing a visible toast is normal there.

---

## `sortie watch` stopped unexpectedly

**Likely causes:**

- Configuration file changed to something invalid. sortie logs the error but keeps running with the last-valid rules — except when the initial load itself is invalid.
- The daemon detected its own binary being replaced and exited gracefully (this is by design — see [self-monitoring](#the-daemon-keeps-restarting-when-i-deploy)).
- An OS-level limit was hit (inotify watches on Linux).

**Fix by platform:**

| Platform | Where to find the crash reason |
|----------|--------------------------------|
| macOS (launchd) | `~/.config/sortie/logs/sortie.log`, or `log show --predicate 'process == "sortie"' --last 1h` |
| Linux (systemd) | `journalctl --user -u sortie -n 200` |
| Windows (Task Scheduler) | Task history in Task Scheduler, plus stdout/stderr redirected log |

If the daemon is refusing to stay up, run `sortie watch` in the foreground from a terminal — the error that was hidden in the background comes to stdout immediately.

---

## Write events aren't triggering my rule

**Likely cause:** by default, `sortie watch` only fires on **Create** and **Rename** events. Writes to an already-existing file are ignored unless you opt in.

**Fix:** add `watch_existing: true` to the relevant directory entry:

```yaml
directories:
  - path: /var/log/myapp
    watch_existing: true
```

Match conditions still apply, so a growing log only dispatches once it crosses `min_size`. Pair with `cooldown:` on the rule to prevent rapid re-firing while the file is still being written.

---

## Windows: notifications don't show up

**Cause:** the `notify` action on Windows uses the `BurntToast` PowerShell module, which isn't installed by default. Without it, sortie falls back to writing `[sortie notify] <title>: <message>` to stderr and returning success.

**Fix:**

```powershell
# Elevated PowerShell
Install-Module -Name BurntToast
```

Then retry a notification. If toasts still don't appear after installing BurntToast, check:

- Focus Assist / Do Not Disturb is off.
- Notifications for PowerShell are allowed (Windows Settings → System → Notifications).
- You're running PowerShell 5.1 or later (`$PSVersionTable.PSVersion`).

To confirm which backend sortie is using right now, run a rule that notifies and then search the daemon's stderr for `[sortie notify]` — if it's there, BurntToast isn't being picked up.

---

## PDF content matching returns no hits

**Cause:** sortie shells out to `pdftotext` (from poppler) for PDFs. Without it installed, `content` and `content_regex` match against raw PDF bytes, which almost never finds human-readable words.

**Fix:**

```sh
# macOS
brew install poppler

# Debian / Ubuntu
sudo apt install poppler-utils

# Windows
# Install via scoop or chocolatey: scoop install poppler  OR  choco install xpdf-utils
```

Also worth knowing:

- Only the **first 10 pages** are extracted (via `pdftotext -layout -l 10`). Invoices, receipts, and two-page contracts work great; 200-page scanned archives may miss content past page 10.
- Detection uses magic bytes, so a text file with a `.pdf` extension is correctly treated as text, not as a broken PDF.

---

## `exec` fails on Windows with an odd error

**Cause:** `exec` runs commands through `sh -c` on Unix and `cmd /c` on Windows. Unix idioms (backticks, `$(…)`, single quotes around `{{.Path}}`, pipes like `|` behave differently in `cmd.exe`) need to be adapted.

**Fix:**

- Use double quotes around path variables on Windows: `"{{.Path}}"` not `'{{.Path}}'`.
- Escape `%` as `%%` inside `cmd.exe` batch strings.
- For anything nontrivial, wrap the command in a `.bat` or PowerShell script and invoke that from the rule.
- Consider using a cross-platform tool (Python, Node) invoked with an absolute path, so the same rule works everywhere.

Example that works on both Unix and Windows (wraps single-line logic in PowerShell on Windows via a named script):

```yaml
- name: stamp-metadata
  match:
    extensions: [.jpg, .png]
  action:
    type: exec
    command: "python3 /usr/local/bin/stamp.py \"{{.Path}}\""
```

---

## Template expansion errors

**Symptoms:**

- `template: dest:1:5: executing "dest" at <.Foo>: can't evaluate field Foo in type rule.TemplateData`
- Empty captures producing dest paths like `~/invoices//file.pdf`

**Causes:**

- A typo in the variable name. The available set is fixed — see [Reference › Template variables](04-reference.md#template-variables). There is no `{{.Dir}}` or `{{.Size}}`.
- An optional named capture didn't match, leaving `{{.Match.company}}` empty.

**Fix:** wrap optional captures in Go's template `or`:

```yaml
dest: '~/invoices/{{or .Match.company "Unknown"}}/{{or .Match.date "undated"}}-{{.Name}}{{.Ext}}'
```

Test the expansion live with `sortie scan --dry-run` on a file you know should match.

---

## A rule seems rate-limited and won't fire

**Cause:** either a global `--rate-limit` is preventing dispatches from happening faster than the configured interval, or the rule has its own per-rule `cooldown:` that hasn't elapsed since the last fire.

**How to inspect:** rate-limit and cooldown state is **in-memory only** — there's no subcommand to inspect it, and restarting `sortie watch` clears all counters.

**Fix:**

1. Confirm whether it's the global `--rate-limit` flag or a per-rule `cooldown:`. Grep your config for `cooldown:`.
2. If you need the rule to fire now, restart the daemon. Watch-mode rate-limit state doesn't persist.
3. For long-running workloads, prefer short cooldowns (`100ms` to `1s`) — they prevent thundering herds without making the daemon feel laggy.

---

## Config hot-reload isn't picking up my changes

**Cause:** the daemon watches both `~/.config/sortie/config.yaml` and any `.sortie.yaml` inside watched directories, with a 500 ms debounce. But if your edit leaves the YAML in an invalid state, the reload fails **silently** — the daemon logs the error and keeps running with the previously-loaded rules, so you don't lose service.

**Fix:**

```sh
sortie validate
```

Run that after every config edit. It catches invalid YAML, unknown action types, and chain ordering bugs (e.g. `compress` before an action that needs the source). If `validate` passes but the daemon still doesn't pick up a change, tail the log:

```sh
tail -f ~/.config/sortie/logs/sortie.log
```

and save the config file — you should see a `config reloaded` or `config reload failed` entry within half a second.

---

## The daemon keeps restarting when I deploy

**Cause:** intentional. `sortie watch` polls its own binary every five seconds and exits gracefully when it detects the file's mtime has changed. That's how `make deploy` can ship a new binary without needing `launchctl unload` — launchd's `KeepAlive` relaunches sortie with the updated code automatically.

**Fix:** nothing — this is the feature. If you want to suppress it for a specific run, run `sortie watch` from a terminal with nohup / screen / tmux rather than under a service manager. The binary-swap detection still fires but nothing restarts it.

---

## Desktop notification reference

How a `notify` action should appear on each platform, side by side:

<!-- TODO: add screenshots — assets/notify-macos.png, assets/notify-linux.png, assets/notify-windows.png -->

- **macOS** — banner with the title at top, body below. Appears in Notification Center history.
- **Linux** — `notify-send` bubble in the top-right (GNOME) or bottom-right (KDE). Styled by your libnotify/desktop-environment theme.
- **Windows** — BurntToast toast in the bottom-right, persisted in Action Center. If you see `[sortie notify] Title: body` in the stderr stream instead of a toast, BurntToast isn't installed (see [above](#windows-notifications-dont-show-up)).

---

## Still stuck?

- Post a question or bug report: https://github.com/msjurset/sortie/issues
- Include: `sortie --version`, the offending YAML, a `sortie scan --dry-run <path>` trace if applicable, and the relevant slice of `sortie history`.
