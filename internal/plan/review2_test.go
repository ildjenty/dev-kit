package plan

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestF1UndecidedRowsNeverBecomeAgreed(t *testing.T) {
	for _, by := range []string{"owner", "principle"} {
		t.Run(by, func(t *testing.T) {
			ctx, _ := repository(t, document{})
			ctx.Dir, ctx.Branch = t.TempDir(), "feature/x"
			requirements(ctx.Dir, "r1")
			d := draft()
			r := entries(d["decisions"])[0]
			r["when"], r["decided_by"], r["decide_when"] = "later", by, "During implementation"
			must(agreeRows(ctx, d))
			testutil.Equal(t, len(must(decisions.List(ctx.Cwd)).Decisions), 0)
		})
	}
}

func TestF1MaterializationRejectsUnboundOwnerAnswer(t *testing.T) {
	ctx, _ := repository(t, document{})
	ctx.Dir, ctx.Branch = t.TempDir(), "feature/x"
	requirements(ctx.Dir, "r1")
	d := draft()
	entries(d["decisions"])[0]["decided_by"] = "owner"
	_, err := agreeRows(ctx, d)
	testutil.Fails(t, err, "answer")
	testutil.Equal(t, len(must(decisions.List(ctx.Cwd)).Decisions), 0)
}

func TestF2StructureAnswerFollowsSkeletonContent(t *testing.T) {
	ctx, a := repository(t, document{})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	testutil.Put(t, ctx.Cwd, "src/new.ext", "first signature\n")
	must(checkCommand(ctx, a, nil, "", "structure"))
	state := must(read(filepath.Join(rounds(a), "latest.json")))
	q := ownerQuestions(state, nil)[0]
	answers := []document{{"target": q["target"], "versions": document{"requirements_version": state["requirements_version"]}}}
	testutil.Equal(t, len(ownerQuestions(state, answers)), 0)
	testutil.Git(t, ctx.Cwd, "commit", "--allow-empty", "-qm", "same tree")
	must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, len(ownerQuestions(must(read(filepath.Join(rounds(a), "latest.json"))), answers)), 0)
	testutil.Put(t, ctx.Cwd, "src/new.ext", "different signature\n")
	must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, len(ownerQuestions(must(read(filepath.Join(rounds(a), "latest.json"))), answers)), 1)
}

func TestF4DetailLaunchesDetectedQualityWithoutRows(t *testing.T) {
	ctx, a := repository(t, document{"decision_paths": []document{{"paths": []string{"src/entry.ext"}, "quality": "cost", "ask": true}}})
	ctx.Dir = t.TempDir()
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	testutil.Put(t, ctx.Cwd, "src/entry.ext", "signature\n")
	must(checkCommand(ctx, a, nil, "", "structure"))
	must(settle(a, map[string]string{"structure": string(must(json.Marshal(judgment(nil))))}, "", "", nil))
	if next := must(advance(ctx, a)); next["proceed"] != true {
		t.Fatalf("%v", outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil))
	}
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
	result := must(checkCommand(ctx, a, nil, "", "detail"))
	testutil.Equal(t, result["cost"], true)
	material := must(read(filepath.Join(rounds(a), "2/cost.json")))
	testutil.Equal(t, len(entries(material["decisions"])), 0)
	testutil.Equal(t, len(entries(material["changes"])), 1)
}

func TestF5PlanRejectsContradictoryDeferred(t *testing.T) {
	for _, q := range []string{"structure", "security"} {
		a := t.TempDir()
		state := settled()
		state["judges"] = []string{q}
		testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
		f := finding(document{"deferred": true})
		output := string(must(json.Marshal(judgment(document{"findings": []document{f}}))))
		result := must(settle(a, map[string]string{q: output}, "", "", nil))
		testutil.Equal(t, result["not_assessed_count"], 1)
	}
}

func TestF13CheckRequiresPhase(t *testing.T) {
	for _, phase := range []string{"", "other"} {
		ctx, a := repository(t, document{})
		inputFiles(t, a)
		_, err := checkCommand(ctx, a, nil, "", phase)
		testutil.Fails(t, err, "phase")
	}
}

func TestF13FinishCannotBypassPhases(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Dir, ctx.RunID, ctx.Branch = t.TempDir(), "p1", "feature/x"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), settled()))
	_, err := finish(ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c")})
	testutil.Fails(t, err, "phase")
}

func TestF16OwnerRowAsksCoveredStructureOnce(t *testing.T) {
	state := settled()
	r := entries(object(state["draft"])["decisions"])[0]
	r["decided_by"] = "owner"
	state["changes"] = []document{{"quality": "structure", "file": "src/entry.ext", "change": "A file"}}
	qs := ownerQuestions(state, nil)
	testutil.Equal(t, len(qs), 1)
	testutil.Equal(t, object(qs[0]["target"])["kind"], "decision")
}

func TestF12PlanKeepsResolvedMechanicalFindingsFromEveryRound(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Dir, ctx.RunID, ctx.Branch = t.TempDir(), "p1", "feature/x"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	must(checkCommand(ctx, a, nil, "", "structure"))
	output := string(must(json.Marshal(judgment(document{"verdict": "fail", "findings": []document{finding(document{"mechanical": true})}}))))
	must(settle(a, map[string]string{"structure": output}, "", "", nil))
	must(checkCommand(ctx, a, nil, "", "structure"))
	passed := string(must(json.Marshal(judgment(nil))))
	must(settle(a, map[string]string{"structure": passed}, "", "", nil))
	testutil.Equal(t, must(advance(ctx, a))["proceed"], true)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
	must(checkCommand(ctx, a, nil, "", "detail"))
	must(settle(a, map[string]string{"delivery": passed}, "", "", nil))
	must(finish(ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c")}))
	judgments := must(record.List(ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 3)
	f := entries(judgments[0]["findings"])[0]
	testutil.Equal(t, f["mechanical"], true)
	testutil.Equal(t, f["resolved"], true)
}

func TestF7PlanPrepareReceivesReturnedEvidence(t *testing.T) {
	ctx, a := repository(t, document{})
	testutil.Standards(t, ctx.Cwd, ".devkit/standards")
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "standards")
	ctx.Dir = t.TempDir()
	must(record.Append(ctx.Dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), "run_id": "r1", "recorded_at": "t"}))
	requirements(ctx.Dir, "r1")
	must(record.Append(ctx.Dir, "run", document{"run_id": "j1", "workflow": "judge", "started_at": "t", "start": "other", "counted": false, "answers": []string{}}))
	must(record.WriteResult(ctx.Dir, "judge", document{"outcome": "back", "back": document{"to": "plan", "premise": "The model preserves order", "observation": "src/entry.ext reorders entries"}}, record.ResultOptions{RunID: "j1", Base: sha("b"), Commit: sha("c")}))
	run(ctx.Dir, ctx.RunID)
	must(prepare(ctx, a, ""))
	returned := entries(must(read(filepath.Join(a, "plan/input.json")))["returned"])
	testutil.Equal(t, len(returned), 1)
	testutil.Equal(t, returned[0]["observation"], "src/entry.ext reorders entries")
}

func TestF2OwnerRowBindsCoveredSkeletonContent(t *testing.T) {
	state := settled()
	row := entries(object(state["draft"])["decisions"])[0]
	row["decided_by"] = "owner"
	state["changes"] = []document{{"quality": "structure", "file": "src/entry.ext", "change": "A file", "tree": "first blob"}}
	questions := ownerQuestions(state, nil)
	testutil.Equal(t, len(questions), 1)
	answers := []document{{"target": questions[0]["target"], "versions": document{"requirements_version": state["requirements_version"]}}}
	testutil.Equal(t, len(ownerQuestions(state, answers)), 0)
	entries(state["changes"])[0]["tree"] = "second blob"
	testutil.Equal(t, len(ownerQuestions(state, answers)), 1)
}

func TestF13FinishRequiresMatchingCompletedPhases(t *testing.T) {
	for _, fault := range []string{"run", "invalid", "not_assessed"} {
		t.Run(fault, func(t *testing.T) {
			ctx, a := startStructure(t)
			ctx.Branch = "feature/x"
			request(ctx.Dir)
			requirements(ctx.Dir, "r1")
			run(ctx.Dir, ctx.RunID)
			testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
			must(checkCommand(ctx, a, nil, "", "detail"))
			must(settle(a, map[string]string{"delivery": string(must(json.Marshal(judgment(nil))))}, "", "", nil))
			fixed := must(read(filepath.Join(a, "plan/structure.json")))
			switch fault {
			case "run":
				fixed["run_id"] = "another-run"
			case "invalid":
				fixed["valid"] = false
			case "not_assessed":
				object(object(fixed["judgments"])["structure"])["verdict"] = "not_assessed"
			}
			testutil.OK(t, write(filepath.Join(a, "plan/structure.json"), fixed))
			_, err := finish(ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c")})
			testutil.Fails(t, err, "completed structure phase")
		})
	}
}

func TestF12PlanRecordsEarlierJudgmentsAfterFinalMachineFailure(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Dir = t.TempDir()
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	must(checkCommand(ctx, a, nil, "", "structure"))
	failed := judgment(document{"verdict": "fail", "findings": []document{finding(document{"mechanical": true})}})
	must(settle(a, map[string]string{"structure": string(must(json.Marshal(failed)))}, "", "", nil))
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), document{}))
	must(checkCommand(ctx, a, nil, "", "structure"))
	must(settle(a, nil, "", "", nil))
	out := must(finish(ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c")}))
	testutil.Equal(t, out["outcome"], "stop")
	judgments := must(record.List(ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 1)
	testutil.Equal(t, entries(judgments[0]["findings"])[0]["mechanical"], true)
	testutil.Equal(t, entries(judgments[0]["findings"])[0]["resolved"], false)
}

func TestPlanPrepareDoesNotReceiveDecisionRecords(t *testing.T) {
	ctx, a := repository(t, document{})
	testutil.Standards(t, ctx.Cwd, ".devkit/standards")
	testutil.Put(t, ctx.Cwd, ".devkit/decisions/D-abcdef.md", "---\nid: D-abcdef\ntitle: Old\nstatus: superseded\n---\nOld body\n")
	testutil.Put(t, ctx.Cwd, ".devkit/decisions/D-fedcba.md", "---\nid: D-fedcba\ntitle: New\nstatus: agreed\nsupersedes: [D-abcdef]\n---\nNew body\n")
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "supersession")
	testutil.Git(t, ctx.Cwd, "branch", "-f", "main", "HEAD")
	ctx.Dir = t.TempDir()
	must(record.Append(ctx.Dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), "run_id": "r1", "recorded_at": "t"}))
	requirements(ctx.Dir, "r1")
	previous := draft()
	previous["requirements_version"], previous["run_id"], previous["recorded_at"], previous["skeleton_commit"] = 1, "p0", "t", sha("a")
	must(record.Append(ctx.Dir, "plan", previous))
	run(ctx.Dir, ctx.RunID)
	must(prepare(ctx, a, ""))
	input := must(read(filepath.Join(a, "plan/input.json")))
	testutil.Equal(t, input["returned"], []document{})
	_, present := input["agreed_decisions"]
	testutil.Equal(t, present, false)
	material := materials("structure", input, previous, nil, nil, nil)
	_, present = material["agreed_decisions"]
	testutil.Equal(t, present, false)
}
