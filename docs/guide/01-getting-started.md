# Getting Started

> **Status:** scaffolding — full content lands in a follow-up PR.

A 10-minute tour: install sortie, generate a starter config, safely preview what it would do, run it for real, and leave a daemon watching your Downloads folder.

## Install

Platform tabs: macOS (tarball; optional `brew install poppler` for PDF matching), Linux (tarball; `apt install libnotify-bin` for desktop notifications), Windows (zip; optional `Install-Module BurntToast` for toast notifications).

## Your first config

Walk through `sortie init` and what it writes to `~/.config/sortie/config.yaml`.

## Safe preview with `--dry-run`

Demo `sortie scan --dry-run`. Asciinema cast of the output.

## Running for real

`sortie scan`, inspecting `sortie history`, undoing with `sortie undo`.

## Adding a second rule

Edit the YAML to add a PDF-filing rule. Brief mention of priority (forward-link to Concepts).

## Watching in real time

`sortie watch`, drop a file, observe auto-move. Asciinema cast.

## What's next

Link to [Concepts](02-concepts.md) and [Cookbook](03-cookbook.md).
