package actionhelp

import "fmt"

// FieldHelp describes a configuration field for an action type.
type FieldHelp struct {
	Name        string
	Description string
}

// ActionHelp contains the complete help information for an action type.
type ActionHelp struct {
	Name        string
	Description string
	Undoable    bool
	Required    []FieldHelp
	Optional    []FieldHelp
	Example     string
	Tips        []string
	UsefulFor   []string
}

var registry = map[string]ActionHelp{
	"move": {
		Name:        "move",
		Description: "Move a file to the destination path, creating parent directories as needed. If the source and destination are on different filesystems, the file is copied then the original is removed.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Destination path (template-expanded)"}},
		Example: `- name: sort-images
  match:
    extensions: [.jpg, .jpeg, .png]
  action:
    type: move
    dest: ~/Pictures/Sorted/{{.Year}}/{{.Month}}/{{.Name}}{{.Ext}}`,
		Tips:      []string{"Use template variables like {{.Year}} and {{.Month}} to organize by date", "If dest is a directory (no extension), the original filename is appended"},
		UsefulFor: []string{"Organizing downloads by type", "Sorting photos into date-based folders", "Routing files to project directories"},
	},
	"copy": {
		Name:        "copy",
		Description: "Copy a file to the destination, preserving the original. File permissions are preserved.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Destination path (template-expanded)"}},
		Example: `- name: backup-invoices
  match:
    extensions: [.pdf]
    content: "invoice"
  action:
    type: copy
    dest: ~/Backups/Invoices/{{.Name}}{{.Ext}}`,
		UsefulFor: []string{"Creating backups of important files", "Mirroring files to a shared folder"},
	},
	"rename": {
		Name:        "rename",
		Description: "Rename a file in place using a template. The file stays in the same directory.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "New file path (template-expanded)"}},
		Example: `- name: date-prefix-screenshots
  match:
    glob: "Screenshot*"
  action:
    type: rename
    dest: ~/Downloads/{{.Date}}_{{.Name}}{{.Ext}}`,
		UsefulFor: []string{"Adding date prefixes to files", "Standardizing filenames"},
	},
	"delete": {
		Name:        "delete",
		Description: "Move a file to the sortie trash directory (~/.config/sortie/trash/). The file can be restored with `sortie undo`.",
		Undoable:    true,
		Example: `- name: cleanup-old-temp
  match:
    extensions: [.tmp, .bak]
    min_age: 30d
  action:
    type: delete`,
		UsefulFor: []string{"Cleaning up old temporary files", "Removing stale downloads"},
	},
	"compress": {
		Name:        "compress",
		Description: "Gzip-compress the file and remove the original. The output file has a .gz extension.",
		Undoable:    true,
		Optional:    []FieldHelp{{Name: "dest", Description: "Output path (defaults to source + .gz)"}},
		Example: `- name: compress-old-logs
  match:
    extensions: [.log]
    min_age: 7d
  action:
    type: compress
    dest: ~/Archives/{{.Name}}{{.Ext}}.gz`,
		UsefulFor: []string{"Archiving old log files", "Saving disk space on large text files"},
	},
	"extract": {
		Name:        "extract",
		Description: "Extract an archive into the destination directory. Supports .zip, .tar, .tar.gz/.tgz, .tar.bz2, and .tar.xz. macOS metadata (__MACOSX, ._* resource forks, .DS_Store) is automatically stripped.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Directory to extract into (template-expanded)"}},
		Example: `- name: extract-archives
  match:
    extensions: [.zip, .tar.gz, .tgz]
  action:
    type: extract
    dest: ~/Downloads/Extracted/{{.Name}}`,
		Tips:      []string{".zip and .tar.gz use Go stdlib (no external tools needed)", ".tar.xz requires the tar command"},
		UsefulFor: []string{"Auto-extracting downloaded archives", "Unpacking source code tarballs"},
	},
	"symlink": {
		Name:        "symlink",
		Description: "Create a symbolic link at dest pointing to the original file. The source file is preserved.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Symlink path (template-expanded)"}},
		Example: `- name: link-configs
  match:
    glob: "*.conf"
  action:
    type: symlink
    dest: ~/configs/{{.Name}}{{.Ext}}`,
		UsefulFor: []string{"Linking config files to a central directory", "Creating shortcuts without copying"},
	},
	"chmod": {
		Name:        "chmod",
		Description: "Change file permissions. The original permissions are stored in history for undo.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "mode", Description: "Octal permission string (e.g., \"0755\", \"644\")"}},
		Example: `- name: make-scripts-executable
  match:
    extensions: [.sh]
  action:
    type: chmod
    mode: "0755"`,
		UsefulFor: []string{"Making downloaded scripts executable", "Fixing permissions on imported files"},
	},
	"checksum": {
		Name:        "checksum",
		Description: "Compute a hash of the file and write a sidecar file (e.g., file.sha256). The sidecar contains the hash and filename in BSD format.",
		Undoable:    true,
		Optional: []FieldHelp{
			{Name: "algorithm", Description: "Hash algorithm: sha256 (default), md5, or sha1"},
			{Name: "dest", Description: "Sidecar output path (defaults to source + .algorithm)"},
		},
		Example: `- name: hash-downloads
  match:
    min_size: 100MB
  action:
    type: checksum
    algorithm: sha256`,
		UsefulFor: []string{"Verifying file integrity after download", "Creating checksums for distribution"},
	},
	"exec": {
		Name:        "exec",
		Description: "Run an arbitrary shell command. The command field supports template variables including {{.Path}} for the full source file path.",
		Undoable:    false,
		Required:    []FieldHelp{{Name: "command", Description: "Shell command (template-expanded, run via sh -c)"}},
		Example: `- name: strip-exif
  match:
    extensions: [.jpg, .jpeg]
  action:
    type: exec
    command: "exiftool -all= '{{.Path}}'"`,
		Tips:      []string{"Use single quotes around {{.Path}} to handle filenames with spaces", "The command runs via sh -c, so pipes and redirects work", "This is the escape hatch — anything you can do in a shell, you can do here"},
		UsefulFor: []string{"Stripping EXIF data from photos", "Running custom scripts on new files", "Any automation not covered by built-in actions"},
	},
	"notify": {
		Name:        "notify",
		Description: "Send a desktop notification (macOS) or HTTP webhook. On macOS, uses osascript. If the message starts with http:// or https://, sends an HTTP POST with file metadata as JSON.",
		Undoable:    false,
		Optional: []FieldHelp{
			{Name: "title", Description: "Notification title (template-expanded, default: \"sortie\")"},
			{Name: "message", Description: "Notification body or webhook URL (template-expanded)"},
		},
		Example: `- name: pdf-alert
  match:
    extensions: [.pdf]
  action:
    type: notify
    title: "New PDF"
    message: "{{.Name}}{{.Ext}} arrived in Downloads"`,
		Tips:      []string{"For webhooks, set message to the URL — file metadata is sent as JSON body", "Combine with action chaining to notify AND move in one rule"},
		UsefulFor: []string{"Alerting when important files arrive", "Triggering webhooks for automation pipelines", "Monitoring download activity"},
	},
	"convert": {
		Name:        "convert",
		Description: "Run an external converter tool. The source file is preserved and output goes to dest. Use {{.Path}} in args for the input file and {{.Dest}} for the output.",
		Undoable:    true,
		Required: []FieldHelp{
			{Name: "tool", Description: "External binary name (e.g., ffmpeg, convert, pandoc)"},
			{Name: "dest", Description: "Output file path (template-expanded)"},
		},
		Optional: []FieldHelp{{Name: "args", Description: "Arguments template — use {{.Path}} for input, {{.Dest}} for output"}},
		Example: `- name: convert-videos
  match:
    extensions: [.mov, .avi, .mkv]
  action:
    type: convert
    tool: ffmpeg
    args: "-i {{.Path}} -c:v libx264 -crf 23 {{.Dest}}"
    dest: ~/Videos/Converted/{{.Name}}.mp4`,
		Tips:      []string{"Install the tool first: brew install ffmpeg", "The source file is never modified — safe to experiment", "Use {{.Dest}} in args to reference the output path"},
		UsefulFor: []string{"Converting video formats (MOV to MP4)", "Transcoding audio (FLAC to MP3)", "Converting documents (DOCX to PDF via pandoc)"},
	},
	"resize": {
		Name:        "resize",
		Description: "Resize an image. Default tool is sips (built into macOS). Set tool to \"convert\" for ImageMagick. Source file is preserved.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Output file path (template-expanded)"}},
		Optional: []FieldHelp{
			{Name: "width", Description: "Target width in pixels"},
			{Name: "height", Description: "Target height in pixels"},
			{Name: "percentage", Description: "Scale percentage (e.g., 50)"},
			{Name: "tool", Description: "Binary name (default: sips, alternative: convert)"},
		},
		Example: `- name: resize-photos
  match:
    extensions: [.jpg, .png]
    min_size: 5MB
  action:
    type: resize
    width: 1920
    dest: ~/Pictures/Resized/{{.Name}}{{.Ext}}`,
		Tips:      []string{"Specify width, height, or percentage (at least one required)", "sips is built into macOS — no install needed"},
		UsefulFor: []string{"Creating thumbnails", "Reducing image size for web upload", "Batch-resizing photos"},
	},
	"watermark": {
		Name:        "watermark",
		Description: "Stamp an image with an overlay using ImageMagick composite. Source file is preserved.",
		Undoable:    true,
		Required: []FieldHelp{
			{Name: "overlay", Description: "Path to watermark image"},
			{Name: "dest", Description: "Output file path (template-expanded)"},
		},
		Optional: []FieldHelp{
			{Name: "gravity", Description: "Placement: center (default), north, south, east, west, northeast, northwest, southeast, southwest"},
			{Name: "tool", Description: "Binary name (default: composite)"},
		},
		Example: `- name: watermark-portfolio
  match:
    extensions: [.jpg, .png]
    glob: "portfolio-*"
  action:
    type: watermark
    overlay: ~/watermark.png
    gravity: southeast
    dest: ~/Pictures/Watermarked/{{.Name}}{{.Ext}}`,
		Tips:      []string{"Install ImageMagick: brew install imagemagick", "Use a transparent PNG for the overlay"},
		UsefulFor: []string{"Protecting portfolio images", "Branding photos before sharing"},
	},
	"ocr": {
		Name:        "ocr",
		Description: "Extract text from images or PDFs using tesseract. Output is a .txt sidecar file.",
		Undoable:    true,
		Optional: []FieldHelp{
			{Name: "dest", Description: "Output .txt path (defaults to sidecar in same directory)"},
			{Name: "language", Description: "Tesseract language code (default: eng)"},
			{Name: "tool", Description: "Binary name (default: tesseract)"},
		},
		Example: `- name: ocr-scans
  match:
    extensions: [.png, .tiff]
    glob: "scan-*"
  action:
    type: ocr
    language: eng
    dest: ~/Documents/OCR/{{.Name}}.txt`,
		Tips:      []string{"Install tesseract: brew install tesseract", "For additional languages: brew install tesseract-lang"},
		UsefulFor: []string{"Extracting text from scanned documents", "Making scanned PDFs searchable"},
	},
	"encrypt": {
		Name:        "encrypt",
		Description: "Encrypt a file using age (default) or gpg. Source file is preserved.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "recipient", Description: "Public key or key ID (age: \"age1...\", gpg: email or key ID)"}},
		Optional: []FieldHelp{
			{Name: "dest", Description: "Output path (template-expanded)"},
			{Name: "tool", Description: "Binary name: age (default) or gpg"},
		},
		Example: `- name: encrypt-sensitive
  match:
    glob: "confidential-*"
  action:
    type: encrypt
    recipient: "age1abc..."
    dest: ~/Encrypted/{{.Name}}{{.Ext}}.age`,
		Tips:      []string{"Install age: brew install age", "Generate keys with: age-keygen -o key.txt"},
		UsefulFor: []string{"Encrypting sensitive documents automatically", "Securing files before cloud upload"},
	},
	"decrypt": {
		Name:        "decrypt",
		Description: "Decrypt a file using age (default) or gpg. Source file is preserved.",
		Undoable:    true,
		Optional: []FieldHelp{
			{Name: "dest", Description: "Output path (template-expanded)"},
			{Name: "key", Description: "Key file path (age: identity file, gpg: keyring)"},
			{Name: "tool", Description: "Binary name: age (default) or gpg"},
		},
		Example: `- name: decrypt-incoming
  match:
    extensions: [.age]
  action:
    type: decrypt
    key: ~/.age/key.txt
    dest: ~/Decrypted/{{.Name}}`,
		UsefulFor: []string{"Auto-decrypting received encrypted files", "Processing encrypted data drops"},
	},
	"upload": {
		Name:        "upload",
		Description: "Upload a file to cloud storage. Auto-detects the tool from URI scheme: s3:// uses aws CLI, gs:// uses gsutil.",
		Undoable:    false,
		Required:    []FieldHelp{{Name: "remote", Description: "Destination URI (template-expanded, e.g., s3://bucket/path)"}},
		Optional:    []FieldHelp{{Name: "tool", Description: "Override auto-detection (e.g., aws, gsutil)"}},
		Example: `- name: backup-to-s3
  match:
    extensions: [.pdf]
  action:
    type: upload
    remote: "s3://my-bucket/documents/{{.Year}}/{{.Name}}{{.Ext}}"`,
		Tips:      []string{"Configure AWS credentials before use: aws configure", "Use a cooldown to avoid rapid uploads: cooldown: 5s"},
		UsefulFor: []string{"Backing up files to S3 or GCS", "Syncing documents to cloud storage"},
	},
	"tag": {
		Name:        "tag",
		Description: "Apply macOS Finder tags to a file via xattr. Tags appear in Finder and can be used for Smart Folders.",
		Undoable:    false,
		Required:    []FieldHelp{{Name: "tags", Description: "List of tag names (e.g., [Red, Finance])"}},
		Example: `- name: tag-invoices
  match:
    regex: "(?i)invoice"
    extensions: [.pdf]
  action:
    type: tag
    tags: [Red, Finance]`,
		Tips:      []string{"Standard macOS colors: Red, Orange, Yellow, Green, Blue, Purple, Gray", "Custom tag names work too — they'll appear in Finder"},
		UsefulFor: []string{"Color-coding files in Finder", "Categorizing documents for Smart Folders"},
	},
	"open": {
		Name:        "open",
		Description: "Open a file with the default application or a specified app. Uses macOS open(1).",
		Undoable:    false,
		Optional:    []FieldHelp{{Name: "app", Description: "Application name (e.g., VLC, Preview, TextEdit)"}},
		Example: `- name: open-videos-vlc
  match:
    extensions: [.mkv, .avi]
  action:
    type: open
    app: VLC`,
		UsefulFor: []string{"Auto-mounting disk images (.dmg)", "Opening downloads in a specific app", "Launching installers"},
	},
	"deduplicate": {
		Name:        "deduplicate",
		Description: "Check if an identical file (by SHA-256 hash) already exists at dest. If no duplicate, moves the file. If a duplicate is found, behavior depends on on_duplicate setting.",
		Undoable:    true,
		Required:    []FieldHelp{{Name: "dest", Description: "Target path to check for duplicates (template-expanded)"}},
		Optional:    []FieldHelp{{Name: "on_duplicate", Description: "Action when duplicate found: skip (default) or delete"}},
		Example: `- name: dedup-downloads
  match:
    extensions: [.pdf, .zip]
  action:
    type: deduplicate
    dest: ~/Documents/{{.Name}}{{.Ext}}
    on_duplicate: skip`,
		Tips:      []string{"skip: leaves the source file in place (safe default)", "delete: removes the source file (saves space, not undoable)"},
		UsefulFor: []string{"Preventing duplicate downloads", "Deduplicating files across directories"},
	},
	"unquarantine": {
		Name:        "unquarantine",
		Description: "Remove the macOS com.apple.quarantine extended attribute. No-op if the attribute is not present.",
		Undoable:    false,
		Example: `- name: unquarantine-trusted
  match:
    extensions: [.dmg, .pkg]
    glob: "trusted-*"
  action:
    type: unquarantine`,
		Tips:      []string{"Only use for files from sources you trust", "Combine with match conditions to be selective"},
		UsefulFor: []string{"Removing Gatekeeper prompts for trusted installers", "Streamlining developer tool installs"},
	},
}

// Get returns the help for an action type by name.
func Get(name string) (ActionHelp, bool) {
	h, ok := registry[name]
	return h, ok
}

// List returns all registered action types sorted by name.
func List() []ActionHelp {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	// Sort for consistent output
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	result := make([]ActionHelp, len(names))
	for i, name := range names {
		result[i] = registry[name]
	}
	return result
}

// Format returns a formatted help string for an action type.
func Format(h ActionHelp) string {
	var b fmt.Stringer = &helpFormatter{h: h}
	return b.String()
}

type helpFormatter struct {
	h ActionHelp
}

func (f *helpFormatter) String() string {
	h := f.h
	s := fmt.Sprintf("\n  %s", h.Name)
	if h.Undoable {
		s += " (undoable)"
	}
	s += fmt.Sprintf("\n\n  %s\n", h.Description)

	if len(h.Required) > 0 {
		s += "\n  REQUIRED FIELDS:\n"
		for _, f := range h.Required {
			s += fmt.Sprintf("    %-15s %s\n", f.Name, f.Description)
		}
	}

	if len(h.Optional) > 0 {
		s += "\n  OPTIONAL FIELDS:\n"
		for _, f := range h.Optional {
			s += fmt.Sprintf("    %-15s %s\n", f.Name, f.Description)
		}
	}

	if h.Example != "" {
		s += "\n  EXAMPLE:\n"
		for _, line := range splitLines(h.Example) {
			s += "    " + line + "\n"
		}
	}

	if len(h.Tips) > 0 {
		s += "\n  TIPS:\n"
		for _, tip := range h.Tips {
			s += "    • " + tip + "\n"
		}
	}

	if len(h.UsefulFor) > 0 {
		s += "\n  USEFUL FOR:\n"
		for _, use := range h.UsefulFor {
			s += "    • " + use + "\n"
		}
	}

	return s
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
