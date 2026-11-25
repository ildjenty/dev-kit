package checks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestRunChecksRejectsLostHistoryWithoutOverwriting(t *testing.T) {
	for _, input := range []string{"", " \n", "1", "missing-index"} {
		t.Run(input, func(t *testing.T) {
			repo, artifacts := testutil.Repo(t), t.TempDir()
			testutil.Put(t, artifacts, "checks/1.jsonl", "preserve history\n")
			if input != "missing-index" {
				testutil.Put(t, artifacts, "checks/round", input)
			}
			if input == "1" {
				testutil.OK(t, os.Remove(filepath.Join(artifacts, "checks/1.jsonl")))
			}
			_, err := RunChecks(Options{Cwd: repo, ArtifactsDir: artifacts, BaseBranch: "main"})
			testutil.Fails(t, err, filepath.Join(artifacts, "checks"))
			if input != "1" {
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/1.jsonl")))), "preserve history\n")
			}
			if input != "missing-index" {
				testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/round")))), input)
			}
		})
	}
}

func TestChecksCLIUsesConfiguredBaseWithoutEnvironment(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "branch", "develop", "HEAD")
	testutil.Put(t, repo, ".devkit/config.json", `{"checks":[{"name":"base","command":"true","timeout_seconds":1}]}`)
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "develop checks")
	testutil.Git(t, repo, "branch", "-f", "develop", "HEAD")
	testutil.Put(t, repo, ".devkit/config.json", `{}`)
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  baseBranch: develop\n")
	artifacts := t.TempDir()
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { return artifacts, key == "ARTIFACTS_DIR" }}
	testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"checks"}, env), 0)
	testutil.Equal(t, len(readResults(t, filepath.Join(artifacts, "checks/1.jsonl"))), 1)
}

func TestRunChecksPreservesExistingNextRoundAndEmptyHistory(t *testing.T) {
	repo, artifacts := testutil.Repo(t), t.TempDir()
	o := Options{Cwd: repo, ArtifactsDir: artifacts, BaseBranch: "main"}
	testutil.Equal(t, testutil.Must(RunChecks(o)).Round, 1)
	testutil.Equal(t, testutil.Must(RunChecks(o)).Round, 2)
	testutil.Put(t, artifacts, "checks/3.jsonl", "preserve history\n")
	_, err := RunChecks(o)
	testutil.Fails(t, err, filepath.Join(artifacts, "checks/3.jsonl"))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/round")))), "2")
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "checks/3.jsonl")))), "preserve history\n")
}
