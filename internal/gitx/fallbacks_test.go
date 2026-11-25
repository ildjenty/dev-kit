package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestNewUsesConfiguredBaseWithoutGuessing(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "branch", "develop", "HEAD")
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  baseBranch: develop\n")
	g := testutil.Must(New(repo, ""))
	testutil.Equal(t, g.Refs, []string{"develop"})
	testutil.Equal(t, testutil.Must(New(repo, "main")).Refs, []string{"main"})
	testutil.OK(t, os.Remove(filepath.Join(repo, ".archon/config.yaml")))
	_, err := New(repo, "")
	testutil.Fails(t, err, ".archon/config.yaml")
	testutil.Fails(t, err, "baseBranch")
	testutil.Equal(t, testutil.Must(New(repo, "main")).Refs, []string{"main"})
	for _, data := range []string{"worktree: [", "worktree: {}", "worktree:\n  baseBranch: ''\n"} {
		testutil.Put(t, repo, ".archon/config.yaml", data)
		_, err := New(repo, "")
		testutil.Fails(t, err, ".archon/config.yaml")
	}
}

func TestBaseBranchPriorityAndOriginDefault(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "branch", "trunk", "main")
	testutil.Git(t, repo, "update-ref", "refs/remotes/origin/trunk", "main")
	testutil.Git(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	testutil.OK(t, os.MkdirAll(filepath.Join(repo, "nested"), 0755))
	for _, config := range []string{"missing", "worktree: {}", "worktree:\n  baseBranch: ''\n", "worktree:\n  baseBranch: develop\n"} {
		if config != "missing" {
			testutil.Put(t, repo, ".archon/config.yaml", config)
		}
		base, source, err := BaseBranch(filepath.Join(repo, "nested"), "")
		testutil.OK(t, err)
		if strings.Contains(config, "develop") {
			testutil.Equal(t, []string{base, source}, []string{"develop", ".archon/config.yaml worktree.baseBranch"})
		} else {
			testutil.Equal(t, []string{base, source}, []string{"trunk", "origin/HEAD"})
		}
		base, source, err = BaseBranch(repo, "explicit")
		testutil.OK(t, err)
		testutil.Equal(t, []string{base, source}, []string{"explicit", "BASE_BRANCH"})
	}
	testutil.Put(t, repo, ".archon/config.yaml", "worktree: [")
	_, _, err := BaseBranch(repo, "")
	testutil.Fails(t, err, ".archon/config.yaml")
	testutil.OK(t, os.Remove(filepath.Join(repo, ".archon/config.yaml")))
	testutil.Git(t, repo, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	_, _, err = BaseBranch(repo, "")
	for _, source := range []string{"BASE_BRANCH", ".archon/config.yaml", "worktree.baseBranch", "origin/HEAD"} {
		testutil.Fails(t, err, source)
	}
}

func TestNewRejectsBrokenRemoteReference(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".git/refs/remotes/origin/main", strings.Repeat("a", 40)+"\n")
	_, err := New(repo, "main")
	testutil.Fails(t, err, "origin/main")
}

func TestMergeBasesRejectsGitFailureAndPreservesUnrelatedHistory(t *testing.T) {
	repo := testutil.Repo(t)
	g := testutil.Must(New(repo, "main"))
	_, err := g.MergeBases("missing-commit")
	testutil.Fails(t, err, "missing-commit")
	testutil.Git(t, repo, "checkout", "--orphan", "unrelated")
	testutil.Git(t, repo, "commit", "--allow-empty", "-qm", "unrelated")
	testutil.Equal(t, testutil.Must(g.MergeBases("HEAD")), []string{})
}

func TestAncestorDistinguishesFalseFromFailure(t *testing.T) {
	repo := testutil.Repo(t)
	g := testutil.Must(New(repo, "main"))
	testutil.Equal(t, testutil.Must(g.IsAncestor("HEAD", "main")), false)
	_, err := g.IsAncestor("missing-commit", "HEAD")
	testutil.Fails(t, err, "missing-commit")
}

func TestPathExistsUsesExactCommittedPaths(t *testing.T) {
	repo := testutil.Repo(t)
	for _, name := range []string{"space name", "line\nbreak", "folder/file[1]", "-option"} {
		testutil.Put(t, repo, name, "source\n")
	}
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "source paths")
	g := testutil.Must(New(repo, "main"))
	for _, name := range []string{"space name", "line\nbreak", "folder", "folder/file[1]", "-option"} {
		testutil.Equal(t, testutil.Must(g.PathExists("HEAD", name)), true)
	}
	testutil.Equal(t, testutil.Must(g.PathExists("HEAD", "folder/file1")), false)
	testutil.Equal(t, testutil.Must(g.PathExists("main", "space name")), false)
	_, err := g.PathExists("missing-commit", "space name")
	testutil.Fails(t, err, "missing-commit")
}
