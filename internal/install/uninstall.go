package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

const uninstallGuidance = "Remove any devkit commands you added to git hooks yourself. Remove the devkit binary separately if no other repository uses it."

// Uninstall reverses Init. It removes runtime state, the skill link,
// and installed files that still match embedded assets, and strips the host fragments from
// host settings, devkit.toml, and .gitignore. Files the user changed are left in
// place and listed in Preserved. It calls confirm with all changes before applying
// them; a nil callback means the caller already authorized them.
func Uninstall(ctx context.Context, cwd string, confirm func([]Change) error) (Result, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return Result{}, err
	}
	result := Result{Root: w.Root(), Guidance: []string{uninstallGuidance}}
	files := kit.FS()
	paths := config.Config{}.RepositoryPaths()
	configuration, err := readOptional(filepath.Join(result.Root, "devkit.toml"))
	if err != nil {
		return result, err
	}
	if configuration != nil {
		c, err := config.Parse(configuration)
		if err != nil {
			return result, err
		}
		paths = c.RepositoryPaths()
	}
	if err := result.removeConfig(files); err != nil {
		return result, err
	}
	for _, name := range []string{".devkit/run"} {
		if err := result.removeAll(name); err != nil {
			return result, err
		}
	}
	norms, err := kit.Norms()
	if err != nil {
		return result, err
	}
	names := make([]string, 0, len(norms))
	for name := range norms {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := result.removeFile(normPath(paths, name), norms[name]); err != nil {
			return result, err
		}
	}
	for _, pair := range [][2]string{
		{"skills/devkit", ".agents/skills/devkit"},
		{"hosts/claude/agents", ".claude/agents"},
		{"hosts/codex/agents", ".codex/agents"},
	} {
		if err := result.removeTree(files, pair[0], pair[1]); err != nil {
			return result, err
		}
	}
	if err := result.removeSkillLink(); err != nil {
		return result, err
	}
	for _, fragment := range hostFragments {
		data, err := fs.ReadFile(files, fragment.source)
		if err != nil {
			return result, err
		}
		strip := stripJSON
		switch filepath.Ext(fragment.target) {
		case ".toml":
			strip = stripTOML
		case ".rules":
			strip = stripLines
		}
		if err := result.strip(fragment.target, data, strip); err != nil {
			return result, fmt.Errorf("%s: %w", fragment.target, err)
		}
	}
	if err := result.strip(".gitignore", []byte(".devkit/run/\n"), stripLines); err != nil {
		return result, err
	}
	if len(result.Changes) != 0 && confirm != nil {
		if err := confirm(result.Changes); err != nil {
			return result, err
		}
	}
	for _, change := range result.Changes {
		target := filepath.Join(result.Root, filepath.FromSlash(change.Path))
		if change.Remove {
			err = os.Remove(target)
		} else {
			err = writeConfig(target, change.After)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return result, err
		}
	}
	// The listed files are gone; this also removes the empty directories left in them.
	for _, name := range []string{".devkit/run"} {
		if err := os.RemoveAll(filepath.Join(result.Root, filepath.FromSlash(name))); err != nil {
			return result, err
		}
	}
	return result, result.pruneDirectories()
}

func (r *Result) removeConfig(files fs.FS) error {
	before, err := readOptional(filepath.Join(r.Root, "devkit.toml"))
	if err != nil || before == nil {
		return err
	}
	c, err := config.Parse(before)
	if err != nil {
		return err
	}
	if c.KitVersion != kit.Version {
		return fmt.Errorf("kit version mismatch: binary %q, devkit.toml %q; uninstall with the matching devkit binary", kit.Version, c.KitVersion)
	}
	deny, err := fs.ReadFile(files, "hosts/deny.toml")
	if err != nil {
		return err
	}
	fragment := append([]byte(fmt.Sprintf("kit_version = %q\n", kit.Version)), deny...)
	after, err := stripTOML(before, fragment)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(after)) != 0 {
		r.Preserved = append(r.Preserved, "devkit.toml")
		return nil
	}
	r.Changes = append(r.Changes, Change{Path: "devkit.toml", Before: before, Remove: true})
	return nil
}

func (r *Result) removeAll(name string) error {
	root := filepath.Join(r.Root, filepath.FromSlash(name))
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, path := range paths {
		rel, err := filepath.Rel(r.Root, path)
		if err != nil {
			return err
		}
		r.Changes = append(r.Changes, Change{Path: filepath.ToSlash(rel), Remove: true})
	}
	return nil
}

func (r *Result) removeTree(files fs.FS, source, target string) error {
	return fs.WalkDir(files, source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		return r.removeFile(target+strings.TrimPrefix(name, source), data)
	})
}

// removeFile removes name when it still holds the kit contents and preserves it otherwise.
// A path written by several kit sources is removed when it matches any of them.
func (r *Result) removeFile(name string, data []byte) error {
	if slices.ContainsFunc(r.Changes, func(change Change) bool { return change.Path == name }) {
		return nil
	}
	path := filepath.Join(r.Root, filepath.FromSlash(name))
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Equal(existing, data) {
			r.Preserved = slices.DeleteFunc(r.Preserved, func(path string) bool { return path == name })
			r.Changes = append(r.Changes, Change{Path: name, Before: existing, Remove: true})
			return nil
		}
	}
	if !slices.Contains(r.Preserved, name) {
		r.Preserved = append(r.Preserved, name)
	}
	return nil
}

func (r *Result) removeSkillLink() error {
	const name = ".claude/skills/devkit"
	existing, err := os.Readlink(filepath.Join(r.Root, name))
	if err == nil && existing == "../../.agents/skills/devkit" {
		r.Changes = append(r.Changes, Change{Path: name, After: []byte(existing), Symlink: true, Remove: true})
	}
	return nil
}

// strip removes the fragment from a settings file and removes the file when nothing remains.
func (r *Result) strip(name string, fragment []byte, strip func([]byte, []byte) ([]byte, error)) error {
	before, err := readOptional(filepath.Join(r.Root, name))
	if err != nil || before == nil {
		return err
	}
	after, err := strip(before, fragment)
	if err != nil || bytes.Equal(before, after) {
		return err
	}
	if len(bytes.TrimSpace(after)) == 0 {
		r.Changes = append(r.Changes, Change{Path: name, Before: before, Remove: true})
		return nil
	}
	r.Changes = append(r.Changes, Change{Path: name, Before: before, After: after})
	return nil
}

// pruneDirectories removes directories that became empty, from the deepest up,
// without leaving the repository root.
func (r *Result) pruneDirectories() error {
	seen := make(map[string]bool)
	var dirs []string
	for _, change := range r.Changes {
		if !change.Remove {
			continue
		}
		for dir := filepath.Dir(filepath.FromSlash(change.Path)); dir != "." && !seen[dir]; dir = filepath.Dir(dir) {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		path := filepath.Join(r.Root, dir)
		entries, err := os.ReadDir(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func stripJSON(before, fragment []byte) ([]byte, error) {
	current, err := decodeJSON(before)
	if err != nil {
		return nil, err
	}
	installed, err := decodeJSON(fragment)
	if err != nil {
		return nil, err
	}
	if !stripSettings(current, installed) {
		return before, nil
	}
	if len(current) == 0 {
		return nil, nil
	}
	after, err := json.MarshalIndent(current, "", "  ")
	return append(after, '\n'), err
}

func stripTOML(before, fragment []byte) ([]byte, error) {
	current := make(map[string]any)
	if err := toml.Unmarshal(before, &current); err != nil {
		return nil, err
	}
	installed := make(map[string]any)
	if err := toml.Unmarshal(fragment, &installed); err != nil {
		return nil, err
	}
	if !stripSettings(current, installed) {
		return before, nil
	}
	if len(current) == 0 {
		return nil, nil
	}
	var after bytes.Buffer
	if err := toml.NewEncoder(&after).Encode(current); err != nil {
		return nil, err
	}
	return after.Bytes(), nil
}

// stripSettings removes the values and array members that mergeSettings added.
// Objects and arrays left empty are removed; differing scalars are kept.
func stripSettings(current, installed map[string]any) bool {
	changed := false
	for key, want := range installed {
		have, exists := current[key]
		if !exists {
			continue
		}
		switch want := want.(type) {
		case map[string]any:
			if object, ok := have.(map[string]any); ok {
				if stripSettings(object, want) {
					changed = true
					if len(object) == 0 {
						delete(current, key)
					}
				}
			}
		case []any:
			if array, ok := have.([]any); ok {
				kept := array[:0:0]
				for _, item := range array {
					if !slices.ContainsFunc(want, func(value any) bool { return reflect.DeepEqual(item, value) }) {
						kept = append(kept, item)
					}
				}
				if len(kept) != len(array) {
					changed = true
				}
				if len(kept) == 0 {
					delete(current, key)
				} else {
					current[key] = kept
				}
			}
		default:
			if reflect.DeepEqual(have, want) {
				delete(current, key)
				changed = true
			}
		}
	}
	return changed
}

func stripLines(before, fragment []byte) ([]byte, error) {
	remove := make(map[string]bool)
	for _, line := range strings.Split(string(fragment), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			remove[line] = true
		}
	}
	var after []byte
	for _, line := range strings.SplitAfter(string(before), "\n") {
		if !remove[strings.TrimSpace(line)] {
			after = append(after, line...)
		}
	}
	if bytes.Equal(after, before) {
		return before, nil
	}
	return after, nil
}
