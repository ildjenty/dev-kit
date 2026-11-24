package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func syncConflictCLI(t *testing.T, names []string) (string, map[string]string) {
	t.Helper()
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "config", "user.name", "t")
	testutil.Git(t, repo, "config", "user.email", "t@example.invalid")
	testutil.Git(t, repo, "config", "core.quotePath", "true")
	testutil.Standards(t, repo, ".devkit/standards")
	for _, name := range names {
		testutil.Put(t, repo, name, "base\n")
	}
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base files")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	fixed := testutil.Git(t, repo, "rev-parse", "HEAD")
	for _, name := range names {
		testutil.Put(t, repo, name, "branch\n")
	}
	testutil.Git(t, repo, "commit", "-qam", "branch change")
	testutil.Git(t, repo, "checkout", "-q", "main")
	for _, name := range names {
		testutil.Put(t, repo, name, "base moved\n")
	}
	testutil.Git(t, repo, "commit", "-qam", "base change")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	state, artifacts := t.TempDir(), t.TempDir()
	dir := record.RecordDir(state, "feature/x")
	must(record.Append(dir, "request", record.Document{"run_id": "sync1", "recorded_at": "t", "owner_words": "", "supplement": "", "criteria_commit": fixed}))
	must(record.Append(dir, "run", record.Document{"run_id": "sync1", "workflow": "sync", "started_at": "t", "start": "other", "counted": false, "answers": []any{}}))
	return repo, map[string]string{"WORKFLOW_ID": "sync1", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main", "INPUTS_GATE": `{"passed":true,"failing":[]}`}
}

func syncRejectedCLI(t *testing.T, repo string, vars map[string]string, command, reason string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	if code := cli.New("test", Commands()).Execute([]string{"sync", command}, env); code == 0 {
		t.Fatalf("sync %s unexpectedly succeeded: %s", command, stdout.String())
	}
	testutil.Contains(t, stderr.String(), reason)
}

func TestSyncStartRejectsPreRebaseHookFailureWithoutPassingResult(t *testing.T) {
	repo, vars := syncConflictCLI(t, []string{"file.txt"})
	previous := testutil.Git(t, repo, "rev-parse", "HEAD")
	index := testutil.Git(t, repo, "write-tree")
	hooks := t.TempDir()
	testutil.OK(t, os.Symlink("/usr/bin/false", filepath.Join(hooks, "pre-rebase")))
	testutil.Git(t, repo, "config", "core.hooksPath", hooks)
	syncRejectedCLI(t, repo, vars, "start", "pre-rebase")
	testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), previous)
	testutil.Equal(t, testutil.Git(t, repo, "write-tree"), index)
	testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
	testutil.Equal(t, must((git{repo, os.Environ()}).rebasing()), false)
	_, err := os.Stat(filepath.Join(vars["ARTIFACTS_DIR"], "sync/state.json"))
	testutil.Equal(t, os.IsNotExist(err), true)
	syncRejectedCLI(t, repo, vars, "finish", "state.json")
	testutil.Equal(t, len(must(record.List(record.RecordDir(vars["STATE_DIR"], "feature/x"), "result"))), 0)
}

func TestAfterRebaseRequiresOntoInHeadBeforeDone(t *testing.T) {
	repo, g, previous := conflictRepo(t)
	onto := testutil.Git(t, repo, "rev-parse", "main")
	s, err := afterRebase(g, State{Previous: previous, Onto: &onto, Conflicts: [][]string{}})
	testutil.Fails(t, err, "did not incorporate")
	if s.State == "done" {
		t.Fatal("unincorporated base was marked done")
	}
	_, err = outcomeOf(s, &Gate{Passed: true})
	if err == nil {
		t.Fatal("unincorporated base passed the gate")
	}
	testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), previous)
}

func TestSyncSpecialConflictPathsKeepMarkersUnstagedUntilResolved(t *testing.T) {
	names := []string{"日本語.txt", "line\nbreak.txt", " spaced name.txt "}
	repo, vars := syncConflictCLI(t, names)
	out := runCLI(t, repo, vars, "start")
	testutil.Equal(t, out["state"], "conflict")
	files := must((git{repo, os.Environ()}).unmerged())
	testutil.Equal(t, len(files), len(names))
	for _, name := range names {
		found := false
		for _, file := range files {
			found = found || file == name
		}
		testutil.Equal(t, found, true)
	}
	var s State
	testutil.OK(t, readJSON(filepath.Join(vars["ARTIFACTS_DIR"], "sync/state.json"), &s))
	testutil.Equal(t, s.Conflicts[0], files)
	var conflict record.Document
	testutil.OK(t, readJSON(filepath.Join(vars["ARTIFACTS_DIR"], "sync/conflict.json"), &conflict))
	testutil.Equal(t, conflict["files"], files)
	head := testutil.Git(t, repo, "rev-parse", "HEAD")
	index := testutil.Git(t, repo, "ls-files", "--stage", "-z")
	vars["INPUTS_RESOLVED"] = `{"resolved":true,"contradiction":""}`
	out = runCLI(t, repo, vars, "continue")
	testutil.Equal(t, out["state"], "conflict")
	testutil.Equal(t, out["files"], files)
	testutil.Equal(t, testutil.Git(t, repo, "rev-parse", "HEAD"), head)
	testutil.Equal(t, testutil.Git(t, repo, "ls-files", "--stage", "-z"), index)
	syncRejectedCLI(t, repo, vars, "finish", "HEAD is detached")
	testutil.Equal(t, len(must(record.List(record.RecordDir(vars["STATE_DIR"], "feature/x"), "result"))), 0)
	for _, name := range names {
		testutil.Equal(t, hasMarkers(string(must(os.ReadFile(filepath.Join(repo, name))))), true)
		testutil.Put(t, repo, name, "base moved\nbranch\n")
	}
	out = runCLI(t, repo, vars, "continue")
	testutil.Equal(t, out["state"], "done")
	testutil.Equal(t, must((git{repo, os.Environ()}).rebasing()), false)
	testutil.Git(t, repo, "merge-base", "--is-ancestor", testutil.Git(t, repo, "rev-parse", "main"), "HEAD")
	for _, name := range names {
		testutil.Equal(t, testutil.Git(t, repo, "show", "HEAD:"+name), "base moved\nbranch")
	}
	testutil.Equal(t, runCLI(t, repo, vars, "finish")["outcome"], "pass")
}
