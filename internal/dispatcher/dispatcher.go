package dispatcher

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
)

// Dispatcher executes file actions and logs them to history.
type Dispatcher struct {
	History  *history.Store
	TrashDir string
}

// New creates a dispatcher with the given history store and trash directory.
func New(hist *history.Store, opts ...Option) *Dispatcher {
	d := &Dispatcher{History: hist}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithTrashDir sets the trash directory for delete actions.
func WithTrashDir(dir string) Option {
	return func(d *Dispatcher) { d.TrashDir = dir }
}

// Result holds the outcome of a dispatch action.
type Result struct {
	Record history.Record
	DryRun bool
}

// Dispatch applies a rule's action(s) to a file. For rules with multiple
// actions (action chaining), each action is executed in order and linked by
// a shared ChainID in the history. If any action fails, the chain stops.
// In dry-run mode, no changes are made but the planned actions are returned.
func (d *Dispatcher) Dispatch(fi rule.FileInfo, r rule.Rule, dryRun bool) (*Result, error) {
	actions := r.ResolvedActions()
	if len(actions) == 0 {
		return nil, fmt.Errorf("rule %q has no actions", r.Name)
	}

	// Single action — no chain overhead
	if len(actions) == 1 {
		return d.dispatchAction(fi, r.Name, actions[0], "", dryRun)
	}

	// Multiple actions — generate a shared chain ID
	chainID := newChainID()
	var lastResult *Result
	for _, action := range actions {
		result, err := d.dispatchAction(fi, r.Name, action, chainID, dryRun)
		if err != nil {
			return nil, err
		}
		lastResult = result

		// If the action moved/renamed the file, update fi.Path for the next
		// action in the chain so it operates on the file's new location.
		if !dryRun && result.Record.Dest != "" {
			switch action.Type {
			case rule.ActionMove, rule.ActionRename:
				newFi, statErr := rule.NewFileInfo(result.Record.Dest)
				if statErr == nil {
					fi = newFi
				}
			}
		}
	}
	return lastResult, nil
}

// dispatchAction executes a single action against a file.
func (d *Dispatcher) dispatchAction(fi rule.FileInfo, ruleName string, action rule.Action, chainID string, dryRun bool) (*Result, error) {
	dest, err := rule.ExpandTemplate(action.Dest, fi)
	if err != nil {
		return nil, fmt.Errorf("expanding template: %w", err)
	}

	rec := history.Record{
		RuleName: ruleName,
		Action:   string(action.Type),
		Src:      fi.Path,
		Dest:     dest,
		ChainID:  chainID,
	}

	if dryRun {
		return &Result{Record: rec, DryRun: true}, nil
	}

	switch action.Type {
	case rule.ActionMove:
		err = doMove(fi.Path, dest)
	case rule.ActionCopy:
		err = doCopy(fi.Path, dest)
	case rule.ActionRename:
		err = doRename(fi.Path, dest)
	case rule.ActionDelete:
		dest, err = d.doDelete(fi.Path)
		rec.Dest = dest
	case rule.ActionCompress:
		dest, err = doCompress(fi.Path, dest)
		rec.Dest = dest
	case rule.ActionExtract:
		var extractDest string
		extractDest, err = rule.ExpandString(action.Dest, fi)
		if err == nil {
			err = doExtract(fi.Path, extractDest)
			rec.Dest = extractDest
		}
	case rule.ActionSymlink:
		err = doSymlink(fi.Path, dest)
	case rule.ActionChmod:
		var oldMode string
		oldMode, err = doChmod(fi.Path, action.Mode)
		rec.Dest = oldMode
	case rule.ActionChecksum:
		dest, err = doChecksum(fi.Path, dest, action.Algorithm)
		rec.Dest = dest
	case rule.ActionExec:
		err = doExec(fi, action)
	case rule.ActionNotify:
		err = doNotify(fi, action)
	case rule.ActionConvert:
		err = doConvert(fi, action, dest)
	case rule.ActionResize:
		err = doResize(fi, action, dest)
	case rule.ActionWatermark:
		err = doWatermark(fi, action, dest)
	case rule.ActionOCR:
		dest, err = doOCR(fi, action, dest)
		rec.Dest = dest
	case rule.ActionEncrypt:
		err = doEncrypt(fi, action, dest)
	case rule.ActionDecrypt:
		err = doDecrypt(fi, action, dest)
	case rule.ActionUpload:
		err = doUpload(fi, action)
		rec.Dest = action.Remote
	case rule.ActionTag:
		err = doTag(fi, action)
		rec.Dest = ""
	case rule.ActionOpen:
		err = doOpen(fi, action)
		rec.Dest = ""
	case rule.ActionDeduplicate:
		var dedupDest string
		dedupDest, err = rule.ExpandString(action.Dest, fi)
		if err == nil {
			var outcome string
			outcome, err = doDeduplicate(fi.Path, dedupDest, action.OnDuplicate)
			rec.Dest = outcome + ":" + dedupDest
		}
	case rule.ActionUnquarantine:
		err = doUnquarantine(fi.Path)
		rec.Dest = ""
	default:
		err = fmt.Errorf("unknown action %q", action.Type)
	}

	if err != nil {
		rec.Error = err.Error()
		_ = d.History.Append(rec)
		return nil, fmt.Errorf("%s %s -> %s: %w", action.Type, fi.Path, dest, err)
	}

	_ = d.History.Append(rec)
	return &Result{Record: rec}, nil
}

func newChainID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("c%d", time.Now().UnixNano())
	}
	return "c" + hex.EncodeToString(b)
}

func doMove(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Try rename first (fast, same filesystem)
	if err := os.Rename(src, dest); err == nil {
		return nil
	}

	// Fall back to copy + remove (cross-filesystem)
	if err := doCopy(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

// Undo reverses a previously dispatched action.
func (d *Dispatcher) Undo(rec history.Record) error {
	switch rec.Action {
	case "move":
		return doMove(rec.Dest, rec.Src)
	case "copy":
		return os.Remove(rec.Dest)
	case "rename":
		return os.Rename(rec.Dest, rec.Src)
	case "delete":
		// Restore from trash
		return doMove(rec.Dest, rec.Src)
	case "compress":
		if err := doDecompress(rec.Dest, rec.Src); err != nil {
			return err
		}
		return os.Remove(rec.Dest)
	case "extract":
		return os.RemoveAll(rec.Dest)
	case "symlink":
		return os.Remove(rec.Dest)
	case "chmod":
		// rec.Dest holds the original mode string
		_, err := doChmod(rec.Src, rec.Dest)
		return err
	case "checksum", "convert", "resize", "watermark", "ocr", "encrypt", "decrypt":
		return os.Remove(rec.Dest)
	case "deduplicate":
		// rec.Dest is "outcome:path"
		parts := strings.SplitN(rec.Dest, ":", 2)
		outcome, dest := parts[0], parts[1]
		switch outcome {
		case "moved":
			return doMove(dest, rec.Src)
		case "skip":
			return nil
		case "delete":
			return fmt.Errorf("cannot undo deduplicate: source was deleted as duplicate")
		default:
			return fmt.Errorf("cannot undo deduplicate: unknown outcome %q", outcome)
		}
	default:
		return fmt.Errorf("cannot undo action %q", rec.Action)
	}
}

func doCopy(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}
	return nil
}

func doRename(src, dest string) error {
	return os.Rename(src, dest)
}

func (d *Dispatcher) doDelete(src string) (string, error) {
	if d.TrashDir == "" {
		return "", fmt.Errorf("trash directory not configured")
	}
	if err := os.MkdirAll(d.TrashDir, 0o755); err != nil {
		return "", fmt.Errorf("creating trash dir: %w", err)
	}
	dest := filepath.Join(d.TrashDir, filepath.Base(src))
	// Avoid collisions in trash
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(dest)
		base := strings.TrimSuffix(filepath.Base(dest), ext)
		for i := 1; i < 1000; i++ {
			candidate := filepath.Join(d.TrashDir, fmt.Sprintf("%s_%03d%s", base, i, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dest = candidate
				break
			}
		}
	}
	return dest, doMove(src, dest)
}

func doCompress(src, dest string) (string, error) {
	if !strings.HasSuffix(dest, ".gz") {
		dest = dest + ".gz"
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return dest, fmt.Errorf("creating directory: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return dest, fmt.Errorf("opening source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return dest, fmt.Errorf("creating dest: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	gz.Name = filepath.Base(src)

	if _, err := io.Copy(gz, in); err != nil {
		gz.Close()
		return dest, fmt.Errorf("compressing: %w", err)
	}
	if err := gz.Close(); err != nil {
		return dest, fmt.Errorf("closing gzip: %w", err)
	}

	return dest, os.Remove(src)
}

func doDecompress(gzPath, destPath string) error {
	in, err := os.Open(gzPath)
	if err != nil {
		return fmt.Errorf("opening gzip: %w", err)
	}
	defer in.Close()

	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating dest: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, gz); err != nil {
		return fmt.Errorf("decompressing: %w", err)
	}
	return nil
}
