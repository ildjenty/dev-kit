package retro

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
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

type fixture struct {
	t              *testing.T
	cwd, artifacts string
	ctx            record.ContextData
}

func envVars(vars map[string]string) cli.Env {
	return cli.Env{LookupEnv: func(k string) (string, bool) {
		v, ok := vars[k]
		if !ok && k == "BASE_BRANCH" {
			return "main", true
		}
		if !ok && k == "DEVKIT_GH" {
			return os.LookupEnv(k)
		}
		return v, ok
	}}
}
func finding() doc {
	return doc{"finding": doc{"id": "F1", "text": "A defect", "blocking": false, "needs_decision": false, "evidence": "path"}, "source": doc{"judge": "structure", "run_id": "judge-1", "judgment": 1, "kind": "code", "target": doc{"commit": strings.Repeat("a", 40)}}}
}
func backlog() doc {
	return doc{"deferred": []doc{finding()}, "carried": []doc{}, "mechanical": []doc{}, "accepted_objections": []doc{}}
}
func setup(t *testing.T, b doc) *fixture {
	t.Helper()
	testutil.IssueCreationCLI(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "repo")
	testutil.OK(t, os.Mkdir(cwd, 0755))
	r := &fixture{t: t, cwd: cwd, artifacts: filepath.Join(root, "artifacts")}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.name", "test")
	r.git("config", "user.email", "test@example.invalid")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "core.hooksPath", "/dev/null")
	testutil.Standards(t, cwd, ".devkit/standards")
	r.put(".devkit/config.json", `{"language":"en"}`)
	if b != nil {
		r.json(".devkit/backlog/topic.json", b)
	}
	r.commit()
	r.git("switch", "-qc", "retro/topic")
	testutil.PublicationRemote(t, cwd)
	r.ctx = record.ContextData{Cwd: cwd, Dir: record.RecordDir(filepath.Join(root, "state"), "retro/topic"), RunID: "r1", Branch: "retro/topic", BaseBranch: "main"}
	return r
}
func (r *fixture) git(args ...string) string { r.t.Helper(); return testutil.Git(r.t, r.cwd, args...) }
func (r *fixture) put(p, s string)           { r.t.Helper(); testutil.Put(r.t, r.cwd, p, s) }
func (r *fixture) json(p string, v any) {
	r.t.Helper()
	testutil.OK(r.t, writeJSON(filepath.Join(r.cwd, p), v))
}
func (r *fixture) commit() {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-qm", "test materials")
}
func (r *fixture) begin() doc {
	r.t.Helper()
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(r.t, e)
	testutil.Equal(r.t, s.Stopped, false)
	return testutil.Must(prepare(r.ctx, r.artifacts))
}
func (r *fixture) draft() doc {
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	return doc{"summary": "Repair the observed defect", "units": []doc{{"key": "repair", "title": "Repair defect", "goal": "Remove the defect", "completion_conditions": []string{"The failure no longer occurs"}, "blocked_by": []string{}, "entries": []string{str(items(input["entries"])[0]["id"])}}}, "checks": []doc{}, "standards": []doc{}, "dismissed": []doc{}, "escapes": []doc{}, "cases": []doc{}}
}
func (r *fixture) write(d doc) {
	r.t.Helper()
	testutil.OK(r.t, writeJSON(file(r.artifacts, "draft.json"), d))
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.OK(r.t, put(file(r.artifacts, "approval.txt"), fmt.Sprintf("Approve the traced correction with approve=%d, or provide answers for changes?", num(input["next_proposal_version"]))))
}
func judgment() doc {
	cs := []doc{}
	for _, a := range aspects {
		cs = append(cs, doc{"aspect": a, "verdict": "pass", "checked": "Checked the proposal against entries", "not_checked": ""})
	}
	return doc{"criteria": cs, "findings": []doc{}}
}
func (r *fixture) review(j doc) (doc, error) {
	r.t.Helper()
	if _, e := check(r.ctx, r.artifacts); e != nil {
		return nil, e
	}
	return round(r.ctx, r.artifacts, j)
}
func (r *fixture) propose() doc {
	r.t.Helper()
	r.write(r.draft())
	testutil.Equal(r.t, testutil.Must(r.review(judgment()))["valid"], true)
	return testutil.Must(finish(r.ctx, r.artifacts))
}
func (r *fixture) errors(d doc) string {
	r.t.Helper()
	return strings.Join(testutil.Must(draftErrors(r.ctx, r.artifacts, d)), "\n")
}
func TestProposalCommitsEverythingAndApprovalOnlyRecordsResult(t *testing.T) {
	b := backlog()
	b["carried"] = []doc{finding()}
	b["mechanical"] = []doc{finding()}
	b["accepted_objections"] = []doc{finding()}
	r := setup(t, b)
	r.json(".devkit/backlog/second.json", backlog())
	r.commit()
	r.git("branch", "-f", "main", "HEAD")
	r.begin()
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	entries := items(input["entries"])
	testutil.Equal(t, len(entries), 5)
	d := r.draft()
	refs := []string{}
	for _, e := range entries {
		if e["kind"] == "deferred" || e["kind"] == "carried" {
			refs = append(refs, str(e["id"]))
		}
	}
	items(d["units"])[0]["entries"] = refs
	for _, e := range entries {
		if e["kind"] == "mechanical" {
			d["checks"] = []doc{{"name": "prevent-defect", "command": "test -f example.txt", "paths": []string{"**"}, "timeout_seconds": 5, "reason": "Prevent the observed failure", "entries": []string{str(e["id"])}}}
		}
		if e["kind"] == "accepted_objections" {
			d["standards"] = []doc{{"path": ".devkit/standards/harness.md", "reason": "Clarify the counterexample", "evidence": "Accepted objection", "entries": []string{str(e["id"])}}}
		}
	}
	standard := string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/harness.md"))))
	r.put(".devkit/standards/harness.md", standard+"\nA clarification supported by the accepted counterexample.\n")
	c := testutil.Must(config.Load(r.cwd))
	c.Checks = append(c.Checks, config.Check{Name: "prevent-defect", Command: "test -f example.txt", Paths: []string{"**"}, TimeoutSeconds: 5})
	r.json(".devkit/config.json", c)
	r.write(d)
	testutil.Equal(t, r.errors(d), "")
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	out := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, out["outcome"], "question")
	testutil.Equal(t, len(items(out["questions"])), 1)
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/harness.md")))), "clarification supported")
	testutil.Equal(t, len(testutil.Must(config.Load(r.cwd)).Checks), len(c.Checks))
	testutil.Equal(t, len(items(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"].(map[string]any)["units"])), 1)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	proposal := testutil.Must(record.Latest(r.ctx.Dir, "proposal"))
	content := obj(proposal["content"])
	testutil.Equal(t, content["backlog"], nil)
	testutil.Equal(t, content["edits"], nil)
	testutil.Equal(t, content["proposal_path"], nil)
	head := r.git("rev-parse", "HEAD")
	testutil.Equal(t, content["commit"], head)
	testutil.Contains(t, str(items(out["questions"])[0]["text"]), head)
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "judgment"))), 1)
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, testutil.Must(record.Latest(r.ctx.Dir, "result"))["issue_numbers"], []any{float64(101)})
	testutil.Equal(t, len(testutil.Must(config.Load(r.cwd)).Checks), len(c.Checks))
	testutil.Contains(t, string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/harness.md")))), "clarification supported")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	changed := r.git("show", "--format=", "--name-only", "HEAD")
	for _, p := range []string{".devkit/backlog/topic.json", ".devkit/backlog/second.json", ".devkit/config.json", ".devkit/standards/harness.md"} {
		testutil.Contains(t, changed, p)
	}
	_, e = os.Stat(filepath.Join(r.cwd, ".devkit/backlog/topic.json"))
	testutil.Equal(t, os.IsNotExist(e), true)
	testutil.Equal(t, len(testutil.Must(record.ListKeyed(r.ctx.Dir, "answer"))), 1)
}
func TestEmptyPassWritesEmptyProposalWithoutQuestionOrCommit(t *testing.T) {
	for _, b := range []doc{nil, {"branch": "x", "commit": "c", "recorded_at": "t", "deferred": []doc{}, "carried": []doc{}, "mechanical": []doc{}, "accepted_objections": []doc{}}} {
		r := setup(t, b)
		before := r.git("rev-parse", "HEAD")
		testutil.Equal(t, r.begin()["empty"], true)
		out := testutil.Must(empty(r.ctx, r.artifacts))
		testutil.Equal(t, out["outcome"], "pass")
		testutil.Equal(t, out["questions"], []doc{})
		testutil.Equal(t, r.git("rev-parse", "HEAD"), before)
		testutil.Equal(t, obj(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"])["units"], []doc{})
	}
}
func TestBacklogReaderPreservesProvenanceAndRejectsMalformedShape(t *testing.T) {
	r := setup(t, backlog())
	entries, files, e := readBacklog(r.cwd)
	testutil.OK(t, e)
	testutil.Equal(t, len(entries), 1)
	testutil.Equal(t, obj(entries[0]["content"]), finding())
	testutil.Equal(t, len(files), 1)
	for _, mutate := range []func(doc){func(b doc) { delete(items(b["deferred"])[0], "source") }, func(b doc) { b["carried"] = nil }, func(b doc) { b["deferred"] = []string{"bad"} }, func(b doc) { delete(obj(items(b["deferred"])[0]["source"]), "run_id") }, func(b doc) { b["accepted_objections"] = []doc{{"finding": doc{"id": "F1"}}} }} {
		b := backlog()
		mutate(b)
		r.json(".devkit/backlog/topic.json", b)
		_, _, e = readBacklog(r.cwd)
		if e == nil {
			t.Fatal("malformed backlog accepted")
		}
	}
	r.put(".devkit/backlog/topic.json", "{broken")
	_, _, e = readBacklog(r.cwd)
	if e == nil {
		t.Fatal("malformed JSON accepted")
	}
}
func TestMachineGateValidatesCoverageSchemaConfigStandardsAndTrace(t *testing.T) {
	for _, tc := range []struct {
		name, part string
		change     func(*fixture, doc)
	}{
		{"unknown", "unknown backlog", func(r *fixture, d doc) { items(d["units"])[0]["entries"] = []string{"missing"} }},
		{"unhandled", "neither used nor dismissed", func(r *fixture, d doc) { d["units"] = []doc{} }},
		{"conflicting dismissal", "both used and dismissed", func(r *fixture, d doc) {
			d["dismissed"] = []doc{{"entries": items(d["units"])[0]["entries"], "reason": "No action"}}
		}},
		{"duplicate", "duplicate key", func(r *fixture, d doc) { d["units"] = append(items(d["units"]), items(d["units"])[0]) }},
		{"schema", "required property", func(r *fixture, d doc) { delete(d, "summary") }},
		{"bad config", "invalid .devkit/config", func(r *fixture, d doc) { r.put(".devkit/config.json", `{"checks":[{"name":"x"}]}`) }},
		{"settings", "preserve other settings", func(r *fixture, d doc) { r.put(".devkit/config.json", `{"language":"ja"}`) }},
		{"missing check", "proposed check additions", func(r *fixture, d doc) {
			d["checks"] = []doc{{"name": "x", "command": "true", "paths": []string{"**"}, "timeout_seconds": 1, "reason": "Evidence", "entries": items(d["units"])[0]["entries"]}}
		}},
		{"invalid map", "missing quality", func(r *fixture, d doc) { r.put(".devkit/standards/concerns.json", `{}`) }},
		{"untraced standard", "no backlog trace", func(r *fixture, d doc) { r.put(".devkit/standards/new.md", "New standard") }},
		{"outside standard", "outside the standards", func(r *fixture, d doc) {
			d["standards"] = []doc{{"path": "other.md", "reason": "Reason", "evidence": "Evidence", "entries": items(d["units"])[0]["entries"]}}
		}},
		{"unedited standard", "no checkout change", func(r *fixture, d doc) {
			d["standards"] = []doc{{"path": ".devkit/standards/harness.md", "reason": "Reason", "evidence": "Evidence", "entries": items(d["units"])[0]["entries"]}}
		}},
		{"direct unit", "outside the allowed paths", func(r *fixture, d doc) { r.put(".devkit/units/U-123456.md", "Direct unit") }},
		{"writer commit", "writer changed HEAD", func(r *fixture, d doc) { r.put("hidden.txt", "committed"); r.commit() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t, backlog())
			r.begin()
			d := r.draft()
			r.write(d)
			tc.change(r, d)
			testutil.Contains(t, r.errors(d), tc.part)
		})
	}
}
func TestDismissalNeedsReasonAndCanClear(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	d := r.draft()
	d["dismissed"] = []doc{{"entries": items(d["units"])[0]["entries"], "reason": "Already repaired on the base branch"}}
	d["units"] = []doc{}
	r.write(d)
	testutil.Equal(t, r.errors(d), "")
	delete(items(d["dismissed"])[0], "reason")
	testutil.Contains(t, r.errors(d), "required property")
}
func TestScopeChecksIndexAndWorktreeSeparately(t *testing.T) {
	for _, kind := range []string{"untracked", "unstaged", "staged", "staged-hidden-by-worktree"} {
		t.Run(kind, func(t *testing.T) {
			r := setup(t, backlog())
			r.put("outside.txt", "before")
			r.commit()
			r.begin()
			d := r.draft()
			r.write(d)
			if kind == "untracked" {
				r.put("new.txt", "outside")
			} else {
				r.put("outside.txt", "after")
				if strings.HasPrefix(kind, "staged") {
					r.git("add", "outside.txt")
				}
				if kind == "staged-hidden-by-worktree" {
					r.put("outside.txt", "before")
				}
			}
			testutil.Contains(t, r.errors(d), "outside the allowed paths")
			for i := 0; i < 3; i++ {
				testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], false)
			}
			head := r.git("rev-parse", "HEAD")
			testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
		})
	}
}
func TestJudgmentDecisionPointsAndRoundLimit(t *testing.T) {
	for _, verdict := range []string{"fail", "not_assessed", "missing", "duplicate", "blocking", "needs_decision", "schema"} {
		t.Run(verdict, func(t *testing.T) {
			r := setup(t, backlog())
			r.begin()
			r.write(r.draft())
			j := judgment()
			switch verdict {
			case "missing":
				j["criteria"] = items(j["criteria"])[:2]
			case "duplicate":
				j["criteria"] = append(items(j["criteria"]), items(j["criteria"])[0])
			case "blocking", "needs_decision":
				j["findings"] = []doc{{"aspect": "mechanism", "item": "repair", "mechanical": false, "blocking": verdict == "blocking", "needs_decision": verdict == "needs_decision", "failure_scenario": "Failure", "evidence": "Evidence", "fix": "Fix"}}
			case "schema":
				j = doc{}
			default:
				items(j["criteria"])[0]["verdict"] = verdict
			}
			for i := 0; i < 3; i++ {
				out := testutil.Must(r.review(j))
				if out["valid"] == true && num(out["blocking_count"]) == 0 && num(out["not_assessed_count"]) == 0 {
					t.Fatal("bad judgment cleared")
				}
			}
			_, e := r.review(j)
			testutil.Fails(t, e, "three rounds")
			testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
			if verdict != "schema" {
				recorded := testutil.Must(record.Latest(r.ctx.Dir, "judgment"))
				if recorded["verdict"] == "pass" {
					t.Fatal("failed judgment recorded as pass")
				}
			}
		})
	}
}
func TestFinishRechecksEditsAndMissingDraftStops(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts))["valid"], false)
	testutil.OK(t, put(file(r.artifacts, "draft.json"), "{broken"))
	_, err := r.review(judgment())
	testutil.Fails(t, err, file(r.artifacts, "draft.json"))
	testutil.Fails(t, err, "invalid character")
	testutil.OK(t, os.Remove(file(r.artifacts, "draft.json")))
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
	r = setup(t, backlog())
	r.begin()
	d := r.draft()
	ref := items(d["units"])[0]["entries"]
	d["standards"] = []doc{{"path": ".devkit/standards/harness.md", "entries": ref, "reason": "Clarify", "evidence": "Counterexample"}}
	original := string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/harness.md"))))
	r.put(".devkit/standards/harness.md", original+"\nFirst clarification\n")
	r.write(d)
	testutil.Must(r.review(judgment()))
	r.put(".devkit/standards/harness.md", original+"\nUnreviewed clarification\n")
	out := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, out["outcome"], "stop")
	testutil.Contains(t, strings.Join(stringsOf(out["reasons"]), "\n"), "after the judgment")
	testutil.Contains(t, r.git("status", "--porcelain"), ".devkit/standards/harness.md")
}
func TestApprovalRequiresStartAndExactRecordedAnswerBeforeWrites(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	head := r.git("rev-parse", "HEAD")
	r.ctx.RunID = "unregistered"
	_, e := approve(r.ctx, "1")
	testutil.Fails(t, e, "passed start")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Change the proposal"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	_, e = approve(r.ctx, "1")
	testutil.Fails(t, e, "recorded approval answer")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
}
func TestStartRejectsWrongBranchOldApprovalChangedHeadAndMixedAnswers(t *testing.T) {
	r := setup(t, backlog())
	r.ctx.Branch = "feature/topic"
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "run")), []doc{})
	r.ctx.Branch = "retro/topic"
	r.begin()
	r.propose()
	r.ctx.RunID = "r2"
	for _, vars := range []map[string]string{{"INPUTS_APPROVE": "2"}, {"INPUTS_APPROVE": "1", "INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Yes"}]`}} {
		s, e = start(r.ctx, envVars(vars))
		testutil.OK(t, e)
		testutil.Equal(t, s.Stopped, true)
	}
	r.put("extra.txt", "changed")
	r.commit()
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	testutil.Contains(t, s.Reason, "HEAD changed")
	testutil.Equal(t, len(testutil.Must(record.ListKeyed(r.ctx.Dir, "answer"))), 0)
}
func TestCorrectionWithdrawsUnitsWithoutLeavingGeneratedRecords(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Dismiss the entry because it was fixed"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	r.artifacts = filepath.Join(filepath.Dir(r.artifacts), "correction")
	testutil.Must(prepare(r.ctx, r.artifacts))
	d := r.draft()
	d["dismissed"] = []doc{{"entries": items(d["units"])[0]["entries"], "reason": "Fixed"}}
	d["units"] = []doc{}
	r.write(d)
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["proposal_version"], 2)
	r.ctx.RunID = "r3"
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "2"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	testutil.Equal(t, testutil.Must(approve(r.ctx, "2"))["outcome"], "pass")
	_, e = os.Stat(filepath.Join(r.cwd, ".devkit/units"))
	testutil.Equal(t, os.IsNotExist(e), true)
}
func TestPrepareReadsAndValidatesFixedMapAndWritesSchemas(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	for _, name := range []string{"draft", "judgment"} {
		b := testutil.Must(os.ReadFile(file(r.artifacts, name+".schema.json")))
		testutil.Equal(t, string(b), string(testutil.Must(schemas.ReadFile("schemas/"+name+".schema.json"))))
	}
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Contains(t, str(input["standards_index"]), "harness.md")
	testutil.Equal(t, str(obj(input["config"])["language"]), "en")
	r = setup(t, backlog())
	r.put(".devkit/standards/concerns.json", `{}`)
	r.commit()
	r.git("branch", "-f", "main", "HEAD")
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	_, e = prepare(r.ctx, r.artifacts)
	testutil.Fails(t, e, "missing quality")
}
func TestCommandsAbsoluteAndRelativePaths(t *testing.T) {
	for _, relative := range []bool{false, true} {
		t.Run(fmt.Sprint(relative), func(t *testing.T) {
			r := setup(t, backlog())
			vars := map[string]string{"STATE_DIR": filepath.Dir(filepath.Dir(r.ctx.Dir)), "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": r.artifacts}
			if relative {
				vars["STATE_DIR"] = "../state"
				vars["ARTIFACTS_DIR"] = "../artifacts"
			}
			run := func(mode string) doc {
				t.Helper()
				var out, errOut bytes.Buffer
				env := envVars(vars)
				env.Cwd = r.cwd
				env.Stdout = &out
				env.Stderr = &errOut
				code := cli.New("test", Commands()).Execute([]string{"retro", mode}, env)
				if code != 0 {
					t.Fatalf("%s: %s", mode, errOut.String())
				}
				var d doc
				testutil.OK(t, json.Unmarshal(out.Bytes(), &d))
				return d
			}
			testutil.Equal(t, run("start")["stopped"], false)
			testutil.Equal(t, run("prepare")["input"], file(vars["ARTIFACTS_DIR"], "input.json"))
			r.write(r.draft())
			testutil.Equal(t, run("check")["valid"], true)
			vars["INPUTS_JUDGMENT"] = string(testutil.Must(json.Marshal(judgment())))
			testutil.Equal(t, run("round")["valid"], true)
			testutil.Equal(t, run("finish")["outcome"], "question")
			vars["WORKFLOW_ID"] = "r2"
			vars["INPUTS_APPROVE"] = "1"
			testutil.Equal(t, run("start")["stopped"], false)
			testutil.Equal(t, run("approve")["outcome"], "pass")
			vars["INPUTS_START"] = `{"reason":"Rejected","remaining":["Reason"]}`
			testutil.Equal(t, run("not-started")["outcome"], "not_started")
		})
	}
}

func TestApprovalQuestionCannotChangeAfterReview(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.write(r.draft())
	testutil.Must(r.review(judgment()))
	testutil.OK(t, put(file(r.artifacts, "approval.txt"), "Unreviewed question approve=1 or answers?"))
	out := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, out["outcome"], "stop")
	testutil.Contains(t, strings.Join(stringsOf(out["reasons"]), "\n"), "question changed")
}
func TestApprovalRefusesDirtyCheckoutBeforeApplying(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.propose()
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	head := r.git("rev-parse", "HEAD")
	r.put("outside.txt", "Do not include")
	_, e = approve(r.ctx, "1")
	testutil.Fails(t, e, "clean worktree")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	_, e = os.Stat(filepath.Join(r.cwd, ".devkit/backlog/topic.json"))
	testutil.Equal(t, os.IsNotExist(e), true)
}
func TestStagedStandardCannotHideDifferentWorktreeContent(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	d := r.draft()
	original := string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, ".devkit/standards/harness.md"))))
	r.put(".devkit/standards/harness.md", original+"\nStaged edit\n")
	r.git("add", ".devkit/standards/harness.md")
	r.put(".devkit/standards/harness.md", original)
	d["standards"] = []doc{{"path": ".devkit/standards/harness.md", "entries": items(d["units"])[0]["entries"], "reason": "Clarify", "evidence": "Evidence"}}
	r.write(d)
	testutil.Contains(t, r.errors(d), "index and worktree disagree")
}
func TestStopPreservesCheckoutWithoutRestoringFiles(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	d := r.draft()
	r.write(d)
	r.put(".devkit/standards/harness.md", "Invalid replacement")
	r.git("add", ".devkit/standards/harness.md")
	r.put(".devkit/standards/new.md", "New rule")
	r.put("outside.txt", "Keep this")
	before := r.git("status", "--porcelain")
	testutil.Must(r.review(judgment()))
	out := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, out["outcome"], "stop")
	testutil.Equal(t, r.git("status", "--porcelain"), before)
}
func TestRestartLimitRequiresRecordedContinuation(t *testing.T) {
	r := setup(t, backlog())
	r.json(".devkit/config.json", doc{"language": "en", "limits": doc{"restarts": 1}})
	r.commit()
	r.begin()
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	testutil.Must(r.review(judgment()))
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
	r.ctx.RunID = "r3"
	s, e = start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_CONTINUE": "Proceed with another attempt"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "continuation"))), 1)
}
func TestRejectedStartDoesNotSaveAnyPartialAnswers(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	o := testutil.Must(resultOptions(r.ctx))
	testutil.Must(record.WriteResult(r.ctx.Dir, "retro", doc{"outcome": "question", "questions": []doc{{"text": "First?", "target": doc{"kind": "proposal", "ref": "1"}, "next_workflows": []string{"retro"}, "invalidates": []string{}}, {"text": "Second?", "target": doc{"kind": "proposal", "ref": "1"}, "next_workflows": []string{"retro"}, "invalidates": []string{}}}}, o))
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"First answer"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	testutil.Equal(t, len(testutil.Must(record.ListKeyed(r.ctx.Dir, "answer"))), 0)
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"First answer"},{"question_id":"Q2","answer":"Second answer"}]`}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
}
func TestCommandsReportMissingContextArtifactsAndMalformedInputs(t *testing.T) {
	r := setup(t, backlog())
	for _, tc := range []struct {
		mode, part string
		vars       map[string]string
	}{
		{"start", "STATE_DIR and WORKFLOW_ID", nil},
		{"prepare", "ARTIFACTS_DIR", map[string]string{"STATE_DIR": "../state", "WORKFLOW_ID": "r1"}},
		{"round", "invalid character", map[string]string{"STATE_DIR": "../state", "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": "../artifacts", "INPUTS_JUDGMENT": "invalid"}},
		{"not-started", "invalid character", map[string]string{"STATE_DIR": "../state", "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": "../artifacts", "INPUTS_START": "invalid"}},
	} {
		var out, errOut bytes.Buffer
		env := envVars(tc.vars)
		env.Cwd = r.cwd
		env.Stdout = &out
		env.Stderr = &errOut
		testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"retro", tc.mode}, env), 1)
		testutil.Contains(t, errOut.String(), tc.part)
		testutil.Equal(t, out.Len(), 0)
	}
}

func TestF12RetroKeepsEveryRoundAndResolution(t *testing.T) {
	r := setup(t, backlog())
	r.begin()
	r.write(r.draft())
	j := judgment()
	j["findings"] = []doc{{"aspect": "evidence", "item": "repair", "blocking": true, "needs_decision": false, "mechanical": true, "failure_scenario": "The proposal lacks evidence", "evidence": "backlog", "fix": "Cite the source"}}
	testutil.Must(r.review(j))
	r.write(r.draft())
	testutil.Must(r.review(judgment()))
	testutil.Must(finish(r.ctx, r.artifacts))
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, items(judgments[0]["findings"])[0]["resolved"], true)
	testutil.Equal(t, items(judgments[0]["findings"])[0]["mechanical"], true)
}

func TestReviewBacklogRequiresRunProvenance(t *testing.T) {
	for _, review := range []any{nil, "text", []doc{{"finding": doc{"id": "review-1", "text": "Review text"}, "source": doc{"run_id": "fix1", "kind": "code", "target": doc{"commit": strings.Repeat("a", 40)}}}}, []doc{{"finding": doc{"id": "review-1", "text": "Review text"}, "source": doc{"run_id": "fix1", "kind": "review", "target": doc{}}}}} {
		b := backlog()
		b["review"] = review
		r := setup(t, b)
		_, _, err := readBacklog(r.cwd)
		if err == nil {
			t.Fatalf("invalid review provenance accepted: %v", review)
		}
	}
}
