package dispatcher

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// doExtract decompresses an archive into the destination directory.
// Supports .zip, .tar, .tar.gz/.tgz, .tar.bz2, and .tar.xz via external tar.
func doExtract(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("creating extract dir: %w", err)
	}

	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(src, dest)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(src, dest)
	case strings.HasSuffix(lower, ".tar.bz2"):
		return extractTarBz2(src, dest)
	case strings.HasSuffix(lower, ".tar.xz"):
		return extractTarXz(src, dest)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(src, dest)
	default:
		return fmt.Errorf("unsupported archive format: %s", filepath.Ext(src))
	}
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)

		// Guard against zip slip
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("creating %s: %w", target, err)
		}

		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		out.Close()
		rc.Close()
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close()

	return extractTarReader(tar.NewReader(gz), dest)
}

func extractTarBz2(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	return extractTarReader(tar.NewReader(bzip2.NewReader(f)), dest)
}

func extractTar(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	return extractTarReader(tar.NewReader(f), dest)
}

func extractTarXz(src, dest string) error {
	cmd := exec.Command("tar", "xJf", src, "-C", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar xz: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func extractTarReader(tr *tar.Reader, dest string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		target := filepath.Join(dest, hdr.Name)

		// Guard against path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path in tar: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fs.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("extracting %s: %w", hdr.Name, err)
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("creating symlink %s: %w", hdr.Name, err)
			}
		}
	}
	return nil
}

// doSymlink creates a symbolic link at dest pointing to the source file.
func doSymlink(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolving source path: %w", err)
	}

	return os.Symlink(absSrc, dest)
}

// doChmod changes file permissions and returns the original mode as a string.
func doChmod(src, mode string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}

	oldMode := fmt.Sprintf("%04o", info.Mode().Perm())

	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return "", fmt.Errorf("invalid mode %q: %w", mode, err)
	}

	if err := os.Chmod(src, fs.FileMode(parsed)); err != nil {
		return oldMode, fmt.Errorf("chmod: %w", err)
	}

	return oldMode, nil
}

// doChecksum computes a hash of the source file and writes a sidecar file.
// Returns the sidecar file path. Algorithm must be "sha256", "md5", or "sha1".
func doChecksum(src, dest, algorithm string) (string, error) {
	if algorithm == "" {
		algorithm = "sha256"
	}

	var h hash.Hash
	switch algorithm {
	case "sha256":
		h = sha256.New()
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	default:
		return "", fmt.Errorf("unsupported algorithm %q (use sha256, md5, or sha1)", algorithm)
	}

	f, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("opening source: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing: %w", err)
	}

	sum := hex.EncodeToString(h.Sum(nil))

	// Determine sidecar path
	sidecar := src + "." + algorithm
	if dest != "" {
		sidecar = dest
	}

	if err := os.MkdirAll(filepath.Dir(sidecar), 0o755); err != nil {
		return sidecar, fmt.Errorf("creating directory: %w", err)
	}

	content := fmt.Sprintf("%s  %s\n", sum, filepath.Base(src))
	if err := os.WriteFile(sidecar, []byte(content), 0o644); err != nil {
		return sidecar, fmt.Errorf("writing sidecar: %w", err)
	}

	return sidecar, nil
}
