# sortie User Guide

A comprehensive, recipe-driven guide for power users of the sortie CLI. For a feature overview or install instructions, start with the [project README](../../README.md). For exhaustive flag and option reference, see the [man page](../../sortie.1).

## Who this guide is for

You're comfortable on a terminal and can edit YAML. You don't need to be a Go programmer or know anything about file-system internals — the guide explains the moving parts as they come up.

## Contents

1. [Getting Started](01-getting-started.md) — install sortie, write your first rule, and see it move a file. ~10 minutes.
2. [Concepts](02-concepts.md) — how rules, matches, actions, chains, and the watch daemon fit together.
3. [Cookbook](03-cookbook.md) — recipes for every action type, plus end-to-end multi-step chains for common workflows.
4. [Reference](04-reference.md) — quick-reference tables for flags, match conditions, actions, and template variables.
5. [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes for the most common problems.
6. [Running as a Service](06-running-as-a-service.md) — full walk-throughs for launchd (macOS), systemd user units (Linux), and Task Scheduler (Windows).
7. [Thinking in sortie](07-thinking-in-sortie.md) — design patterns, iterative workflow, migration recipes from Hazel/Maid/cron+find, and anti-patterns to skip past.

## Platform support

sortie runs on macOS, Linux, and Windows. The watcher and core file actions (move, copy, rename, delete, compress, extract, deduplicate) are portable. A handful of actions are macOS-only (`open`, `tag`, `unquarantine`) and the `notify` action uses a different backend on each OS (osascript / notify-send / BurntToast). Platform-specific install steps and caveats are called out inline throughout the guide.
