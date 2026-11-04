package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/source"
)

// sourceChange is a difference between an editable copy and the fixed document it started from.
// Base is empty for an added document; Body is nil for a deleted document.
type sourceChange struct {
	Source string
	ID     string
	Base   string
	Body   *string
}

func (c sourceChange) contentHash() string {
	if c.Body == nil {
		return ""
	}
	return source.ContentHash(*c.Body)
}

func (e *Engine) editDirectory(name string) string {
	return filepath.Join(e.workRoot(), "sources", "edit", name)
}

func (e *Engine) editBasePath(name string) string {
	return filepath.Join(e.workRoot(), "sources", "edit", name+".base.json")
}

// prepareEditCopies creates an editable copy of each writable source outside the repository
// from the documents fixed for ref. An existing copy keeps the edits made in earlier rounds.
func (e *Engine) prepareEditCopies(ctx context.Context, ref string) error {
	external, err := e.externalSources(ctx, ref)
	if err != nil || len(external) == 0 {
		return err
	}
	pin, err := e.pin(ctx, ref, false)
	if err != nil {
		return err
	}
	for name, item := range external {
		if !item.CanWrite() {
			continue
		}
		if _, err := os.Stat(e.editBasePath(name)); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := e.resetEditCopy(name, pin[name]); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) resetEditCopy(name string, manifest sourceManifest) error {
	directory := e.editDirectory(name)
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	if err := e.writeManifest(directory, manifest, 0o644); err != nil {
		return err
	}
	return e.writeEditBase(name, manifest)
}

func (e *Engine) writeEditBase(name string, manifest sourceManifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return e.store().Publish(filepath.Join("sources", "edit", name+".base.json"), data)
}

func (e *Engine) editBase(name string) (sourceManifest, error) {
	data, err := os.ReadFile(e.editBasePath(name))
	if err != nil {
		return nil, err
	}
	manifest := sourceManifest{}
	return manifest, json.Unmarshal(data, &manifest)
}

// editCopies returns the names of sources that have an editable copy, in sorted order.
func (e *Engine) editCopies() ([]string, error) {
	if e.workRoot() == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(filepath.Join(e.workRoot(), "sources", "edit"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if name, ok := strings.CutSuffix(entry.Name(), ".base.json"); ok && !entry.IsDir() {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// editDocuments reads the current documents of an editable copy.
func (e *Engine) editDocuments(name string) (map[string]string, error) {
	directory := e.editDirectory(name)
	documents := map[string]string{}
	err := filepath.WalkDir(directory, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(directory, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		documents[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if errors.Is(err, os.ErrNotExist) {
		return documents, nil
	}
	return documents, err
}

// sourceIdentity hashes the set of source names, document IDs, and content hashes of the
// editable copies. It is empty when no source outside the repository has a copy.
func (e *Engine) sourceIdentity() (string, error) {
	names, err := e.editCopies()
	if err != nil || len(names) == 0 {
		return "", err
	}
	var text strings.Builder
	for _, name := range names {
		documents, err := e.editDocuments(name)
		if err != nil {
			return "", err
		}
		manifest := sourceManifest{}
		for id, body := range documents {
			manifest[id] = source.ContentHash(body)
		}
		fmt.Fprintf(&text, "%s\x00%s\n", name, manifestHash(manifest))
	}
	return source.ContentHash(text.String()), nil
}

// treeTarget identifies a git tree together with the editable copies of outside sources.
func (e *Engine) treeTarget(tree string) (form.TargetVersion, error) {
	sources, err := e.sourceIdentity()
	return form.TargetVersion{Kind: form.TargetTree, Tree: tree, Sources: sources}, err
}

// approvalTree returns the approval target for a tree target version.
func approvalTree(target form.TargetVersion) form.ApprovalTarget {
	return form.ApprovalTarget{Kind: form.ApprovalTree, Tree: target.Tree, Sources: target.Sources}
}

// editChanges compares an editable copy with the fixed documents it is based on.
func (e *Engine) editChanges(name string) ([]sourceChange, error) {
	base, err := e.editBase(name)
	if err != nil {
		return nil, err
	}
	documents, err := e.editDocuments(name)
	if err != nil {
		return nil, err
	}
	changes := []sourceChange{}
	for id, body := range documents {
		if base[id] != source.ContentHash(body) {
			copy := body
			changes = append(changes, sourceChange{Source: name, ID: id, Base: base[id], Body: &copy})
		}
	}
	for id, hash := range base {
		if _, ok := documents[id]; !ok {
			changes = append(changes, sourceChange{Source: name, ID: id, Base: hash})
		}
	}
	slices.SortFunc(changes, func(a, b sourceChange) int { return strings.Compare(a.ID, b.ID) })
	return changes, nil
}

// sourceChanges returns the changes of every editable copy, optionally only of norm sources.
func (e *Engine) sourceChanges(ctx context.Context, ref string, normsOnly bool) ([]sourceChange, error) {
	names, err := e.editCopies()
	if err != nil || len(names) == 0 {
		return nil, err
	}
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	changes := []sourceChange{}
	for _, name := range names {
		if normsOnly && !sources[name].Norm {
			continue
		}
		changed, err := e.editChanges(name)
		if err != nil {
			return nil, err
		}
		changes = append(changes, changed...)
	}
	return changes, nil
}

// editLocation names where a worker edits a source: the configured path inside the repository,
// or the editable copy under the run directory for a writable source outside it.
func (e *Engine) editLocation(name, configured string) string {
	if _, err := os.Stat(e.editBasePath(name)); err == nil {
		return e.editDirectory(name)
	}
	return configured
}

// CurrentIdentity returns the target version of a working-tree snapshot with the editable
// copies of outside sources, and the norms fixed for it, for display by the CLI.
func (e *Engine) CurrentIdentity(ctx context.Context, tree string) (form.TargetVersion, []form.NormReference, error) {
	target, err := e.treeTarget(tree)
	if err != nil {
		return form.TargetVersion{}, nil, err
	}
	norms, err := e.norms(ctx, tree)
	return target, norms, err
}
