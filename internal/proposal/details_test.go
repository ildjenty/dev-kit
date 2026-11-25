package proposal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestShowPrintsFullDetailsForRequestedBranch(t *testing.T) {
	root := testutil.Repo(t)
	state := t.TempDir()
	branch := "refine/full-details"
	ctx := record.ContextData{Dir: record.RecordDir(state, branch), Cwd: root, Branch: branch, RunID: "run-1"}
	content := doc{
		"commit":  testutil.Git(t, root, "rev-parse", "HEAD"),
		"mapping": doc{"N1": "D-abcdef"}, "topic": doc{"number": 42, "title": "Selected topic"},
		"draft": doc{
			"pr_title": "Record the chosen default", "pr_summary": "Preserve the chosen default and its reason.",
			"parent":    doc{"title": "Parent topic", "body": "Full parent body"},
			"decisions": []doc{{"key": "N1", "title": "Default", "body": "Full decision body\nCandidates, choice, reason and uncertainty."}},
			"bindings":  []doc{{"path": "AGENTS.md", "decision": "N1", "reason": "Apply the chosen agent rule"}},
			"units":     []doc{{"key": "U1", "title": "Delivery", "goal": "Deliver the choice", "completion_conditions": []string{"Observable result"}, "blocked_by": []string{"T1"}}},
			"topics":    []doc{{"key": "T1", "title": "Open point", "body": "Full question body", "blocked_by": []string{}}},
			"summary":   "Cost estimate: 10 per month", "owner_points": []string{"Default needs approval"},
		},
	}
	p := testutil.Must(record.Append(ctx.Dir, "proposal", doc{"content": content, "request": 1, "run_id": ctx.RunID, "recorded_at": "t"})).Entry
	testutil.OK(t, SaveDetails(ctx, p))
	saved := testutil.Must(os.ReadFile(detailsPath(ctx.Dir, p)))
	registry := cli.New("test", Commands())
	var out bytes.Buffer
	env := cli.Env{Cwd: root, Stdout: &out, LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
	testutil.OK(t, registry.Run([]string{"proposal", "show", branch}, env))
	testutil.Equal(t, out.String(), string(saved))
	var got doc
	testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
	expected := testutil.Must(json.Marshal(p))
	var want doc
	testutil.OK(t, json.Unmarshal(expected, &want))
	testutil.Equal(t, got, want)
	testutil.Equal(t, obj(got["content"])["commit"], content["commit"])
	testutil.Equal(t, num(got["version"]), 1)
	testutil.Equal(t, testutil.Git(t, root, "branch", "--show-current"), "feature/x")
	testutil.OK(t, registry.Run([]string{"proposal", "show", branch}, env))
	testutil.Fails(t, registry.Run([]string{"proposal", "show", "retro/missing"}, env), "no proposal")
	testutil.OK(t, os.Remove(filepath.Join(ctx.Dir, "proposals", "1.details.json")))
	testutil.Fails(t, registry.Run([]string{"proposal", "show", branch}, env), "1.details.json")
	for _, args := range [][]string{{"proposal", "show"}, {"proposal", "show", branch, "extra"}, {"proposal", "show", ""}, {"proposal", "show", "--invalid"}} {
		testutil.Fails(t, registry.Run(args, env), "Usage:")
	}
}

func TestShowFindsProjectStateFromLinkedWorktree(t *testing.T) {
	root := testutil.Repo(t)
	worktree := filepath.Join(t.TempDir(), "checkout")
	testutil.Git(t, root, "worktree", "add", "--detach", worktree, "HEAD")
	bin, home := t.TempDir(), t.TempDir()
	listing := string(testutil.Must(json.Marshal(doc{"projects": []doc{{"key": "owner/repo", "default_cwd": root}}})))
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '"+listing+"'\n")
	testutil.OK(t, os.Chmod(filepath.Join(bin, "archon"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	state := filepath.Join(home, "workspaces/owner/repo/state")
	branch := "retro/review"
	ctx := record.ContextData{Dir: record.RecordDir(state, branch)}
	p := testutil.Must(record.Append(ctx.Dir, "proposal", doc{"content": doc{"draft": doc{"summary": "Full correction"}}, "request": 1, "run_id": "run-1", "recorded_at": "t"})).Entry
	testutil.OK(t, SaveDetails(ctx, p))
	var out bytes.Buffer
	env := cli.Env{Cwd: worktree, Stdout: &out, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return home, k == "ARCHON_HOME" }}
	testutil.OK(t, cli.New("test", Commands()).Run([]string{"proposal", "show", branch}, env))
	testutil.Equal(t, out.String(), string(testutil.Must(os.ReadFile(detailsPath(ctx.Dir, p)))))
}
