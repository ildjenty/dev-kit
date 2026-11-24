package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestHelpersRequireScratchWorkspace(t *testing.T) {
	env := cli.Env{Cwd: t.TempDir(), LookupEnv: func(string) (string, bool) { return "", false }}
	testutil.Fails(t, before([]string{"plan", "check"}, env), "scratch workspace")
	testutil.Fails(t, after([]string{"plan", "finish"}, env), "scratch workspace")
}
func TestDraftIsReadFromFixtureRepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "fixture-exec-local")
	artifacts := filepath.Join(root, "artifacts")
	testutil.Put(t, artifacts, "fixture.json", `{"workflow":"plan"}`)
	testutil.Put(t, artifacts, "plan/draft.json", `{}`)
	draft := `{"local_fixture":true}`
	testutil.Put(t, repo, ".archon/workflows/dev/plan/fixtures/executed/draft.json", draft)
	vars := map[string]string{"DEVKIT_FIXTURE": "1", "ARTIFACTS_DIR": artifacts, "STATE_DIR": filepath.Join(root, "state")}
	env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, before([]string{"plan", "check"}, env))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "plan/draft.json")))), draft)
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(repo, "fixture.txt")))), "fixture result\nPlan skeleton.\n")
}

func TestRefineFixtureKeepsIssueDraftsWithoutProse(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "fixture-exec-local")
	artifacts := filepath.Join(root, "artifacts")
	testutil.Put(t, artifacts, "fixture.json", `{"workflow":"refine"}`)
	testutil.Put(t, artifacts, "refine/draft.json", `{}`)
	testutil.Put(t, repo, ".archon/workflows/dev/refine/fixtures/executed/draft.json", `{}`)
	vars := map[string]string{"DEVKIT_FIXTURE": "1", "ARTIFACTS_DIR": artifacts, "STATE_DIR": filepath.Join(root, "state"), "INPUTS_PHASE": "document"}
	env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, before([]string{"refine", "check"}, env))
	_, err := os.Stat(filepath.Join(repo, "docs"))
	testutil.Equal(t, os.IsNotExist(err), true)
}
func TestRetroDraftAndApprovalArePreparedForExecutedFixture(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "fixture-exec-local")
	artifacts := filepath.Join(root, "artifacts")
	testutil.Put(t, artifacts, "fixture.json", `{"workflow":"retro"}`)
	testutil.Put(t, artifacts, "retro/draft.json", `{}`)
	draft := `{"summary":"traced correction"}`
	testutil.Put(t, repo, ".archon/workflows/dev/retro/fixtures/executed/draft.json", draft)
	vars := map[string]string{"DEVKIT_FIXTURE": "1", "ARTIFACTS_DIR": artifacts, "STATE_DIR": filepath.Join(root, "state")}
	env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, before([]string{"retro", "check"}, env))
	testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "retro/draft.json")))), draft)
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(artifacts, "retro/approval.txt")))), "approve=1")
}

func TestRetroFixtureSetupUsesRetroBranch(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".archon/workflows/dev/retro/fixtures/executed/backlog.json", `{"deferred":[],"carried":[],"mechanical":[],"accepted_objections":[]}`)
	vars := map[string]string{"BASE_BRANCH": "fixture-base"}
	env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, setup("retro", env))
	testutil.Equal(t, testutil.Git(t, repo, "branch", "--show-current"), "retro/"+filepath.Base(repo))
	testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
}

func TestFixtureSetupPreservesProvider(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			repo := testutil.Repo(t)
			testutil.Put(t, repo, ".devkit/config.json", `{"provider":"`+provider+`"}`)
			env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) { return "fixture-base", k == "BASE_BRANCH" }}
			testutil.OK(t, setup("requirements", env))
			testutil.Equal(t, testutil.Must(config.Load(repo)).Provider, provider)
		})
	}
}
