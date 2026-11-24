package judge

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func treeContents(t *testing.T, root string) map[string]doc {
	t.Helper()
	result := map[string]doc{}
	testutil.OK(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := doc{"mode": info.Mode().String()}
		if info.Mode()&os.ModeSymlink != 0 {
			value["link"], err = os.Readlink(path)
		} else if !info.IsDir() {
			value["bytes"], err = os.ReadFile(path)
		}
		name, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		result[name] = value
		return err
	}))
	return result
}

func treeGitLog(t *testing.T) string {
	t.Helper()
	git := must(exec.LookPath("git"))
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	testutil.Put(t, bin, "git", `#!/bin/sh
printf '%s\n' "$1" >> "$DEVKIT_TEST_GIT_LOG"
if [ "$1" = diff ] && [ -n "$DEVKIT_TEST_MARKER" ]; then
  test ! -e "$DEVKIT_TEST_MARKER" || exit 1
fi
if [ "$1" = cat-file ] && [ -f "$DEVKIT_TEST_FAIL_ONCE" ]; then
  rm "$DEVKIT_TEST_FAIL_ONCE"
  exit 1
fi
exec "$DEVKIT_TEST_GIT" "$@"
`)
	testutil.OK(t, os.Chmod(filepath.Join(bin, "git"), 0755))
	t.Setenv("DEVKIT_TEST_GIT", git)
	t.Setenv("DEVKIT_TEST_GIT_LOG", log)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return log
}

func TestJudgedTreeExportsThenUpdatesOnlyChangedEntries(t *testing.T) {
	repo := testutil.Repo(t)
	for name, content := range map[string]string{
		"unchanged": "same\x00bytes\n\n", "modified": "before", "deleted/last": "gone",
		"to-folder": "file", "to-file/child": "child", "executable": "up", "non-executable": "down",
		"to-link": "file", ".gitattributes": "ignored export-ignore\nsubstituted export-subst\n",
		"ignored": "included", "substituted": "$Format:%H$\n",
	} {
		testutil.Put(t, repo, name, content)
	}
	testutil.OK(t, os.Chmod(filepath.Join(repo, "non-executable"), 0755))
	testutil.OK(t, os.Symlink("modified", filepath.Join(repo, "link")))
	testutil.OK(t, os.Symlink("unchanged", filepath.Join(repo, "from-link")))
	testutil.OK(t, os.Symlink("../outside", filepath.Join(repo, "link-to-folder")))
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "before")
	ctx := record.ContextData{Cwd: repo, Dir: record.RecordDir(t.TempDir(), "feature/x"), Branch: "feature/x"}
	g := must(gitx.New(repo, "main"))
	log := treeGitLog(t)
	before := must(g.Head())
	tree := must(judgedTree(ctx, g, before))
	testutil.Equal(t, strings.Count(string(must(os.ReadFile(log))), "archive\n"), 1)
	expected := t.TempDir()
	testutil.OK(t, g.ExportTree(before, expected))
	testutil.Equal(t, treeContents(t, tree), treeContents(t, expected))
	unchanged := must(os.Stat(filepath.Join(tree, "unchanged")))
	marker := filepath.Join(filepath.Dir(tree), ".commits", record.EncodeURIComponent(ctx.Branch))
	testutil.Equal(t, string(must(os.ReadFile(marker))), before+"\n")
	testutil.OK(t, os.WriteFile(log, nil, 0644))
	t.Setenv("DEVKIT_TEST_MARKER", marker)
	for _, path := range []string{"deleted/last", "to-folder", "to-file", "link", "from-link", "link-to-folder"} {
		testutil.OK(t, os.RemoveAll(filepath.Join(repo, path)))
	}
	for name, content := range map[string]string{
		"added space\n日本語": "\x00\xffnew\n\n", "modified": "\n after \x00\n", "to-folder/child": "child",
		"to-file": "file", "from-link": "regular", "link-to-folder/child": "inside",
	} {
		testutil.Put(t, repo, name, content)
	}
	testutil.OK(t, os.Remove(filepath.Join(repo, "to-link")))
	testutil.OK(t, os.Symlink("unchanged", filepath.Join(repo, "to-link")))
	testutil.OK(t, os.Symlink("to-folder/child", filepath.Join(repo, "link")))
	testutil.OK(t, os.Chmod(filepath.Join(repo, "executable"), 0755))
	testutil.OK(t, os.Chmod(filepath.Join(repo, "non-executable"), 0644))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "after")
	after := must(g.Head())
	testutil.Equal(t, must(judgedTree(ctx, g, after)), tree)
	if strings.Contains(string(must(os.ReadFile(log))), "archive\n") {
		t.Fatal("the second prepare exported the whole tree")
	}
	if !os.SameFile(unchanged, must(os.Stat(filepath.Join(tree, "unchanged")))) {
		t.Fatal("an unchanged file was replaced")
	}
	testutil.OK(t, g.ExportTree(after, expected))
	testutil.Equal(t, treeContents(t, tree), treeContents(t, expected))
	testutil.Equal(t, string(must(os.ReadFile(marker))), after+"\n")
	testutil.OK(t, os.WriteFile(log, nil, 0644))
	must(judgedTree(ctx, g, after))
	if strings.Contains(string(must(os.ReadFile(log))), "archive\n") {
		t.Fatal("an unchanged commit exported the whole tree")
	}
}

func TestJudgedTreeReexportsMissingStaleAndInterruptedMarkers(t *testing.T) {
	for _, scenario := range []string{"missing", "stale", "interrupted", "failed update", "missing tree"} {
		t.Run(scenario, func(t *testing.T) {
			repo := testutil.Repo(t)
			testutil.Put(t, repo, "file", "before")
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "before")
			ctx := record.ContextData{Cwd: repo, Dir: record.RecordDir(t.TempDir(), "feature/x"), Branch: "feature/x"}
			g := must(gitx.New(repo, "main"))
			log := treeGitLog(t)
			tree := must(judgedTree(ctx, g, must(g.Head())))
			marker := filepath.Join(filepath.Dir(tree), ".commits", record.EncodeURIComponent(ctx.Branch))
			testutil.Put(t, repo, "file", "after\x00\n")
			testutil.Git(t, repo, "commit", "-qam", "after")
			commit := must(g.Head())
			switch scenario {
			case "missing", "interrupted":
				testutil.OK(t, os.Remove(marker))
			case "stale":
				testutil.OK(t, os.WriteFile(marker, []byte(strings.Repeat("a", 40)), 0644))
			case "failed update":
				fail := filepath.Join(t.TempDir(), "fail")
				testutil.Put(t, filepath.Dir(fail), "fail", "once")
				t.Setenv("DEVKIT_TEST_FAIL_ONCE", fail)
			case "missing tree":
				testutil.OK(t, os.RemoveAll(tree))
			}
			if scenario != "missing tree" {
				testutil.Put(t, tree, "partial", "incomplete export")
			}
			testutil.OK(t, os.WriteFile(log, nil, 0644))
			must(judgedTree(ctx, g, commit))
			testutil.Equal(t, strings.Count(string(must(os.ReadFile(log))), "archive\n"), 1)
			expected := t.TempDir()
			testutil.OK(t, g.ExportTree(commit, expected))
			testutil.Equal(t, treeContents(t, tree), treeContents(t, expected))
			testutil.Equal(t, string(must(os.ReadFile(marker))), commit+"\n")
		})
	}
}

func TestJudgedTreePrunesOnlyDeletedBranches(t *testing.T) {
	repo := testutil.Repo(t)
	state := t.TempDir()
	g := must(gitx.New(repo, "main"))
	commit := must(g.Head())
	ctx := record.ContextData{Cwd: repo, Dir: record.RecordDir(state, "feature/x"), Branch: "feature/x"}
	for _, branch := range []string{"keep/a+!()'%日本語", "deleted/a", "feature/x", "judged/deleted"} {
		if branch != "feature/x" {
			testutil.Git(t, repo, "branch", branch)
		}
		other := ctx
		other.Branch = branch
		must(judgedTree(other, g, commit))
	}
	for _, branch := range []string{"deleted/a", "judged/deleted"} {
		testutil.Git(t, repo, "branch", "-D", branch)
	}
	ctx.Branch = "judged/deleted"
	current := must(judgedTree(ctx, g, commit))
	for _, branch := range []string{"keep/a+!()'%日本語", "feature/x", "judged/deleted"} {
		_, err := os.Stat(filepath.Join(state, "trees", record.EncodeURIComponent(branch)))
		testutil.OK(t, err)
	}
	_, err := os.Stat(filepath.Join(state, "trees", record.EncodeURIComponent("deleted/a")))
	testutil.Equal(t, os.IsNotExist(err), true)
	_, err = os.Stat(filepath.Join(state, "trees/.commits", record.EncodeURIComponent("deleted/a")))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.Equal(t, current, filepath.Join(state, "trees", record.EncodeURIComponent(ctx.Branch)))
}
