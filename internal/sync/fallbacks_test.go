package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func failGitCommand(t *testing.T, match string) string {
	t.Helper()
	realGit := must(exec.LookPath("git"))
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "attempts")
	script := "#!/bin/sh\nif [ \"$*\" = '" + match + "' ]; then\n  printf 'attempt\\n' >> '" + log + "'\n  printf 'injected Git failure\\n' >&2\n  exit 1\nfi\nexec '" + realGit + "' \"$@\"\n"
	testutil.Put(t, bin, "git", script)
	testutil.OK(t, os.Chmod(filepath.Join(bin, "git"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func TestContinueReturnsFailureWithoutRetryOrEmptyConflict(t *testing.T) {
	for _, action := range []string{"proceed", "afterRebase"} {
		t.Run(action, func(t *testing.T) {
			repo, g, _ := conflictRepo(t)
			s := must(start(g, "main"))
			testutil.Put(t, repo, "file.txt", "base moved\nbranch\n")
			if action == "afterRebase" {
				testutil.Git(t, repo, "add", "-A")
			}
			log := failGitCommand(t, "rebase --continue")
			g.env = os.Environ()
			var err error
			if action == "proceed" {
				_, err = proceed(g, s, Resolution{Resolved: true}, reader(repo))
			} else {
				_, err = afterRebase(g, s)
			}
			testutil.Fails(t, err, "injected Git failure")
			testutil.Equal(t, strings.Count(string(must(os.ReadFile(log))), "attempt"), 1)
		})
	}
}

func TestStartRejectsRemoteDiscoveryFailure(t *testing.T) {
	repo := testutil.Repo(t)
	failGitCommand(t, "remote get-url origin")
	testutil.Git(t, repo, "remote", "add", "origin", "unused")
	_, err := start(git{repo, os.Environ()}, "main")
	testutil.Fails(t, err, "injected Git failure")
}

func TestConflictRejectsMissingReplayCommit(t *testing.T) {
	repo, vars := syncConflictCLI(t, []string{"file.txt"})
	testutil.Equal(t, runCLI(t, repo, vars, "start")["state"], "conflict")
	path := testutil.Git(t, repo, "rev-parse", "--git-path", "REBASE_HEAD")
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	testutil.OK(t, os.Remove(path))
	vars["INPUTS_RESOLVED"] = `{"resolved":true,"contradiction":""}`
	syncRejectedCLI(t, repo, vars, "continue", "REBASE_HEAD")
}

func TestSyncCLIUsesConfiguredBaseWithoutEnvironment(t *testing.T) {
	repo, vars := syncConflictCLI(t, []string{"file.txt"})
	testutil.Git(t, repo, "branch", "develop", "main")
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  baseBranch: develop\n")
	testutil.Git(t, repo, "add", ".archon/config.yaml")
	testutil.Git(t, repo, "commit", "-qm", "base configuration")
	delete(vars, "BASE_BRANCH")
	out := runCLI(t, repo, vars, "start")
	testutil.Equal(t, out["ref"], "develop")
	testutil.Equal(t, out["state"], "conflict")
}
