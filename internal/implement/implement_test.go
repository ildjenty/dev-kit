package implement

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func requirementsFixture() record.Document {
	return record.Document{"version": 1, "goal": "Allow cancellation", "completion_conditions": []record.Document{{"id": "C1", "text": "Cancel a booking"}}}
}
func rowFixture(extra record.Document) record.Document {
	row := record.Document{"id": "D1", "what": "Represent cancellation", "chosen": "An explicit state", "candidates": []string{"An explicit state", "Deletion"}, "principles": []string{"designing.md principle 5"}, "evidence": []record.Document{{"source": "author", "text": "The existing state is explicit"}}, "decided_by": "provisional", "when": "now", "decide_when": "", "quality": "data", "touches": []string{"src/state.mjs"}}
	for k, v := range extra {
		row[k] = v
	}
	return row
}
func planFixture() record.Document {
	return record.Document{"version": 1, "requirements_version": 1, "run_id": "p1", "recorded_at": "t", "skeleton_commit": strings.Repeat("a", 40), "decisions": []record.Document{}, "increments": []record.Document{{"id": "U1", "summary": "Allow cancellation", "criteria": []string{"C1"}}}, "tests": []record.Document{}, "kept_behaviors": []record.Document{}}
}
func stateFixture() state {
	units := unitsOf(requirementsFixture(), planFixture())
	units[0].Done = true
	return state{Round: 1, Units: units, OwnerDecisions: []ownerDecision{}, DecisionErrors: []string{}}
}
func gateFixture() *checks.Gate {
	return &checks.Gate{Round: 1, Passed: true, Failing: []string{}, StuckChecks: []string{}, Skipped: []string{}, Results: "checks/1.jsonl"}
}
func raw(value any) json.RawMessage { return testutil.Must(json.Marshal(value)) }
func putJSON(t *testing.T, file string, value any) {
	t.Helper()
	testutil.OK(t, writeJSON(file, value))
}
func setup(t *testing.T, withPlan bool) (record.ContextData, string) {
	t.Helper()
	root := t.TempDir()
	cwd := filepath.Join(root, "repo")
	testutil.OK(t, os.MkdirAll(cwd, 0755))
	testutil.Git(t, cwd, "init", "-q", "-b", "trunk")
	for key, value := range map[string]string{"user.name": "Test", "user.email": "test@example.invalid", "commit.gpgsign": "false", "core.hooksPath": "/dev/null"} {
		testutil.Git(t, cwd, "config", key, value)
	}
	putJSON(t, filepath.Join(cwd, ".devkit/config.json"), record.Document{"language": "fr", "standards": "standards", "checks": []record.Document{{"name": "unit", "command": "node --test", "paths": []string{"src/**"}, "timeout_seconds": 30}}})
	testutil.Standards(t, cwd, "standards")
	testutil.Put(t, cwd, "standards/git.md", "Use `change:` or `repair:` for commits.\n")
	testutil.Git(t, cwd, "add", "-A")
	testutil.Git(t, cwd, "commit", "-q", "-m", "base")
	base := testutil.Git(t, cwd, "rev-parse", "HEAD")
	testutil.Git(t, cwd, "checkout", "-q", "-b", "work")
	ctx := record.ContextData{Cwd: cwd, Dir: filepath.Join(root, "records"), RunID: "i1", BaseBranch: "trunk"}
	_, err := record.Append(ctx.Dir, "request", record.Document{"owner_words": "Cancel a booking", "supplement": "", "criteria_commit": base, "run_id": "r1", "recorded_at": "t"})
	testutil.OK(t, err)
	req := requirementsFixture()
	delete(req, "version")
	req["request"], req["constraints"], req["open_points"], req["run_id"], req["recorded_at"] = 1, []string{}, []any{}, "r1", "t"
	_, err = record.Append(ctx.Dir, "requirements", req)
	testutil.OK(t, err)
	if withPlan {
		_, err = record.Append(ctx.Dir, "plan", planFixture())
		testutil.OK(t, err)
	}
	_, err = record.Append(ctx.Dir, "run", record.Document{"run_id": ctx.RunID, "workflow": "implement", "started_at": "t", "start": "next", "counted": false, "answers": []any{}})
	testutil.OK(t, err)
	return ctx, filepath.Join(root, "artifacts")
}

func TestSelectsPlanIncrements(t *testing.T) {
	units := unitsOf(requirementsFixture(), planFixture())
	testutil.Equal(t, len(units), 1)
	testutil.Equal(t, units[0].ID, "U1")
	testutil.Equal(t, units[0].Summary, "Allow cancellation")
	testutil.Equal(t, units[0].Criteria, []string{"C1"})
	units = unitsOf(requirementsFixture(), nil)
	testutil.Equal(t, len(units), 0)
}
func TestResumesOnlyUnitsWithTrailersForSameInputVersions(t *testing.T) {
	message := "change: allow cancellation\n\nArchon-Unit: U1\nArchon-Inputs: requirements=1 plan=1\n"
	testutil.Equal(t, doneUnits([]string{message}, inputsKey(1, 1)), map[string]bool{"U1": true})
	testutil.Equal(t, len(doneUnits([]string{message}, inputsKey(1, 2))), 0)
	testutil.Equal(t, len(doneUnits([]string{"Archon-Unit: U1"}, inputsKey(1, nil))), 0)
}
func TestTakesPrefixesFromFixedCriteriaAndAvoidsIDsOrMultilineSubjects(t *testing.T) {
	prefixes := testutil.Must(commitPrefixes("Use `change:` or `repair:`."))
	testutil.Equal(t, prefixes, []string{"change", "repair"})
	testutil.Equal(t, testutil.Must(commitPrefixes("Use change: or repair: for a subject.")), prefixes)
	u := unitsOf(requirementsFixture(), planFixture())[0]
	testutil.Equal(t, commitSubject("repair: preserve cancellation", u, requirementsFixture(), planFixture(), prefixes), "repair: preserve cancellation")
	for _, value := range []string{"feat: wrong prefix", "repair: U1: cancellation", "repair: C1 cancellation", "repair: first\nsecond"} {
		testutil.Equal(t, commitSubject(value, u, requirementsFixture(), nil, prefixes), "change: Allow cancellation")
	}
	_, err := commitPrefixes("No prefixes")
	testutil.Fails(t, err, "git.md does not name any commit prefixes")
}
func TestRejectsMalformedDuplicateAndOwnerWrittenDecisionRows(t *testing.T) {
	errors, merged, err := checkRows(planFixture(), nil)
	testutil.OK(t, err)
	testutil.Equal(t, errors, []string{})
	if merged != nil {
		t.Fatal("unexpected merge")
	}
	for _, value := range []any{[]any{}, record.Document{"rows": []any{nil}}, record.Document{"rows": []record.Document{{"id": "D1"}}}, nil} {
		errors, _, err = checkRows(planFixture(), raw(value))
		testutil.OK(t, err)
		if len(errors) == 0 {
			t.Fatalf("accepted invalid rows: %v", value)
		}
	}
	errors, _, err = checkRows(planFixture(), raw(record.Document{"rows": []record.Document{rowFixture(nil), rowFixture(nil)}}))
	testutil.OK(t, err)
	testutil.Contains(t, strings.Join(errors, "\n"), "twice")
	errors, _, err = checkRows(planFixture(), raw(record.Document{"rows": []record.Document{rowFixture(record.Document{"decided_by": "owner"})}}))
	testutil.OK(t, err)
	testutil.Contains(t, strings.Join(errors, "\n"), "plan")
	errors, _, err = checkRows(planFixture(), raw(record.Document{"rows": []record.Document{rowFixture(nil)}}))
	testutil.OK(t, err)
	testutil.Equal(t, len(errors), 0)
}
func TestQuestionsTakePriorityOverStuckCheckAndNeverPublishPlan(t *testing.T) {
	s := stateFixture()
	s.OwnerDecisions = []ownerDecision{{"D1", "Which storage?"}}
	s.Stuck = &stuckChecks{Failing: []string{"unit"}, StuckChecks: []string{"unit"}}
	doc, plan, err := outcome(s, planFixture(), raw(record.Document{"rows": []record.Document{rowFixture(nil)}}), &stepBack{Next: "plan"})
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "back")
	testutil.Equal(t, doc["back"].(map[string]any)["to"], "plan")
	if plan != nil {
		t.Fatal("published plan")
	}
}
func TestRoutesStuckCheckBackOnlyToEarlierStagesOtherwiseStops(t *testing.T) {
	for _, next := range []string{"requirements", "plan", "implement", "environment"} {
		s := stateFixture()
		s.Stuck = &stuckChecks{Failing: []string{"unit"}, StuckChecks: []string{"unit"}}
		doc, _, err := outcome(s, planFixture(), nil, &stepBack{next, "Premise", "Log"})
		testutil.OK(t, err)
		want := "stop"
		if next == "requirements" || next == "plan" {
			want = "back"
		}
		testutil.Equal(t, doc["outcome"], want)
	}
}
func TestStopsForRemainingUnitsInvalidRowsNoPlanDecisionsOrDeferredDecisions(t *testing.T) {
	s := stateFixture()
	s.Units[0].Done = false
	s.Round = 5
	doc, _, err := outcome(s, planFixture(), nil, nil)
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "stop")
	doc, _, err = outcome(stateFixture(), planFixture(), raw(record.Document{"rows": []record.Document{{}}}), nil)
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "stop")
	doc, _, err = outcome(stateFixture(), nil, raw(record.Document{"rows": []record.Document{rowFixture(nil)}}), nil)
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "stop")
	plan := planFixture()
	plan["decisions"] = []record.Document{rowFixture(record.Document{"when": "later", "decide_when": "After implementation"})}
	doc, _, err = outcome(stateFixture(), plan, nil, nil)
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "stop")
	doc, _, err = outcome(stateFixture(), planFixture(), nil, nil)
	testutil.OK(t, err)
	testutil.Equal(t, doc["outcome"], "pass")
}
func TestCommitsPassingUnitAndResumesNextUnitWithConfiguredChecksAndLanguage(t *testing.T) {
	ctx, artifacts := setup(t, true)
	plan := planFixture()
	plan["increments"] = append(documents(plan["increments"]), record.Document{"id": "U2", "summary": "Report cancellation", "criteria": []string{"C1"}})
	_, err := record.Append(ctx.Dir, "plan", plan)
	testutil.OK(t, err)
	testutil.Equal(t, testutil.Must(prepare(ctx, artifacts))["remaining"], 2)
	testutil.Equal(t, testutil.Must(next(artifacts))["tests_written"], false)
	input := testutil.Must(readJSON[record.Document](filepath.Join(artifacts, "implement/unit.json")))
	testutil.Equal(t, input["language"], "fr")
	testutil.Equal(t, input["standards"], "standards")
	testutil.Equal(t, input["checks"], []record.Document{{"name": "unit", "command": "node --test", "paths": []string{"src/**"}}})
	testutil.Equal(t, input["commit_prefixes"], []string{"change", "repair"})
	testutil.Must(testsWritten(artifacts))
	testutil.Equal(t, testutil.Must(next(artifacts))["tests_written"], true)
	testutil.Put(t, ctx.Cwd, "state.mjs", "export const cancelled = true;\n")
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: "change: allow cancellation"}, nil))["committed"], true)
	testutil.Contains(t, testutil.Git(t, ctx.Cwd, "log", "-1", "--format=%B"), "Archon-Run: i1\nArchon-Unit: U1\nArchon-Inputs: requirements=1 plan=2")
	later := filepath.Join(artifacts, "later")
	testutil.Equal(t, testutil.Must(prepare(ctx, later))["remaining"], 1)
	testutil.Equal(t, testutil.Must(next(later)), record.Document{"unit": "U2", "tests_written": false, "round": 1})
}
func TestDoesNotCommitFailedChecksOwnerDecisionsOrInvalidDecisionRows(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	gate := gateFixture()
	gate.Passed = false
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, gate, &codeOutput{}, nil))["committed"], false)
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, gateFixture(), &codeOutput{OwnerDecisions: []ownerDecision{{"D1", "Choose?"}}}, nil))["committed"], false)
	putJSON(t, filepath.Join(artifacts, "implement/decisions.json"), record.Document{"rows": []record.Document{rowFixture(record.Document{"decided_by": "owner"})}})
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, gateFixture(), &codeOutput{}, nil))["committed"], false)
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), head)
}
func TestEachReleaseStepHasItsOwnPassLimit(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	failing := func(name string) *checks.Gate {
		gate := gateFixture()
		gate.Passed = false
		gate.Failing = []string{name}
		return gate
	}
	first := testutil.Must(end(ctx, artifacts, failing("lint"), &codeOutput{}, nil))
	testutil.Equal(t, first["finished"], false)
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, failing("types"), &codeOutput{}, nil))["finished"], false)
	third := testutil.Must(end(ctx, artifacts, failing("tests"), &codeOutput{}, nil))
	testutil.Equal(t, third["finished"], true)
	s := testutil.Must(readJSON[state](filepath.Join(artifacts, "implement/state.json")))
	testutil.Equal(t, s.Exhausted, first["unit"])
	result, _, err := outcome(s, planFixture(), nil, nil)
	testutil.OK(t, err)
	testutil.Equal(t, result["outcome"], "stop")
	testutil.Contains(t, result["stop"].(record.Document)["state"].(string), "did not pass its checks within its pass limit")
	testutil.Contains(t, strings.Join(stringsOf(result["stop"].(record.Document)["unresolved"]), "\n"), "failing check: tests")
}

func TestPassingStepsDoNotUseUpTheNextStepsPasses(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	failing := gateFixture()
	failing.Passed = false
	failing.Failing = []string{"lint"}
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, failing, &codeOutput{}, nil))["finished"], false)
	testutil.Equal(t, testutil.Must(end(ctx, artifacts, failing, &codeOutput{}, nil))["finished"], false)
	passed := testutil.Must(end(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: "feat: first step"}, nil))
	testutil.Equal(t, passed["committed"], true)
	s := testutil.Must(readJSON[state](filepath.Join(artifacts, "implement/state.json")))
	testutil.Equal(t, s.Exhausted, "")
}

func TestWritesImplementationDecisionsIntoNewPlanVersionAndResultUsesIt(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	putJSON(t, filepath.Join(artifacts, "implement/decisions.json"), record.Document{"rows": []record.Document{rowFixture(nil)}})
	testutil.Must(end(ctx, artifacts, gateFixture(), &codeOutput{CommitMessage: "change: allow cancellation"}, nil))
	result := testutil.Must(finish(ctx, artifacts, nil))
	testutil.Equal(t, result["outcome"], "pass")
	testutil.Equal(t, result["inputs"].(map[string]any)["plan_version"], 2)
	testutil.Equal(t, testutil.Must(record.Latest(ctx.Dir, "plan"))["decisions"], []record.Document{rowFixture(nil)})
	testutil.Equal(t, len(testutil.Must(record.List(ctx.Dir, "plan"))), 2)
}
func TestRejectsMissingPlanOrPlanMadeFromOlderRequirements(t *testing.T) {
	ctx, artifacts := setup(t, false)
	_, missing := prepare(ctx, artifacts)
	testutil.Fails(t, missing, "no plan")
	plan := planFixture()
	plan["requirements_version"] = 2
	_, err := record.Append(ctx.Dir, "plan", plan)
	testutil.OK(t, err)
	_, err = prepare(ctx, filepath.Join(artifacts, "stale"))
	testutil.Fails(t, err, "not the latest")
}

func TestCommandsUseEnvironmentAndWriteArtifactsAndResults(t *testing.T) {
	ctx, artifacts := setup(t, true)
	vars := map[string]string{"STATE_DIR": "state", "WORKFLOW_ID": ctx.RunID, "BASE_BRANCH": ctx.BaseBranch, "ARTIFACTS_DIR": "artifacts"}
	ctx.Dir = record.RecordDir(filepath.Join(ctx.Cwd, "state"), "work")
	source := filepath.Join(filepath.Dir(artifacts), "records")
	testutil.OK(t, os.MkdirAll(filepath.Dir(ctx.Dir), 0755))
	testutil.OK(t, os.Rename(source, ctx.Dir))
	registry := cli.New("test", Commands())
	run := func(mode string) record.Document {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := registry.Execute([]string{"implement", mode}, cli.Env{Cwd: ctx.Cwd, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }})
		if code != 0 {
			t.Fatalf("%s: %d: %s", mode, code, stderr.String())
		}
		var out record.Document
		testutil.OK(t, json.Unmarshal(stdout.Bytes(), &out))
		return out
	}
	testutil.Equal(t, run("prepare")["remaining"], 1)
	testutil.Equal(t, run("next")["unit"], "U1")
	input := testutil.Must(readJSON[record.Document](filepath.Join(ctx.Cwd, "artifacts/implement/unit.json")))
	testutil.Equal(t, input["criteria_folder"], "artifacts/criteria/standards")
	run("tests-written")
	testutil.Equal(t, run("next")["tests_written"], true)
	vars["GATE"] = string(raw(gateFixture()))
	vars["CODE"] = `{"owner_decisions":[],"commit_message":"repair: allow cancellation"}`
	testutil.Equal(t, run("end")["all_done"], true)
	testutil.Equal(t, run("state")["stuck"], false)
	testutil.Equal(t, run("finish")["outcome"], "pass")
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: ctx.Cwd, Stdout: &stdout, Stderr: &stderr, LookupEnv: func(key string) (string, bool) { v, ok := vars[key]; return v, ok }}
	for _, passed := range []bool{true, false} {
		gate := gateFixture()
		gate.Passed = passed
		vars["GATE"] = string(raw(gate))
		stderr.Reset()
		testutil.Equal(t, registry.Execute([]string{"implement", "end"}, env), 1)
		testutil.Contains(t, stderr.String(), "Cannot read properties of undefined (reading 'id')")
	}
	delete(vars, "ARTIFACTS_DIR")
	stderr.Reset()
	testutil.Equal(t, registry.Execute([]string{"implement", "next"}, env), 1)
	testutil.Contains(t, stderr.String(), "ARTIFACTS_DIR must be set")
	testutil.Equal(t, registry.Execute([]string{"implement", "unknown"}, env), 2)
}

func TestOnlyProvisionalAdditionsAndLaterSettlementsAreDerived(t *testing.T) {
	for _, change := range []record.Document{rowFixture(record.Document{"decided_by": "owner"}), rowFixture(record.Document{"decided_by": "principle"})} {
		out, _, err := outcome(stateFixture(), planFixture(), raw(record.Document{"rows": []record.Document{change}}), nil)
		testutil.OK(t, err)
		testutil.Equal(t, out["outcome"], "back")
		testutil.Equal(t, out["back"].(map[string]any)["to"], "plan")
	}
	p := planFixture()
	old := rowFixture(record.Document{"when": "later", "decide_when": "After measurement"})
	p["decisions"] = []record.Document{old}
	update := rowFixture(nil)
	errors, _, err := checkRows(p, raw(record.Document{"rows": []record.Document{update}}))
	testutil.OK(t, err)
	testutil.Equal(t, len(errors), 0)
	update["touches"] = []string{"elsewhere"}
	out, _, err := outcome(stateFixture(), p, raw(record.Document{"rows": []record.Document{update}}), nil)
	testutil.OK(t, err)
	testutil.Equal(t, out["outcome"], "back")
}

func TestF7ImplementPrepareReceivesReturnedEvidence(t *testing.T) {
	ctx, a := setup(t, true)
	_, err := record.Append(ctx.Dir, "run", record.Document{"run_id": "j1", "workflow": "judge", "started_at": "t", "start": "other", "counted": false, "answers": []string{}})
	testutil.OK(t, err)
	_, err = record.WriteResult(ctx.Dir, "judge", record.Document{"outcome": "back", "back": record.Document{"to": "implement", "premise": "The implementation preserves order", "observation": "The check log shows reordered entries"}}, record.ResultOptions{RunID: "j1", Base: strings.Repeat("a", 40), Commit: strings.Repeat("b", 40)})
	testutil.OK(t, err)
	ctx.RunID = "i2"
	_, err = record.Append(ctx.Dir, "run", record.Document{"run_id": ctx.RunID, "workflow": "implement", "started_at": "t", "start": "step_back", "counted": true, "answers": []string{}})
	testutil.OK(t, err)
	testutil.Must(prepare(ctx, a))
	input := testutil.Must(readJSON[record.Document](filepath.Join(a, "implement/input.json")))
	returned := documents(input["returned"])
	testutil.Equal(t, len(returned), 1)
	testutil.Equal(t, returned[0]["observation"], "The check log shows reordered entries")
}

func TestBaseFailureStopsImmediatelyWithoutPassOrStepBack(t *testing.T) {
	ctx, artifacts := setup(t, true)
	testutil.Must(prepare(ctx, artifacts))
	file := filepath.Join(artifacts, "implement/state.json")
	s := testutil.Must(readJSON[state](file))
	s.Round, s.Units[0].Passes = 2, 2
	putJSON(t, file, s)
	results := filepath.Join(artifacts, "checks.jsonl")
	testutil.Put(t, artifacts, "checks.jsonl", `{"name":"unit","status":"fail","base":"failed","log":"/checks/unit.log"}`+"\n")
	gate := &checks.Gate{Failing: []string{"unit"}, Preexisting: []string{"unit"}, Results: results}
	before := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	ended := testutil.Must(end(ctx, artifacts, gate, nil, nil))
	testutil.Equal(t, ended["finished"], true)
	testutil.Equal(t, ended["stuck"], false)
	testutil.Equal(t, ended["committed"], false)
	s = testutil.Must(readJSON[state](file))
	testutil.Equal(t, s.Round, 2)
	testutil.Equal(t, s.Units[0].Passes, 2)
	testutil.Equal(t, s.Stuck == nil, true)
	testutil.Equal(t, s.Exhausted, "")
	result := testutil.Must(finish(ctx, artifacts, nil))
	testutil.Equal(t, result["outcome"], "stop")
	stop := result["stop"].(record.Document)
	testutil.Equal(t, stop["base_failed"], true)
	testutil.Equal(t, stop["state"], checks.BaseMessage([]string{"unit"}))
	testutil.Equal(t, stop["unresolved"], []string{"unit: /checks/unit.log"})
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), before)
	history := testutil.Must(record.History(ctx.Dir))
	testutil.Equal(t, testutil.Must(record.LimitState(history, "implement", nil, 0)).Start, "retry_after_base")
	testutil.Equal(t, testutil.Must(record.LimitState(history, "implement", nil, 0)).Counted, false)
}

func TestRegressedOrUnavailableChecksStillConsumeImplementationPass(t *testing.T) {
	for _, base := range []string{"passed", "unavailable"} {
		t.Run(base, func(t *testing.T) {
			ctx, artifacts := setup(t, true)
			testutil.Must(prepare(ctx, artifacts))
			gate := &checks.Gate{Failing: []string{"unit"}, Stuck: true, StuckChecks: []string{"unit"}}
			if base == "passed" {
				gate.Regressed = []string{"unit"}
			}
			ended := testutil.Must(end(ctx, artifacts, gate, nil, nil))
			testutil.Equal(t, ended["stuck"], true)
			s := testutil.Must(readJSON[state](filepath.Join(artifacts, "implement/state.json")))
			testutil.Equal(t, s.Round, 1)
			testutil.Equal(t, s.Units[0].Passes, 1)
		})
	}
}
