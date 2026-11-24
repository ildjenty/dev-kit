package retro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func (r *fixture) command(mode string, inputs map[string]string) (int, doc, string) {
	r.t.Helper()
	vars := map[string]string{"STATE_DIR": filepath.Dir(filepath.Dir(r.ctx.Dir)), "WORKFLOW_ID": r.ctx.RunID, "ARTIFACTS_DIR": r.artifacts}
	for k, v := range inputs {
		vars[k] = v
	}
	var out, errOut bytes.Buffer
	env := envVars(vars)
	env.Cwd, env.Stdout, env.Stderr = r.cwd, &out, &errOut
	code := cli.New("test", Commands()).Execute([]string{"retro", mode}, env)
	result := doc{}
	if code == 0 {
		testutil.OK(r.t, json.Unmarshal(out.Bytes(), &result))
	}
	return code, result, errOut.String()
}

func TestProposalCommitPreservesBytesExecutableModesAndDeletions(t *testing.T) {
	r := setup(t, backlog())
	deleted := ".devkit/standards/obsolete.bin"
	modeChanged := ".devkit/standards/existing.sh"
	r.put(deleted, "obsolete")
	r.put(modeChanged, "#!/bin/sh\nexit 0\n")
	r.commit()
	r.begin()
	base := r.git("rev-parse", "HEAD")
	d := r.draft()
	binary := ".devkit/standards/asset.bin"
	executable := ".devkit/standards/new.sh"
	r.put(binary, string([]byte{0xff, 0xfe, 0, '\n'}))
	r.put(executable, "#!/bin/sh\nexit 0\n")
	testutil.OK(t, os.Chmod(filepath.Join(r.cwd, executable), 0755))
	testutil.OK(t, os.Chmod(filepath.Join(r.cwd, modeChanged), 0755))
	testutil.OK(t, os.Remove(filepath.Join(r.cwd, deleted)))
	edits := []doc{}
	for _, path := range []string{deleted, modeChanged, binary, executable} {
		edits = append(edits, doc{"path": path, "reason": "Preserve the proposed correction", "evidence": "Backlog", "entries": items(d["units"])[0]["entries"]})
	}
	d["standards"] = edits
	r.write(d)
	code, checked, stderr := r.command("check", nil)
	testutil.Equal(t, code, 0)
	testutil.Equal(t, stderr, "")
	testutil.Equal(t, checked["valid"], true)
	head := r.git("rev-parse", "HEAD")
	testutil.Equal(t, r.git("rev-parse", "HEAD^"), base)
	g := gitx.Git{Cwd: r.cwd}
	testutil.Equal(t, testutil.Must(g.Bytes("show", head+":"+binary)), []byte{0xff, 0xfe, 0, '\n'})
	for _, path := range []string{executable, modeChanged} {
		testutil.Contains(t, r.git("ls-tree", head, "--", path), "100755 blob")
		testutil.Equal(t, testutil.Must(os.Stat(filepath.Join(r.cwd, path))).Mode().Perm(), os.FileMode(0755))
	}
	for _, path := range []string{deleted, ".devkit/backlog/topic.json"} {
		testutil.Equal(t, r.git("ls-tree", "--name-only", head, "--", path), "")
	}
	testutil.Equal(t, len(testutil.Must(filepath.Glob(filepath.Join(r.cwd, ".devkit/units/retro-*.json")))), 0)
	code, reviewed, stderr := r.command("round", map[string]string{"INPUTS_JUDGMENT": string(testutil.Must(json.Marshal(judgment())))})
	testutil.Equal(t, code, 0)
	testutil.Equal(t, stderr, "")
	testutil.Equal(t, reviewed["valid"], true)
	code, proposed, _ := r.command("finish", nil)
	testutil.Equal(t, code, 0)
	testutil.Equal(t, proposed["outcome"], "question")
	testutil.Contains(t, str(items(proposed["questions"])[0]["text"]), head)
	r.ctx.RunID = "r2"
	code, started, _ := r.command("start", map[string]string{"INPUTS_APPROVE": "1"})
	testutil.Equal(t, code, 0)
	testutil.Equal(t, started["stopped"], false)
	code, approved, _ := r.command("approve", map[string]string{"INPUTS_APPROVE": "1"})
	testutil.Equal(t, code, 0)
	testutil.Equal(t, approved["outcome"], "pass")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	result := testutil.Must(record.Latest(r.ctx.Dir, "result"))
	testutil.Equal(t, result["commit"], head)
	testutil.Equal(t, num(result["proposal_version"]), 1)
}

func TestApprovalRejectionsLeaveHeadTreeAndRecordsUnchanged(t *testing.T) {
	for _, kind := range []string{"dirty-before-start", "dirty-after-start", "another-commit"} {
		t.Run(kind, func(t *testing.T) {
			r := setup(t, backlog())
			r.begin()
			r.propose()
			r.ctx.RunID = "r2"
			if kind != "dirty-before-start" {
				s, e := start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
				testutil.OK(t, e)
				testutil.Equal(t, s.Stopped, false)
			}
			r.put("owner.txt", "Owner's work\n")
			if kind == "another-commit" {
				r.commit()
			}
			head, status := r.git("rev-parse", "HEAD"), r.git("status", "--porcelain")
			results := testutil.Must(record.List(r.ctx.Dir, "result"))
			answers := testutil.Must(record.ListKeyed(r.ctx.Dir, "answer"))
			if kind == "dirty-before-start" {
				code, out, _ := r.command("start", map[string]string{"INPUTS_APPROVE": "1"})
				testutil.Equal(t, code, 0)
				testutil.Equal(t, out["stopped"], true)
				testutil.Contains(t, str(out["reason"]), "clean worktree")
			}
			code, _, stderr := r.command("approve", map[string]string{"INPUTS_APPROVE": "1"})
			testutil.Equal(t, code, 1)
			if kind == "another-commit" {
				testutil.Contains(t, stderr, "HEAD changed")
			} else {
				testutil.Contains(t, stderr, "clean worktree")
			}
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
			testutil.Equal(t, r.git("status", "--porcelain"), status)
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, "owner.txt")))), "Owner's work\n")
			testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "result")), results)
			testutil.Equal(t, testutil.Must(record.ListKeyed(r.ctx.Dir, "answer")), answers)
		})
	}
}

func TestRequestedRevisionKeepsUnitIDAndRevertsByNewCommit(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	base := r.git("rev-parse", "HEAD")
	d := r.draft()
	path := ".devkit/standards/new-rule.md"
	r.put(path, "Proposed rule\n")
	d["standards"] = []doc{{"path": path, "entries": items(d["units"])[0]["entries"], "reason": "Correction", "evidence": "Backlog"}}
	d["checks"] = []doc{{"name": "proposed-check", "command": "true", "paths": []string{"**"}, "timeout_seconds": 5, "reason": "Correction", "entries": items(d["units"])[0]["entries"]}}
	c := testutil.Must(config.Load(r.cwd))
	c.Checks = append(c.Checks, config.Check{Name: "proposed-check", Command: "true", Paths: []string{"**"}, TimeoutSeconds: 5})
	r.json(".devkit/config.json", c)
	r.write(d)
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	testutil.Must(finish(r.ctx, r.artifacts))
	firstHead := r.git("rev-parse", "HEAD")
	firstKey := str(items(obj(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"])["units"])[0]["key"])
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Keep the unit, change its goal, withdraw the rule"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Equal(t, testutil.Must(prepare(r.ctx, r.artifacts))["empty"], false)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), firstHead)
	testutil.OK(t, os.Remove(filepath.Join(r.cwd, path)))
	r.put(".devkit/config.json", `{"language":"en"}`)
	d = r.draft()
	items(d["units"])[0]["goal"] = "A revised goal"
	r.write(d)
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	second := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, second["proposal_version"], 2)
	testutil.Equal(t, str(items(obj(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"])["units"])[0]["key"]), firstKey)
	testutil.Equal(t, r.git("rev-parse", "HEAD^"), firstHead)
	testutil.Equal(t, r.git("rev-list", "--count", base+"..HEAD"), "2")
	testutil.Contains(t, r.git("show", firstHead+":"+path), "Proposed rule")
	testutil.Equal(t, r.git("ls-tree", "--name-only", "HEAD", "--", path), "")
	testutil.Equal(t, len(testutil.Must(config.Load(r.cwd)).Checks), len(c.Checks)-1)
	testutil.Equal(t, str(items(obj(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"])["units"])[0]["goal"]), "A revised goal")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}

func TestOldSnapshotProposalIsRefusedWithoutApplyingFiles(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	p := testutil.Must(record.Latest(r.ctx.Dir, "proposal"))
	p["content"] = doc{"ready": true, "proposal_path": ".devkit/units/retro-old.json", "edits": doc{".devkit/standards/legacy.md": "Must not be applied"}, "backlog": doc{}}
	testutil.OK(t, writeJSON(filepath.Join(r.ctx.Dir, "proposals/1.json"), p))
	r.ctx.RunID = "r2"
	head := r.git("rev-parse", "HEAD")
	results := testutil.Must(record.List(r.ctx.Dir, "result"))
	code, out, _ := r.command("start", map[string]string{"INPUTS_APPROVE": "1"})
	testutil.Equal(t, code, 0)
	testutil.Equal(t, out["stopped"], true)
	testutil.Contains(t, str(out["reason"]), "old-format")
	testutil.Contains(t, str(out["reason"]), "start retro again")
	for _, mode := range []string{"approve", "prepare"} {
		code, _, stderr := r.command(mode, map[string]string{"INPUTS_APPROVE": "1"})
		testutil.Equal(t, code, 1)
		testutil.Contains(t, stderr, "old-format")
	}
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "result")), results)
	_, e := os.Stat(filepath.Join(r.cwd, ".devkit/standards/legacy.md"))
	testutil.Equal(t, os.IsNotExist(e), true)
}

func TestRevisedEscapeCasesRemainCommittedOrAreWithdrawnByCommit(t *testing.T) {
	for _, withdrawn := range []bool{false, true} {
		t.Run(map[bool]string{false: "retained", true: "withdrawn"}[withdrawn], func(t *testing.T) {
			r := escapeFixture(t)
			d := r.escapeDraft()
			r.put(casePath+"/base/asset.bin", string([]byte{0xff, 0xfe}))
			r.write(d)
			testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
			entry := testutil.Must(readJSON(file(r.artifacts, "rounds/latest.json")))
			testutil.OK(t, writeJSON(file(r.artifacts, "verification.json"), doc{"review": reviewHash(entry)}))
			testutil.OK(t, writeJSON(file(r.artifacts, "verification-text.json"), doc{"text": "Evaluation reviewed"}))
			testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "question")
			first := r.git("rev-parse", "HEAD")
			r.ctx.RunID = "r2"
			s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Revise the proposed correction"}]`}))
			testutil.OK(t, e)
			testutil.Equal(t, s.Stopped, false)
			r.artifacts = t.TempDir()
			testutil.Equal(t, testutil.Must(prepare(r.ctx, r.artifacts))["empty"], false)
			if withdrawn {
				d["cases"] = []doc{}
				d["dismissed"] = []doc{{"entries": []string{escapeID}, "reason": "The request did not cover this behavior"}}
				d["escapes"] = []doc{{"entry": escapeID, "stage": "none", "quality": "", "reason": "A request gap", "evidence": "Original request", "judge_should_catch": false}}
				testutil.OK(t, os.RemoveAll(filepath.Join(r.cwd, casePath)))
			} else {
				g := gitx.Git{Cwd: r.cwd}
				testutil.Equal(t, testutil.Must(g.Bytes("show", "HEAD:"+casePath+"/base/asset.bin")), []byte{0xff, 0xfe})
			}
			r.write(d)
			testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
			testutil.Equal(t, r.git("rev-parse", "HEAD^"), first)
			proposal := testutil.Must(record.Latest(r.ctx.Dir, "proposal"))
			testutil.Equal(t, num(proposal["version"]), 2)
			_, e = os.Stat(filepath.Join(r.cwd, casePath, "base/asset.bin"))
			testutil.Equal(t, os.IsNotExist(e), withdrawn)
			testutil.Equal(t, r.git("status", "--porcelain"), "")
		})
	}
}

func TestRejectedRevisionKeepsThePreviousProposalForTheNextRun(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	previous := testutil.Must(record.Latest(r.ctx.Dir, "proposal"))
	head := r.git("rev-parse", "HEAD")
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Revise this correction"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Must(prepare(r.ctx, r.artifacts))
	d := r.draft()
	d["units"] = []doc{}
	r.write(d)
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], false)
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
	testutil.Equal(t, testutil.Must(record.Latest(r.ctx.Dir, "proposal")), previous)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	r.ctx.RunID = "r3"
	s, e = start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Equal(t, testutil.Must(prepare(r.ctx, r.artifacts))["empty"], false)
	r.write(r.draft())
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	out := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, out["proposal_version"], 2)
	testutil.Equal(t, out["outcome"], "question")
	testutil.Equal(t, r.git("rev-parse", "HEAD^"), head)
}

func TestLaterRevisionsReadConsumedBacklogFromProposalHistory(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	r.json(".devkit/backlog/later.json", backlog())
	r.commit()
	for _, run := range []struct{ id, question string }{{"r2", "Q1"}, {"r3", "Q2"}} {
		r.ctx.RunID = run.id
		s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": fmt.Sprintf(`[{"question_id":%q,"answer":"Include both findings in the correction"}]`, run.question)}))
		testutil.OK(t, e)
		testutil.Equal(t, s.Stopped, false)
		r.artifacts = t.TempDir()
		testutil.Equal(t, testutil.Must(prepare(r.ctx, r.artifacts))["empty"], false)
		input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
		testutil.Equal(t, len(items(input["entries"])), 2)
		d := r.draft()
		refs := []string{}
		for _, entry := range items(input["entries"]) {
			refs = append(refs, str(entry["id"]))
		}
		items(d["units"])[0]["entries"] = refs
		r.write(d)
		testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
		testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "question")
		_, e = os.Stat(filepath.Join(r.cwd, ".devkit/backlog/later.json"))
		testutil.Equal(t, os.IsNotExist(e), true)
	}
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "proposal"))), 3)
}
