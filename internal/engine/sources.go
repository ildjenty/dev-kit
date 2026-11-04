package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/source"
)

// sourceManifest maps document IDs of one source to their content hashes.
type sourceManifest map[string]string

// sourcePin holds the manifests of one immutable outside-source snapshot.
type sourcePin map[string]sourceManifest

var fullObjectID = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

func (e *Engine) knowledgeSources(ctx context.Context, ref string) (map[string]config.Source, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	return settings.KnowledgeSources(), nil
}

// repositoryLocation returns the repository-relative slash path of a source inside the checkout.
func (e *Engine) repositoryLocation(item config.Source) (string, bool) {
	return sourcePath(e.Root(), item.Path)
}

func (e *Engine) treeFS(ctx context.Context, ref, location string) (fs.FS, error) {
	key := ref + "\x00" + location
	if tree, ok := e.trees[key]; ok {
		return tree, nil
	}
	tree, err := e.workspace.TreeFS(ctx, gitReference(ref), location)
	if err != nil {
		return nil, err
	}
	if fullObjectID.MatchString(ref) {
		if e.trees == nil {
			e.trees = map[string]fs.FS{}
		}
		e.trees[key] = tree
	}
	return tree, nil
}

// selectDocuments selects documents from the snapshot fixed for ref. Sources inside the
// repository are read from the git tree; sources outside it from the copy fixed for ref.
func (e *Engine) selectDocuments(ctx context.Context, ref, name, selector string) ([]source.Document, error) {
	return e.selectFrom(ctx, ref, ref, name, selector)
}

// selectFrom uses the source settings fixed for ref and reads the documents fixed for content,
// such as products of a round target read with the round's norms.
func (e *Engine) selectFrom(ctx context.Context, ref, content, name, selector string) ([]source.Document, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	item, ok := sources[name]
	if !ok {
		return nil, fmt.Errorf("unknown source: %q", name)
	}
	adapter, err := e.fixedAdapter(ctx, content, name, item)
	if err != nil {
		return nil, err
	}
	return adapter.Select(selector)
}

func (e *Engine) fixedAdapter(ctx context.Context, ref, name string, item config.Source) (*source.Files, error) {
	if location, inside := e.repositoryLocation(item); inside {
		tree, err := e.treeFS(ctx, ref, location)
		if err != nil {
			return nil, err
		}
		return source.NewFiles(e.Root(), location, tree, ref, false)
	}
	if e.workRoot() == "" {
		return source.NewFiles(e.Root(), item.Path, nil, "", false)
	}
	pin, err := e.pin(ctx, ref, false)
	if err != nil {
		return nil, err
	}
	directory, err := e.fixedDirectory(name, pin[name])
	if err != nil {
		return nil, err
	}
	return source.NewFiles(directory, ".", os.DirFS(directory), "", false)
}

// document returns one document by ID from the snapshot fixed for ref.
func (e *Engine) document(ctx context.Context, ref, name, id string) (source.Document, error) {
	selector := id
	if strings.ContainsAny(id, "*?[\\") {
		selector = ""
	}
	documents, err := e.selectDocuments(ctx, ref, name, selector)
	if err != nil {
		return source.Document{}, err
	}
	for _, document := range documents {
		if document.ID == id {
			return document, nil
		}
	}
	return source.Document{}, fmt.Errorf("%s/%s: %w", name, id, fs.ErrNotExist)
}

// sourceText joins every document of a source, such as the single rules document.
func (e *Engine) sourceText(ctx context.Context, ref, name string) (string, bool, error) {
	documents, err := e.selectDocuments(ctx, ref, name, "")
	if err != nil {
		return "", false, err
	}
	if len(documents) == 1 {
		return documents[0].Body, true, nil
	}
	var text strings.Builder
	for _, document := range documents {
		fmt.Fprintf(&text, "## %s/%s\n\n%s\n\n", name, document.ID, document.Body)
	}
	return text.String(), len(documents) > 0, nil
}

// logicalDocument maps a logical norm name such as stages/x.md or rules.md to a source document.
func logicalDocument(name string) (string, string, bool) {
	switch name {
	case "rules.md":
		return "rules", "", true
	case "requirements.md":
		return "requirements", "", true
	case "structure.json":
		return "structure", "", true
	}
	sourceName, id, ok := strings.Cut(name, "/")
	return sourceName, id, ok && id != ""
}

func (e *Engine) logicalAsset(ctx context.Context, ref, name string) ([]byte, error) {
	sourceName, id, ok := logicalDocument(name)
	if !ok {
		return nil, fmt.Errorf("unknown logical norm: %s", name)
	}
	if id == "" {
		text, exists, err := e.sourceText(ctx, ref, sourceName)
		if err == nil && !exists {
			err = fmt.Errorf("%s: %w", name, fs.ErrNotExist)
		}
		return []byte(text), err
	}
	document, err := e.document(ctx, ref, sourceName, id)
	return []byte(document.Body), err
}

func manifestHash(manifest sourceManifest) string {
	ids := make([]string, 0, len(manifest))
	for id := range manifest {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var text strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&text, "%s\x00%s\n", id, manifest[id])
	}
	return source.ContentHash(text.String())
}

func (e *Engine) externalSources(ctx context.Context, ref string) (map[string]config.Source, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	external := map[string]config.Source{}
	for name, item := range sources {
		if _, inside := e.repositoryLocation(item); !inside {
			external[name] = item
		}
	}
	return external, nil
}

// gitReference removes the immutable outside-source pin from a snapshot reference.
func gitReference(ref string) string {
	git, _, _ := strings.Cut(ref, "@")
	return git
}

func pinReference(ref string, pin sourcePin) string {
	if len(pin) == 0 {
		return gitReference(ref)
	}
	data, _ := json.Marshal(pin)
	return gitReference(ref) + "@" + hash(data)
}

// pin reads immutable content-addressed copies. A plain git reference retains its first
// copy; refreshing publishes a new pin without replacing that original association.
func (e *Engine) pin(ctx context.Context, ref string, refresh bool) (sourcePin, error) {
	if e.workRoot() == "" || !localName(ref) {
		return nil, fmt.Errorf("sources outside the repository cannot be fixed for %q", ref)
	}
	git, id, fixed := strings.Cut(ref, "@")
	name := filepath.Join("sources", "refs", git+".json")
	if fixed {
		name = filepath.Join("sources", "pins", id+".json")
	}
	if !refresh || fixed {
		data, err := os.ReadFile(filepath.Join(e.workRoot(), name))
		if err == nil {
			pin := sourcePin{}
			return pin, json.Unmarshal(data, &pin)
		}
		if fixed || !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	external, err := e.externalSources(ctx, git)
	if err != nil {
		return nil, err
	}
	pin := sourcePin{}
	for sourceName, item := range external {
		if pin[sourceName], err = e.fixExternal(sourceName, item); err != nil {
			return nil, err
		}
	}
	data, err := json.Marshal(pin)
	if err != nil {
		return nil, err
	}
	if err := e.store().Publish(filepath.Join("sources", "pins", hash(data)+".json"), data); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(e.workRoot(), name)); errors.Is(err, os.ErrNotExist) {
		if err := e.store().Publish(name, data); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return pin, nil
}

// refreshSources returns a reference to the current outside documents and the given git tree.
func (e *Engine) refreshSources(ctx context.Context, ref string) (string, error) {
	external, err := e.externalSources(ctx, ref)
	if err != nil || len(external) == 0 {
		return ref, err
	}
	pin, err := e.pin(ctx, gitReference(ref), true)
	return pinReference(ref, pin), err
}

func (e *Engine) fixExternal(name string, item config.Source) (sourceManifest, error) {
	adapter, err := source.NewFiles(e.Root(), item.Path, nil, "", false)
	if err != nil {
		return nil, err
	}
	documents, err := adapter.Select("")
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", name, err)
	}
	manifest := sourceManifest{}
	for _, document := range documents {
		hash, err := e.storeBlob(document.Body)
		if err != nil {
			return nil, err
		}
		manifest[document.ID] = hash
	}
	if _, err := e.fixedDirectory(name, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (e *Engine) storeBlob(body string) (string, error) {
	hash := source.ContentHash(body)
	name := filepath.Join("sources", "blobs", hash)
	if _, err := os.Stat(filepath.Join(e.workRoot(), name)); err == nil {
		return hash, nil
	}
	return hash, e.store().Publish(name, []byte(body))
}

func (e *Engine) blob(hash string) (string, error) {
	data, err := os.ReadFile(filepath.Join(e.workRoot(), "sources", "blobs", hash))
	return string(data), err
}

// fixedDirectory materializes a fixed manifest once as read-only files under the run directory.
func (e *Engine) fixedDirectory(name string, manifest sourceManifest) (string, error) {
	directory := filepath.Join(e.workRoot(), "sources", "fixed", name, manifestHash(manifest))
	marker := directory + ".complete"
	if _, err := os.Stat(marker); err == nil {
		return directory, nil
	}
	if err := os.RemoveAll(directory); err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	if err := e.writeManifest(directory, manifest, 0o444); err != nil {
		return "", err
	}
	return directory, os.WriteFile(marker, nil, 0o444)
}

func (e *Engine) writeManifest(directory string, manifest sourceManifest, mode fs.FileMode) error {
	for id, hash := range manifest {
		body, err := e.blob(hash)
		if err != nil {
			return err
		}
		name := filepath.Join(directory, filepath.FromSlash(id))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(name, []byte(body), mode); err != nil {
			return err
		}
	}
	return nil
}

func normApproval(ref string) form.ApprovalTarget {
	git, pin, _ := strings.Cut(ref, "@")
	return form.ApprovalTarget{Kind: form.ApprovalTree, Tree: git, SourcePin: pin}
}

func approvalReference(target form.ApprovalTarget) string {
	if target.SourcePin == "" {
		return target.Tree
	}
	return target.Tree + "@" + target.SourcePin
}

func (e *Engine) currentApprovalTarget(ctx context.Context, target form.ApprovalTarget, tree string) (form.ApprovalTarget, error) {
	if target.SourcePin != "" {
		ref, err := e.refreshSources(ctx, tree)
		return normApproval(ref), err
	}
	current := target
	current.Tree = tree
	if target.Sources != "" {
		sources, err := e.sourceIdentity()
		current.Sources = sources
		return current, err
	}
	return current, nil
}
