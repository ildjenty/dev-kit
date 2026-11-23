package checks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestEmptyGateAndSkips(t *testing.T) {
	testutil.Equal(t, Judge(1, nil, nil, nil), Gate{Preexisting: []string{}, Regressed: []string{}, Round: 1, Passed: true, Failing: []string{}, StuckChecks: []string{}, Skipped: []string{}})
	reason := "not applicable"
	gate := Judge(1, []Result{{Name: "optional", Status: "skip", Reason: &reason}}, nil, nil)
	testutil.Equal(t, gate.Passed, true)
	testutil.Equal(t, gate.Skipped, []string{"optional (not applicable)"})
}
func TestConsecutiveFailures(t *testing.T) {
	fail := func(name string) Result { return Result{Name: name, Status: "fail"} }
	testutil.Equal(t, Judge(1, []Result{fail("a")}, nil, nil).Stuck, false)
	testutil.Equal(t, Judge(2, []Result{fail("a")}, []Result{fail("a")}, nil).StuckChecks, []string{"a"})
	testutil.Equal(t, Judge(2, []Result{fail("a")}, []Result{fail("b")}, nil).Stuck, false)
	testutil.Equal(t, Judge(2, []Result{{Name: "a", Status: "pass"}}, []Result{fail("a")}, nil).Stuck, false)
	testutil.Equal(t, Judge(3, []Result{fail("a")}, []Result{{Name: "a", Status: "skip"}}, nil).Stuck, false)
	testutil.Equal(t, Judge(1, []Result{{Name: "a", Status: "error"}}, nil, nil).StuckChecks, []string{"a (environment failure)"})
	testutil.Equal(t, Judge(1, nil, nil, []string{"a"}).Failing, []string{"a (environment failure: no record)"})
}
func TestCheckExitCodesAndRetry(t *testing.T) {
	dir := t.TempDir()
	execute := func(name, command string) Result {
		return testutil.Must(RunCheck(config.Check{Name: name, Command: command, TimeoutSeconds: 5}, CheckOptions{Cwd: dir, Dir: dir, Round: 1}))
	}
	testutil.Equal(t, execute("pass", "printf success").Status, "pass")
	testutil.Equal(t, execute("fail", "printf failure >&2; exit 2").Status, "fail")
	skipped := execute("skip", "printf 'not applicable\n'; exit 77")
	testutil.Equal(t, skipped.Status, "skip")
	testutil.Equal(t, *skipped.Reason, "not applicable")
	testutil.Equal(t, execute("retry-pass", "if [ -f retried ]; then exit 0; fi; touch retried; exit 75").Status, "pass")
	result := execute("environment", "printf attempt >> attempts; exit 75")
	testutil.Equal(t, result.Status, "error")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(dir, "attempts")))), "attemptattempt")
	testutil.Contains(t, string(testutil.Must(os.ReadFile(result.Log))), "retried after an environment failure")
	raw := testutil.Must(os.ReadFile(filepath.Join(dir, "1.jsonl")))
	records := []Result{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var r Result
		testutil.OK(t, json.Unmarshal(line, &r))
		records = append(records, r)
	}
	testutil.Equal(t, len(records), 5)
	testutil.Contains(t, string(testutil.Must(os.ReadFile(records[1].Log))), "failure")
}
func TestCheckTimeout(t *testing.T) {
	dir := t.TempDir()
	r := testutil.Must(RunCheck(config.Check{Name: "slow", Command: "sleep 10", TimeoutSeconds: 0.05}, CheckOptions{Cwd: dir, Dir: dir, Round: 1}))
	testutil.Equal(t, r.Status, "error")
	testutil.Contains(t, string(testutil.Must(os.ReadFile(r.Log))), "Timed out")
}
func TestSnapshotsAndRounds(t *testing.T) {
	repo := testutil.Repo(t)
	artifacts := t.TempDir()
	testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"always","command":"exit 1","timeout_seconds":2},{"name":"source","command":"exit 0","paths":["src/**"],"timeout_seconds":2},{"name":"docs","command":"exit 1","paths":["docs/**"],"timeout_seconds":2}]}`)
	testutil.Git(t, repo, "add", ".devkit")
	testutil.Git(t, repo, "commit", "-qm", "settings")
	testutil.Put(t, repo, "src/new.txt", "untracked content\n")
	before := testutil.Git(t, repo, "status", "--porcelain")
	index := testutil.Must(os.ReadFile(filepath.Join(repo, ".git/index")))
	o := Options{Cwd: repo, ArtifactsDir: artifacts}
	first := testutil.Must(RunChecks(o))
	testutil.Equal(t, first.Round, 1)
	testutil.Equal(t, first.Passed, false)
	testutil.Equal(t, first.Stuck, false)
	testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), before)
	testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(repo, ".git/index"))), index)
	patch := testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/1-diff.patch")))
	testutil.Contains(t, string(patch), "untracked content")
	testutil.Equal(t, testutil.Must(os.ReadFile(filepath.Join(artifacts, "diff.patch"))), patch)
	records := testutil.Must(os.ReadFile(first.Results))
	var names []string
	for _, line := range bytes.Split(bytes.TrimSpace(records), []byte("\n")) {
		var r Result
		testutil.OK(t, json.Unmarshal(line, &r))
		names = append(names, r.Name)
	}
	testutil.Equal(t, names, []string{"always", "source"})
	second := testutil.Must(RunChecks(o))
	testutil.Equal(t, second.Round, 2)
	testutil.Equal(t, second.StuckChecks, []string{})
	testutil.Equal(t, second.Preexisting, []string{"always"})
	o.Only = []string{"source"}
	third := testutil.Must(RunChecks(o))
	testutil.Equal(t, third.Passed, true)
	testutil.Equal(t, third.Stuck, false)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/round")))), "3")
	o.Only = []string{"unknown"}
	_, err := RunChecks(o)
	testutil.Fails(t, err, "configured checks")
}
func TestGitRenameDirectories(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "old/file.txt", "unchanged\n")
	testutil.Git(t, repo, "add", "old")
	testutil.Git(t, repo, "commit", "-qm", "old file")
	from := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.OK(t, os.Mkdir(filepath.Join(repo, "new"), 0755))
	testutil.OK(t, os.Rename(filepath.Join(repo, "old/file.txt"), filepath.Join(repo, "new/file.txt")))
	testutil.OK(t, os.Remove(filepath.Join(repo, "old")))
	g := testutil.Must(gitx.New(repo, ""))
	changes := testutil.Must(g.WorkingChanges(from))
	testutil.Equal(t, changes.Files, []gitx.File{{Status: "R100", OldPath: "old/file.txt", Path: "new/file.txt"}})
	testutil.Equal(t, changes.AddedDirectories, []string{"new"})
	testutil.Equal(t, changes.RemovedDirectories, []string{"old"})
}
func TestCLIEmptyChecks(t *testing.T) {
	repo := testutil.Repo(t)
	artifacts := t.TempDir()
	var stdout, stderr bytes.Buffer
	r := cli.New("test", Commands())
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) {
		if key == "ARTIFACTS_DIR" {
			return artifacts, true
		}
		if key == "BASE_BRANCH" {
			return "main", true
		}
		return "", false
	}}
	testutil.Equal(t, r.Execute([]string{"checks"}, env), 0)
	var gate Gate
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &gate))
	testutil.Equal(t, gate.Passed, true)
	testutil.Equal(t, gate.Failing, []string{})
	testutil.Equal(t, string(testutil.Must(os.ReadFile(gate.Results))), "")
	_, err := os.Stat(filepath.Join(artifacts, "checks/1-diff.patch"))
	testutil.OK(t, err)
}

func TestCLIRunsFromASessionWithoutArtifactsDir(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"always","command":"exit 0","timeout_seconds":30}]}`)
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) {
		if key == "BASE_BRANCH" {
			return "main", true
		}
		return "", false
	}}
	testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"checks"}, env), 0)
	var gate Gate
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &gate))
	testutil.Equal(t, gate.Passed, true)
	testutil.Contains(t, string(testutil.Must(os.ReadFile(gate.Results))), `"name":"always"`)
	testutil.OK(t, os.RemoveAll(filepath.Dir(filepath.Dir(gate.Results))))
}

func TestBaseChecksStillRunWhenRemovedOrReplaced(t *testing.T) {
	for _, current := range []string{`{"checks":[]}`, `{"checks":[{"name":"required","command":"printf current; exit 0","timeout_seconds":2}]}`} {
		repo := testutil.Repo(t)
		testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"required","command":"printf base; exit 1","timeout_seconds":2}]}`)
		testutil.Git(t, repo, "add", ".")
		testutil.Git(t, repo, "commit", "-qm", "base check")
		testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
		testutil.Put(t, repo, ".devkit/config.json", current)
		gate := testutil.Must(RunChecks(Options{Cwd: repo, ArtifactsDir: t.TempDir()}))
		testutil.Equal(t, gate.Passed, false)
		testutil.Equal(t, gate.Failing, []string{"required"})
		lines := bytes.Split(bytes.TrimSpace(testutil.Must(os.ReadFile(gate.Results))), []byte("\n"))
		first := Result{}
		testutil.OK(t, json.Unmarshal(lines[0], &first))
		testutil.Contains(t, string(testutil.Must(os.ReadFile(first.Log))), "base")
		if current != `{"checks":[]}` {
			testutil.Equal(t, len(lines), 2)
			second := Result{}
			testutil.OK(t, json.Unmarshal(lines[1], &second))
			if second.Log == first.Log {
				t.Fatal("current check overwrote base check log")
			}
			testutil.Equal(t, second.Status, "pass")
		}
	}
}

func TestF18DesignDescribesTimeoutWithoutRetry(t *testing.T) {
	data := testutil.Must(os.ReadFile("../../docs/design.md"))
	text := string(data)
	if strings.Contains(text, "75とタイムアウトは環境エラーとして一度再実行") {
		t.Fatal("design promises a timeout retry")
	}
	testutil.Contains(t, text, "75だけ一度再実行")
	testutil.Contains(t, text, "タイムアウトは環境エラー")
}
