package kit

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	devkit "github.com/ildjenty/dev-kit"
)

// Version is the version of the assets embedded in the binary.
const Version = "v1"

// Kit represents a repository copy of a single version of the assets.
// It implements fs.FS and does not fall back to embedded assets when reading.
type Kit struct {
	dir     string
	version string
}

// Open opens a copy of the specified version. Only Version is readable.
// When a Work exists, call CheckVersion before reading.
func Open(repoRoot, version string) (*Kit, error) {
	if version != Version {
		return nil, fmt.Errorf("kit: readable version is %q, requested %q", Version, version)
	}
	dir := filepath.Join(repoRoot, ".devkit", "kit", version)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("kit: open copy: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("kit: copy is not a directory: %s", dir)
	}
	return &Kit{dir: dir, version: version}, nil
}

// Dir returns the directory containing the asset copy.
func (k *Kit) Dir() string {
	return k.dir
}

// Version returns the version of the open copy.
func (k *Kit) Version() string {
	return k.version
}

// CheckVersion checks that the version recorded in the Work matches the copy.
func (k *Kit) CheckVersion(workVersion string) error {
	if workVersion != k.version {
		return fmt.Errorf("kit: Work version %q does not match copy version %q", workVersion, k.version)
	}
	return nil
}

// Open opens a relative path in the copy. Paths follow the fs.FS format.
func (k *Kit) Open(name string) (fs.File, error) {
	return os.DirFS(k.dir).Open(name)
}

// ReadFile reads an asset from a relative path in the copy.
func (k *Kit) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(os.DirFS(k.dir), name)
}

// Copy copies embedded assets to .devkit/kit/<Version>/ for init.
// It preserves identical existing files and rejects different contents without overwriting them.
func Copy(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".devkit", "kit", Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("kit: create copy directory: %w", err)
	}
	return walkEmbedded(func(source, name string, entry fs.DirEntry) error {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("kit: create copy directory: %w", err)
			}
			return nil
		}
		data, err := fs.ReadFile(devkit.FS(), source)
		if err != nil {
			return fmt.Errorf("kit: read embedded asset: %w", err)
		}
		existing, err := os.ReadFile(target)
		if err == nil {
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("kit: existing copy has different contents: %s", target)
			}
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("kit: read existing copy: %w", err)
		}
		// Keep unpublished files outside the version directory so readers and Verify ignore them.
		file, err := os.CreateTemp(filepath.Dir(dir), ".kit-*")
		if err != nil {
			return fmt.Errorf("kit: create temporary copy: %w", err)
		}
		defer os.Remove(file.Name())
		if err := file.Chmod(0o644); err != nil {
			return fmt.Errorf("kit: set copy permissions: %w", errors.Join(err, file.Close()))
		}
		_, writeErr := file.Write(data)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return fmt.Errorf("kit: write copy %s: %w", target, err)
		}
		if err := os.Rename(file.Name(), target); err != nil {
			return fmt.Errorf("kit: publish copy %s: %w", target, err)
		}
		return nil
	})
}

// Verify checks that the copy matches the embedded layout and contents for doctor.
// Missing or extra entries and differences in type or contents cause errors; the copy is not modified.
func (k *Kit) Verify() error {
	expected := make(map[string]fs.DirEntry)
	if err := walkEmbedded(func(source, name string, entry fs.DirEntry) error {
		expected[name] = entry
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(devkit.FS(), source)
		if err != nil {
			return fmt.Errorf("kit: read embedded asset: %w", err)
		}
		copied, err := k.ReadFile(name)
		if err != nil {
			return fmt.Errorf("kit: read copy: %w", err)
		}
		if !bytes.Equal(copied, data) {
			return fmt.Errorf("kit: copy and embedded contents differ: %s", name)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := fs.WalkDir(k, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("kit: walk copy: %w", err)
		}
		if name == "." {
			return nil
		}
		original, ok := expected[name]
		if !ok {
			return fmt.Errorf("kit: copy contains an entry absent from embedded assets: %s", name)
		}
		if entry.Type() != original.Type() {
			return fmt.Errorf("kit: copy and embedded entry types differ: %s", name)
		}
		delete(expected, name)
		return nil
	}); err != nil {
		return err
	}
	for name := range expected {
		return fmt.Errorf("kit: copy is missing an entry: %s", name)
	}
	return nil
}

func walkEmbedded(visit func(source, name string, entry fs.DirEntry) error) error {
	return fs.WalkDir(devkit.FS(), ".", func(source string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("kit: walk embedded assets: %w", err)
		}
		if source == "." || source == "kit" {
			return nil
		}
		if strings.HasPrefix(source, "kit/") && entry.Name() == "README.md" && !entry.IsDir() {
			return nil
		}
		return visit(source, strings.TrimPrefix(source, "kit/"), entry)
	})
}
