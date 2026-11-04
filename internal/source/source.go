// Package source selects knowledge documents from fixed snapshots.
// Callers supply a repository reader fixed to the desired git snapshot.
package source

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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

// Files selects files relative to a source path.
type Files struct {
	reader fs.FS
	path   string
	origin string
}

// NewFiles requires a caller-supplied snapshot reader for sources inside repositoryRoot.
// The reader is rooted at the repository root. External sources use the live filesystem.
func NewFiles(repositoryRoot, sourcePath string, repositoryReader fs.FS, origin string) (*Files, error) {
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
	f := &Files{origin: origin}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if repositoryReader == nil {
			return nil, fmt.Errorf("repository source requires a fixed snapshot reader")
		}
		f.reader, f.path = repositoryReader, filepath.ToSlash(rel)
	} else {
		f.reader, f.path = os.DirFS(filepath.Dir(location)), filepath.Base(location)
	}
	_, err = fs.Stat(f.reader, f.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
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
