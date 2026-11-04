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

func (w *Workspace) indexPaths(ctx context.Context, env []string) ([]byte, error) {
	output, err := w.run(ctx, env, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	var paths []byte
	for _, entry := range bytes.Split(output, []byte{0}) {
		metadata, path, ok := bytes.Cut(entry, []byte{'\t'})
		if ok && bytes.HasSuffix(metadata, []byte(" 0")) {
			paths = append(paths, path...)
			paths = append(paths, 0)
		}
	}
	return paths, nil
}

// MergeResult describes a tree and unresolved paths from git's merge machinery.
type MergeResult struct {
	Tree      string
	Conflicts []string
}

// CommitObject creates an unbranched commit with the supplied parents and retains it as a snapshot.
func (w *Workspace) CommitObject(ctx context.Context, workID, tree string, parents ...string) (Snapshot, error) {
	args := []string{"commit-tree", tree, "-m", "devkit unit snapshot"}
	for _, parent := range parents {
		args = append(args, "-p", parent)
	}
	commit, err := w.line(ctx, nil, args...)
	if err != nil {
		return Snapshot{}, err
	}
	return w.SnapshotFromCommit(ctx, workID, commit)
}

// AddWorktree creates or verifies a detached unit checkout at the supplied commit.
func (w *Workspace) AddWorktree(ctx context.Context, path, commit string) (*Workspace, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if _, err := w.run(ctx, nil, "worktree", "add", "--detach", path, commit); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	unit, err := Open(ctx, path)
	if err != nil {
		return nil, err
	}
	head, err := unit.Head(ctx)
	if err != nil {
		return nil, err
	}
	if unit.Root() != path || head != commit {
		return nil, fmt.Errorf("unit worktree does not match its starting commit: %s", path)
	}
	if branch, _ := unit.Branch(ctx); branch != "" {
		return nil, fmt.Errorf("unit worktree must have detached HEAD: %s", path)
	}
	return unit, nil
}

// RemoveWorktree removes a unit checkout whose contents are already kept elsewhere: unchanged
// from base, or, when captured is set, still identical to that frozen and merged tree.
// False means it was retained.
func (w *Workspace) RemoveWorktree(ctx context.Context, path, base, captured string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	unit, err := Open(ctx, path)
	if err != nil {
		return false, err
	}
	removable, err := unit.unchangedFrom(ctx, base)
	if err != nil {
		return false, err
	}
	if !removable && captured != "" {
		tree, err := unit.SnapshotTree(ctx)
		if err != nil {
			return false, err
		}
		removable = tree == captured
	}
	if !removable {
		return false, nil
	}
	_, err = w.run(ctx, nil, "worktree", "remove", "--force", "--", path)
	return err == nil, err
}

func (w *Workspace) unchangedFrom(ctx context.Context, base string) (bool, error) {
	head, err := w.Head(ctx)
	if err != nil {
		return false, err
	}
	status, err := w.run(ctx, nil, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return false, err
	}
	return len(status) == 0 && head == base, nil
}

func (w *Workspace) PreviewMerge(ctx context.Context, ours, theirs string) (MergeResult, error) {
	output, err := w.run(ctx, nil, "merge-tree", "--write-tree", "--name-only", "-z", ours, theirs)
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return MergeResult{}, err
	}
	lines := strings.Split(string(output), "\x00")
	result := MergeResult{Tree: lines[0]}
	if len(result.Tree) != 40 && len(result.Tree) != 64 {
		return MergeResult{}, fmt.Errorf("git merge-tree did not return a tree: %s", output)
	}
	for _, path := range lines[1:] {
		if path == "" {
			break
		}
		result.Conflicts = append(result.Conflicts, path)
	}
	return result, nil
}

// Merge applies one three-way merge to the work checkout and index without moving HEAD.
// The caller records the inputs and expected result before calling it.
func (w *Workspace) Merge(ctx context.Context, base, ours, theirs string) error {
	if _, err := w.run(ctx, nil, "add", "--all", "--", "."); err != nil {
		return err
	}
	_, err := w.run(ctx, nil, "merge-recursive", base, "--", ours, theirs)
	var exit *exec.ExitError
	if err != nil && errors.As(err, &exit) && exit.ExitCode() == 1 {
		conflicts, conflictErr := w.Conflicts(ctx)
		if conflictErr != nil {
			return conflictErr
		}
		if len(conflicts) > 0 {
			return nil
		}
	}
	return err
}

// Conflicts returns unresolved paths from the git index.
func (w *Workspace) Conflicts(ctx context.Context) ([]string, error) {
	output, err := w.run(ctx, nil, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil || len(output) == 0 {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"), nil
}

// ResolveMerge stages the worker's conflict resolution and returns its tree.
// HEAD remains unchanged.
func (w *Workspace) ResolveMerge(ctx context.Context) (string, error) {
	if _, err := w.run(ctx, nil, "add", "--all", "--", "."); err != nil {
		return "", err
	}
	return w.SnapshotTree(ctx)
}

// AddMergeWorktree creates a detached checkout with the source checkout's unmerged index.
// Existing worker checkouts are preserved when an instruction is retried.
func (w *Workspace) AddMergeWorktree(ctx context.Context, path, commit string) (*Workspace, error) {
	_, err := os.Lstat(path)
	fresh := errors.Is(err, os.ErrNotExist)
	unit, err := w.AddWorktree(ctx, path, commit)
	if err != nil || !fresh {
		return unit, err
	}
	index, err := w.run(ctx, nil, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	if _, err := unit.run(ctx, nil, "read-tree", "--empty"); err != nil {
		return nil, err
	}
	command := unit.command(ctx, nil, "update-index", "-z", "--index-info")
	command.Stdin = bytes.NewReader(index)
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git update-index: %w: %s", err, output)
	}
	return unit, nil
}
