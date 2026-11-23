package status

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func write(t *testing.T, dir, kind string, n int, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	testutil.OK(t, err)
	testutil.Put(t, dir, filepath.Join(kind, itoa(n)+".json"), string(data))
}

func itoa(n int) string { return strconv.Itoa(n) }

func branch(t *testing.T, records, name string) string {
	dir := filepath.Join(records, strings.ReplaceAll(name, "/", "%2F"))
	testutil.OK(t, os.MkdirAll(dir, 0755))
	return dir
}

func run1(t *testing.T, dir, workflow string, n int, result map[string]any) {
	id := workflow + "-" + itoa(n)
	write(t, dir, "runs", n, map[string]any{"run_id": id, "workflow": workflow, "started_at": "2026-11-24T00:0" + itoa(n) + ":00Z", "start": "first", "counted": false, "answers": []string{}})
	if result != nil {
		result["run_id"], result["workflow"], result["recorded_at"] = id, workflow, "2026-11-24T00:0"+itoa(n)+":30Z"
		write(t, dir, "results", n, result)
	}
}

func TestReadsEveryBranchAndNamesTheNextStep(t *testing.T) {
	records := t.TempDir()
	q := branch(t, records, "feature/questions")
	run1(t, q, "requirements", 1, map[string]any{"outcome": "question", "questions": []any{
		map[string]any{"id": "Q1", "text": "Which shops?", "target": map[string]any{"kind": "open_point", "ref": "O1"}, "next_workflows": []any{"requirements"}},
		map[string]any{"id": "Q2", "text": "Already answered", "target": map[string]any{"kind": "open_point", "ref": "O2"}, "next_workflows": []any{"requirements"}},
	}})
	testutil.Put(t, q, "answers/Q2.json", `{"question_id":"Q2","answer":"yes"}`)
	approval := branch(t, records, "refine/attendance")
	run1(t, approval, "refine", 1, map[string]any{"outcome": "question", "questions": []any{map[string]any{"id": "Q1", "text": "Approve version 2?", "target": map[string]any{"kind": "proposal", "ref": "2"}, "next_workflows": []any{"refine"}}}})
	back := branch(t, records, "feature/back")
	run1(t, back, "implement", 1, map[string]any{"outcome": "back", "back": map[string]any{"to": "plan", "premise": "P", "observation": "O"}})
	blocked := branch(t, records, "feature/blocked")
	run1(t, blocked, "judge", 1, map[string]any{"outcome": "stop", "stop": map[string]any{"state": "Blocking findings remain; use the fix workflow", "unresolved": []any{"x"}, "premise": ""}})
	running := branch(t, records, "feature/running")
	run1(t, running, "plan", 1, nil)
	synced := branch(t, records, "feature/synced")
	run1(t, synced, "sync", 1, map[string]any{"outcome": "pass", "loops": []any{map[string]any{"name": "conflicts", "ended": "cleared", "rounds": 2}}})
	limited := branch(t, records, "feature/limited")
	run1(t, limited, "implement", 1, map[string]any{"outcome": "stop", "stop": map[string]any{"state": "Units remain", "unresolved": []any{}, "premise": ""}})

	got := map[string]Branch{}
	for _, b := range testutil.Must(Read(records, "", 0)) {
		got[b.Branch] = b
	}
	testutil.Equal(t, len(got), 7)
	testutil.Equal(t, len(got["feature/questions"].Questions), 1)
	testutil.Equal(t, got["feature/questions"].Questions[0].ID, "Q1")
	testutil.Contains(t, got["feature/questions"].Next, "Relay Q1")
	testutil.Contains(t, got["feature/questions"].Next, "start requirements on feature/questions")
	testutil.Contains(t, got["refine/attendance"].Next, "--input approve=2")
	testutil.Contains(t, got["feature/back"].Next, "Start plan on feature/back")
	testutil.Contains(t, got["feature/blocked"].Next, "Start fix on feature/blocked")
	testutil.Equal(t, got["feature/running"].Outcome, "no result")
	testutil.Contains(t, got["feature/running"].Next, "archon workflow get plan-1")
	testutil.Contains(t, got["feature/synced"].Next, "start judge on feature/synced")
	testutil.Contains(t, got["feature/limited"].Next, "restart limit")
	only := testutil.Must(Read(records, "feature/back", 3))
	testutil.Equal(t, len(only), 1)
	testutil.Equal(t, only[0].Branch, "feature/back")
}

func TestStateDirFromArchonProjectsAndOverrides(t *testing.T) {
	repo := testutil.Repo(t)
	bin, home := t.TempDir(), t.TempDir()
	listing := `{"projects":[{"key":"other/repo","default_cwd":"/elsewhere"},{"key":"owner/name","default_cwd":"` + repo + `"}]}`
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '"+listing+"'\n")
	testutil.OK(t, os.Chmod(filepath.Join(bin, "archon"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	values := map[string]string{"HOME": home}
	env := cli.Env{Cwd: repo, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	dir, why := StateDir(env, repo)
	testutil.Equal(t, why, "")
	testutil.Equal(t, dir, filepath.Join(home, ".archon/workspaces/owner/name/state"))
	values["ARCHON_HOME"] = filepath.Join(home, "custom")
	dir, _ = StateDir(env, repo)
	testutil.Equal(t, dir, filepath.Join(home, "custom/workspaces/owner/name/state"))
	values["STATE_DIR"] = "/explicit"
	dir, _ = StateDir(env, repo)
	testutil.Equal(t, dir, "/explicit")
	delete(values, "STATE_DIR")
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf '%s' '{\"projects\":[]}'\n")
	dir, why = StateDir(env, repo)
	testutil.Equal(t, dir, "")
	testutil.Contains(t, why, "No workflow has run in this repository yet")
}

func TestCommandPrintsAndRejectsBadArguments(t *testing.T) {
	repo := testutil.Repo(t)
	state := t.TempDir()
	dir := branch(t, filepath.Join(state, "records"), "feature/x")
	run1(t, dir, "plan", 1, map[string]any{"outcome": "pass"})
	var out bytes.Buffer
	values := map[string]string{"STATE_DIR": state}
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &bytes.Buffer{}, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	testutil.Equal(t, cli.New("", Commands()).Execute([]string{"status"}, env), 0)
	testutil.Contains(t, out.String(), "feature/x: plan pass")
	testutil.Contains(t, out.String(), "next: Start implement on feature/x.")
	out.Reset()
	testutil.Equal(t, cli.New("", Commands()).Execute([]string{"status", "--json", "feature/x"}, env), 0)
	var parsed struct{ Branches []Branch }
	testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
	testutil.Equal(t, parsed.Branches[0].Next, "Start implement on feature/x.")
	testutil.Equal(t, cli.New("", Commands()).Execute([]string{"status", "a", "b"}, env) != 0, true)
}

func TestStatusShowsRequestUnitInTextAndJSON(t *testing.T) {
	repo, state := testutil.Repo(t), t.TempDir()
	dir := branch(t, filepath.Join(state, "records"), "feature/x")
	run1(t, dir, "requirements", 1, map[string]any{"outcome": "pass"})
	write(t, dir, "request", 1, map[string]any{"unit": "U-abcdef"})
	write(t, dir, "request", 2, map[string]any{"unit": ""})
	var out bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, LookupEnv: func(k string) (string, bool) {
		if k == "STATE_DIR" {
			return state, true
		}
		return "", false
	}}
	testutil.OK(t, run(nil, env))
	testutil.Contains(t, out.String(), "unit: U-abcdef")
	out.Reset()
	testutil.OK(t, run([]string{"--json"}, env))
	testutil.Contains(t, out.String(), `"unit":"U-abcdef"`)
}

func TestStatusReadsPullRequestStatesAndReportsNonfatalFailures(t *testing.T) {
	for _, tc := range []struct {
		name, response, next, state string
		failed                      bool
	}{
		{"open", `{"state":"OPEN","url":"https://example.invalid/pr/1","reviewDecision":"APPROVED","statusCheckRollup":[]}`, "owner reviews", "open", false},
		{"checks", `{"state":"OPEN","url":"https://example.invalid/pr/1","reviewDecision":"CHANGES_REQUESTED","statusCheckRollup":[{"name":"tests","conclusion":"FAILURE"},{"context":"legacy","state":"ERROR"},{"name":"ok","conclusion":"SUCCESS"},{"name":"pending","state":"PENDING"}]}`, "failing check's log", "open", false},
		{"changes requested", `{"state":"OPEN","url":"https://example.invalid/pr/1","reviewDecision":"CHANGES_REQUESTED","statusCheckRollup":null}`, "review comments to fix", "open", false},
		{"merged", `{"state":"MERGED","url":"https://example.invalid/pr/1","reviewDecision":"APPROVED","statusCheckRollup":[]}`, "archon complete feature/x", "merged", false},
		{"closed", `{"state":"CLOSED","url":"https://example.invalid/pr/1","reviewDecision":"","statusCheckRollup":[]}`, "Closed without merging", "closed", false},
		{"failed", "", "gh pr view failed", "", true},
		{"malformed", "{broken", "could not read gh pr view", "", true},
		{"unknown state", `{"state":"UNKNOWN"}`, "unknown pull request state", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state, bin := testutil.Repo(t), t.TempDir(), t.TempDir()
			script := "#!/bin/sh\n[ \"$*\" = 'pr view feature/x --json state,url,reviewDecision,statusCheckRollup' ] || exit 2\nprintf '%s' '" + tc.response + "'\n"
			if tc.name == "failed" {
				script = "#!/bin/sh\nprintf 'authentication failed' >&2\nexit 1\n"
			}
			testutil.Put(t, bin, "gh", script)
			testutil.OK(t, os.Chmod(filepath.Join(bin, "gh"), 0755))
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			dir := branch(t, filepath.Join(state, "records"), "feature/x")
			run1(t, dir, "pr", 1, map[string]any{"outcome": "pass"})
			var out bytes.Buffer
			env := cli.Env{Cwd: repo, Stdout: &out, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
			testutil.OK(t, run([]string{"--json"}, env))
			var parsed struct{ Branches []Branch }
			testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
			b := parsed.Branches[0]
			if tc.failed {
				testutil.Contains(t, b.PullRequestError, tc.next)
			} else {
				testutil.Equal(t, b.PullRequest.State, tc.state)
				testutil.Equal(t, b.PullRequest.URL, "https://example.invalid/pr/1")
				testutil.Contains(t, b.Next, tc.next)
				if tc.name == "checks" {
					testutil.Equal(t, b.PullRequest.FailingChecks, []string{"tests", "legacy"})
				}
			}
			out.Reset()
			testutil.OK(t, run(nil, env))
			if tc.failed {
				testutil.Contains(t, out.String(), tc.next)
			} else {
				testutil.Contains(t, out.String(), "pull request: "+tc.state)
				testutil.Equal(t, strings.Contains(out.String(), "review decision:"), b.PullRequest.ReviewDecision != "")
			}
			if tc.name == "checks" {
				testutil.Contains(t, out.String(), "failing check: tests")
			}
		})
	}
}

func TestPullRequestLookupIsOptionalAndBounded(t *testing.T) {
	t.Run("missing gh", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		b := Branch{Next: "previous"}
		readPullRequest(cli.Env{}, &b)
		testutil.Equal(t, b.Next, "previous")
		testutil.Equal(t, b.PullRequestError, "")
	})
	t.Run("timeout", func(t *testing.T) {
		bin := t.TempDir()
		testutil.Put(t, bin, "gh", "#!/bin/sh\nexec /bin/sleep 30\n")
		testutil.OK(t, os.Chmod(filepath.Join(bin, "gh"), 0755))
		t.Setenv("PATH", bin)
		b := Branch{Branch: "feature/x"}
		readPullRequest(cli.Env{}, &b)
		testutil.Contains(t, b.PullRequestError, "deadline exceeded")
	})
	t.Run("only passing pr", func(t *testing.T) {
		repo, state, bin := testutil.Repo(t), t.TempDir(), t.TempDir()
		testutil.Put(t, bin, "gh", "#!/bin/sh\ntouch '"+filepath.Join(bin, "called")+"'\nexit 1\n")
		testutil.OK(t, os.Chmod(filepath.Join(bin, "gh"), 0755))
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		for _, workflow := range []string{"fix", "pr"} {
			dir := branch(t, filepath.Join(state, "records"), workflow)
			outcome := "pass"
			if workflow == "pr" {
				outcome = "stop"
			}
			run1(t, dir, workflow, 1, map[string]any{"outcome": outcome})
		}
		env := cli.Env{Cwd: repo, Stdout: &bytes.Buffer{}, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { return state, k == "STATE_DIR" }}
		testutil.OK(t, run(nil, env))
		_, err := os.Stat(filepath.Join(bin, "called"))
		testutil.Equal(t, os.IsNotExist(err), true)
	})
}

func TestBaseFailureNextExplainsRepairAndUncountedRestartAtLimit(t *testing.T) {
	for _, workflow := range []string{"implement", "fix", "judge"} {
		b := Branch{Branch: "feature/x", Workflow: workflow, Outcome: "stop", Restarts: 3, Limit: 3, Stop: map[string]any{"state": "the base branch fails unit already: fix the base branch first, then start this workflow again", "base_failed": true}}
		advice := next(b, nil)
		testutil.Contains(t, advice, "Fix the base branch first")
		testutil.Contains(t, advice, "not counted against limits.restarts")
		testutil.Contains(t, advice, "log paths")
		if strings.Contains(advice, "Start fix") || strings.Contains(advice, "continue=...") {
			t.Fatal(advice)
		}
	}
}

func TestStatusSupersessionUsesEachBranchAndConfiguredBase(t *testing.T) {
	repo, state := testutil.Repo(t), t.TempDir()
	testutil.Put(t, repo, ".archon/config.yaml", "worktree:\n  baseBranch: trunk\n")
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "---\nid: D-abcdef\ntitle: Old\nstatus: superseded\n---\nOld body\n")
	testutil.Put(t, repo, ".devkit/decisions/D-fedcba.md", "---\nid: D-fedcba\ntitle: New\nstatus: agreed\nsupersedes: [D-abcdef]\n---\nNew body\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "decisions")
	testutil.Git(t, repo, "branch", "trunk")
	testutil.Git(t, repo, "branch", "feature/plan")
	testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\ndecisions: [D-abcdef]\n---\nUnit\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "branch unit")
	testutil.Git(t, repo, "branch", "feature/unit")
	testutil.Git(t, repo, "checkout", "-q", "feature/plan")
	for _, name := range []string{"feature/unit", "feature/plan"} {
		dir := branch(t, filepath.Join(state, "records"), name)
		run1(t, dir, "implement", 1, map[string]any{"outcome": "pass"})
		write(t, dir, "request", 1, map[string]any{"unit": "U-abcdef"})
		write(t, dir, "plans", 1, map[string]any{"decisions": []any{map[string]any{"id": "P1", "ref": "D-abcdef"}}})
	}
	var out bytes.Buffer
	values := map[string]string{"STATE_DIR": state}
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &bytes.Buffer{}, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := values[k]; return v, ok }}
	registry := cli.New("", Commands())
	testutil.Equal(t, registry.Execute([]string{"status"}, env), 0)
	for _, text := range []string{"decision D-abcdef was superseded on trunk by D-fedcba", "Run sync on feature/unit, then start requirements", "Run sync on feature/plan, then start plan"} {
		testutil.Contains(t, out.String(), text)
	}
	out.Reset()
	testutil.Equal(t, registry.Execute([]string{"status", "--json"}, env), 0)
	var parsed struct {
		Branches []Branch `json:"branches"`
	}
	testutil.OK(t, json.Unmarshal(out.Bytes(), &parsed))
	testutil.Equal(t, len(parsed.Branches), 2)
	for _, b := range parsed.Branches {
		target := "plan"
		if b.Branch == "feature/unit" {
			target = "requirements"
		}
		testutil.Equal(t, b.DecisionCheck.Target, target)
		testutil.Equal(t, b.DecisionCheck.Superseded[0].SupersededBy, []string{"D-fedcba"})
	}
	values["BASE_BRANCH"] = "absent"
	out.Reset()
	testutil.Equal(t, registry.Execute([]string{"status"}, env), 0)
	testutil.Contains(t, out.String(), "decision check skipped")
}
