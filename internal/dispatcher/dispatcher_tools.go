package dispatcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

// doConvert runs an external converter tool with the given arguments template.
// The source file is preserved; output goes to dest.
func doConvert(fi rule.FileInfo, action rule.Action, dest string) error {
	tool := action.Tool
	if tool == "" {
		return fmt.Errorf("convert action requires tool field (e.g., ffmpeg, convert, pandoc)")
	}

	if err := lookPathOrError(tool, "convert"); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	args, err := rule.ExpandString(action.Args, fi)
	if err != nil {
		return fmt.Errorf("expanding args template: %w", err)
	}

	// Replace {{.Dest}} manually since it's not part of TemplateData
	args = strings.ReplaceAll(args, "{{.Dest}}", dest)

	cmd := exec.Command("sh", "-c", tool+" "+args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// doResize resizes an image using sips (macOS default) or imagemagick.
// The source file is preserved; output goes to dest.
func doResize(fi rule.FileInfo, action rule.Action, dest string) error {
	tool := action.Tool
	if tool == "" {
		tool = "sips"
	}

	if err := lookPathOrError(tool, "resize"); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Copy source to dest first, then resize in-place (for sips) or direct output
	switch tool {
	case "sips":
		if err := doCopy(fi.Path, dest); err != nil {
			return fmt.Errorf("copying for resize: %w", err)
		}
		args := []string{}
		if action.Width > 0 {
			args = append(args, "--resampleWidth", fmt.Sprintf("%d", action.Width))
		}
		if action.Height > 0 {
			args = append(args, "--resampleHeight", fmt.Sprintf("%d", action.Height))
		}
		args = append(args, dest)
		cmd := exec.Command("sips", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sips: %s: %w", strings.TrimSpace(string(out)), err)
		}
	default:
		// imagemagick / other tools: convert src -resize WxH dest
		dim := resizeDimension(action)
		cmd := exec.Command(tool, fi.Path, "-resize", dim, dest)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", tool, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func resizeDimension(action rule.Action) string {
	if action.Percentage > 0 {
		return fmt.Sprintf("%d%%", action.Percentage)
	}
	w, h := action.Width, action.Height
	if w > 0 && h > 0 {
		return fmt.Sprintf("%dx%d", w, h)
	}
	if w > 0 {
		return fmt.Sprintf("%d", w)
	}
	if h > 0 {
		return fmt.Sprintf("x%d", h)
	}
	return "100%"
}

// doWatermark stamps an image with an overlay using imagemagick composite.
// The source file is preserved; output goes to dest.
func doWatermark(fi rule.FileInfo, action rule.Action, dest string) error {
	if action.Overlay == "" {
		return fmt.Errorf("watermark action requires overlay field")
	}

	tool := action.Tool
	if tool == "" {
		tool = "composite"
	}

	if err := lookPathOrError(tool, "watermark"); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	gravity := action.Gravity
	if gravity == "" {
		gravity = "center"
	}

	cmd := exec.Command(tool, "-gravity", gravity, action.Overlay, fi.Path, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// doOCR extracts text from an image or PDF using tesseract.
// Returns the output file path. Source is preserved.
func doOCR(fi rule.FileInfo, action rule.Action, dest string) (string, error) {
	tool := action.Tool
	if tool == "" {
		tool = "tesseract"
	}

	if err := lookPathOrError(tool, "ocr"); err != nil {
		return "", err
	}

	// Default dest: sidecar .txt in same directory
	if dest == "" {
		dest = strings.TrimSuffix(fi.Path, filepath.Ext(fi.Path)) + ".txt"
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return dest, fmt.Errorf("creating directory: %w", err)
	}

	lang := action.Language
	if lang == "" {
		lang = "eng"
	}

	// Tesseract appends .txt automatically, so strip it for the output base
	outBase := strings.TrimSuffix(dest, ".txt")

	args := []string{fi.Path, outBase, "-l", lang}
	cmd := exec.Command(tool, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return dest, fmt.Errorf("%s: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}

	// Tesseract creates outBase.txt
	if !strings.HasSuffix(dest, ".txt") {
		dest = outBase + ".txt"
	}
	return dest, nil
}
