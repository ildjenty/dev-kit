package gitx

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/backlog"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Status  string `json:"status"`
	OldPath string `json:"oldPath,omitempty"`
	Path    string `json:"path"`
}
type Changes struct {
	Files              []File   `json:"files"`
	AddedDirectories   []string `json:"addedDirectories"`
	RemovedDirectories []string `json:"removedDirectories"`
	Patch              string   `json:"patch"`
	Excluded           []string `json:"-"`
}
type Git struct {
	Cwd              string
	Refs             []string
	RecordDir        string
	BaseBranch       string
	BaseBranchSource string
}

func raw(cwd string, args []string, input string, env []string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(input)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Command failed in %s: git %s: %w\n%s", cwd, strings.Join(args, " "), err, strings.TrimRight(stderr.String(), "\n"))
	}
	return string(out), nil
}

func Run(cwd string, args ...string) (string, error) {
	out, err := raw(cwd, args, "", nil)
	return strings.TrimSpace(out), err
}
func BaseBranch(cwd, explicit string) (string, string, error) {
	if explicit != "" {
		return explicit, "BASE_BRANCH", nil
	}
	root, err := Run(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(root, ".archon/config.yaml")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("%s: %w", path, err)
	}
	var settings struct {
		Worktree struct {
			BaseBranch string `yaml:"baseBranch"`
		} `yaml:"worktree"`
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &settings); err != nil {
			return "", "", fmt.Errorf("%s: %w", path, err)
		}
	}
	if strings.TrimSpace(settings.Worktree.BaseBranch) != "" {
		return settings.Worktree.BaseBranch, ".archon/config.yaml worktree.baseBranch", nil
	}
	ref, err := Run(root, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && strings.HasPrefix(ref, "refs/remotes/origin/") && ref != "refs/remotes/origin/HEAD" {
		return strings.TrimPrefix(ref, "refs/remotes/origin/"), "origin/HEAD", nil
	}
	return "", "", fmt.Errorf("cannot resolve base branch from BASE_BRANCH, worktree.baseBranch in %s, or origin/HEAD (git symbolic-ref refs/remotes/origin/HEAD): %v", path, err)
}
func New(cwd, baseBranch string, recordDirs ...string) (*Git, error) {
	baseBranch, source, err := BaseBranch(cwd, baseBranch)
	if err != nil {
		return nil, err
	}
	g := &Git{Cwd: cwd, Refs: []string{}, BaseBranch: baseBranch, BaseBranchSource: source}
	if len(recordDirs) > 0 {
		g.RecordDir = recordDirs[0]
		if g.RecordDir != "" && !filepath.IsAbs(g.RecordDir) {
			g.RecordDir = filepath.Join(cwd, g.RecordDir)
		}
	}
	for _, ref := range []string{"origin/" + baseBranch, baseBranch} {
		full := "refs/heads/" + baseBranch
		if ref != baseBranch {
			full = "refs/remotes/" + ref
		}
		_, exists, err := g.query("show-ref", "--verify", "--quiet", full)
		if err != nil {
			return nil, err
		}
		if exists {
			if _, err := g.Run("rev-parse", "--verify", ref+"^{commit}"); err != nil {
				return nil, err
			}
			g.Refs = append(g.Refs, ref)
		}
	}
	if len(g.Refs) == 0 {
		return nil, fmt.Errorf("neither origin/%s nor %s is a commit", baseBranch, baseBranch)
	}
	return g, nil
}
func (g *Git) Run(args ...string) (string, error) { return Run(g.Cwd, args...) }
func (g *Git) OK(args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Cwd
	return cmd.Run() == nil
}
func (g *Git) query(args ...string) (string, bool, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Cwd
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && stderr.Len() == 0 {
		return "", false, nil
	}
	return "", false, fmt.Errorf("Command failed in %s: git %s: %w\n%s", g.Cwd, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
}
func (g *Git) PathExists(commit, path string) (bool, error) {
	out, err := g.Bytes("ls-tree", "--full-tree", "-z", commit, "--", ":(literal)"+path)
	if err != nil {
		return false, err
	}
	for _, entry := range strings.Split(string(out), "\x00") {
		_, name, ok := strings.Cut(entry, "\t")
		if ok && name == path {
			return true, nil
		}
	}
	return false, nil
}
func (g *Git) MergeBases(commit string) ([]string, error) {
	bases := []string{}
	seen := map[string]bool{}
	for _, ref := range g.Refs {
		base, related, err := g.query("merge-base", commit, ref)
		if err != nil {
			return nil, err
		}
		if related {
			if !seen[base] {
				bases = append(bases, base)
				seen[base] = true
			}
		}
	}
	return bases, nil
}
func (g *Git) MergeBase(commit string) (string, error) {
	if commit == "" {
		commit = "HEAD"
	}
	bases, err := g.MergeBases(commit)
	if err != nil || len(bases) == 0 {
		return "", err
	}
	return bases[0], nil
}
func (g *Git) snapshot(from, to string, env []string) (Changes, error) {
	result := Changes{Files: []File{}, AddedDirectories: []string{}, RemovedDirectories: []string{}}
	out, err := raw(g.Cwd, []string{"diff", "--name-status", "-z", "--find-renames", "--no-ext-diff", from, to}, "", env)
	if err != nil {
		return result, err
	}
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts) && parts[i] != ""; {
		status, first := parts[i], parts[i+1]
		i += 2
		file := File{Status: status, Path: first}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			file.OldPath = first
			file.Path = parts[i]
			i++
		}
		result.Files = append(result.Files, file)
	}
	directories := func(ref string) (map[string]bool, error) {
		out, err := raw(g.Cwd, []string{"ls-tree", "-r", "--name-only", "-z", ref}, "", env)
		if err != nil {
			return nil, err
		}
		dirs := map[string]bool{}
		for _, path := range strings.Split(out, "\x00") {
			p := strings.Split(path, "/")
			for i := 1; i < len(p); i++ {
				dirs[strings.Join(p[:i], "/")] = true
			}
		}
		return dirs, nil
	}
	before, err := directories(from)
	if err != nil {
		return result, err
	}
	after, err := directories(to)
	if err != nil {
		return result, err
	}
	for path := range after {
		if !before[path] {
			result.AddedDirectories = append(result.AddedDirectories, path)
		}
	}
	for path := range before {
		if !after[path] {
			result.RemovedDirectories = append(result.RemovedDirectories, path)
		}
	}
	sort.Strings(result.AddedDirectories)
	sort.Strings(result.RemovedDirectories)
	result.Excluded, err = g.excludedPaths(to)
	if err != nil {
		return result, err
	}
	result.Patch, err = raw(g.Cwd, []string{"diff", "--no-color", "--no-ext-diff", "--binary", from, to}, "", env)
	return result, err
}
func (g *Git) Changes(from, to string) (Changes, error) {
	var err error
	if from == "" {
		from, err = g.MergeBase("HEAD")
		if err != nil {
			return Changes{}, err
		}
	}
	if to == "" {
		to = "HEAD"
	}
	return g.snapshot(from, to, nil)
}
func (g *Git) ChangedFiles(from, to string) ([]File, error) {
	c, err := g.Changes(from, to)
	return c.Files, err
}
func (g *Git) DirectoryChanges(from, to string) (added, removed []string, err error) {
	c, err := g.Changes(from, to)
	return c.AddedDirectories, c.RemovedDirectories, err
}
func (g *Git) WorkingChanges(from string) (Changes, error) {
	var err error
	if from == "" {
		from, err = g.MergeBase("HEAD")
		if err != nil {
			return Changes{}, err
		}
	}
	if from == "" {
		return Changes{}, fmt.Errorf("HEAD has no merge base with the base branch")
	}
	temp, err := os.MkdirTemp("", "devkit-index-")
	if err != nil {
		return Changes{}, err
	}
	defer os.RemoveAll(temp)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(temp, "index")}
	for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A", "--", ":/"}} {
		if _, err := raw(g.Cwd, args, "", env); err != nil {
			return Changes{}, err
		}
	}
	tree, err := raw(g.Cwd, []string{"write-tree"}, "", env)
	if err != nil {
		return Changes{}, err
	}
	return g.snapshot(from, strings.TrimSpace(tree), env)
}
func (g *Git) IsAncestor(a, b string) (bool, error) {
	_, ancestor, err := g.query("merge-base", "--is-ancestor", a, b)
	return ancestor, err
}
func (g *Git) excludedPaths(commit string) ([]string, error) {
	branch, err := g.Branch()
	if err != nil {
		return nil, err
	}
	var slug strings.Builder
	for _, c := range []byte(branch) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.!~*'()", rune(c)) {
			slug.WriteByte(c)
		} else {
			fmt.Fprintf(&slug, "%%%02X", c)
		}
	}
	paths := []string{}
	path := ".devkit/backlog/" + slug.String() + ".json"
	if data, e := g.Bytes("show", commit+":"+path); e == nil && backlog.Validate(data) == nil {
		mode, e := g.Run("ls-tree", commit, "--", path)
		if e != nil {
			return nil, e
		}
		if strings.HasPrefix(mode, "100644 blob ") || strings.HasPrefix(mode, "100755 blob ") {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (g *Git) ChangeID(commit string) (*string, error) {
	from, err := g.MergeBase(commit)
	if err != nil || from == "" {
		return nil, err
	}
	args := []string{"diff", "-U0", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--binary", from, commit, "--", "."}
	paths, err := g.excludedPaths(commit)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		args = append(args, ":(exclude,literal)"+path)
	}
	diff, err := raw(g.Cwd, args, "", nil)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "index ") {
			continue
		}
		lines = append(lines, line)
	}
	data, err := json.Marshal(lines)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("%x", sha256.Sum256(data))
	return &id, nil
}

func (g *Git) Head() (string, error)   { return g.Run("rev-parse", "HEAD") }
func (g *Git) Branch() (string, error) { return g.Run("rev-parse", "--abbrev-ref", "HEAD") }
func (g *Git) Clean() (bool, error) {
	out, err := g.Run("status", "--porcelain", "--untracked-files=all")
	return out == "", err
}

func (g *Git) Bytes(args ...string) ([]byte, error) {
	out, err := raw(g.Cwd, args, "", nil)
	return []byte(out), err
}

func (g *Git) ExportTree(commit, destination string) error {
	archive, err := g.Bytes("archive", "--format=tar", commit)
	if err != nil {
		return err
	}
	if err = os.RemoveAll(destination); err != nil {
		return err
	}
	if err = os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, e := reader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		name := filepath.Join(destination, filepath.FromSlash(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
				return err
			}
			data, e := io.ReadAll(reader)
			if e != nil {
				return e
			}
			if err = os.WriteFile(name, data, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
				return err
			}
			if err = os.Symlink(header.Linkname, name); err != nil {
				return err
			}
		}
	}
	listing, err := g.Bytes("ls-tree", "-r", "-z", commit)
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(string(listing), "\x00") {
		metadata, path, ok := strings.Cut(entry, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(metadata)
		if fields[1] != "blob" {
			continue
		}
		data, e := g.Bytes("cat-file", "blob", fields[2])
		if e != nil {
			return e
		}
		name := filepath.Join(destination, filepath.FromSlash(path))
		if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		if err = os.Remove(name); err != nil && !os.IsNotExist(err) {
			return err
		}
		if fields[0] == "120000" {
			err = os.Symlink(string(data), name)
		} else {
			mode := os.FileMode(0644)
			if fields[0] == "100755" {
				mode = 0755
			}
			err = os.WriteFile(name, data, mode)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
