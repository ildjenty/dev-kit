package pr

import (
	"bytes"
	"github.com/ildjenty/dev-kit/internal/unitdone"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/retro"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestBacklogPreservesEachCategoryAndItsSourceJudgment(t *testing.T) {
	f := doc{"id": "structure-1", "text": "Failure", "evidence": "src", "blocking": false, "needs_decision": false, "deferred": true, "mechanical": true}
	j := judgment("j1", "structure", "base", "head", doc{"findings": []doc{f}})
	dispute := judgment("fix1", "dispute", "base", "head", doc{"findings": []doc{{"id": "j1/structure/structure-1", "blocking": false, "text": "Accepted objection", "evidence": "Evidence"}, {"id": "j1/structure/structure-2", "blocking": true, "text": "Rejected objection"}}})
	carried := doc{"id": "structure-1", "judge": "structure", "text": "Carried finding"}
	out := backlog([]doc{j, dispute}, []doc{{"run_id": "j1", "carried": []doc{carried}}})
	testutil.Equal(t, len(out), 4)
	for _, key := range []string{"deferred", "mechanical", "carried", "accepted_objections"} {
		testutil.Equal(t, len(docs(out[key])), 1)
	}
	testutil.Equal(t, object(docs(out["deferred"])[0]["source"]), doc{"run_id": "j1", "judgment": 1, "judge": "structure", "kind": "code", "target": j["target"]})
	testutil.Equal(t, object(docs(out["carried"])[0]["source"])["judgment"], 1)
	testutil.Equal(t, object(docs(out["carried"])[0]["finding"]), carried)
	testutil.Equal(t, object(docs(out["accepted_objections"])[0]["source"])["run_id"], "fix1")
	empty := backlog(nil, nil)
	for _, key := range []string{"deferred", "mechanical", "carried", "accepted_objections"} {
		testutil.Equal(t, empty[key], []doc{})
	}
}
func TestPRCommitsBacklogBeforePushAndKeepsJudgedChangeIdentity(t *testing.T) {
	repo := testutil.Repo(t)
	for key, value := range map[string]string{"user.name": "Test", "user.email": "test@example.invalid", "commit.gpgsign": "false", "core.hooksPath": "/dev/null"} {
		testutil.Git(t, repo, "config", key, value)
	}
	testutil.Put(t, repo, "source.ext", "changed source\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "source")
	g := must(gitx.New(repo, "main"))
	head := must(g.Head())
	id := must(g.ChangeID(head))
	base := must(g.MergeBase(head))
	remote := t.TempDir()
	testutil.Git(t, remote, "init", "--bare", "-q")
	testutil.Git(t, repo, "remote", "add", "origin", remote)
	state, a := t.TempDir(), t.TempDir()
	ctx := record.ContextData{Cwd: repo, Dir: record.RecordDir(state, "feature/x"), Branch: "feature/x", BaseBranch: "main", RunID: "pr1"}
	testutil.Put(t, ctx.Dir, "request/1.json", `{"unit":"U-abcdef"}`)
	g.RecordDir = ctx.Dir
	report := judgment("pr1", "report", base, head, doc{"kind": "report", "criteria_commit": base, "materials": reportMaterials([]byte("Title\n"), []byte("Body\n"))})
	must(record.Append(ctx.Dir, "judgment", report))
	testutil.OK(t, os.MkdirAll(filepath.Join(a, "pr"), 0755))
	testutil.OK(t, save(filepath.Join(a, "pr/facts.json"), doc{"head": head}))
	testutil.Put(t, a, "pr/pr-title.txt", "Title\n")
	testutil.Put(t, a, "pr/pr.md", "Body\n")
	testutil.Put(t, a, "pr/report-state.json", `{"passed":true}`)
	bin := t.TempDir()
	testutil.Put(t, bin, "gh", "#!/bin/sh\nif [ \"$2\" = create ]; then printf 'https://example.invalid/pr/1'; fi\n")
	testutil.OK(t, os.Chmod(filepath.Join(bin, "gh"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr1", "ARTIFACTS_DIR": a, "BASE_BRANCH": "main"}
	var stdout bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: io.Discard, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	testutil.OK(t, openPR(nil, env))
	next := must(g.Head())
	if head == next {
		t.Fatal("backlog was not committed")
	}
	testutil.Contains(t, testutil.Git(t, repo, "log", "-1", "--format=%s"), "chore:")
	testutil.Equal(t, testutil.Git(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"), ".devkit/backlog/feature%2Fx.json\n.devkit/units/U-abcdef.done.json")
	testutil.Equal(t, testutil.Git(t, remote, "show", "refs/heads/feature/x:.devkit/units/U-abcdef.done.json"), `{"unit":"U-abcdef","branch":"feature/x"}`)
	testutil.Equal(t, testutil.Git(t, remote, "rev-parse", "refs/heads/feature/x"), next)
	testutil.Equal(t, must(g.ChangeID(next)), id)
	testutil.Equal(t, must(read(filepath.Join(repo, ".devkit/backlog/feature%2Fx.json"))), backlog([]doc{report}, nil))
	testutil.Contains(t, stdout.String(), next)
	testutil.Equal(t, testutil.Git(t, repo, "status", "--porcelain"), "")
	testutil.Equal(t, must(commitBacklog(ctx, []doc{report}, nil)), next)
	testutil.Equal(t, strings.HasPrefix(testutil.Git(t, repo, "log", "-1", "--format=%B"), "chore:"), true)
}

func TestProducedBacklogIsConsumedByRetroWithoutLosingJudgmentReferences(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Git(t, repo, "config", "user.name", "Test")
	testutil.Git(t, repo, "config", "user.email", "test@example.invalid")
	testutil.Git(t, repo, "config", "commit.gpgsign", "false")
	testutil.Git(t, repo, "config", "core.hooksPath", "/dev/null")
	f := doc{"id": "structure-1", "text": "Failure", "evidence": "src", "blocking": false, "needs_decision": false, "deferred": true, "mechanical": true}
	j := judgment("j1", "structure", strings.Repeat("a", 40), strings.Repeat("b", 40), doc{"findings": []doc{f}})
	dispute := judgment("fix1", "dispute", strings.Repeat("a", 40), strings.Repeat("b", 40), doc{"findings": []doc{{"id": "j1/structure/structure-1", "blocking": false, "text": "Accepted objection", "evidence": "Evidence"}}})
	produced := backlog([]doc{j, dispute}, []doc{{"run_id": "j1", "carried": []doc{{"id": "structure-1", "judge": "structure", "text": "Carried"}}}})
	testutil.OK(t, os.MkdirAll(filepath.Join(repo, ".devkit/backlog"), 0755))
	testutil.OK(t, save(filepath.Join(repo, ".devkit/backlog/feature%2Fx.json"), produced))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "Backlog on base")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Git(t, repo, "switch", "-qc", "retro/backlog")
	artifacts := t.TempDir()
	vars := map[string]string{"STATE_DIR": t.TempDir(), "WORKFLOW_ID": "retro1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	registry := cli.New("test", retro.Commands())
	for _, command := range []string{"start", "prepare"} {
		if code := registry.Execute([]string{"retro", command}, env); code != 0 {
			t.Fatalf("retro %s: exit %d: %s", command, code, stderr.String())
		}
	}
	input := must(read(filepath.Join(artifacts, "retro/input.json")))
	entries := docs(input["entries"])
	testutil.Equal(t, len(entries), 4)
	for _, entry := range entries {
		kind := text(entry["kind"])
		testutil.Equal(t, entry["id"], ".devkit/backlog/feature%2Fx.json#"+kind+"/0")
		testutil.Equal(t, object(entry["content"]), docs(produced[kind])[0])
	}
}

func TestF12ResolvedMechanicalFindingsStillReachBacklog(t *testing.T) {
	f := doc{"id": "structure-1", "text": "A resolved defect", "evidence": "src", "blocking": false, "needs_decision": false, "deferred": true, "mechanical": true, "resolved": true}
	j := judgment("p1", "structure", "base", "head", doc{"kind": "plan", "phase": "structure", "round": 1, "findings": []doc{f}})
	out := backlog([]doc{j}, nil)
	testutil.Equal(t, len(docs(out["deferred"])), 0)
	testutil.Equal(t, len(docs(out["mechanical"])), 1)
	source := object(docs(out["mechanical"])[0]["source"])
	testutil.Equal(t, source["round"], 1)
	testutil.Equal(t, source["phase"], "structure")
	testutil.Equal(t, object(docs(out["mechanical"])[0]["finding"])["resolved"], true)
}

func TestBacklogCommitWritesAndPreservesUnitDoneMarker(t *testing.T) {
	for _, unit := range []string{"", "U-abcdef"} {
		t.Run(unit, func(t *testing.T) {
			repo, dir := testutil.Repo(t), t.TempDir()
			testutil.Put(t, repo, ".devkit/config.json", `{"units":"work/units"}`)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "config")
			ctx := record.ContextData{Cwd: repo, Dir: dir, Branch: "feature/x", RunID: "pr1"}
			if unit != "" {
				testutil.Put(t, dir, "request/1.json", `{"unit":"U-abcdef"}`)
			}
			g := must(gitx.New(repo, "main", dir))
			before := must(g.ChangeID("HEAD"))
			head := must(commitBacklog(ctx, nil, nil))
			path := filepath.Join(repo, "work/units/U-abcdef.done.json")
			if unit == "" {
				_, err := os.Stat(path)
				testutil.Equal(t, os.IsNotExist(err), true)
			} else {
				marker := must(unitdone.Parse(must(os.ReadFile(path))))
				testutil.Equal(t, marker, unitdone.Marker{Unit: unit, Branch: ctx.Branch})
				testutil.Equal(t, testutil.Git(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"), ".devkit/backlog/feature%2Fx.json\nwork/units/U-abcdef.done.json")
				testutil.Equal(t, must(g.ChangeID(head)), before)
				kept := "{\n  \"branch\": \"feature/x\", \"unit\": \"U-abcdef\"\n}\n"
				testutil.OK(t, os.WriteFile(path, []byte(kept), 0644))
				testutil.Git(t, repo, "add", ".")
				testutil.Git(t, repo, "commit", "-qm", "existing marker formatting")
				head = must(g.Head())
				testutil.Equal(t, must(commitBacklog(ctx, nil, nil)), head)
				testutil.Equal(t, string(must(os.ReadFile(path))), kept)
			}
			testutil.Equal(t, must(commitBacklog(ctx, nil, nil)), head)
		})
	}
}

func TestReviewFindingsReachBacklogWithSourceAndRemainConsumable(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "config", "user.name", "Test")
	testutil.Git(t, repo, "config", "user.email", "test@example.invalid")
	testutil.Git(t, repo, "config", "commit.gpgsign", "false")
	testutil.Put(t, repo, "a.ext", "source\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "source")
	ctx := record.ContextData{Cwd: repo, Dir: t.TempDir(), Branch: "feature/x", RunID: "pr2"}
	g := must(gitx.New(repo, "main", ctx.Dir))
	id := must(g.ChangeID("HEAD"))
	head := must(g.Head())
	review := "CI job integration failed: assertion error\n  full text\n"
	must(record.Append(ctx.Dir, "run", doc{"run_id": "fix-review", "workflow": "fix", "started_at": "t", "start": "next", "counted": false, "answers": []string{}, "review": doc{"text": review, "commit": head}}))
	next := must(commitBacklog(ctx, nil, nil))
	value := must(read(filepath.Join(repo, ".devkit/backlog/feature%2Fx.json")))
	entries := docs(value["review"])
	testutil.Equal(t, len(entries), 1)
	testutil.Equal(t, object(entries[0]["source"]), doc{"kind": "review", "run_id": "fix-review", "target": doc{"commit": head}})
	testutil.Equal(t, object(entries[0]["finding"]), doc{"id": "fix-review/review", "text": review})
	testutil.Equal(t, must(g.ChangeID(next)), id)
	testutil.Equal(t, must(commitBacklog(ctx, nil, nil)), next)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "standards")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.Git(t, repo, "switch", "-qc", "retro/review")
	artifacts := t.TempDir()
	vars := map[string]string{"STATE_DIR": t.TempDir(), "WORKFLOW_ID": "retro-review", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	registry := cli.New("", retro.Commands())
	for _, command := range []string{"start", "prepare"} {
		if code := registry.Execute([]string{"retro", command}, env); code != 0 {
			t.Fatalf("%s: %s", command, stderr.String())
		}
	}
	input := must(read(filepath.Join(artifacts, "retro/input.json")))
	found := false
	for _, entry := range docs(input["entries"]) {
		if entry["kind"] == "review" {
			found = true
			testutil.Equal(t, object(entry["content"]), entries[0])
		}
	}
	testutil.Equal(t, found, true)
}
