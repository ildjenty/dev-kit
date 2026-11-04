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

// Init installs repository assets and merges the supplied host fragments.
// It stages the kit copy first, then calls confirm with all remaining changes
// before applying them. A nil callback means the caller already authorized them.
// Existing skills, templates, aspects, rules, and agent definitions are preserved.
func Init(ctx context.Context, cwd string, confirm func([]Change) error) (Result, error) {
	w, err := workspace.Open(ctx, cwd)
	if err != nil {
		return Result{}, err
	}
	result := Result{Root: w.Root(), Guidance: []string{trustGuidance}}
	configuration := []byte(fmt.Sprintf("kit_version = %q\n", kit.Version))
	existing, err := os.ReadFile(filepath.Join(result.Root, "devkit.toml"))
	if err == nil {
		configuration = existing
	} else if !errors.Is(err, fs.ErrNotExist) {
		return result, err
	}
	c, err := config.Parse(configuration)
	if err != nil {
		return result, err
	}
	if c.KitVersion != kit.Version {
		return result, fmt.Errorf("kit version mismatch: binary %q, devkit.toml %q", kit.Version, c.KitVersion)
	}
	if err := kit.Copy(result.Root); err != nil {
		return result, err
	}
	k, err := kit.Open(result.Root, c.KitVersion)
	if err != nil {
		return result, err
	}
	deny, err := k.ReadFile("hosts/deny.toml")
	if err != nil {
		return result, err
	}
	configuration, err = mergeTOML(configuration, deny)
	if err != nil {
		return result, err
	}
	if _, err := config.Parse(configuration); err != nil {
		return result, err
	}
	result.addChange("devkit.toml", existing, configuration)
	for _, pair := range [][2]string{
		{"plans", ".devkit/plans"},
		{"checks", ".devkit/checks"},
		{"examples", ".devkit/checks"},
		{"skills/devkit", ".agents/skills/devkit"},
		{"hosts/claude/agents", ".claude/agents"},
		{"hosts/codex/agents", ".codex/agents"},
	} {
		if err := result.addTree(k, pair[0], pair[1]); err != nil {
			return result, err
		}
	}
	rules, err := k.ReadFile("rules/rules.md")
	if err != nil {
		return result, err
	}
	if err := result.addFile(".devkit/rules.md", rules); err != nil {
		return result, err
	}
	if err := result.addSkillLink(); err != nil {
		return result, err
	}
	for _, fragment := range hostFragments {
		data, err := k.ReadFile(fragment.source)
		if err != nil {
			return result, err
		}
		before, err := readOptional(filepath.Join(result.Root, fragment.target))
		if err != nil {
			return result, err
		}
		after, err := fragment.merge(before, data)
		if err != nil {
			return result, fmt.Errorf("%s: %w", fragment.target, err)
		}
		result.addChange(fragment.target, before, after)
	}
	before, err := readOptional(filepath.Join(result.Root, ".gitignore"))
	if err != nil {
		return result, err
	}
	ignored, err := runtimeIgnored(ctx, result.Root)
	if err != nil {
		return result, err
	}
	after := appendLines(before, []byte(".devkit/run/\n"))
	if !ignored && bytes.Equal(before, after) {
		after = bytes.Clone(before)
		if len(after) != 0 && after[len(after)-1] != '\n' {
			after = append(after, '\n')
		}
		after = append(after, ".devkit/run/\n"...)
	}
	result.addChange(".gitignore", before, after)
	if len(result.Changes) != 0 && confirm != nil {
		if err := confirm(result.Changes); err != nil {
			return result, err
		}
	}
	for _, name := range []string{".devkit/run/sessions", ".devkit/run/work"} {
		if err := os.MkdirAll(filepath.Join(result.Root, name), 0o755); err != nil {
			return result, err
		}
	}
	for _, change := range result.Changes {
		target := filepath.Join(result.Root, filepath.FromSlash(change.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, err
		}
		if change.Symlink {
			err = os.Symlink(string(change.After), target)
		} else {
			err = writeConfig(target, change.After)
		}
		if err != nil {
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
	cmd := exec.CommandContext(ctx, "git", "-C", root, "check-ignore", "--quiet", "--", ".devkit/run/")
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

func (r *Result) addTree(k *kit.Kit, source, target string) error {
	return fs.WalkDir(k, source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := k.ReadFile(name)
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
