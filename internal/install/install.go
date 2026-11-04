package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

// Change describes a repository-relative file addition, configuration edit, or removal.
// For a symbolic link, After contains its relative target and Symlink is true.
// Remove is set when the path is deleted.
type Change struct {
	Path    string
	Before  []byte
	After   []byte
	Symlink bool
	Remove  bool `json:",omitempty"`
}

// Result describes the installation changes and existing files left in place.
type Result struct {
	Root      string
	Changes   []Change
	Preserved []string
	Guidance  []string
}

const trustGuidance = "Open Codex interactively, trust the project, and trust each project hook through /hooks. In Claude Code, accept workspace trust and check the project hook settings. Start a new session and run devkit doctor to verify SessionStart observation."

const commandGuidance = "The bundled default and fix templates require command:test. If it is not registered, the first work creates the tests and registers the command under [commands.test] in devkit.toml, and a human confirms the registration."

// InitOptions controls whether initialization commits the files it writes.
type InitOptions struct {
	NoCommit bool
}

// Init installs repository norms and merges the embedded host fragments.
// It calls confirm before applying changes and commits only the files it writes.
// A nil callback means the caller already authorized the changes.
// Existing norms, skills, and agent definitions are preserved.
func Init(ctx context.Context, cwd string, confirm func([]Change) error, options ...InitOptions) (Result, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return Result{}, err
	}
	result := Result{Root: w.Root(), Guidance: []string{trustGuidance, commandGuidance}}
	sources, err := result.addConfiguration()
	if err != nil {
		return result, err
	}
	if err := result.addNorms(sources); err != nil {
		return result, err
	}
	for _, pair := range [][2]string{
		{"skills/devkit", ".agents/skills/devkit"},
		{"hosts/claude/agents", ".claude/agents"},
		{"hosts/codex/agents", ".codex/agents"},
	} {
		if err := result.addTree(kit.FS(), pair[0], pair[1]); err != nil {
			return result, err
		}
	}
	if err := result.addSkillLink(); err != nil {
		return result, err
	}
	if err := result.addHostSettings(ctx); err != nil {
		return result, err
	}
	if len(result.Changes) != 0 && confirm != nil {
		if err := confirm(result.Changes); err != nil {
			return result, err
		}
	}
	if err := result.writeInstallation(); err != nil {
		return result, err
	}
	if len(options) == 0 || !options[0].NoCommit {
		if err := result.commit(ctx); err != nil {
			return result, err
		}
	}
	return result, nil
}

func writeConfig(path string, data []byte) error {
	target := path
	mode := fs.FileMode(0o644)
	info, err := os.Stat(path)
	if err == nil {
		target, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".devkit-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		return errors.Join(err, file.Close())
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), target)
}

func runtimeIgnored(ctx context.Context, root string) (bool, error) {
	return pathIgnored(ctx, root, ".devkit/run/")
}

func pathIgnored(ctx context.Context, root, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "--quiet", "--", path)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git check-ignore: %w: %s", err, output)
}

func (r *Result) addChange(name string, before, after []byte) {
	if !bytes.Equal(before, after) {
		r.Changes = append(r.Changes, Change{Path: name, Before: before, After: after})
	}
}

func (r *Result) addFile(name string, data []byte) error {
	_, err := os.Lstat(filepath.Join(r.Root, filepath.FromSlash(name)))
	if err == nil {
		r.Preserved = append(r.Preserved, name)
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	r.Changes = append(r.Changes, Change{Path: name, After: data})
	return nil
}

func (r *Result) addTree(files fs.FS, source, target string) error {
	return fs.WalkDir(files, source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		return r.addFile(target+strings.TrimPrefix(name, source), data)
	})
}

func (r *Result) addSkillLink() error {
	const name = ".claude/skills/devkit"
	const target = "../../.agents/skills/devkit"
	path := filepath.Join(r.Root, name)
	existing, err := os.Readlink(path)
	if err == nil && existing == target {
		return nil
	}
	if _, err := os.Lstat(path); err == nil {
		r.Preserved = append(r.Preserved, name)
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	r.Changes = append(r.Changes, Change{Path: name, After: []byte(target), Symlink: true})
	return nil
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func normPath(sources map[string]config.Source, name string) string {
	root, rest, _ := strings.Cut(name, "/")
	if root == "rules.md" {
		root = "rules"
	}
	base := sources[root].Path
	if rest == "" {
		return base
	}
	return base + "/" + rest
}

func sourceLocation(root, name string) string {
	if filepath.IsAbs(name) {
		return filepath.FromSlash(name)
	}
	return filepath.Join(root, filepath.FromSlash(name))
}

func localNormPath(root string, sources map[string]config.Source, name string) (string, bool) {
	source, _, _ := strings.Cut(name, "/")
	if source == "rules.md" {
		source = "rules"
	}
	if !sources[source].CanWrite() {
		return "", false
	}
	rel, err := filepath.Rel(root, sourceLocation(root, normPath(sources, name)))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (r *Result) addNorms(sources map[string]config.Source) error {
	files, err := kit.Norms()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target, local := localNormPath(r.Root, sources, name)
		if !local {
			continue
		}
		if err := r.addFile(target, files[name]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Result) commit(ctx context.Context) error {
	if len(r.Changes) == 0 {
		return nil
	}
	paths := make([]string, 0, len(r.Changes))
	for _, change := range r.Changes {
		paths = append(paths, change.Path)
	}
	if _, err := runGit(ctx, r.Root, append([]string{"add", "--"}, paths...)...); err != nil {
		return err
	}
	_, err := runGit(ctx, r.Root, append([]string{"commit", "--only", "-m", "devkit: init", "--"}, paths...)...)
	return err
}

func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--literal-pathspecs", "-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func (r *Result) addConfiguration() (map[string]config.Source, error) {
	configuration := []byte(fmt.Sprintf("kit_version = %q\n", kit.Version))
	existing, err := os.ReadFile(filepath.Join(r.Root, "devkit.toml"))
	if err == nil {
		configuration = existing
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	c, err := config.Parse(configuration)
	if err != nil {
		return nil, err
	}
	if err := c.ValidateSourcePaths(r.Root); err != nil {
		return nil, err
	}
	if c.KitVersion != kit.Version {
		return nil, fmt.Errorf("kit version mismatch: binary %q, devkit.toml %q", kit.Version, c.KitVersion)
	}
	deny, err := kit.HostFragment("deny.toml")
	if err != nil {
		return nil, err
	}
	configuration, err = mergeTOML(configuration, deny)
	if err != nil {
		return nil, err
	}
	if _, err := config.Parse(configuration); err != nil {
		return nil, err
	}
	r.addChange("devkit.toml", existing, configuration)
	return c.KnowledgeSources(), nil
}

func (r *Result) addHostSettings(ctx context.Context) error {
	for _, fragment := range hostFragments {
		data, err := kit.HostFragment(strings.TrimPrefix(fragment.source, "hosts/"))
		if err != nil {
			return err
		}
		before, err := readOptional(filepath.Join(r.Root, fragment.target))
		if err != nil {
			return err
		}
		after, err := fragment.merge(before, data)
		if err != nil {
			return fmt.Errorf("%s: %w", fragment.target, err)
		}
		r.addChange(fragment.target, before, after)
	}
	before, err := readOptional(filepath.Join(r.Root, ".gitignore"))
	if err != nil {
		return err
	}
	ignored, err := runtimeIgnored(ctx, r.Root)
	if err != nil {
		return err
	}
	after := appendLines(before, []byte(".devkit/run/\n"))
	if !ignored && bytes.Equal(before, after) {
		after = bytes.Clone(before)
		if len(after) != 0 && after[len(after)-1] != '\n' {
			after = append(after, '\n')
		}
		after = append(after, ".devkit/run/\n"...)
	}
	ignored, err = pathIgnored(ctx, r.Root, ".devkit/worktrees/")
	if err != nil {
		return err
	}
	updated := appendLines(after, []byte(".devkit/worktrees/\n"))
	if !ignored && bytes.Equal(after, updated) {
		updated = append(updated, ".devkit/worktrees/\n"...)
	}
	after = updated
	r.addChange(".gitignore", before, after)
	return nil
}

func (r *Result) writeInstallation() error {
	var err error
	for _, name := range []string{".devkit/run/sessions", ".devkit/run/work"} {
		if err := os.MkdirAll(filepath.Join(r.Root, name), 0o755); err != nil {
			return err
		}
	}
	for _, change := range r.Changes {
		target := filepath.Join(r.Root, filepath.FromSlash(change.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if change.Symlink {
			err = os.Symlink(string(change.After), target)
		} else {
			err = writeConfig(target, change.After)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
