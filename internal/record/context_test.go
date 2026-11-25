package record

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestContextUsesSharedBaseResolutionAndRecordsSource(t *testing.T) {
	repo, state := testutil.Repo(t), t.TempDir()
	testutil.Git(t, repo, "branch", "trunk", "main")
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1", "BASE_BRANCH": "trunk"}
	var out, stderr bytes.Buffer
	env := commandEnv(repo, vars, &out, &stderr)
	testutil.Equal(t, must(Context(env)).BaseBranchSource, "BASE_BRANCH")
	vars["BASE_BRANCH"] = ""
	_, err := Context(env)
	testutil.Fails(t, err, "origin/HEAD")
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  baseBranch: trunk\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "base setting")
	ctx := must(Context(env))
	testutil.Equal(t, []string{ctx.BaseBranch, ctx.BaseBranchSource}, []string{"trunk", ".archon/config.yaml worktree.baseBranch"})
	testutil.OK(t, cli.New("test", Commands()).Run([]string{"record", "start", "requirements"}, env))
	run := must(Latest(ctx.Dir, "run"))
	testutil.Equal(t, []any{run["base_branch"], run["base_branch_source"]}, []any{ctx.BaseBranch, ctx.BaseBranchSource})
	testutil.Put(t, repo, ".archon/config.yaml", "worktree: [")
	_, err = Context(env)
	testutil.Fails(t, err, ".archon/config.yaml")
	testutil.OK(t, os.Remove(filepath.Join(repo, ".archon/config.yaml")))
	testutil.Git(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	ctx = must(Context(env))
	testutil.Equal(t, []string{ctx.BaseBranch, ctx.BaseBranchSource}, []string{"trunk", "origin/HEAD"})
}

func TestRecordCommandsRejectInvalidLimitsWithoutSaving(t *testing.T) {
	for _, value := range []string{"0", "-1", `"1"`} {
		repo, state := testutil.Repo(t), t.TempDir()
		testutil.Put(t, repo, ".devkit/config.json", `{"limits":{"restarts":`+value+`}}`)
		var out, stderr bytes.Buffer
		env := commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1"}, &out, &stderr)
		for _, mode := range []string{"limit", "start"} {
			testutil.Fails(t, cli.New("test", Commands()).Run([]string{"record", mode, "requirements"}, env), "limits.restarts must be an integer of at least 1")
		}
		testutil.Equal(t, len(must(List(RecordDir(state, "feature/x"), "run"))), 0)
	}
}
