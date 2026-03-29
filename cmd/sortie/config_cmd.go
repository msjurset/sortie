package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration",
	Args:  cobra.NoArgs,
	RunE:  runConfigShow,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter config file",
	Args:  cobra.NoArgs,
	RunE:  runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	Args:  cobra.NoArgs,
	RunE:  runConfigPath,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Printf("Config file: %s\n", configPath())
	fmt.Printf("Log dir:     %s\n", cfg.LogDir)
	fmt.Printf("History:     %s\n", cfg.HistoryFile)
	fmt.Printf("Trash:       %s\n", cfg.TrashDir)

	fmt.Printf("\nDirectories: %d\n", len(cfg.Directories))
	for _, d := range cfg.Directories {
		r := ""
		if d.Recursive {
			r = " (recursive)"
		}
		fmt.Printf("  %s%s\n", d.Path, r)
	}

	fmt.Printf("\nGlobal rules: %d\n", len(cfg.Rules))
	for _, r := range cfg.Rules {
		actions := r.ResolvedActions()
		if len(actions) <= 1 {
			fmt.Printf("  %s (%s)\n", r.Name, r.Action.Type)
		} else {
			types := make([]string, len(actions))
			for i, a := range actions {
				types[i] = string(a.Type)
			}
			fmt.Printf("  %s (%s)\n", r.Name, strings.Join(types, " → "))
		}
	}

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Println(configPath())
	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	path := configPath()

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	starter := `# sortie configuration
# See: sortie --help

# Directories to watch (used by 'sortie watch' and 'sortie scan')
directories:
  - path: ~/Downloads
    recursive: false
  # - path: ~/Desktop
  #   recursive: false

# Global rules — evaluated in order, first match wins.
# Per-directory rules (.sortie.yaml) take precedence over these.
rules:
  - name: screen-captures
    match:
      glob: "screencapture-*"
      extensions: [.jpg, .jpeg, .png, .heic, .gif, .webp]
    action:
      type: move
      dest: ~/Downloads/Screenshots/screencap-{{.Date}}_{{.Time}}{{.Ext}}

  # - name: images-to-photos
  #   match:
  #     extensions: [.jpg, .jpeg, .png, .heic, .gif, .webp]
  #   action:
  #     type: move
  #     dest: ~/Downloads/Images/{{.Year}}/{{.Month}}

  - name: installers
    match:
      extensions: [.dmg, .pkg, .app]
    action:
      type: move
      dest: ~/Downloads/Installers

  # - name: pdfs
  #   match:
  #     extensions: [.pdf]
  #   action:
  #     type: move
  #     dest: ~/Downloads/PDFs/{{.Year}}-{{.Month}}

  # - name: old-downloads
  #   match:
  #     min_age: 90d
  #   action:
  #     type: delete

  - name: large-files
    match:
      min_size: 500MB
    action:
      type: move
      dest: ~/Downloads/LargeFiles

  # --- Additional action types ---

  # - name: extract-archives
  #   match:
  #     extensions: [.zip, .tar.gz, .tgz]
  #   action:
  #     type: extract
  #     dest: ~/Downloads/Extracted/{{.Name}}

  # - name: make-executable
  #   match:
  #     extensions: [.sh]
  #   action:
  #     type: chmod
  #     mode: "0755"

  # - name: hash-large-downloads
  #   match:
  #     min_size: 100MB
  #   action:
  #     type: checksum
  #     algorithm: sha256

  # - name: symlink-configs
  #   match:
  #     glob: "*.conf"
  #   action:
  #     type: symlink
  #     dest: ~/configs/{{.Name}}{{.Ext}}

  # - name: strip-exif
  #   match:
  #     extensions: [.jpg, .jpeg]
  #   action:
  #     type: exec
  #     command: "exiftool -all= '{{.Path}}'"

  # - name: new-pdf-alert
  #   match:
  #     extensions: [.pdf]
  #   action:
  #     type: notify
  #     title: "New PDF"
  #     message: "{{.Name}}{{.Ext}} arrived"

  # - name: convert-videos
  #   match:
  #     extensions: [.mov, .avi]
  #   action:
  #     type: convert
  #     tool: ffmpeg
  #     args: "-i {{.Path}} -c:v libx264 {{.Dest}}"
  #     dest: ~/Videos/Converted/{{.Name}}.mp4

  # - name: resize-photos
  #   match:
  #     extensions: [.jpg, .png]
  #     min_size: 5MB
  #   action:
  #     type: resize
  #     width: 1920
  #     dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}

  # - name: watermark-photos
  #   match:
  #     extensions: [.jpg, .png]
  #     glob: "portfolio-*"
  #   action:
  #     type: watermark
  #     overlay: ~/watermark.png
  #     gravity: southeast
  #     dest: ~/Pictures/Watermarked/{{.Name}}{{.Ext}}

  # - name: ocr-scans
  #   match:
  #     extensions: [.png, .tiff]
  #     glob: "scan-*"
  #   action:
  #     type: ocr
  #     language: eng
  #     dest: ~/Documents/OCR/{{.Name}}.txt

  # - name: encrypt-sensitive
  #   match:
  #     glob: "confidential-*"
  #   action:
  #     type: encrypt
  #     recipient: "age1..."
  #     dest: ~/Encrypted/{{.Name}}{{.Ext}}.age

  # - name: decrypt-incoming
  #   match:
  #     extensions: [.age]
  #   action:
  #     type: decrypt
  #     key: ~/.age/key.txt
  #     dest: ~/Decrypted/{{.Name}}

  # - name: backup-to-s3
  #   match:
  #     extensions: [.pdf]
  #   action:
  #     type: upload
  #     remote: "s3://my-bucket/documents/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}"

  # - name: tag-invoices
  #   match:
  #     regex: "(?i)invoice"
  #     extensions: [.pdf]
  #   action:
  #     type: tag
  #     tags: [Red, Finance]

  # - name: open-dmg
  #   match:
  #     extensions: [.dmg]
  #   action:
  #     type: open

  # - name: open-videos-vlc
  #   match:
  #     extensions: [.mkv, .avi]
  #   action:
  #     type: open
  #     app: VLC

  # - name: dedup-downloads
  #   match:
  #     extensions: [.pdf, .zip]
  #   action:
  #     type: deduplicate
  #     dest: ~/Documents/{{.Name}}{{.Ext}}
  #     on_duplicate: skip

  # - name: unquarantine-trusted
  #   match:
  #     extensions: [.dmg, .pkg]
  #     glob: "trusted-*"
  #   action:
  #     type: unquarantine

  # --- Action chaining (multiple actions per rule) ---

  # - name: sort-and-notify
  #   match:
  #     extensions: [.pdf]
  #   actions:
  #     - type: notify
  #       title: "New PDF"
  #       message: "{{.Name}}{{.Ext}} arrived"
  #     - type: move
  #       dest: ~/Documents/PDFs/{{.Year}}/{{.Name}}{{.Ext}}

  # - name: move-chmod-tag
  #   match:
  #     extensions: [.sh]
  #   actions:
  #     - type: move
  #       dest: ~/Scripts/{{.Name}}{{.Ext}}
  #     - type: chmod
  #       mode: "0755"
  #     - type: tag
  #       tags: [Green, Scripts]
`

	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}

func configPath() string {
	if cfgPath != "" {
		return cfgPath
	}
	return defaultConfigPath()
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sortie", "config.yaml")
}
