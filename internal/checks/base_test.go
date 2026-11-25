package checks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func baseRepository(t *testing.T, checks []config.Check) string {
	t.Helper()
	repo := testutil.Repo(t)
	data := testutil.Must(json.Marshal(map[string]any{"checks": checks}))
	testutil.Put(t, repo, ".devkit/config.json", string(data))
	testutil.Put(t, repo, "value", "base\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base settings")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Put(t, repo, "value", "change\n")
	return repo
}

func readResults(t *testing.T, path string) []Result {
	t.Helper()
	results := []Result{}
	for _, line := range strings.Split(strings.TrimSpace(string(testutil.Must(os.ReadFile(path)))), "\n") {
		if line == "" {
			continue
		}
		var result Result
		testutil.OK(t, json.Unmarshal([]byte(line), &result))
		results = append(results, result)
	}
	return results
}

func TestBaseComparisonRunsOnlyFailuresAtDetachedMergeBase(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "attempts")
	checks := []config.Check{
		{Name: "existing", Command: "printf existing >> '" + counter + "'; exit 1", TimeoutSeconds: 2},
		{Name: "regression", Command: "printf regression >> '" + counter + "'; test \"$(cat value)\" = base", TimeoutSeconds: 2},
		{Name: "passing", Command: "printf passing >> '" + counter + "'", TimeoutSeconds: 2},
	}
	repo := baseRepository(t, checks)
	testutil.Git(t, repo, "add", "value")
	testutil.Git(t, repo, "commit", "-qm", "change")
	testutil.Git(t, repo, "checkout", "-q", "main")
	testutil.Put(t, repo, "value", "newer base tip\n")
	testutil.Git(t, repo, "commit", "-qam", "advance base")
	testutil.Git(t, repo, "checkout", "-q", "feature/x")
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  postCreate:\n    - test -z \"$(git branch --show-current)\" && test \"$(cat value)\" = base && touch prepared\n  postCreateTimeout: 1000\n")
	before := testutil.Git(t, repo, "worktree", "list", "--porcelain")
	options := Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main"}
	gate := testutil.Must(RunChecks(options))
	testutil.Equal(t, gate.Preexisting, []string{"existing"})
	testutil.Equal(t, gate.Regressed, []string{"regression"})
	testutil.Equal(t, gate.OnlyPreexisting(), false)
	results := readResults(t, gate.Results)
	testutil.Equal(t, results[0].Base, "failed")
	testutil.Equal(t, results[1].Base, "passed")
	testutil.Equal(t, results[2].Base, "")
	for _, result := range results[:2] {
		_, err := os.Stat(result.BaseLog)
		testutil.OK(t, err)
	}
	attempts := string(testutil.Must(os.ReadFile(counter)))
	for name, count := range map[string]int{"existing": 2, "regression": 2, "passing": 1} {
		testutil.Equal(t, strings.Count(attempts, name), count)
	}
	testutil.Equal(t, testutil.Git(t, repo, "worktree", "list", "--porcelain"), before)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(repo, "value")))), "change\n")
	gate = testutil.Must(RunChecks(options))
	testutil.Equal(t, gate.StuckChecks, []string{"regression"})
}

func TestBaseOnRepeatComparesOnlyChecksThatFailedInThePreviousRound(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "prepared")
	checks := []config.Check{{Name: "existing", Command: "exit 1", TimeoutSeconds: 2}}
	repo := baseRepository(t, checks)
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  postCreate:\n    - printf x >> '"+marker+"'\n")
	options := Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main", BaseOnRepeat: true}
	first := testutil.Must(RunChecks(options))
	testutil.Equal(t, readResults(t, first.Results)[0].Base, "not_compared")
	testutil.Equal(t, first.OnlyPreexisting(), false)
	_, err := os.Stat(marker)
	testutil.Equal(t, os.IsNotExist(err), true)
	second := testutil.Must(RunChecks(options))
	testutil.Equal(t, readResults(t, second.Results)[0].Base, "failed")
	testutil.Equal(t, second.OnlyPreexisting(), true)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(marker))), "x")
}

func TestBasePreparationFailureAndTimeoutAreUnavailableAndCleaned(t *testing.T) {
	for _, command := range []string{"exit 1", "exit 75", "exit 77", "sleep 10"} {
		t.Run(command, func(t *testing.T) {
			repo := baseRepository(t, []config.Check{{Name: "check", Command: "exit 1", TimeoutSeconds: 2}})
			path := filepath.Join(t.TempDir(), "worktree-path")
			testutil.Put(t, repo, ".archon/config.yaml", fmt.Sprintf("worktree:\n  postCreate:\n    - pwd > '%s'\n    - %s\n    - touch should-not-run\n  postCreateTimeout: 200\n", path, command))
			before := testutil.Git(t, repo, "worktree", "list", "--porcelain")
			gate := testutil.Must(RunChecks(Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main"}))
			testutil.Equal(t, readResults(t, gate.Results)[0].Base, "unavailable")
			testutil.Equal(t, gate.Preexisting, []string{})
			testutil.Equal(t, gate.Regressed, []string{})
			worktree := strings.TrimSpace(string(testutil.Must(os.ReadFile(path))))
			_, err := os.Stat(worktree)
			testutil.Equal(t, os.IsNotExist(err), true)
			testutil.Equal(t, testutil.Git(t, repo, "worktree", "list", "--porcelain"), before)
			_, err = os.Stat(filepath.Join(filepath.Dir(gate.Results), "1-base/1"))
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestBasePreparationCommandsRunInOrderWithEnvironment(t *testing.T) {
	repo := baseRepository(t, []config.Check{{Name: "check", Command: "test -f ready && exit 1; exit 75", TimeoutSeconds: 2}})
	testutil.Put(t, repo, "ready", "")
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  postCreate:\n    - test \"$PREPARE\" = value && touch first\n    - test -f first && touch ready\n")
	gate := testutil.Must(RunChecks(Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main", Env: append(os.Environ(), "PREPARE=value")}))
	testutil.Equal(t, gate.OnlyPreexisting(), true)
}

func TestBaseEnvironmentSkipAndTimeoutAreUnavailable(t *testing.T) {
	for _, command := range []string{"exit 75", "exit 77", "sleep 10"} {
		t.Run(command, func(t *testing.T) {
			repo := baseRepository(t, []config.Check{{Name: "check", Command: "if [ \"$(cat value)\" = change ]; then exit 1; fi; " + command, TimeoutSeconds: 0.2}})
			gate := testutil.Must(RunChecks(Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main"}))
			testutil.Equal(t, readResults(t, gate.Results)[0].Base, "unavailable")
			testutil.Equal(t, gate.OnlyPreexisting(), false)
		})
	}
}

func TestEnvironmentFailuresAndNoBaseNeverPrepareComparison(t *testing.T) {
	for _, mode := range []string{"environment", "passing", "no-base"} {
		t.Run(mode, func(t *testing.T) {
			code := "1"
			if mode == "environment" {
				code = "75"
			}
			if mode == "passing" {
				code = "0"
			}
			repo := baseRepository(t, []config.Check{{Name: "check", Command: "exit " + code, TimeoutSeconds: 2}})
			marker := filepath.Join(t.TempDir(), "prepared")
			testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  postCreate:\n    - touch '"+marker+"'\n")
			var out, stderr bytes.Buffer
			artifacts := t.TempDir()
			env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) {
				if key == "ARTIFACTS_DIR" {
					return artifacts, true
				}
				if key == "BASE_BRANCH" {
					return "main", true
				}
				return "", false
			}}
			args := []string{"checks"}
			if mode == "no-base" {
				args = append(args, "--no-base", "--only", "check")
			}
			testutil.Equal(t, cli.New("test", Commands()).Execute(args, env), 0)
			var gate Gate
			testutil.OK(t, json.Unmarshal(out.Bytes(), &gate))
			_, err := os.Stat(marker)
			testutil.Equal(t, os.IsNotExist(err), true)
			result := readResults(t, gate.Results)[0]
			if mode == "environment" {
				testutil.Equal(t, result.Status, "error")
				testutil.Equal(t, result.Base, "")
			}
			if mode == "no-base" {
				testutil.Equal(t, result.Base, "unavailable")
			}
			testutil.Equal(t, gate.Preexisting, []string{})
		})
	}
}

func TestPreexistingGateStopsWithoutStuckAndMixedFailuresDoNot(t *testing.T) {
	previous := []Result{{Name: "check", Status: "fail"}}
	gate := Judge(2, []Result{{Name: "check", Status: "fail", Base: "failed"}}, previous, nil)
	testutil.Equal(t, gate.Stuck, false)
	testutil.Equal(t, gate.OnlyPreexisting(), true)
	for _, result := range []Result{{Name: "other", Status: "fail", Base: "passed"}, {Name: "other", Status: "fail", Base: "unavailable"}, {Name: "other", Status: "error"}, {Name: "check", Status: "fail", Base: "unavailable"}} {
		mixed := Judge(2, []Result{{Name: "check", Status: "fail", Base: "failed"}, result}, previous, nil)
		testutil.Equal(t, mixed.OnlyPreexisting(), false)
	}
}

func TestReplacedCheckCommandsKeepTheirOwnBaseResultAndLogs(t *testing.T) {
	repo := baseRepository(t, []config.Check{{Name: "check", Command: "exit 1", TimeoutSeconds: 2}})
	testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"check","command":"test \"$(cat value)\" = base","timeout_seconds":2}]}`)
	gate := testutil.Must(RunChecks(Options{Cwd: repo, ArtifactsDir: t.TempDir(), BaseBranch: "main"}))
	testutil.Equal(t, gate.Failing, []string{"check", "check"})
	testutil.Equal(t, gate.Preexisting, []string{"check"})
	testutil.Equal(t, gate.Regressed, []string{"check"})
	testutil.Equal(t, gate.OnlyPreexisting(), false)
	results := readResults(t, gate.Results)
	testutil.Equal(t, results[0].Base, "failed")
	testutil.Equal(t, results[1].Base, "passed")
	if results[0].BaseLog == results[1].BaseLog || results[0].Log == results[1].Log {
		t.Fatal("check variants share a log")
	}
}
