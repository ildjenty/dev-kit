package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Workspace runs the git CLI in a single checkout.
type Workspace struct {
	root string
}

// Snapshot holds a commit outside the branches and the target version represented by its tree.
type Snapshot struct {
	Tree   string
	Commit string
}

// CommitResult holds the compared tree, the actual commit, and hook output.
// A nonempty Commit means the commit was created, even if an error is returned.
type CommitResult struct {
	BeforeTree string
	Commit     string
	Tree       string
	Output     string
}

// ErrTargetChanged indicates a mismatch with the evaluated tree before or after a commit.
var ErrTargetChanged = errors.New("tree does not match the evaluated target")

// StructureRevision represents structure.json history, ordered from newest to oldest commit.
type StructureRevision struct {
	Commit  string
	Blob    string
	Content []byte
	Deleted bool
}

// Open resolves the root of the checkout containing cwd.
func Open(ctx context.Context, cwd string) (*Workspace, error) {
	w := &Workspace{root: cwd}
	root, err := w.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	w.root = strings.TrimSuffix(string(root), "\n")
	return w, nil
}

// Root returns the absolute path of the checkout root.
func (w *Workspace) Root() string {
	return w.root
}

// Branch returns the current branch name, or a git error for a detached HEAD.
func (w *Workspace) Branch(ctx context.Context) (string, error) {
	return w.line(ctx, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
}

// Head returns the commit ID of the current HEAD.
func (w *Workspace) Head(ctx context.Context) (string, error) {
	return w.line(ctx, nil, "rev-parse", "--verify", "HEAD^{commit}")
}

// PreCommitPath returns the effective pre-commit hook path relative to the checkout.
func (w *Workspace) PreCommitPath(ctx context.Context) (string, error) {
	output, err := w.run(ctx, nil, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-commit")
	if err != nil {
		return "", err
	}
	path, err := filepath.Rel(w.root, strings.TrimSpace(string(output)))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(path), nil
}

// CreateBranch creates a branch from the current HEAD and switches to it.
// Pass devkit/<work ID> as the work branch name.
func (w *Workspace) CreateBranch(ctx context.Context, name string) error {
	_, err := w.run(ctx, nil, "switch", "-c", name)
	return err
}

// SwitchBranch switches to an existing branch.
func (w *Workspace) SwitchBranch(ctx context.Context, name string) error {
	_, err := w.run(ctx, nil, "switch", "--", name)
	return err
}

// SnapshotTree creates a tree from tracked files and untracked files that are not ignored.
// It leaves the normal index and HEAD unchanged.
func (w *Workspace) SnapshotTree(ctx context.Context) (tree string, err error) {
	index, cleanup, err := w.workingIndex(ctx)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	return w.line(ctx, index, "write-tree")
}

// Snapshot captures the working tree target and retains it at refs/devkit/<work ID>/<tree ID>.
func (w *Workspace) Snapshot(ctx context.Context, workID string) (Snapshot, error) {
	tree, err := w.SnapshotTree(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	commit, err := w.line(ctx, nil, "commit-tree", tree, "-m", "devkit snapshot")
	if err != nil {
		return Snapshot{}, err
	}
	return w.SnapshotFromCommit(ctx, workID, commit)
}

// SnapshotFromCommit retains an existing commit as a target, including commits changed by hooks.
func (w *Workspace) SnapshotFromCommit(ctx context.Context, workID, commit string) (Snapshot, error) {
	commitID, err := w.line(ctx, nil, "rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return Snapshot{}, err
	}
	tree, err := w.CommitTree(ctx, commitID)
	if err != nil {
		return Snapshot{}, err
	}
	if _, err := w.run(ctx, nil, "update-ref", "refs/devkit/"+workID+"/"+tree, commitID); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Tree: tree, Commit: commitID}, nil
}

// DeleteSnapshots deletes refs for completed or canceled work after its artifacts are persisted.
func (w *Workspace) DeleteSnapshots(ctx context.Context, workID string) error {
	refs, err := w.run(ctx, nil, "for-each-ref", "--format=%(refname)", "refs/devkit/"+workID+"/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(string(refs)) {
		if _, err := w.run(ctx, nil, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return nil
}

// CommitTree returns the tree ID of a commit.
func (w *Workspace) CommitTree(ctx context.Context, commit string) (string, error) {
	return w.line(ctx, nil, "rev-parse", "--verify", commit+"^{tree}")
}

// ReadFile reads a file from the specified normative reference using git show.
func (w *Workspace) ReadFile(ctx context.Context, ref, path string) ([]byte, error) {
	return w.run(ctx, nil, "show", ref+":"+path)
}

// NormativeIDs returns tree and blob IDs for each normative root, omitting missing roots.
func (w *Workspace) NormativeIDs(ctx context.Context, ref string, paths ...string) (map[string]string, error) {
	return w.treeEntries(ctx, ref, paths...)
}

// RequireCommitted checks that each path exists in HEAD and has no pending changes.
func (w *Workspace) RequireCommitted(ctx context.Context, paths ...string) error {
	entries, err := w.treeEntries(ctx, "HEAD", paths...)
	if err != nil {
		return err
	}
	pending := []string{}
	for _, path := range paths {
		if entries[path] == "" {
			pending = append(pending, path+" (not committed)")
		}
	}
	args := append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, paths...)
	status, err := w.run(ctx, nil, args...)
	if err != nil {
		return err
	}
	if len(status) != 0 {
		pending = append(pending, strings.TrimSpace(string(status)))
	}
	if len(pending) != 0 {
		return fmt.Errorf("initialization files have pending changes: %s; commit the devkit init changes first", strings.Join(pending, "\n"))
	}
	return nil
}

// Expand extracts a target into a new directory without applying export-ignore or export-subst.
// Create the destination once per target version and give evaluators read-only access to it.
func (w *Workspace) Expand(ctx context.Context, tree, destination string) (err error) {
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	index, cleanup, err := w.emptyIndex()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	if _, err := w.run(ctx, index, "read-tree", tree); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	_, err = w.run(ctx, index, "checkout-index", "--all", "--prefix="+destination+string(filepath.Separator))
	if err != nil {
		return errors.Join(err, os.RemoveAll(destination))
	}
	return nil
}

// Commit compares the current working tree and commits its temporary index with hooks enabled.
// After success, it resets the normal index to the new HEAD without touching the working tree.
// A mismatch before or after the commit returns ErrTargetChanged without undoing any created commit.
func (w *Workspace) Commit(ctx context.Context, expectedTree, workID, stage, message string) (result CommitResult, err error) {
	index, cleanup, err := w.workingIndex(ctx)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	result.BeforeTree, err = w.line(ctx, index, "write-tree")
	if err != nil {
		return result, err
	}
	if result.BeforeTree != expectedTree {
		return result, ErrTargetChanged
	}
	env := append(index, "DEVKIT_STAGE="+stage, "DEVKIT_WORK="+workID)
	args := []string{"commit", "--allow-empty", "-m", message}
	output, err := w.command(ctx, env, args...).CombinedOutput()
	result.Output = string(output)
	if err != nil {
		return result, fmt.Errorf("git commit: %w: %s", err, output)
	}
	result.Commit, err = w.Head(ctx)
	if err != nil {
		return result, err
	}
	result.Tree, err = w.CommitTree(ctx, result.Commit)
	if err != nil {
		return result, err
	}
	_, err = w.run(ctx, nil, "reset", "--mixed", "HEAD")
	if result.Tree != expectedTree {
		err = errors.Join(ErrTargetChanged, err)
	}
	return result, err
}

// ChangedFiles returns paths changed between two targets, treating renames as deletions and additions.
func (w *Workspace) ChangedFiles(ctx context.Context, before, after string) ([]string, error) {
	output, err := w.run(ctx, nil, "diff", "--name-only", "--no-renames", "-z", before, after, "--")
	if err != nil {
		return nil, err
	}
	if len(output) == 0 {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"), nil
}

// Diff returns the patch between two fixed targets.
func (w *Workspace) Diff(ctx context.Context, before, after string) ([]byte, error) {
	return w.run(ctx, nil, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--binary", before, after, "--")
}

// StructureHistory reads additions, changes, and deletions of structure.json from the given reference.
func (w *Workspace) StructureHistory(ctx context.Context, ref, path string) ([]StructureRevision, error) {
	output, err := w.run(ctx, nil, "log", "--format=%H", ref, "--", path)
	if err != nil {
		return nil, err
	}
	var history []StructureRevision
	for _, commit := range strings.Fields(string(output)) {
		entries, err := w.treeEntries(ctx, commit, path)
		if err != nil {
			return nil, err
		}
		revision := StructureRevision{Commit: commit, Blob: entries[path]}
		revision.Deleted = revision.Blob == ""
		if !revision.Deleted {
			revision.Content, err = w.ReadFile(ctx, commit, path)
			if err != nil {
				return nil, err
			}
		}
		history = append(history, revision)
	}
	return history, nil
}

func (w *Workspace) treeEntries(ctx context.Context, ref string, paths ...string) (map[string]string, error) {
	args := append([]string{"ls-tree", "-z", ref, "--"}, paths...)
	output, err := w.run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]string)
	for _, entry := range bytes.Split(output, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		metadata, path, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid git ls-tree output: %q", entry)
		}
		entries[path] = fields[2]
	}
	return entries, nil
}

func (w *Workspace) emptyIndex() ([]string, func() error, error) {
	directory, err := os.MkdirTemp("", "devkit-index-")
	if err != nil {
		return nil, nil, err
	}
	return []string{"GIT_INDEX_FILE=" + filepath.Join(directory, "index")}, func() error { return os.RemoveAll(directory) }, nil
}

func (w *Workspace) workingIndex(ctx context.Context) ([]string, func() error, error) {
	env, cleanup, err := w.emptyIndex()
	if err != nil {
		return nil, nil, err
	}
	indexPath, err := w.line(ctx, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	data, err := os.ReadFile(indexPath)
	if err == nil {
		err = os.WriteFile(strings.TrimPrefix(env[0], "GIT_INDEX_FILE="), data, 0o600)
	} else if errors.Is(err, os.ErrNotExist) {
		_, err = w.run(ctx, env, "read-tree", "--empty")
	}
	if err == nil {
		var paths []byte
		paths, err = w.run(ctx, env, "ls-files", "-z")
		if err == nil {
			for _, flag := range []string{"--no-assume-unchanged", "--no-skip-worktree"} {
				cmd := w.command(ctx, env, "update-index", flag, "-z", "--stdin")
				cmd.Stdin = bytes.NewReader(paths)
				output, updateErr := cmd.CombinedOutput()
				if updateErr != nil {
					err = fmt.Errorf("git update-index: %w: %s", updateErr, output)
					break
				}
			}
		}
	}
	if err == nil {
		_, err = w.run(ctx, env, "add", "--all", "--", ".")
	}
	if err != nil {
		return nil, nil, errors.Join(err, cleanup())
	}
	return env, cleanup, nil
}

func (w *Workspace) command(ctx context.Context, env []string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = w.root
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func (w *Workspace) run(ctx context.Context, env []string, args ...string) ([]byte, error) {
	output, err := w.command(ctx, env, args...).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, exitError.Stderr)
		}
		return output, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func (w *Workspace) line(ctx context.Context, env []string, args ...string) (string, error) {
	output, err := w.run(ctx, env, args...)
	return strings.TrimSuffix(string(output), "\n"), err
}
