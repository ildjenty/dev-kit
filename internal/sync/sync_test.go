package sync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func must[T any](v T, err error) T { return testutil.Must(v, err) }
func conflictRepo(t *testing.T) (string, git, string) {
	t.Helper()
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "config", "user.name", "t")
	testutil.Git(t, repo, "config", "user.email", "t@example.invalid")
	testutil.Put(t, repo, "file.txt", "base\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base file")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Put(t, repo, "file.txt", "branch\n")
	testutil.Git(t, repo, "commit", "-qam", "branch change")
	previous := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Git(t, repo, "checkout", "-q", "main")
	testutil.Put(t, repo, "file.txt", "base moved\n")
	testutil.Git(t, repo, "commit", "-qam", "base change")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	return repo, git{repo, os.Environ()}, previous
}
func reader(repo string) func(string) (*string, error) {
	return func(f string) (*string, error) {
		b, err := os.ReadFile(filepath.Join(repo, f))
		if os.IsNotExist(err) {
			return nil, nil
		}
		s := string(b)
		return &s, err
	}
}
func TestSyncDoesNotRebaseDirtyOrCurrentBranch(t *testing.T) {
	repo := testutil.Repo(t)
	g := git{repo, os.Environ()}
	testutil.Equal(t, must(start(g, "main")).State, "current")
	testutil.Put(t, repo, "dirty", "change")
	testutil.Equal(t, must(start(g, "main")).State, "refused")
}
func TestSyncRebasesDisjointChangesWithoutResolverRound(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "config", "user.name", "t")
	testutil.Git(t, repo, "config", "user.email", "t@example.invalid")
	testutil.Put(t, repo, "branch.txt", "branch")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "branch file")
	testutil.Git(t, repo, "checkout", "-q", "main")
	testutil.Put(t, repo, "base.txt", "base")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base file")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	s := must(start(git{repo, os.Environ()}, "main"))
	testutil.Equal(t, s.State, "done")
	testutil.Equal(t, s.Rounds, 0)
	for file, want := range map[string]string{"branch.txt": "branch", "base.txt": "base"} {
		testutil.Equal(t, string(must(os.ReadFile(filepath.Join(repo, file)))), want)
	}
}
func TestConflictsPreserveBothChangesAndContinueRebase(t *testing.T) {
	repo, g, _ := conflictRepo(t)
	s := must(start(g, "main"))
	testutil.Equal(t, s.State, "conflict")
	testutil.Equal(t, s.Conflicts[len(s.Conflicts)-1], []string{"file.txt"})
	testutil.Put(t, repo, "file.txt", "base moved\nbranch\n")
	s = must(proceed(g, s, Resolution{Resolved: true}, reader(repo)))
	testutil.Equal(t, s.State, "done")
	testutil.Equal(t, s.Rounds, 1)
	testutil.Equal(t, must(g.rebasing()), false)
	testutil.Equal(t, must(g.run("status", "--porcelain", "--untracked-files=all")), "")
}
func TestContradictionsAbortAndRestoreExactOriginalCommit(t *testing.T) {
	repo, g, previous := conflictRepo(t)
	s := must(start(g, "main"))
	s = must(proceed(g, s, Resolution{Contradiction: "both cannot hold"}, reader(repo)))
	s = must(settle(g, s))
	testutil.Equal(t, s.State, "contradiction")
	testutil.Equal(t, must(g.run("rev-parse", "HEAD")), previous)
	testutil.Equal(t, must(g.run("status", "--porcelain", "--untracked-files=all")), "")
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(repo, "file.txt")))), "branch\n")
}
func TestRemainingMarkersConsumeFiveRoundsAndAbort(t *testing.T) {
	repo, g, previous := conflictRepo(t)
	s := must(start(g, "main"))
	for range 5 {
		s = must(proceed(g, s, Resolution{Resolved: true}, reader(repo)))
	}
	testutil.Equal(t, s.State, "conflict")
	testutil.Equal(t, s.Rounds, 5)
	testutil.Equal(t, must(settle(g, s)).State, "unresolved")
	testutil.Equal(t, must(g.run("rev-parse", "HEAD")), previous)
	testutil.Equal(t, must(g.run("status", "--porcelain", "--untracked-files=all")), "")
}
func TestMarkerDetectionIncludesDiff3ButNotOrdinarySeparators(t *testing.T) {
	testutil.Equal(t, hasMarkers("<<<<<<< HEAD\na\n=======\nb\n>>>>>>> branch"), true)
	testutil.Equal(t, hasMarkers("||||||| parent\nbase"), true)
	testutil.Equal(t, hasMarkers("======== heading"), false)
}
func TestOutcomesDistinguishRefusalExhaustionContradictionChecksAndPass(t *testing.T) {
	s := State{State: "done", Rounds: 1, Conflicts: [][]string{{"file.txt"}}, Contradiction: "incompatible"}
	testutil.Equal(t, must(outcomeOf(s, &Gate{Passed: true}))["outcome"], "pass")
	s.State = "current"
	s.Rounds = 0
	testutil.Equal(t, must(outcomeOf(s, &Gate{Passed: true}))["outcome"], "pass")
	s.State = "done"
	testutil.Equal(t, must(outcomeOf(s, &Gate{Failing: []string{"verify"}}))["outcome"], "stop")
	for _, ended := range []string{"refused", "contradiction", "unresolved"} {
		s.State = ended
		d := must(outcomeOf(s, nil))
		testutil.Equal(t, d["outcome"], "stop")
		if ended == "unresolved" {
			testutil.Equal(t, d["loops"].([]record.Document)[0]["ended"], "max_iterations")
		}
	}
}
func runCLI(t *testing.T, repo string, vars map[string]string, args ...string) record.Document {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, Environ: os.Environ()}
	if code := cli.New("test", Commands()).Execute(append([]string{"sync"}, args...), env); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var out record.Document
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &out))
	return out
}
func TestFinishRecordsConflictRoundsAndRequiresJudgingOnlyAfterConflict(t *testing.T) {
	repo, g, _ := conflictRepo(t)
	s := must(start(g, "main"))
	testutil.Put(t, repo, "file.txt", "base moved\nbranch\n")
	s = must(proceed(g, s, Resolution{Resolved: true}, reader(repo)))
	artifacts := t.TempDir()
	testutil.OK(t, os.MkdirAll(filepath.Join(artifacts, "sync"), 0755))
	testutil.OK(t, saveJSON(filepath.Join(artifacts, "sync/state.json"), s))
	state := t.TempDir()
	dir := record.RecordDir(state, "feature/x")
	must(record.Append(dir, "request", record.Document{"run_id": "sync-1", "recorded_at": "t", "owner_words": "", "supplement": "", "criteria_commit": testutil.Git(t, repo, "rev-parse", "main")}))
	vars := map[string]string{"WORKFLOW_ID": "sync-1", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main", "INPUTS_GATE": `{"passed":true,"failing":[]}`}
	addRun := func(id string) {
		must(record.Append(dir, "run", record.Document{"run_id": id, "workflow": "sync", "started_at": "t", "start": "other", "counted": false, "answers": []any{}}))
	}
	addRun("sync-1")
	out := runCLI(t, repo, vars, "finish")
	testutil.Equal(t, out["outcome"], "pass")
	testutil.Equal(t, out["needs_judge"], true)
	testutil.Equal(t, out["conflict_rounds"], 1)
	s.Rounds = 0
	s.Conflicts = [][]string{}
	testutil.OK(t, saveJSON(filepath.Join(artifacts, "sync/state.json"), s))
	addRun("sync-2")
	vars["WORKFLOW_ID"] = "sync-2"
	testutil.Equal(t, runCLI(t, repo, vars, "finish")["needs_judge"], false)
}

func TestConflictResolverReceivesMappedMaterialsFromTheRequestsFixedStandards(t *testing.T) {
	repo, _, _ := conflictRepo(t)
	testutil.Git(t, repo, "checkout", "-q", "main")
	testutil.Standards(t, repo, "rules")
	testutil.Put(t, repo, ".devkit/config.json", `{"language":"de","standards":"rules"}`)
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "Install standards")
	fixed := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	testutil.Git(t, repo, "cherry-pick", fixed)
	authority := string(must(os.ReadFile(filepath.Join(repo, "rules/authority.md"))))
	testutil.Put(t, repo, "rules/authority.md", authority+"\nWorking-copy-only change.\n")
	testutil.Git(t, repo, "commit", "-qam", "Update working standards")
	state, artifacts := t.TempDir(), t.TempDir()
	dir := record.RecordDir(state, "feature/x")
	started := must(record.Start(dir, "sync", record.StartOptions{RunID: "sync-1", BaseCommit: fixed, Commit: testutil.Git(t, repo, "rev-parse", "HEAD")}))
	testutil.Equal(t, started.Stopped, false)
	vars := map[string]string{"WORKFLOW_ID": "sync-1", "STATE_DIR": state, "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
	out := runCLI(t, repo, vars, "start")
	testutil.Equal(t, out["state"], "conflict")
	var input record.Document
	testutil.OK(t, readJSON(filepath.Join(artifacts, "sync/conflict.json"), &input))
	testutil.Equal(t, input["language"], "de")
	materials, ok := input["writer_materials"].(map[string]any)
	if !ok {
		t.Fatal("resolver has no mapped materials")
	}
	path := filepath.Join(artifacts, "criteria/rules/authority.md")
	testutil.Equal(t, materials["authority"], path)
	testutil.Equal(t, string(must(os.ReadFile(path))), authority)
	testutil.Equal(t, materials["common"], filepath.Join(artifacts, "criteria/rules/review/README.md"))
}
