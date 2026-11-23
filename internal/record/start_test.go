package record

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestFirstRun(t *testing.T) {
	dir := newRecord(t)
	out := begin(dir, "requirements", "r1")
	testutil.Equal(t, []any{out.Stopped, out.Start, out.Counted}, []any{false, "first", false})
	testutil.Equal(t, must(List(dir, "run")), []Document{{"run_id": "r1", "workflow": "requirements", "started_at": "t", "start": "first", "counted": false, "answers": []any{}}})
}
func TestDirtyWorkflowsRecordNothing(t *testing.T) {
	dir := newRecord(t)
	out := must(Start(dir, "requirements", StartOptions{RunID: "r1", Commit: sha("d"), BaseCommit: sha("c"), Now: "t", WorkflowsChanged: []string{" M .archon/workflows/dev/judge/judge.yaml"}}))
	testutil.Equal(t, out.Stopped, true)
	testutil.Contains(t, out.Reason, "not committed")
	testutil.Equal(t, must(List(dir, "run")), []Document{})
}
func TestDirtyWorktreeRecordsNothing(t *testing.T) {
	dir := newRecord(t)
	out := must(Start(dir, "plan", StartOptions{RunID: "p1", Commit: sha("d"), BaseCommit: sha("c"), Now: "t", WorktreeChanged: []string{"M  src/auth/issue.ts"}}))
	testutil.Equal(t, out.Stopped, true)
	testutil.Contains(t, out.Reason, "worktree of the branch")
	testutil.Equal(t, must(List(dir, "run")), []Document{})
}
func TestMissingInputsRecordNothing(t *testing.T) {
	dir := newRecord(t)
	out := begin(dir, "plan", "p1")
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"no requirements are recorded yet"})
	testutil.Equal(t, runIDs(dir), []string{})
}
func TestNextWorkflowAfterPass(t *testing.T) {
	out := begin(afterRequirements(t), "plan", "p1")
	testutil.Equal(t, []any{out.Stopped, out.Start}, []any{false, "next"})
}
func TestWaitsForAnswer(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	requirements(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("Which option?", []string{"requirements"})}})
	blocked := begin(dir, "plan", "p1")
	testutil.Equal(t, blocked.Stopped, true)
	testutil.Equal(t, blocked.Remaining, []string{"requirements version 1 is the latest, and no valid result of requirements passed on it", "Q1 (asked by requirements, run r1) has no answer"})
	out := begin(dir, "requirements", "r2", []Answer{{"Q1", "Choose A"}})
	testutil.Equal(t, []any{out.Stopped, out.Start, out.Counted}, []any{false, "answer", false})
	answer := must(ReadKeyed(dir, "answer", "Q1"))
	testutil.Equal(t, []any{answer["answer"], answer["question"], answer["run_id"], answer["versions"]}, []any{"Choose A", "Which option?", "r2", Document{"requirements_version": 1, "plan_version": nil}})
	h := must(History(dir))
	testutil.Equal(t, h.Runs[len(h.Runs)-1]["answers"], []string{"Q1"})
}
func TestInvalidAnswerBatchRecordsNothing(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("First question", []string{"requirements"}), ask("Second question", []string{"plan"}, "decision")}})
	out := begin(dir, "requirements", "r2", []Answer{{"Q1", "Yes"}, {"Q2", "Yes"}, {"Q9", "Yes"}})
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"Q2 is to be followed by plan, not requirements", "Q9 is not a question in the record"})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1")), nil)
	testutil.Equal(t, runIDs(dir), []string{"r1"})
}
func TestAnswerRecordedOnce(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("First question", []string{"requirements"})}})
	begin(dir, "requirements", "r2", []Answer{{"Q1", "Yes"}})
	out := begin(dir, "requirements", "r3", []Answer{{"Q1", "No"}})
	testutil.Equal(t, out.Remaining, []string{"Q1 is already answered"})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1"))["answer"], "Yes")
}
func TestRejectedStartDoesNotSaveAnswers(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "implement", "i1")
	end(dir, "implement", "i1", Document{"outcome": "pass"})
	begin(dir, "judge", "j1")
	judgment(dir, "j1", "scope", nil)
	end(dir, "judge", "j1", Document{"outcome": "pass"})
	begin(dir, "pr", "pr1")
	end(dir, "pr", "pr1", Document{"outcome": "question", "questions": []Document{ask("First question", []string{"pr"}, "change"), ask("Second question", []string{"pr"}, "change")}})
	out := begin(dir, "pr", "pr2", []Answer{{"Q1", "Allow"}})
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"Q2 (asked by pr, run pr1) has no answer"})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1")), nil)
	testutil.Equal(t, begin(dir, "pr", "pr3", []Answer{{"Q1", "Allow"}, {"Q2", "Allow"}}).Stopped, false)
	testutil.Equal(t, strings.Contains(strings.Join(runIDs(dir), ","), "pr2"), false)
}
func TestRejectsInvalidatedQuestion(t *testing.T) {
	dir := afterRequirements(t)
	begin(dir, "plan", "p1")
	end(dir, "plan", "p1", Document{"outcome": "question", "questions": []Document{ask("What should D1 choose?", []string{"plan"}, "decision")}})
	testutil.Equal(t, begin(dir, "requirements", "r2").Stopped, false)
	out := begin(dir, "plan", "p2", []Answer{{"Q1", "A"}})
	testutil.Equal(t, out.Remaining, []string{"Q1 was asked by a result that a later start of an earlier workflow made invalid"})
}

func TestLatestPlanNeedsPassingResult(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "plan", "p2")
	plan(dir, "p2")
	out := begin(dir, "implement", "i1")
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"plan version 2 is the latest, and no valid result of plan passed on it"})
}
func TestBackInvalidatesLaterResults(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "implement", "i1")
	end(dir, "implement", "i1", Document{"outcome": "pass"})
	begin(dir, "judge", "j1")
	judgment(dir, "j1", "scope", nil)
	end(dir, "judge", "j1", Document{"outcome": "pass"})
	begin(dir, "pr", "pr1")
	end(dir, "pr", "pr1", Document{"outcome": "back", "back": Document{"to": "plan", "premise": "Premise", "observation": "Observation"}})
	back := begin(dir, "plan", "p2")
	testutil.Equal(t, []any{back.Stopped, back.Start, back.Counted, back.Restarts}, []any{false, "step_back", true, 1})
	plan(dir, "p2")
	end(dir, "plan", "p2", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "pr", "pr2").Remaining, []string{"judge has no valid result, so no commit passed the judgment"})
	testutil.Equal(t, begin(dir, "implement", "i2").Stopped, false)
}
func TestEveryImplementationNeedsPlan(t *testing.T) {
	testutil.Equal(t, begin(afterRequirements(t), "implement", "i1").Remaining, []string{"the latest requirements have no plan"})
}
func TestOldRequirementsPlanIsIgnored(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "requirements", "r2")
	requirements(dir, "r2")
	end(dir, "requirements", "r2", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "implement", "i1").Remaining, []string{"the latest requirements have no plan"})
	testutil.Equal(t, begin(dir, "plan", "p2").Stopped, false)
}
func TestJudgeAndSyncEmptyBranch(t *testing.T) {
	testutil.Equal(t, begin(newRecord(t), "judge", "j1").Stopped, false)
	testutil.Equal(t, begin(newRecord(t), "sync", "s1").Stopped, false)
}
func TestReviewFixesCriteriaCommit(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "judge", "j1")
	testutil.Equal(t, must(List(dir, "request")), []Document{{"owner_words": "", "supplement": "", "criteria_commit": sha("c"), "run_id": "j1", "recorded_at": "t"}})
	testutil.Equal(t, end(dir, "judge", "j1", Document{"outcome": "pass"})["criteria_commit"], sha("c"))
	must(Start(dir, "sync", StartOptions{RunID: "s1", Commit: sha("d"), BaseCommit: sha("f"), Now: "t"}))
	testutil.Equal(t, len(must(List(dir, "request"))), 1)
}
func TestRequirementsOwnRequestAndMissingBase(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	begin(dir, "plan", "p1")
	testutil.Equal(t, must(List(dir, "request")), []Document{})
	missing := newRecord(t)
	_, err := Start(missing, "judge", StartOptions{RunID: "j1", Commit: sha("d"), Now: "t"})
	testutil.Fails(t, err, "base branch's commit")
	testutil.Equal(t, must(List(missing, "run")), []Document{})
}
func TestJudgmentChecksExistingRequirements(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "requirements", "r1")
	request(dir)
	requirements(dir)
	end(dir, "requirements", "r1", stopped())
	testutil.Equal(t, begin(dir, "judge", "j1").Remaining, []string{"requirements version 1 is the latest, and no valid result of requirements passed on it"})
}
func TestFixRequiresBlockingFindingsAtHEAD(t *testing.T) {
	testutil.Equal(t, begin(judged(t, stopped(), []Document{finding(true)}), "fix", "f1").Stopped, false)
	testutil.Equal(t, begin(judged(t, Document{"outcome": "pass"}, []Document{finding(false)}), "fix", "f1").Remaining, []string{"the latest result of judge (run j1) has no blocking finding"})
	testutil.Equal(t, beginAt(judged(t, stopped(), []Document{finding(true)}), "fix", "f1", sha("e"), sha("a")).Remaining, []string{"the latest result of judge (run j1) judged " + sha("d") + ", but HEAD is " + sha("e")})
	testutil.Equal(t, begin(newRecord(t), "fix", "f1").Remaining, []string{"judge has no valid result, so there are no findings to fix"})
}
func TestPRRequiresPassedJudgedPatch(t *testing.T) {
	testutil.Equal(t, begin(judged(t, Document{"outcome": "pass"}, []Document{}), "pr", "pr1").Stopped, false)
	testutil.Equal(t, begin(judged(t, stopped(), []Document{finding(true)}), "pr", "pr1").Remaining, []string{"the latest result of judge (run j1) is stop"})
	testutil.Equal(t, beginAt(judged(t, Document{"outcome": "pass"}, []Document{}), "pr", "pr1", sha("e"), sha("f")).Remaining, []string{"the latest result of judge (run j1) judged the change of " + sha("d") + ", and the change of HEAD " + sha("e") + " (change-id " + sha("f") + ") is not the one it judged"})
}
func TestPRSamePatchAfterSync(t *testing.T) {
	dir := judged(t, Document{"outcome": "pass"}, []Document{})
	begin(dir, "pr", "pr1")
	begin(dir, "sync", "s1")
	must(WriteResult(dir, "sync", Document{"outcome": "pass", "loops": []Document{{"name": "conflicts", "ended": "cleared", "rounds": 0}}}, ResultOptions{RunID: "s1", Base: sha("c"), Commit: sha("e"), Now: "t"}))
	testutil.Equal(t, beginAt(dir, "pr", "pr2", sha("e"), strings.Repeat("a", 64)).Stopped, false)
	testutil.Contains(t, beginAt(dir, "pr", "pr3", sha("e"), sha("f")).Remaining[0], "is not the one it judged")
}
func TestExactJudgedCommitDoesNotNeedChangeID(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "judge", "j1")
	judgment(dir, "j1", "scope", Document{"target": Document{"requirements_version": 1, "plan_version": 1, "base": sha("b"), "commit": sha("d")}})
	end(dir, "judge", "j1", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "pr", "pr1").Stopped, false)
}
func TestJudgeAgainAfterFix(t *testing.T) {
	dir := judged(t, stopped(), []Document{finding(true)})
	begin(dir, "fix", "f1")
	end(dir, "fix", "f1", Document{"outcome": "pass"})
	again := begin(dir, "judge", "j2")
	testutil.Equal(t, []any{again.Stopped, again.Counted}, []any{false, false})
}
func TestFourthRestartDoesNotRegister(t *testing.T) {
	dir := atLimit(t)
	out := begin(dir, "plan", "p4")
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"the branch already restarted 3 times (limit 3) and this start (retry_after_error) would be another; it starts when the owner's words that let it go on are passed as continue"})
	testutil.Equal(t, strings.Contains(strings.Join(runIDs(dir), ","), "p4"), false)
}
func TestOwnerContinuationAtLimit(t *testing.T) {
	dir := atLimit(t)
	out := goOn(dir, "plan", "p4", "Continue", nil)
	testutil.Equal(t, []any{out.Stopped, out.Start, out.Counted, out.Restarts}, []any{false, "retry_after_error", true, 0})
	testutil.Equal(t, must(List(dir, "continuation")), []Document{{"by": "owner", "answer": "Continue", "restarts": 3, "limit": 3, "run_id": "p4", "workflow": "plan", "start": "retry_after_error", "recorded_at": "t"}})
	counts := []int{}
	for _, id := range []string{"p5", "p6", "p7"} {
		counts = append(counts, begin(dir, "plan", id).Restarts)
	}
	testutil.Equal(t, counts, []int{1, 2, 3})
	testutil.Equal(t, begin(dir, "plan", "p8").Stopped, true)
}
func TestPrematureContinuationRecordsNothing(t *testing.T) {
	dir := afterRequirements(t)
	out := goOn(dir, "plan", "p1", "Continue", nil)
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"this start (next) is not counted, and the branch restarted 0 times (limit 3); start it without continue"})
	testutil.Equal(t, runIDs(dir), []string{"r1"})
	testutil.Equal(t, must(List(dir, "continuation")), []Document{})
}
func TestContinuationWithAnswersRecordsNeither(t *testing.T) {
	dir := atLimit(t)
	end(dir, "plan", "p3", Document{"outcome": "question", "questions": []Document{ask("Which option?", []string{"plan"}, "decision")}})
	out := goOn(dir, "plan", "p4", "Continue", []Answer{{"Q1", "A"}})
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1")), nil)
	testutil.Equal(t, must(List(dir, "continuation")), []Document{})
}
func TestOtherConditionBlocksContinuation(t *testing.T) {
	dir := newRecord(t)
	for _, id := range []string{"p1", "p2", "p3"} {
		run(dir, id, "plan", Document{"start": "retry_after_error", "counted": true})
	}
	out := goOn(dir, "plan", "p4", "Continue", nil)
	testutil.Equal(t, out.Stopped, true)
	testutil.Equal(t, out.Remaining, []string{"no requirements are recorded yet"})
	testutil.Equal(t, must(List(dir, "continuation")), []Document{})
}
func TestStartIsIdempotent(t *testing.T) {
	dir := afterRequirements(t)
	begin(dir, "plan", "p1")
	again := begin(dir, "plan", "p1")
	testutil.Equal(t, []any{again.Stopped, again.Start}, []any{false, "next"})
	testutil.Equal(t, runIDs(dir), []string{"r1", "p1"})
}
func TestStartCLIAnswers(t *testing.T) {
	repo := testutil.Repo(t)
	state := t.TempDir()
	dir := RecordDir(state, "feature/x")
	begin(dir, "requirements", "r1")
	request(dir)
	end(dir, "requirements", "r1", Document{"outcome": "question", "questions": []Document{ask("Which option?", []string{"requirements"})}})
	var out, stderr bytes.Buffer
	r := cli.New("test", Commands())
	code := r.Execute([]string{"record", "start", "requirements"}, commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r2", "INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Choose B"}]`}, &out, &stderr))
	testutil.Equal(t, code, 0)
	var actual StartOutput
	testutil.OK(t, json.Unmarshal(out.Bytes(), &actual))
	testutil.Equal(t, actual, StartOutput{Stage: "start", Details: dir, Remaining: []string{}, Start: "answer"})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1"))["answer"], "Choose B")
}
func TestStartCLIContinuation(t *testing.T) {
	repo := testutil.Repo(t)
	state := t.TempDir()
	dir := RecordDir(state, "feature/x")
	begin(dir, "requirements", "r1")
	request(dir)
	for _, id := range []string{"r2", "r3", "r4"} {
		run(dir, id, "requirements", Document{"start": "retry_after_error", "counted": true})
	}
	r := cli.New("test", Commands())
	startWith := func(words string) StartOutput {
		var out, stderr bytes.Buffer
		testutil.Equal(t, r.Execute([]string{"record", "start", "requirements"}, commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r5", "INPUTS_CONTINUE": words}, &out, &stderr)), 0)
		var actual StartOutput
		testutil.OK(t, json.Unmarshal(out.Bytes(), &actual))
		return actual
	}
	testutil.Equal(t, startWith("  ").Stopped, true)
	testutil.Equal(t, startWith("Continue").Stopped, false)
	testutil.Equal(t, must(List(dir, "continuation"))[0]["answer"], "Continue")
}
func TestRefineAndSeparateRequirements(t *testing.T) {
	testutil.Equal(t, Workflows[0], "refine")
	dir := newRecord(t)
	testutil.Equal(t, begin(dir, "refine", "f1").Stopped, false)
	testutil.Equal(t, must(List(dir, "request")), []Document{})
	testutil.Equal(t, begin(newRecord(t), "requirements", "r1").Stopped, false)
}
func TestProposalVersionStamping(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "refine", "f1")
	request(dir)
	ctx := ContextData{Dir: dir, RunID: "f1"}
	first := must(Write(ctx, "proposal", Document{"content": Document{"decisions": []any{}, "units": []any{}}}, "t"))
	second := must(Write(ctx, "proposal", Document{"content": Document{"memo": "Revised"}}, "t"))
	testutil.Equal(t, strings.HasSuffix(first.Path, "/proposals/1.json"), true)
	testutil.Equal(t, []any{first.Entry["version"], second.Entry["version"], second.Entry["request"], second.Entry["run_id"]}, []any{1, 2, 1, "f1"})
	testutil.Equal(t, must(Latest(dir, "proposal")), second.Entry)
	_, err := Write(ctx, "proposal", Document{"version": 3, "content": Document{}}, "")
	testutil.Fails(t, err, "leave them out")
}
func TestStrictAnswers(t *testing.T) {
	testutil.Equal(t, must(ParseAnswers("  ")), []Answer{})
	testutil.Equal(t, must(ParseAnswers(`[{"question_id":"Q1","answer":"Yes"}]`)), []Answer{{"Q1", "Yes"}})
	for _, text := range []string{"not JSON", "null", "{}", "[null]", `[{"question_id":"Q1","answer":" "}]`, `[{"question_id":"Q1","answer":1}]`, `[{"question_id":"Q1","answer":"Yes","extra":1}]`} {
		_, err := ParseAnswers(text)
		testutil.Fails(t, err, "answers")
	}
}
func TestDuplicateAnswersWriteNothing(t *testing.T) {
	dir := newRecord(t)
	begin(dir, "refine", "f1")
	request(dir)
	end(dir, "refine", "f1", Document{"outcome": "question", "questions": []Document{ask("Choose", []string{"refine"})}})
	testutil.Equal(t, begin(dir, "refine", "f2", []Answer{{"Q1", "A"}, {"Q1", "A"}}).Remaining, []string{"Q1 is answered twice in the input"})
	testutil.Equal(t, must(ReadKeyed(dir, "answer", "Q1")), nil)
}
func TestConfiguredLimitAndBaseBranchCLI(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "branch", "trunk", "main")
	testutil.Git(t, repo, "branch", "-D", "main")
	testutil.Put(t, repo, ".devkit/config.json", `{"limits":{"restarts":0}}`)
	testutil.Git(t, repo, "add", ".devkit")
	testutil.Git(t, repo, "commit", "-qm", "settings")
	state := t.TempDir()
	dir := RecordDir(state, "feature/x")
	run(dir, "r1", "requirements", nil)
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r2", "INPUTS_ANSWERS": "", "INPUTS_CONTINUE": "", "BASE_BRANCH": "trunk"}
	r := cli.New("test", Commands())
	var out, stderr bytes.Buffer
	env := commandEnv(repo, vars, &out, &stderr)
	testutil.Equal(t, r.Execute([]string{"record", "limit", "requirements"}, env), 0)
	var limited LimitStatus
	testutil.OK(t, json.Unmarshal(out.Bytes(), &limited))
	testutil.Equal(t, []any{limited.Limit, limited.Reached}, []any{0, true})
	out.Reset()
	testutil.Equal(t, r.Execute([]string{"record", "start", "requirements"}, env), 0)
	var started StartOutput
	testutil.OK(t, json.Unmarshal(out.Bytes(), &started))
	testutil.Equal(t, started.Stopped, true)
	testutil.Equal(t, runIDs(dir), []string{"r1"})
	testutil.Equal(t, must(Context(env)).BaseBranch, "trunk")
	vars["BASE_BRANCH"] = ""
	testutil.Equal(t, must(Context(env)).BaseBranch, "main")
}

func TestImplementationDerivedPlansUseTheirPassedAncestor(t *testing.T) {
	dir := afterPlan(t)
	testutil.Equal(t, begin(dir, "implement", "i1").Stopped, false)
	parent := must(Latest(dir, "plan"))
	body := clone(parent)
	for _, key := range []string{"version", "requirements_version", "run_id", "recorded_at"} {
		delete(body, key)
	}
	body["derived_from"] = 1
	must(Write(ContextData{Dir: dir, RunID: "i1"}, "plan", body, "t"))
	testutil.Equal(t, begin(dir, "judge", "before-result").Stopped, true)
	end(dir, "implement", "i1", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "judge", "after-result").Stopped, false)
	testutil.Equal(t, begin(dir, "implement", "i2").Stopped, false)
	body["derived_from"] = 2
	must(Write(ContextData{Dir: dir, RunID: "i2"}, "plan", body, "t"))
	end(dir, "implement", "i2", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "judge", "after-second-derivation").Stopped, false)
}

func TestDownstreamOwnerDecisionsSurviveTheStartOfPlanAndClearOnPass(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "implement", "i1")
	rows := []Document{{"id": "D9", "what": "Choose audience", "decided_by": "owner"}}
	end(dir, "implement", "i1", BackToPlan(rows))
	testutil.Equal(t, begin(dir, "plan", "p2").Stopped, false)
	pending := must(PendingOwnerRows(dir))
	testutil.Equal(t, pending, rows)
	plan(dir, "p2")
	end(dir, "plan", "p2", Document{"outcome": "pass"})
	testutil.Equal(t, must(PendingOwnerRows(dir)), []Document{})
}

func TestF15StartResolvesRemoteOnlyBase(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "update-ref", "refs/remotes/origin/main", "main")
	testutil.Git(t, repo, "branch", "-D", "main")
	vars := map[string]string{"STATE_DIR": t.TempDir(), "WORKFLOW_ID": "r1", "BASE_BRANCH": "main"}
	var out, stderr bytes.Buffer
	env := commandEnv(repo, vars, &out, &stderr)
	testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
	testutil.Contains(t, out.String(), `"stopped":false`)
}

func TestRequirementsUnitStartConditions(t *testing.T) {
	for _, tc := range []struct {
		name, marker, reason string
		recorded             bool
	}{
		{"ready", "", "", false},
		{"done", "U-abcdef", "unit U-abcdef is already done", false},
		{"waiting", "waiting", "unit U-abcdef waits for U-fedcba", false},
		{"ready dependency", "U-fedcba", "", false},
		{"recorded done unit", "U-abcdef", "unit U-abcdef is already done", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state := testutil.Repo(t), t.TempDir()
			deps := "[]"
			if tc.marker == "waiting" || tc.marker == "U-fedcba" {
				deps = "[U-fedcba]"
			}
			for _, id := range []string{"U-abcdef", "U-fedcba"} {
				unitDeps := "[]"
				if id == "U-abcdef" {
					unitDeps = deps
				}
				testutil.Put(t, repo, ".devkit/units/"+id+".md", "---\nid: "+id+"\ntitle: Unit\ndecisions: []\ndepends_on: "+unitDeps+"\n---\nGoal\n")
			}
			if tc.marker != "" && tc.marker != "waiting" {
				testutil.Put(t, repo, ".devkit/units/"+tc.marker+".done.json", `{}`)
			}
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "units")
			dir := RecordDir(state, "feature/x")
			input := "U-abcdef"
			if tc.recorded {
				testutil.Put(t, dir, "request/1.json", `{"unit":"U-abcdef"}`)
				input = ""
			}
			var out, stderr bytes.Buffer
			env := commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r1", "INPUTS_UNIT": input}, &out, &stderr)
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
			var got StartOutput
			testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
			testutil.Equal(t, got.Stopped, tc.reason != "")
			if tc.reason != "" {
				testutil.Equal(t, got.Remaining, []string{tc.reason})
				testutil.Equal(t, len(must(List(dir, "run"))), 0)
				testutil.Equal(t, len(must(ListKeyed(dir, "answer"))), 0)
			}
		})
	}
}

func TestReviewFixStartUsesPRIdentityAndRecordsVerbatimInput(t *testing.T) {
	for _, tc := range []struct {
		name, commit, change string
		review               bool
		blocked              bool
	}{
		{"head", sha("d"), strings.Repeat("a", 64), true, false},
		{"backlog", sha("e"), strings.Repeat("a", 64), true, false},
		{"changed", sha("e"), strings.Repeat("b", 64), true, true},
		{"without review", sha("d"), strings.Repeat("a", 64), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := afterPlan(t)
			begin(dir, "judge", "j1")
			judgment(dir, "j1", "scope", nil)
			end(dir, "judge", "j1", Document{"outcome": "pass"})
			begin(dir, "pr", "pr1")
			end(dir, "pr", "pr1", Document{"outcome": "pass"})
			review := ""
			if tc.review {
				review = "Review comment by alice on a.go:3: fix this\n  verbatim\n"
			}
			out := must(Start(dir, "fix", StartOptions{RunID: "f1", Commit: tc.commit, Review: review, ChangeID: func() (*string, error) { return &tc.change, nil }, Now: "t"}))
			testutil.Equal(t, out.Stopped, tc.blocked)
			runs := must(List(dir, "run"))
			if !tc.blocked {
				testutil.Equal(t, object(runs[len(runs)-1]["review"]), Document{"text": review, "commit": tc.commit})
				testutil.Equal(t, beginAt(dir, "pr", "pr2", sha("f"), strings.Repeat("b", 64)).Stopped, true)
			} else {
				testutil.Equal(t, runs[len(runs)-1]["run_id"], "pr1")
			}
		})
	}
}

func TestReviewFixRequiresPassedRequirementsPlanAndValidJudge(t *testing.T) {
	for _, stage := range []string{"empty", "requirements", "plan", "judge", "blocked judge", "invalidated judge"} {
		t.Run(stage, func(t *testing.T) {
			dir := newRecord(t)
			if stage != "empty" {
				dir = afterRequirements(t)
			}
			if stage != "empty" && stage != "requirements" {
				begin(dir, "plan", "p1")
				plan(dir)
				end(dir, "plan", "p1", Document{"outcome": "pass"})
			}
			if stage == "judge" || stage == "blocked judge" || stage == "invalidated judge" {
				begin(dir, "judge", "j1")
				judgment(dir, "j1", "scope", nil)
				doc := Document{"outcome": "pass"}
				if stage == "blocked judge" {
					doc = stopped()
				}
				end(dir, "judge", "j1", doc)
			}
			if stage == "invalidated judge" {
				begin(dir, "plan", "p2")
				plan(dir, "p2")
				end(dir, "plan", "p2", Document{"outcome": "pass"})
			}
			out := must(Start(dir, "fix", StartOptions{RunID: "f1", Commit: sha("d"), Review: "CI job test failed: error", Now: "t"}))
			testutil.Equal(t, out.Stopped, stage != "judge" && stage != "blocked judge")
		})
	}
}

func TestRequirementsReopensOnlyItsOwnPreviouslyPublishedUnit(t *testing.T) {
	for _, tc := range []struct {
		name, markerBranch string
		published          bool
		blocked            bool
	}{
		{"own published", "feature/x", true, false},
		{"other branch", "feature/other", true, true},
		{"unpublished", "feature/x", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, state := testutil.Repo(t), t.TempDir()
			testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\nid: U-abcdef\ntitle: Unit\ndecisions: []\ndepends_on: []\n---\nGoal\n")
			testutil.Put(t, repo, ".devkit/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"`+tc.markerBranch+`"}`)
			testutil.Git(t, repo, "add", ".")
			testutil.Git(t, repo, "commit", "-qm", "unit marker")
			dir := RecordDir(state, "feature/x")
			request(dir)
			saved := must(Latest(dir, "request"))
			saved["unit"] = "U-abcdef"
			data := must(json.Marshal(saved))
			testutil.Put(t, dir, "request/1.json", string(data))
			if tc.published {
				result(dir, "pr1", "pr", "pass", nil)
			}
			var out, stderr bytes.Buffer
			env := commandEnv(repo, map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "r2"}, &out, &stderr)
			testutil.Equal(t, cli.New("", Commands()).Execute([]string{"record", "start", "requirements"}, env), 0)
			var got StartOutput
			testutil.OK(t, json.Unmarshal(out.Bytes(), &got))
			testutil.Equal(t, got.Stopped, tc.blocked)
		})
	}
}

func TestReviewFixRequiresNewJudgeEvenWhenChangeIdentityIsUnchanged(t *testing.T) {
	dir := afterPlan(t)
	begin(dir, "judge", "j1")
	judgment(dir, "j1", "scope", nil)
	end(dir, "judge", "j1", Document{"outcome": "pass"})
	out := must(Start(dir, "fix", StartOptions{RunID: "f1", Commit: sha("d"), Review: "Review comment by alice: verify input", Now: "t"}))
	testutil.Equal(t, out.Stopped, false)
	end(dir, "fix", "f1", Document{"outcome": "pass"})
	testutil.Contains(t, begin(dir, "pr", "pr1").Remaining[0], "run judge again")
	begin(dir, "judge", "j2")
	judgment(dir, "j2", "scope", nil)
	end(dir, "judge", "j2", Document{"outcome": "pass"})
	testutil.Equal(t, begin(dir, "pr", "pr2").Stopped, false)
}
