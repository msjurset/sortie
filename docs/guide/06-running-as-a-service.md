# Running sortie as a Service

`sortie watch` is a long-running daemon. Running it from a terminal is fine for trying things out, but you'll want it under a process supervisor for permanent operation: starts at login, restarts on crash, survives logout, and writes logs to a known place.

This page is one walk-through per platform. Each section is self-contained — skip to your OS.

- [macOS — launchd user agent](#macos--launchd-user-agent)
- [Linux — systemd user unit](#linux--systemd-user-unit)
- [Windows — Task Scheduler](#windows--task-scheduler)
- [Cross-platform notes](#cross-platform-notes) — binary self-monitoring, log rotation, debugging tips

---

## macOS — launchd user agent

launchd is macOS's native service manager. Running sortie as a **user agent** (under your login session) is the right model: it starts when you log in, runs as your user, and has access to the directories your config watches.

### Install via the Makefile

The repo ships a launchd template and a Makefile target that does the right thing:

```sh
make install-launchd
```

This:

1. Unloads any existing `com.msjurset.sortie` agent (idempotent — safe to re-run).
2. Creates `~/Library/LaunchAgents/` and `~/.config/sortie/logs/` if missing.
3. Renders `com.msjurset.sortie.plist.tpl` with your binary path and log path baked in, and writes it to `~/Library/LaunchAgents/com.msjurset.sortie.plist`.
4. Loads the new plist with `launchctl load`. The daemon starts immediately.

You can verify with:

```sh
launchctl list | grep sortie
# 12345  0  com.msjurset.sortie
```

The first column is the PID. If it's `-`, the agent is loaded but not running; check the logs.

### Install manually (without `make`)

If you installed sortie from a release archive and don't have the source repo, write the plist by hand:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.msjurset.sortie</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/sortie</string>
        <string>watch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/.config/sortie/logs/sortie.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/.config/sortie/logs/sortie.log</string>
    <key>ProcessType</key>
    <string>Background</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
```

Save as `~/Library/LaunchAgents/com.msjurset.sortie.plist`, edit the binary path and `YOUR_USERNAME`, then:

```sh
launchctl load ~/Library/LaunchAgents/com.msjurset.sortie.plist
```

> **Why absolute paths?** launchd doesn't expand `~` or `$HOME` inside the plist. `StandardOutPath` and `ProgramArguments` must be absolute.

> **Why the `PATH` env var?** launchd starts services with a minimal `PATH` (`/usr/bin:/bin:/usr/sbin:/sbin`) that doesn't include Homebrew's `/opt/homebrew/bin` or `/usr/local/bin`. Without this entry, any rule that runs an `exec` action invoking a Homebrew-installed tool (`ocrmypdf`, `ocrit`, `ffmpeg`, `pdftotext`, etc.) fails with `command not found` — even if the tool works fine when you run sortie from a terminal. The terminal inherits your shell's `PATH`; the daemon doesn't.

### Check status and logs

```sh
launchctl list | grep sortie         # is it loaded?
sortie status                        # what is it doing right now?
tail -f ~/.config/sortie/logs/sortie.log
```

`sortie status` shows the live state of the watcher — directories monitored, debounce settings, recent dispatches. It's the most useful single command for "is this thing actually working?"

For the OS-level perspective:

```sh
log show --predicate 'process == "sortie"' --last 1h
```

That pulls sortie-related entries from the unified log, which catches problems that happen before the daemon's own log file is opened.

### Update sortie binary

The daemon polls its own binary's mtime every 5 seconds and exits gracefully when it detects a change. Combined with `KeepAlive`, that means deploying a new binary auto-restarts the daemon:

```sh
make deploy
# Within ~5 seconds, the running daemon detects the new binary and exits.
# launchd's KeepAlive immediately relaunches it with the updated code.
```

Tail the log during the deploy to watch the handoff:

```
INFO  binary changed, restarting old=... new=...
INFO  shutting down...
INFO  starting watcher version=v0.4.2 ...
```

If you don't want the auto-restart for some reason (e.g. you're scripting a coordinated deploy), unload the agent first, deploy, then load it back.

> **macOS code-signing note.** `make deploy` uses `install -m 755` rather than `cp` to write the new binary. `install` writes to a temp file then `rename(2)`s into place, giving the new bytes a fresh inode. If you replace an in-use binary in place with `cp`, macOS's code-signing cache can flag the inode mismatch and `SIGKILL` future invocations of the binary. If your home-brewed deploy script uses `cp`, switch to `install` for this reason.

### Uninstall

```sh
make uninstall-launchd
```

Or manually:

```sh
launchctl unload ~/Library/LaunchAgents/com.msjurset.sortie.plist
rm ~/Library/LaunchAgents/com.msjurset.sortie.plist
```

The config and history files are left in place — uninstalling the service doesn't touch your data.

### Troubleshooting (macOS)

**The agent loads but exits immediately.** Check the log file. The most common cause is a missing or invalid `~/.config/sortie/config.yaml`. Run `sortie validate` from a terminal first.

**`launchctl load` says "Operation not permitted."** macOS sometimes asks for permission to run background agents on first install. Check System Settings → General → Login Items & Extensions; you should see "sortie" listed.

**The daemon runs but doesn't dispatch any files.** Run `sortie watch` directly in a terminal (not via launchd) and drop a test file. If it works in the terminal but not via launchd, the agent likely doesn't have Full Disk Access. Grant it under System Settings → Privacy & Security → Full Disk Access; you may need to add the sortie binary explicitly.

**Logs are empty even though the daemon is "running."** The plist's `StandardOutPath` directory must exist before launchd opens the file. If `~/.config/sortie/logs/` was deleted, the daemon writes to `/dev/null`. Recreate the directory and re-load the agent.

---

## Linux — systemd user unit

A user unit is the right level of isolation: sortie runs as your user, with access to your home directory, and doesn't need root privileges. The unit lives in `~/.config/systemd/user/` and is managed with `systemctl --user`.

### Install

Save this file as `~/.config/systemd/user/sortie.service`:

```ini
[Unit]
Description=sortie file dispatcher
After=default.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sortie watch
Restart=on-failure
RestartSec=2
StandardOutput=append:%h/.config/sortie/logs/sortie.log
StandardError=append:%h/.config/sortie/logs/sortie.log

[Install]
WantedBy=default.target
```

Then:

```sh
mkdir -p ~/.config/sortie/logs
systemctl --user daemon-reload
systemctl --user enable --now sortie
```

`enable --now` enables the unit at next login AND starts it immediately.

### Make it survive logout

By default, user units stop when you log out of the last session. For a long-running watcher this is rarely what you want. Enable lingering for your user:

```sh
sudo loginctl enable-linger $USER
```

This causes systemd to run your user manager continuously (regardless of login state), so `sortie.service` keeps running even when you're not logged in. Combine with `Restart=on-failure` and you have a daemon that comes up on boot and stays up across crashes and updates.

### Check status and logs

```sh
systemctl --user status sortie
sortie status
journalctl --user -u sortie -n 200 -f
```

`journalctl -f` follows the log in real time — great for watching what the daemon is doing during a deploy or while debugging a rule.

### Update sortie binary

Same self-monitoring pattern as macOS: deploy the new binary and the daemon exits gracefully within ~5 seconds. systemd's `Restart=on-failure` brings it back. Watch the handoff:

```sh
journalctl --user -u sortie -f
# Drop in a new binary in another terminal: cp sortie-new /usr/local/bin/sortie
# You'll see: "binary changed, restarting" → "shutting down" → "starting watcher version=..."
```

### Uninstall

```sh
systemctl --user disable --now sortie
rm ~/.config/systemd/user/sortie.service
systemctl --user daemon-reload
```

If you enabled linger and don't have any other user units, you can disable it: `sudo loginctl disable-linger $USER`.

### Troubleshooting (Linux)

**`systemctl --user start sortie` fails with "Unit not found."** You forgot `daemon-reload` after writing the file, or the file is in the wrong directory. It must be at `~/.config/systemd/user/sortie.service` exactly (or in `/etc/systemd/user/` for system-wide user units, but per-user is preferred).

**The unit reports "active (running)" but logs are empty.** Confirm `~/.config/sortie/logs/` exists. systemd's `append:%h/...` syntax requires the directory; it doesn't create it.

**`Restart=on-failure` keeps restarting in a loop.** That's the daemon erroring out repeatedly. Check `journalctl --user -u sortie -n 50`. The most common cause is invalid YAML in the config — run `sortie validate` from a normal terminal.

**On a remote server, the service stops when I log out.** You haven't enabled lingering. See [Make it survive logout](#make-it-survive-logout) above.

**inotify watch limit exceeded.** Linux has a per-user inotify watch limit (`/proc/sys/fs/inotify/max_user_watches`). For very large watched trees, raise it:

```sh
echo fs.inotify.max_user_watches=524288 | sudo tee /etc/sysctl.d/40-sortie.conf
sudo sysctl -p /etc/sysctl.d/40-sortie.conf
```

---

## Windows — Task Scheduler

Windows has no first-class user-service equivalent of launchd or systemd-user, but Task Scheduler can run `sortie watch` at logon and restart it on failure. The setup is wordier than the Unix versions; use the PowerShell path below for a one-shot install.

### Install via PowerShell (recommended)

Open PowerShell **as your normal user** (not elevated — it's a per-user task) and run:

```powershell
$action = New-ScheduledTaskAction `
    -Execute "C:\Users\$env:USERNAME\bin\sortie.exe" `
    -Argument "watch" `
    -WorkingDirectory "C:\Users\$env:USERNAME"

$trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERDOMAIN\$env:USERNAME"

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -RestartCount 999 `
    -RestartInterval (New-TimeSpan -Minutes 1) `
    -ExecutionTimeLimit ([TimeSpan]::Zero)   # never time out

$principal = New-ScheduledTaskPrincipal `
    -UserId "$env:USERDOMAIN\$env:USERNAME" `
    -LogonType Interactive

Register-ScheduledTask -TaskName "sortie" `
    -Action $action -Trigger $trigger -Settings $settings -Principal $principal `
    -Description "sortie file dispatcher daemon"
```

That registers a logon-triggered task that runs `sortie.exe watch` as you, retries up to 999 times on failure with one-minute spacing, and never times out. Adjust the binary path if you put `sortie.exe` somewhere other than `~\bin\`.

To start it without logging out:

```powershell
Start-ScheduledTask -TaskName "sortie"
Get-ScheduledTask -TaskName "sortie" | Get-ScheduledTaskInfo
```

### Install via the GUI

If you'd rather click through it: **Task Scheduler → Create Task** (not the wizard).

- **General tab:**
  - Name: `sortie`
  - Run only when user is logged on
  - Configure for: Windows 10/11
- **Triggers tab → New:**
  - Begin the task: At log on
  - Specific user: your account
- **Actions tab → New:**
  - Action: Start a program
  - Program/script: `C:\Users\<you>\bin\sortie.exe`
  - Add arguments: `watch`
  - Start in: `C:\Users\<you>` (not the directory the binary lives in — sortie reads its config relative to your home directory)
- **Conditions tab:** uncheck "Start the task only if the computer is on AC power"
- **Settings tab:**
  - Allow task to be run on demand (default)
  - If the task fails, restart every: 1 minute
  - Attempt to restart up to: 999 times
  - Stop the task if it runs longer than: uncheck (or set to a very large value)

### Check status and logs

By default, Task Scheduler doesn't capture stdout. To get a logfile, wrap the invocation in a tiny PowerShell launcher:

Save this as `C:\Users\<you>\bin\run-sortie.ps1`:

```powershell
$logDir = "$env:USERPROFILE\.config\sortie\logs"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
& "$env:USERPROFILE\bin\sortie.exe" watch *>&1 | Tee-Object -FilePath "$logDir\sortie.log" -Append
```

Update the scheduled task's **Action** to:

- Program/script: `powershell.exe`
- Arguments: `-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File C:\Users\<you>\bin\run-sortie.ps1`

Now `~\.config\sortie\logs\sortie.log` accumulates everything sortie prints. Tail it with:

```powershell
Get-Content "$env:USERPROFILE\.config\sortie\logs\sortie.log" -Wait -Tail 50
```

Task-level state (last run time, next run, last result code) lives in Task Scheduler. Get a quick summary with:

```powershell
Get-ScheduledTask -TaskName "sortie" | Get-ScheduledTaskInfo
```

You can also check `sortie status` like on the other platforms.

### Update sortie binary

The binary self-monitor works the same way on Windows as on Unix — the daemon polls mtime and exits when it changes. Task Scheduler's restart settings then bring it back. To deploy a new build:

```powershell
Stop-ScheduledTask -TaskName "sortie"     # optional: stop cleanly
Copy-Item .\sortie-new.exe "$env:USERPROFILE\bin\sortie.exe" -Force
Start-ScheduledTask -TaskName "sortie"
```

If you skip the manual stop/start, the daemon will detect the binary swap and exit on its own; Task Scheduler retries within 1 minute.

### Uninstall

```powershell
Stop-ScheduledTask -TaskName "sortie"
Unregister-ScheduledTask -TaskName "sortie" -Confirm:$false
```

Config, history, and logs under `~\.config\sortie\` are left in place.

### Troubleshooting (Windows)

**The task shows "Last Run Result: 0x1" and exits immediately.** The binary path or arguments are wrong, or `sortie.exe` errored on startup. Run the same command in a normal PowerShell window and read the error.

**Toasts don't appear when the task is running but do when I run sortie manually.** BurntToast may need a logged-in interactive session to display. Confirm the task's principal has "Interactive" logon type (the PowerShell snippet above sets this; the GUI default is also "Run only when user is logged on" which is interactive).

**The task runs but doesn't restart on crash.** Check Task Scheduler → sortie → Settings tab. The "If the task fails, restart every" checkbox must be ticked. If it isn't, restart counts are ignored.

**Permissions errors writing to `C:\Users\<you>\.config\sortie\logs\sortie.log`.** Use `$env:USERPROFILE` rather than hardcoded paths in the launcher script — it resolves correctly even when usernames have spaces or non-ASCII characters.

---

## Cross-platform notes

A few behaviors that apply to all three setups.

### Binary self-monitoring

`sortie watch` polls its own executable's mtime every 5 seconds. When it detects a change, it logs `binary changed, restarting` and exits with a normal status. The supervisor (launchd / systemd / Task Scheduler) restarts it within seconds. No manual coordination needed during deploys — copy the new binary into place and the daemon updates itself.

This works because the daemon resolves symlinks at startup and stats the resolved path, so it works whether you deploy by replacing the binary in-place or by atomic-renaming a new binary over the old one (the safer pattern under load).

### Log rotation

sortie writes a single growing logfile by default. For long-running daemons this isn't great. Three options:

1. **External rotator** — `logrotate` on Linux, `newsyslog` on macOS, a scheduled PowerShell rotator on Windows. Rotate by size or time.
2. **Use sortie itself** — point a sortie rule at `~/.config/sortie/logs/` with `watch_existing: true`, `min_size: 100MB`, and a chain of `compress` and `delete` (see the [log rotator recipe](03-cookbook.md#log-rotator)). Yes, you can use sortie to rotate sortie's own logs.
3. **Switch to JSON logs and pipe to a journal** — start the daemon with `--log-format json` and pipe stdout to `journalctl` (Linux), the Apple unified log (macOS), or Event Viewer (Windows via PowerShell `Write-EventLog`). Removes the file-on-disk problem entirely.

### Debugging tips

When a service-managed daemon misbehaves, **stop the supervisor and run sortie watch directly in a terminal first**. The terminal session shows errors instantly that the supervisor would swallow into a logfile you may not be tailing.

```sh
# macOS
launchctl unload ~/Library/LaunchAgents/com.msjurset.sortie.plist
sortie watch                       # see errors live; Ctrl-C when done
launchctl load ~/Library/LaunchAgents/com.msjurset.sortie.plist
```

```sh
# Linux
systemctl --user stop sortie
sortie watch
systemctl --user start sortie
```

```powershell
# Windows
Stop-ScheduledTask -TaskName "sortie"
sortie watch
Start-ScheduledTask -TaskName "sortie"
```

This single trick catches roughly 80% of "service won't start" reports — almost always a config issue that the supervisor's log-redirection makes invisible.

### Permissions and access

| Platform | What the daemon needs |
|----------|----------------------|
| macOS | Full Disk Access if watching `~/Documents`, `~/Desktop`, or `~/Downloads` (modern macOS). System Settings → Privacy & Security → Full Disk Access → add `sortie`. |
| Linux | Read/write on every watched directory and every `dest:` path. inotify watch limit (`fs.inotify.max_user_watches`) raised for large trees. |
| Windows | Read/write on watched and dest directories. If running unattended, a non-expiring user account password (or the task configured to "Run whether user is logged on or not"). |

### Where to go next

- [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes including the "daemon stopped unexpectedly" entry that maps to all three platforms.
- [Cookbook → Log rotator](03-cookbook.md#log-rotator) — using sortie to rotate its own logs.
- The [main README's "Running as a Service" section](../../README.md#running-as-a-service-macos) covers the macOS Makefile workflow in less detail.
