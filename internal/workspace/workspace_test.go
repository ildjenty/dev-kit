package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newWorkspace(t *testing.T) *Workspace {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.name", "devkit test")
	git(t, root, "config", "user.email", "devkit@example.invalid")
	git(t, root, "config", "commit.gpgsign", "false")
	write(t, root, ".gitignore", ".devkit/run/\nignored\n")
	write(t, root, "tracked", "initial\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "initial")
	w, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSuffix(string(output), "\n")
}

func write(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hook(t *testing.T, w *Workspace, body string) {
	t.Helper()
	path := filepath.Join(w.Root(), ".git/hooks/pre-commit")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAndBranches(t *testing.T) {
	w := newWorkspace(t)
	ctx := context.Background()
	write(t, w.Root(), "sub/file", "content")
	sub, err := Open(ctx, filepath.Join(w.Root(), "sub"))
	if err != nil || sub.Root() != w.Root() {
		t.Fatalf("ルートの解決: %v, %v", sub, err)
	}
	if err := w.CreateBranch(ctx, "devkit/work"); err != nil {
		t.Fatal(err)
	}
	branch, err := w.Branch(ctx)
	if err != nil || branch != "devkit/work" {
		t.Fatalf("ブランチ: %q, %v", branch, err)
	}
	if err := w.SwitchBranch(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	linkedPath := filepath.Join(t.TempDir(), "checkout")
	git(t, w.Root(), "worktree", "add", "-b", "linked", linkedPath)
	linked, err := Open(ctx, linkedPath)
	if err != nil {
		t.Fatal(err)
	}
	write(t, linked.Root(), "tracked", "linked change\n")
	indexPath := git(t, linked.Root(), "rev-parse", "--path-format=absolute", "--git-path", "index")
	index := read(t, indexPath)
	linkedSnapshot, err := linked.Snapshot(ctx, "linked")
	if err != nil || !bytes.Equal(index, read(t, indexPath)) {
		t.Fatalf("linked worktree の index が変わりました: %v", err)
	}
	content, err := linked.ReadFile(ctx, linkedSnapshot.Tree, "tracked")
	if err != nil || string(content) != "linked change\n" {
		t.Fatalf("linked worktree の対象: %q, %v", content, err)
	}
	git(t, w.Root(), "checkout", "--detach")
	if _, err := w.Branch(ctx); err == nil {
		t.Fatal("detached HEAD のエラーがありません")
	}
}

func TestSnapshotAndExpand(t *testing.T) {
	w := newWorkspace(t)
	ctx := context.Background()
	head := git(t, w.Root(), "rev-parse", "HEAD")
	write(t, w.Root(), "tracked", "staged\n")
	write(t, w.Root(), "ignored", "forced\n")
	git(t, w.Root(), "add", "tracked")
	git(t, w.Root(), "add", "-f", "ignored")
	index := read(t, filepath.Join(w.Root(), ".git/index"))
	write(t, w.Root(), "tracked", "working\n")
	write(t, w.Root(), "untracked\nfile", "new\n")
	write(t, w.Root(), ".devkit/run/work/generated", "excluded\n")
	write(t, w.Root(), ".gitattributes", "tracked export-ignore\nsubstitute export-subst\n")
	write(t, w.Root(), "substitute", "$Format:%H$\n")
	snap, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(index, read(t, filepath.Join(w.Root(), ".git/index"))) || head != git(t, w.Root(), "rev-parse", "HEAD") {
		t.Fatal("スナップショットが通常の index または HEAD を変更しました")
	}
	if got := git(t, w.Root(), "rev-parse", "refs/devkit/work/"+snap.Tree); got != snap.Commit {
		t.Fatalf("ref: %s", got)
	}
	files, err := w.ChangedFiles(ctx, head, snap.Tree)
	if err != nil || !strings.Contains(strings.Join(files, "\x00"), "untracked\nfile") {
		t.Fatalf("変更パス: %q, %v", files, err)
	}
	destination := filepath.Join(t.TempDir(), "snap", snap.Tree)
	if err := w.Expand(ctx, snap.Tree, destination); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"tracked": "working\n", "ignored": "forced\n", "substitute": "$Format:%H$\n", "untracked\nfile": "new\n"} {
		if got := string(read(t, filepath.Join(destination, path))); got != want {
			t.Fatalf("%s: %q", path, got)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, ".devkit/run")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ignore された生成物が含まれています: %v", err)
	}
	if err := w.DeleteSnapshots(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if got := git(t, w.Root(), "for-each-ref", "--format=%(refname)", "refs/devkit/work/"); got != "" {
		t.Fatalf("ref が残っています: %s", got)
	}
}

func TestNormativeIDsAndReadFile(t *testing.T) {
	w := newWorkspace(t)
	ctx := context.Background()
	write(t, w.Root(), "devkit.toml", "kit_version = 'v1'\n")
	write(t, w.Root(), ".devkit/rules.md", "規則\n")
	write(t, w.Root(), ".claude/agents/judge.md", "判定者\n")
	before, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := w.NormativeIDs(ctx, before.Tree)
	if err != nil || len(ids) != 3 || ids[".claude/agents"] == "" {
		t.Fatalf("規範の識別: %v, %v", ids, err)
	}
	write(t, w.Root(), "tracked", "code changed\n")
	after, err := w.Snapshot(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	newIDs, err := w.NormativeIDs(ctx, after.Tree)
	if err != nil || !reflect.DeepEqual(ids, newIDs) {
		t.Fatalf("コードだけの変更で規範が変わりました: %v, %v", newIDs, err)
	}
	write(t, w.Root(), "devkit.toml", "kit_version = 'v2'\n")
	content, err := w.ReadFile(ctx, before.Tree, "devkit.toml")
	if err != nil || string(content) != "kit_version = 'v1'\n" {
		t.Fatalf("固定した規範の読み出し: %q, %v", content, err)
	}
}

func TestCommit(t *testing.T) {
	for _, mode := range []string{"success", "before_changed", "hook_changed", "hook_failed", "empty"} {
		t.Run(mode, func(t *testing.T) {
			w := newWorkspace(t)
			ctx := context.Background()
			head := git(t, w.Root(), "rev-parse", "HEAD")
			if mode != "empty" {
				write(t, w.Root(), "tracked", "judged\n")
			}
			snap, err := w.Snapshot(ctx, "work")
			if err != nil {
				t.Fatal(err)
			}
			index := read(t, filepath.Join(w.Root(), ".git/index"))
			hook(t, w, "test \"$DEVKIT_STAGE\" = implement\ntest \"$DEVKIT_WORK\" = work\necho hook-ran >&2")
			switch mode {
			case "before_changed":
				write(t, w.Root(), "tracked", "later\n")
			case "hook_changed":
				hook(t, w, "echo formatted > tracked\ngit add tracked")
			case "hook_failed":
				hook(t, w, "echo failure >&2\necho formatted > tracked\nexit 1")
			}
			result, err := w.Commit(ctx, snap.Tree, "work", "implement", "complete")
			switch mode {
			case "before_changed", "hook_failed":
				if err == nil || result.Commit != "" || git(t, w.Root(), "rev-parse", "HEAD") != head {
					t.Fatalf("commit を止めていません: %+v, %v", result, err)
				}
				if !bytes.Equal(index, read(t, filepath.Join(w.Root(), ".git/index"))) {
					t.Fatal("失敗時に通常の index が変わりました")
				}
				if mode == "before_changed" && !errors.Is(err, ErrTargetChanged) {
					t.Fatalf("対象の変化を報告していません: %v", err)
				}
				if mode == "hook_failed" && (!strings.Contains(result.Output, "failure") || string(read(t, filepath.Join(w.Root(), "tracked"))) != "formatted\n") {
					t.Fatalf("hook の失敗内容が残っていません: %+v", result)
				}
			case "hook_changed":
				if !errors.Is(err, ErrTargetChanged) || result.Commit == "" || result.Tree == snap.Tree {
					t.Fatalf("hook による変更を報告していません: %+v, %v", result, err)
				}
				updated, err := w.SnapshotFromCommit(ctx, "work", result.Commit)
				if err != nil || updated.Tree != result.Tree {
					t.Fatalf("新しい対象を保持できません: %+v, %v", updated, err)
				}
			default:
				if err != nil || result.Tree != snap.Tree || !strings.Contains(result.Output, "hook-ran") {
					t.Fatalf("commit: %+v, %v", result, err)
				}
			}
			if result.Commit != "" && git(t, w.Root(), "diff", "--cached", "--name-only") != "" {
				t.Fatal("通常の index と HEAD が食い違っています")
			}
		})
	}
}

func TestStructureHistory(t *testing.T) {
	w := newWorkspace(t)
	ctx := context.Background()
	const path = ".devkit/structure.json"
	for _, content := range []string{"{\"modules\": []}\n", "{\"modules\": [\"workspace\"]}\n"} {
		write(t, w.Root(), path, content)
		git(t, w.Root(), "add", path)
		git(t, w.Root(), "commit", "-m", "structure")
	}
	git(t, w.Root(), "rm", path)
	git(t, w.Root(), "commit", "-m", "delete structure")
	history, err := w.StructureHistory(ctx, "HEAD")
	if err != nil || len(history) != 3 {
		t.Fatalf("履歴: %+v, %v", history, err)
	}
	if !history[0].Deleted || history[1].Blob == "" || string(history[1].Content) != "{\"modules\": [\"workspace\"]}\n" {
		t.Fatalf("履歴の内容: %+v", history)
	}
}
