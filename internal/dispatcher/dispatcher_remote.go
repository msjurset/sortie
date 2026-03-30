package dispatcher

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

// doUpload pushes a file to a remote destination. The tool is auto-detected
// from the URI scheme (s3:// -> aws, gs:// -> gsutil) or can be set explicitly.
func doUpload(fi rule.FileInfo, action rule.Action, captures map[string]string) error {
	remote, err := rule.ExpandString(action.Remote, fi, captures)
	if err != nil {
		return fmt.Errorf("expanding remote template: %w", err)
	}

	if remote == "" {
		return fmt.Errorf("upload action requires remote field")
	}

	tool := action.Tool
	if tool == "" {
		tool = detectUploadTool(remote)
	}

	if err := lookPathOrError(tool, "upload"); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch tool {
	case "aws":
		cmd = exec.Command("aws", "s3", "cp", fi.Path, remote)
	case "gsutil":
		cmd = exec.Command("gsutil", "cp", fi.Path, remote)
	default:
		// Generic: tool src remote
		cmd = exec.Command(tool, fi.Path, remote)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s upload: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func detectUploadTool(remote string) string {
	switch {
	case strings.HasPrefix(remote, "s3://"):
		return "aws"
	case strings.HasPrefix(remote, "gs://"):
		return "gsutil"
	default:
		return "aws"
	}
}

// doTag applies macOS Finder tags to a file using xattr.
func doTag(fi rule.FileInfo, action rule.Action) error {
	if len(action.Tags) == 0 {
		return fmt.Errorf("tag action requires tags field")
	}

	plist := buildTagsPlist(action.Tags)

	cmd := exec.Command("xattr", "-w", "com.apple.metadata:_kMDItemUserTags", plist, fi.Path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xattr: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// buildTagsPlist constructs an XML plist representing a string array of tags.
func buildTagsPlist(tags []string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`)
	b.WriteString(`<plist version="1.0"><array>`)
	for _, tag := range tags {
		b.WriteString("<string>")
		b.WriteString(xmlEscape(tag))
		b.WriteString("</string>")
	}
	b.WriteString(`</array></plist>`)
	return b.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
