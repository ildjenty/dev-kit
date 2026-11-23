package fix

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func sha(c string) string { return strings.Repeat(c, 40) }
func finding(extra record.Document) record.Document {
	f := record.Document{"id": "F1", "blocking": true, "needs_decision": false, "text": "Reject invalid input", "evidence": "a.mjs:3"}
	for k, v := range extra {
		f[k] = v
	}
	return f
}
func judgment(extra record.Document) record.Document {
	j := record.Document{"kind": "code", "judge": "contract", "run_id": "j1", "target": record.Document{"base": sha("b"), "commit": sha("c")}, "findings": []record.Document{finding(nil)}}
	for k, v := range extra {
		j[k] = v
	}
	return j
}
func gateFixture() *checks.Gate {
	return &checks.Gate{Round: 1, Passed: true, Failing: []string{}, StuckChecks: []string{}, Skipped: []string{}, Results: "checks/1.jsonl"}
}
func preparedFixture() preparedFile {
	return preparedFile{prepared: prepared{Ready: true, Limit: 3}, FindingsList: []record.Document{{"id": "j1/contract/F1", "text": "Reject invalid input"}}}
}
func stateFixture() state {
	return state{Start: sha("c"), Round: 1, Gate: gateFixture(), OwnerDecisions: []record.Document{}, BrokenDecisions: []record.Document{}, Objections: objections{[]record.Document{}, []record.Document{}}}
}
func putJSON(t *testing.T, file string, value any) {
	t.Helper()
	testutil.OK(t, writeJSON(file, value))
}
func setup(t *testing.T, limit int) (record.ContextData, string, string, string) {
	t.Helper()
	root := t.TempDir()
	cwd := filepath.Join(root, "repo")
	testutil.OK(t, os.MkdirAll(cwd, 0755))
	testutil.Git(t, cwd, "init", "-q", "-b", "trunk")
	for key, value := range map[string]string{"user.name": "Test", "user.email": "test@example.invalid", "commit.gpgsign": "false", "core.hooksPath": "/dev/null"} {
		testutil.Git(t, cwd, "config", key, value)
	}
	putJSON(t, filepath.Join(cwd, ".devkit/config.json"), record.Document{"language": "fr", "standards": "standards", "limits": record.Document{"fixes": limit}, "checks": []any{}})
	testutil.Standards(t, cwd, "standards")
	testutil.Put(t, cwd, "standards/git.md", "Use `change:` or `repair:`.\n")
	testutil.Git(t, cwd, "add", "-A")
	testutil.Git(t, cwd, "commit", "-q", "-m", "base")
	base := testutil.Git(t, cwd, "rev-parse", "HEAD")
	testutil.Git(t, cwd, "checkout", "-q", "-b", "work")
	testutil.Put(t, cwd, "a.mjs", "export const valid = false;\n")
	testutil.Git(t, cwd, "add", "-A")
	testutil.Git(t, cwd, "commit", "-q", "-m", "change: accept input")
	head := testutil.Git(t, cwd, "rev-parse", "HEAD")
	ctx := record.ContextData{Cwd: cwd, Dir: filepath.Join(root, "records"), RunID: "f1", BaseBranch: "trunk"}
	_, err := record.Append(ctx.Dir, "request", record.Document{"owner_words": "Review this change", "supplement": "", "criteria_commit": base, "run_id": "j1", "recorded_at": "t"})
	testutil.OK(t, err)
	_, err = record.Append(ctx.Dir, "run", record.Document{"run_id": ctx.RunID, "workflow": "fix", "started_at": "t", "start": "next", "counted": false, "answers": []any{}})
	testutil.OK(t, err)
	j := judgment(record.Document{"target": record.Document{"base": base, "commit": head}, "criteria_commit": base, "recorded_at": "t", "materials": []any{}, "observations": []any{}, "verdict": "fail"})
	_, err = record.Append(ctx.Dir, "judgment", j)
	testutil.OK(t, err)
	return ctx, filepath.Join(root, "artifacts"), base, head
}
func TestSelectsOnlyBlockingNonOwnerFindingsFromLastJudgmentOnHEAD(t *testing.T) {
	judgments := []record.Document{judgment(record.Document{"run_id": "old", "findings": []record.Document{finding(record.Document{"id": "Old"})}}), judgment(record.Document{"findings": []record.Document{finding(nil), finding(record.Document{"id": "Owner", "needs_decision": true}), finding(record.Document{"id": "Advice", "blocking": false})}}), judgment(record.Document{"target": record.Document{"commit": sha("d")}})}
	_, findings := findingsOn(judgments, sha("c"), "")
	testutil.Equal(t, len(findings), 1)
	testutil.Equal(t, findings[0]["id"], "j1/contract/F1")
	judged, _ := findingsOn(judgments, sha("a"), "")
	testutil.Equal(t, judged, false)
	judgments = append(judgments, judgment(record.Document{"judge": "dispute", "findings": []record.Document{finding(record.Document{"id": "j1/contract/F1", "blocking": false})}}))
	_, findings = findingsOn(judgments, sha("c"), "")
	testutil.Equal(t, len(findings), 0)
}
func TestCountsFixesSinceLastImplementationIncludingStoppedFixes(t *testing.T) {
	testutil.Equal(t, len(fixesSinceImplement([]record.Document{{"workflow": "fix"}, {"workflow": "implement"}, {"workflow": "judge"}, {"workflow": "fix", "outcome": "stop"}})), 1)
	testutil.Equal(t, len(fixesSinceImplement([]record.Document{{"workflow": "fix"}})), 1)
}
func TestAcceptsOnlyEvidencedUniqueUndecidedObjectionsToFindingsOfThisFix(t *testing.T) {
	findings := []record.Document{{"id": "j1/contract/F1", "judge": "contract", "text": "Failure", "evidence": "Judge evidence"}, {"id": "test/F2"}}
	written := record.Document{"objections": []record.Document{{"finding": "missing", "evidence": "x"}, {"finding": "test/F2", "evidence": ""}, {"finding": "test/F2", "evidence": "x"}, {"finding": "j1/contract/F1", "evidence": "Objection evidence"}, {"finding": "j1/contract/F1", "evidence": "Second objection"}}}
	valid, invalid := checkObjections(written, findings, map[string]bool{"test/F2": true})
	testutil.Equal(t, len(invalid), 4)
	testutil.Equal(t, len(valid), 1)
	testutil.Equal(t, valid[0]["evidence"], "Objection evidence")
	testutil.Equal(t, valid[0]["finding_evidence"], "Judge evidence")
}
func TestRequiresExactlyOneAcceptedOrRejectedDecisionPerObjection(t *testing.T) {
	objections := []record.Document{{"finding": "j1/contract/F1"}}
	decision := record.Document{"finding": "j1/contract/F1", "decision": "accepted", "reason": "The failure cannot occur"}
	testutil.Fails(t, checkDecisions(objections, nil), "objections were")
	testutil.Fails(t, checkDecisions(objections, []record.Document{decision, decision}), "objections were")
	invalid := clone(decision)
	invalid["decision"] = "maybe"
	testutil.Fails(t, checkDecisions(objections, []record.Document{invalid}), "accepted or rejected")
	testutil.OK(t, checkDecisions(objections, []record.Document{decision}))
}
func TestRecordsAcceptedObjectionsAsNonblockingDisputeFindingsOnOriginalTarget(t *testing.T) {
	patch := sha("d")
	target := disputeTarget(state{Start: sha("c"), ChangeID: &patch}, sha("b"))
	doc := disputeJudgment(disputeRound{1, []record.Document{{"finding": "j1/contract/F1", "decision": "accepted", "reason": "Not reachable", "observations": []string{"a.mjs:3"}}}}, target)
	testutil.Equal(t, documents(doc["findings"])[0]["blocking"], false)
	testutil.Equal(t, object(doc["target"])["commit"], sha("c"))
	testutil.Equal(t, object(doc["target"])["change_id"], sha("d"))
	testutil.Equal(t, doc["verdict"], "pass")
	if _, ok := target["requirements_version"]; ok {
		t.Fatal("unexpected requirements_version")
	}
}
func TestStopsBeforeFixingWhenNotReadyOrConfiguredLimitReached(t *testing.T) {
	p := preparedFixture()
	p.Ready = false
	p.Reason = "No findings"
	testutil.Equal(t, outcome(p, stateFixture(), nil)["outcome"], "stop")
	p = preparedFixture()
	p.LimitReached = true
	p.Limit = 1
	result := outcome(p, stateFixture(), nil)
	testutil.Equal(t, result["outcome"], "stop")
	testutil.Contains(t, text(object(result["stop"])["state"]), "limit (1)")
}
func TestQuestionsTakePriorityAndBrokenPremiseGoesOnlyToEarlierStage(t *testing.T) {
	broken := stateFixture()
	broken.BrokenDecisions = []record.Document{{"row": "D1", "observation": "a.mjs:3"}}
	owner := broken
	owner.OwnerDecisions = []record.Document{{"target": "finding", "ref": "j1/contract/F1", "question": "Choose?"}}
	testutil.Equal(t, outcome(preparedFixture(), owner, nil)["outcome"], "back")
	for _, next := range []string{"requirements", "plan", "implement", "fix", "environment"} {
		want := "stop"
		if next == "requirements" || next == "plan" || next == "implement" {
			want = "back"
		}
		testutil.Equal(t, outcome(preparedFixture(), broken, &stepBack{next, "Premise", "Log"})["outcome"], want)
	}
	s := stateFixture()
	s.Gate.Passed = false
	s.Gate.Stuck = true
	testutil.Equal(t, outcome(preparedFixture(), s, &stepBack{"plan", "Premise", "Log"})["outcome"], "back")
}
func TestStopsAfterThreePassesForPendingRejectionsOrFailuresOtherwisePasses(t *testing.T) {
	s := stateFixture()
	s.Pending = 1
	s.Round = 3
	testutil.Equal(t, outcome(preparedFixture(), s, nil)["outcome"], "stop")
	s = stateFixture()
	s.Gate.Passed = false
	s.Round = 3
	testutil.Equal(t, outcome(preparedFixture(), s, nil)["outcome"], "stop")
	testutil.Equal(t, outcome(preparedFixture(), stateFixture(), nil)["outcome"], "pass")
}
func TestUsesRepositoryCommitPrefixes(t *testing.T) {
	prefixes := testutil.Must(commitPrefixes("Use `repair:` or `change:`."))
	testutil.Equal(t, commitSubject("repair: validate input", prefixes), "repair: validate input")
	testutil.Equal(t, commitSubject("fix: validate input", prefixes), "repair: address blocking findings")
}
func TestReadsConfiguredFixLimitAndLanguageForReviewOnlyRequest(t *testing.T) {
	ctx, artifacts, _, _ := setup(t, 0)
	result := testutil.Must(prepare(ctx, artifacts))
	testutil.Equal(t, result.Limit, 0)
	testutil.Equal(t, result.LimitReached, true)
	testutil.Equal(t, testutil.Must(proceed(artifacts))["proceed"], false)
	testutil.Equal(t, testutil.Must(readJSON[record.Document](filepath.Join(artifacts, "fix/input.json")))["language"], "fr")
	if testutil.Must(readJSON[record.Document](filepath.Join(artifacts, "fix/requirements.json"))) != nil {
		t.Fatal("unexpected requirements")
	}
	testutil.Equal(t, testutil.Must(finish(ctx, artifacts, nil))["outcome"], "stop")
}
func TestRejectedObjectionRequiresAnotherPassIsNotJudgedTwiceAndCommitsOnlyPassingChecks(t *testing.T) {
	ctx, artifacts, _, head := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	testutil.Equal(t, testutil.Must(proceed(artifacts))["proceed"], true)
	putJSON(t, filepath.Join(artifacts, "objections.json"), record.Document{"objections": []record.Document{{"finding": "j1/contract/F1", "evidence": "a.mjs:3 shows the opposite"}}})
	testutil.Equal(t, testutil.Must(takeObjections(artifacts))["count"], 1)
	testutil.Equal(t, testutil.Must(disputeEnd(artifacts, record.Document{"decisions": []record.Document{{"finding": "j1/contract/F1", "decision": "rejected", "reason": "The failure is reachable", "observations": []string{"a.mjs:3"}}}}))["pending"], 1)
	code := &codeOutput{CommitMessage: "repair: validate input"}
	gate := gateFixture()
	gate.Passed = false
	testutil.Equal(t, testutil.Must(roundEnd(ctx, artifacts, gate, code))["finished"], false)
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
	putJSON(t, filepath.Join(artifacts, "objections.json"), record.Document{"objections": []record.Document{{"finding": "j1/contract/F1", "evidence": "Try again"}}})
	testutil.Equal(t, testutil.Must(takeObjections(artifacts))["count"], 0)
	testutil.Equal(t, testutil.Must(disputeEnd(artifacts, nil))["pending"], 0)
	testutil.Put(t, ctx.Cwd, "a.mjs", "export const valid = true;\n")
	testutil.Equal(t, testutil.Must(roundEnd(ctx, artifacts, gateFixture(), code))["finished"], true)
	testutil.Contains(t, testutil.Git(t, ctx.Cwd, "log", "-1", "--format=%B"), "Archon-Run: f1\nArchon-Fixed-From: "+head)
	testutil.Equal(t, testutil.Must(finish(ctx, artifacts, nil))["outcome"], "pass")
	judgments := testutil.Must(record.List(ctx.Dir, "judgment"))
	testutil.Equal(t, documents(judgments[len(judgments)-1]["findings"])[0]["blocking"], true)
}
func TestPersistsAcceptedObjectionSoExcludedFromFindingsOfJudgedCommit(t *testing.T) {
	ctx, artifacts, _, head := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	putJSON(t, filepath.Join(artifacts, "objections.json"), record.Document{"objections": []record.Document{{"finding": "j1/contract/F1", "evidence": "The guard rejects this input"}}})
	testutil.Must(takeObjections(artifacts))
	testutil.Must(disputeEnd(artifacts, record.Document{"decisions": []record.Document{{"finding": "j1/contract/F1", "decision": "accepted", "reason": "The guard rejects it", "observations": []string{"a.mjs:3"}}}}))
	testutil.Must(roundEnd(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: "repair: retain the guard"}))
	testutil.Equal(t, testutil.Must(finish(ctx, artifacts, nil))["outcome"], "pass")
	_, findings := findingsOn(testutil.Must(record.List(ctx.Dir, "judgment")), head, "")
	testutil.Equal(t, len(findings), 0)
}
func TestStepBackRunsForStuckChecksOrBrokenDecisionsUnlessOwnerMustDecide(t *testing.T) {
	ctx, artifacts, _, _ := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	stuck, broken := stateFixture(), stateFixture()
	stuck.Gate.Stuck = true
	broken.BrokenDecisions = []record.Document{{"row": "D1", "observation": "a.mjs:3"}}
	file := filepath.Join(artifacts, "fix/state.json")
	for _, s := range []state{stuck, broken} {
		putJSON(t, file, s)
		testutil.Equal(t, testutil.Must(judgeBack(artifacts))["judge_back"], true)
		s.OwnerDecisions = []record.Document{{"target": "decision", "ref": "D1", "question": "Choose?"}}
		putJSON(t, file, s)
		testutil.Equal(t, testutil.Must(judgeBack(artifacts))["judge_back"], false)
	}
}
func TestAllowsConfiguredNumberOfFixesAndResetsCountAfterImplement(t *testing.T) {
	ctx, artifacts, base, head := setup(t, 1)
	testutil.Equal(t, testutil.Must(prepare(ctx, artifacts)).LimitReached, false)
	result := record.Document{"run_id": "previous-fix", "workflow": "fix", "recorded_at": "t", "outcome": "pass", "inputs": record.Document{"requirements_version": nil, "plan_version": nil}, "base": base, "commit": head, "criteria_commit": base, "loops": []any{}}
	_, err := record.Append(ctx.Dir, "result", result)
	testutil.OK(t, err)
	testutil.Equal(t, testutil.Must(prepare(ctx, filepath.Join(artifacts, "at-limit"))).LimitReached, true)
	result["run_id"], result["workflow"] = "new-implementation", "implement"
	_, err = record.Append(ctx.Dir, "result", result)
	testutil.OK(t, err)
	testutil.Equal(t, testutil.Must(prepare(ctx, filepath.Join(artifacts, "after-implement"))).LimitReached, false)
}

func TestCommandsUseEnvironmentAndPersistDisputes(t *testing.T) {
	ctx, _, _, _ := setup(t, 3)
	source := ctx.Dir
	ctx.Dir = record.RecordDir(filepath.Join(ctx.Cwd, "state"), "work")
	testutil.OK(t, os.MkdirAll(filepath.Dir(ctx.Dir), 0755))
	testutil.OK(t, os.Rename(source, ctx.Dir))
	vars := map[string]string{"STATE_DIR": "state", "WORKFLOW_ID": ctx.RunID, "BASE_BRANCH": ctx.BaseBranch, "ARTIFACTS_DIR": "artifacts"}
	registry := cli.New("test", Commands())
	run := func(mode string) record.Document {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := registry.Execute([]string{"fix", mode}, cli.Env{Cwd: ctx.Cwd, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }})
		if code != 0 {
			t.Fatalf("%s: %d: %s", mode, code, stderr.String())
		}
		var out record.Document
		testutil.OK(t, json.Unmarshal(stdout.Bytes(), &out))
		return out
	}
	testutil.Equal(t, run("prepare")["ready"], true)
	testutil.Equal(t, run("proceed")["proceed"], true)
	input := testutil.Must(readJSON[record.Document](filepath.Join(ctx.Cwd, "artifacts/fix/input.json")))
	testutil.Equal(t, input["criteria_folder"], "artifacts/criteria/standards")
	putJSON(t, filepath.Join(ctx.Cwd, "artifacts/objections.json"), record.Document{"objections": []record.Document{{"finding": "j1/contract/F1", "evidence": "The guard rejects it"}}})
	testutil.Equal(t, run("objections")["count"], 1)
	vars["DISPUTE"] = `{"decisions":[{"finding":"j1/contract/F1","decision":"accepted","reason":"Not reachable","observations":["a.mjs:3"]}]}`
	testutil.Equal(t, run("dispute-end")["pending"], 0)
	vars["GATE"] = string(testutil.Must(json.Marshal(gateFixture())))
	vars["CODE"] = `{"owner_decisions":[],"broken_decisions":[],"commit_message":"repair: retain the guard"}`
	testutil.Equal(t, run("round-end")["finished"], true)
	testutil.Equal(t, run("state")["judge_back"], false)
	testutil.Equal(t, run("finish")["outcome"], "pass")
	delete(vars, "ARTIFACTS_DIR")
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: ctx.Cwd, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
	testutil.Equal(t, registry.Execute([]string{"fix", "proceed"}, env), 1)
	testutil.Contains(t, stderr.String(), "ARTIFACTS_DIR must be set")
	testutil.Equal(t, registry.Execute([]string{"fix", "unknown"}, env), 2)
}

func TestAcceptedObjectionCannotSuppressFindingFromAnotherJudgeRun(t *testing.T) {
	old := judgment(nil)
	_, original := findingsOn([]record.Document{old}, sha("c"), "")
	accepted := judgment(record.Document{"judge": "dispute", "run_id": "fix-1", "findings": []record.Document{finding(record.Document{"id": original[0]["id"], "blocking": false})}})
	next := judgment(record.Document{"run_id": "j2", "findings": []record.Document{finding(record.Document{"text": "A new failure at the same commit"})}})
	_, remaining := findingsOn([]record.Document{old, accepted, next}, sha("c"), "")
	testutil.Equal(t, len(remaining), 1)
	testutil.Equal(t, remaining[0]["id"], "j2/contract/F1")
	_, remaining = findingsOn([]record.Document{old, accepted}, sha("c"), "")
	testutil.Equal(t, len(remaining), 0)
}

func TestReviewFixPreparesRecordedFindingsAfterBacklogAndRequiresJudgeAgain(t *testing.T) {
	ctx, artifacts, _, head := setup(t, 3)
	review := "Review comment by alice on a.mjs:1: reject invalid input\n  keep spaces\n"
	runs := testutil.Must(record.List(ctx.Dir, "run"))
	runs[0]["review"] = record.Document{"text": review, "commit": head}
	putJSON(t, filepath.Join(ctx.Dir, "runs/1.json"), runs[0])
	testutil.Put(t, ctx.Cwd, ".devkit/backlog/work.json", `{"deferred":[],"carried":[],"mechanical":[],"accepted_objections":[]}`)
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "backlog")
	testutil.Equal(t, testutil.Must(prepare(ctx, artifacts)).Ready, true)
	findings := testutil.Must(readJSON[record.Document](filepath.Join(artifacts, "fix/findings.json")))
	list := documents(findings["findings"])
	testutil.Equal(t, len(list), 1)
	testutil.Equal(t, list[0]["source"], "review")
	testutil.Equal(t, list[0]["text"], review)
	testutil.Must(takeObjections(artifacts))
	testutil.Must(disputeEnd(artifacts, nil))
	testutil.Put(t, ctx.Cwd, "a.mjs", "export const valid = true;\n")
	testutil.Must(roundEnd(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: "repair: validate input"}))
	testutil.Equal(t, testutil.Must(finish(ctx, artifacts, nil))["outcome"], "pass")
	reasons := testutil.Must(record.StartConditions(ctx.Dir, "pr", record.ConditionOptions{Commit: testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")}))
	testutil.Equal(t, len(reasons) > 0, true)
}

func TestOutsideReviewItemsReturnToRequirementsWithoutCommitOrDispute(t *testing.T) {
	for _, objection := range []bool{false, true} {
		t.Run(strconv.FormatBool(objection), func(t *testing.T) {
			ctx, artifacts, _, head := setup(t, 3)
			review := "Review comment by alice on a.mjs:1: add a public endpoint"
			runs := testutil.Must(record.List(ctx.Dir, "run"))
			runs[0]["review"] = record.Document{"text": review, "commit": head}
			putJSON(t, filepath.Join(ctx.Dir, "runs/1.json"), runs[0])
			testutil.Must(prepare(ctx, artifacts))
			item := record.Document{"item": review, "reason": "The requirements and plan do not cover a public endpoint"}
			code := &codeOutput{OutsideRequirements: []record.Document{item}}
			if objection {
				putJSON(t, filepath.Join(artifacts, "objections.json"), record.Document{"objections": []record.Document{{"finding": ctx.RunID + "/review", "evidence": "The reported failure cannot happen"}}})
				item["reason"] = "The reported failure cannot happen"
				code = nil
			}
			testutil.Equal(t, testutil.Must(takeObjections(artifacts))["count"], 0)
			testutil.Must(disputeEnd(artifacts, nil))
			gate := gateFixture()
			if objection {
				gate.Passed = false
				gate.Stuck = true
			}
			testutil.Equal(t, testutil.Must(roundEnd(ctx, artifacts, gate, code))["finished"], true)
			testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
			testutil.Equal(t, testutil.Must(judgeBack(artifacts))["judge_back"], false)
			result := testutil.Must(finish(ctx, artifacts, nil))
			testutil.Equal(t, result["outcome"], "back")
			testutil.Equal(t, object(result["back"]), record.Document{"to": "requirements", "premise": text(item["item"]) + ": " + text(item["reason"]), "observation": review})
			testutil.Equal(t, documents(result["outside_requirements"]), []record.Document{item})
			judgments := testutil.Must(record.List(ctx.Dir, "judgment"))
			testutil.Equal(t, len(judgments), 1)
			returned := testutil.Must(record.Returned(record.ContextData{Dir: ctx.Dir}, "requirements"))
			testutil.Equal(t, len(returned), 1)
			testutil.Equal(t, returned[0]["observation"], review)
		})
	}
}

func TestBaseFailureStopsFixWithoutPassOrStepBack(t *testing.T) {
	ctx, artifacts, _, head := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	testutil.Must(takeObjections(artifacts))
	testutil.Put(t, artifacts, "checks.jsonl", `{"name":"unit","status":"fail","base":"failed","log":"/checks/unit.log"}`+"\n")
	gate := &checks.Gate{Failing: []string{"unit"}, Preexisting: []string{"unit"}, Results: filepath.Join(artifacts, "checks.jsonl")}
	ended := testutil.Must(roundEnd(ctx, artifacts, gate, nil))
	testutil.Equal(t, ended["finished"], true)
	testutil.Equal(t, ended["stuck"], false)
	testutil.Equal(t, ended["round"], 0)
	testutil.Equal(t, testutil.Must(judgeBack(artifacts))["judge_back"], false)
	result := testutil.Must(finish(ctx, artifacts, nil))
	testutil.Equal(t, result["outcome"], "stop")
	stop := result["stop"].(record.Document)
	testutil.Equal(t, stop["base_failed"], true)
	testutil.Equal(t, stop["state"], checks.BaseMessage([]string{"unit"}))
	testutil.Equal(t, stop["unresolved"], []string{"unit: /checks/unit.log"})
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
	history := testutil.Must(record.History(ctx.Dir))
	testutil.Equal(t, testutil.Must(record.LimitState(history, "fix", nil, 0)).Start, "retry_after_base")
	testutil.Equal(t, testutil.Must(record.LimitState(history, "fix", nil, 0)).Counted, false)
}

func TestRegressedChecksStillSendFixToStepBack(t *testing.T) {
	ctx, artifacts, _, _ := setup(t, 3)
	testutil.Must(prepare(ctx, artifacts))
	testutil.Must(takeObjections(artifacts))
	gate := &checks.Gate{Failing: []string{"unit"}, Regressed: []string{"unit"}, Stuck: true, StuckChecks: []string{"unit"}}
	ended := testutil.Must(roundEnd(ctx, artifacts, gate, nil))
	testutil.Equal(t, ended["round"], 1)
	testutil.Equal(t, ended["stuck"], true)
	testutil.Equal(t, testutil.Must(judgeBack(artifacts))["judge_back"], true)
}
