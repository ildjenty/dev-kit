// Package source selects knowledge documents and applies checked file updates.
// Callers supply a repository reader fixed to the desired git snapshot.
package source

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// Document contains a source-relative ID, text, and optional version provenance.
type Document struct {
	ID     string
	Body   string
	Origin string
}

// Adapter selects documents; an empty selector enumerates the entire source.
type Adapter interface {
	Select(selector string) ([]Document, error)
}

// Writer is the optional ability to update or delete a document.
// A nil body deletes it; an empty expected hash requires that it does not exist.
type Writer interface {
	Write(id string, body *string, expectedHash string) error
}

// ErrConflict means the current document differs from its expected content hash.
var ErrConflict = errors.New("source document changed; read it again and review the diff")

// Files selects files relative to a source path and serializes adapter writes.
type Files struct {
	location string
	reader   fs.FS
	path     string
	origin   string
	writable bool
	fileID   string
}

// NewFiles requires a caller-supplied snapshot reader for sources inside repositoryRoot.
// The reader is rooted at the repository root. External sources use the live filesystem.
func NewFiles(repositoryRoot, sourcePath string, repositoryReader fs.FS, origin string, writable bool) (*Files, error) {
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return nil, err
	}
	location := filepath.FromSlash(sourcePath)
	if !filepath.IsAbs(location) {
		location = filepath.Join(root, location)
	}
	rel, err := filepath.Rel(root, location)
	if err != nil {
		return nil, err
	}
	f := &Files{location: location, origin: origin, writable: writable}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if repositoryReader == nil {
			return nil, fmt.Errorf("repository source requires a fixed snapshot reader")
		}
		f.reader, f.path = repositoryReader, filepath.ToSlash(rel)
	} else {
		f.reader, f.path = os.DirFS(filepath.Dir(location)), filepath.Base(location)
	}
	info, err := fs.Stat(f.reader, f.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err == nil && !info.IsDir() {
		f.fileID = path.Base(f.path)
	}
	return f, nil
}

// Select uses slash globs, including ** for zero or more path components.
// A single-file source uses its base name as the document ID.
func (f *Files) Select(selector string) ([]Document, error) {
	if selector != "" && (!fs.ValidPath(selector) || strings.Contains(selector, "\\")) {
		return nil, fmt.Errorf("invalid files selector: %q", selector)
	}
	if _, err := match(selector, ""); err != nil {
		return nil, fmt.Errorf("invalid files selector %q: %w", selector, err)
	}
	info, err := fs.Stat(f.reader, f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return []Document{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return f.selectFile(selector, f.path, path.Base(f.path))
	}
	documents := []Document{}
	err = fs.WalkDir(f.reader, f.path, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		id := strings.TrimPrefix(name, f.path+"/")
		selected, err := f.selectFile(selector, name, id)
		documents = append(documents, selected...)
		return err
	})
	return documents, err
}

func (f *Files) selectFile(selector, name, id string) ([]Document, error) {
	selected, err := match(selector, id)
	if err != nil || !selected {
		return nil, err
	}
	body, err := fs.ReadFile(f.reader, name)
	if err != nil {
		return nil, err
	}
	return []Document{{ID: id, Body: string(body), Origin: f.origin}}, nil
}

func match(selector, id string) (bool, error) {
	if selector == "" {
		return true, nil
	}
	patterns := strings.Split(selector, "/")
	for _, pattern := range patterns {
		if _, err := path.Match(pattern, ""); err != nil {
			return false, err
		}
	}
	return matchParts(patterns, strings.Split(id, "/")), nil
}

func matchParts(patterns, parts []string) bool {
	if len(patterns) == 0 {
		return len(parts) == 0
	}
	if patterns[0] == "**" {
		return matchParts(patterns[1:], parts) || len(parts) > 0 && matchParts(patterns, parts[1:])
	}
	if len(parts) == 0 {
		return false
	}
	matched, _ := path.Match(patterns[0], parts[0])
	return matched && matchParts(patterns[1:], parts[1:])
}

// ContentHash returns the SHA-256 identity of a document's body.
func ContentHash(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
}

// Write compares and updates while holding the source directory's exclusive adapter lock.
// The source directory must exist; repository files are normally edited directly and committed by the caller.
func (f *Files) Write(id string, body *string, expectedHash string) error {
	if !f.writable {
		return fmt.Errorf("source is read-only")
	}
	if !fs.ValidPath(id) || id == "." || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid source document ID: %q", id)
	}
	location := f.location
	if f.fileID != "" {
		if id != f.fileID {
			return fmt.Errorf("single-file source document ID must be %q", f.fileID)
		}
		location = filepath.Dir(location)
	}
	root, err := os.OpenRoot(location)
	if err != nil {
		return err
	}
	defer root.Close()
	lock, err := root.Open(".")
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("lock source for update: %w", err)
	}
	return update(root, id, body, expectedHash)
}

func update(root *os.Root, id string, body *string, expectedHash string) error {
	current, err := root.ReadFile(id)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	exists := err == nil
	if exists && (expectedHash == "" || ContentHash(string(current)) != expectedHash) || !exists && expectedHash != "" {
		return ErrConflict
	}
	if body == nil {
		if !exists {
			return nil
		}
		return root.Remove(id)
	}
	mode := fs.FileMode(0o644)
	if exists {
		info, err := root.Stat(id)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
	}
	if err := root.MkdirAll(path.Dir(id), 0o755); err != nil {
		return err
	}
	temporary := path.Join(path.Dir(id), ".devkit-source-"+rand.Text())
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, writeErr := file.WriteString(*body)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return root.Rename(temporary, id)
}
