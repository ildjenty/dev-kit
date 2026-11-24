package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func structureDraft() document {
	d := draft()
	for _, key := range []string{"increments", "tests", "kept_behaviors"} {
		d[key] = []document{}
	}
	return d
}
func startStructure(t *testing.T) (record.ContextData, string) {
	t.Helper()
	ctx, a := repository(t, document{})
	ctx.Dir = t.TempDir()
	ctx.RunID = "plan-phases"
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	checked := must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, checked["valid"], true)
	testutil.Equal(t, checked["structure"], true)
	testutil.Equal(t, checked["data"], false)
	j := string(must(json.Marshal(judgment(nil))))
	must(settle(a, map[string]string{"structure": j}, "", "May work proceed?", nil))
	testutil.Equal(t, must(advance(ctx, a))["proceed"], true)
	return ctx, a
}
func TestStructureNatureSchemaAndPhaseCoverage(t *testing.T) {
	d := structureDraft()
	testutil.Equal(t, must(draftErrors(d, condition(), "", "structure")), []string{})
	testutil.Contains(t, strings.Join(must(draftErrors(d, condition(), "", "detail")), "\n"), "no test-plan entry")
	row := entries(d["decisions"])[0]
	delete(row, "nature")
	testutil.Contains(t, strings.Join(must(draftErrors(d, condition(), "", "structure")), "\n"), "nature")
	row["ref"] = "D-abcdef"
	testutil.Equal(t, must(draftErrors(d, condition(), "", "structure")), []string{})
	delete(row, "ref")
	row["nature"] = document{"kind": document{"value": "module", "evidence": "src/entry.ext"}, "reasons_for_change": "unknown", "volatility": "unknown", "callers": "unknown"}
	testutil.Equal(t, must(draftErrors(d, condition(), "", "structure")), []string{})
	delete(object(object(row["nature"])["kind"]), "evidence")
	if len(must(draftErrors(d, condition(), "", "structure"))) == 0 {
		t.Fatal("nature without evidence passed")
	}
	d = draft()
	testutil.Contains(t, strings.Join(must(draftErrors(d, condition(), "", "structure")), "\n"), "leaves increments empty")
	d = structureDraft()
	entries(d["decisions"])[0]["quality"] = "data"
	testutil.Contains(t, strings.Join(must(draftErrors(d, condition(), "", "structure")), "\n"), "only structure rows")
}
func TestDetailAllowsSignaturesAndUsesItsOwnRoundCounter(t *testing.T) {
	ctx, a := startStructure(t)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
	testutil.Put(t, ctx.Cwd, "src/entry.ext", "changed signature\n")
	r := must(checkCommand(ctx, a, nil, "", "detail"))
	testutil.Equal(t, r["valid"], true)
	testutil.Equal(t, r["structure"], false)
	testutil.Equal(t, r["phase_round"], 1)
	testutil.Equal(t, r["round"], 2)
	state := must(read(filepath.Join(rounds(a), "latest.json")))
	testutil.Equal(t, state["judges"], []string{"delivery"})
	j := string(must(json.Marshal(judgment(nil))))
	testutil.Equal(t, must(settle(a, map[string]string{"delivery": j}, "", "", nil))["done"], true)
}
func TestDetailStructureChangesStopImmediately(t *testing.T) {
	for _, kind := range []string{"add", "remove", "move", "row"} {
		t.Run(kind, func(t *testing.T) {
			ctx, a := startStructure(t)
			d := draft()
			switch kind {
			case "add":
				testutil.Put(t, ctx.Cwd, "new/folder/file.ext", "signature\n")
			case "remove":
				testutil.Git(t, ctx.Cwd, "rm", "src/peer.ext")
			case "move":
				testutil.Git(t, ctx.Cwd, "mv", "src/peer.ext", "src/moved.ext")
			case "row":
				entries(d["decisions"])[0]["chosen"] = "A different placement"
			}
			testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), d))
			r := must(checkCommand(ctx, a, nil, "", "detail"))
			testutil.Equal(t, r["valid"], false)
			testutil.Equal(t, r["structure_changed"], true)
			testutil.Contains(t, strings.Join(stringsOf(r["errors"]), "\n"), "Restart plan")
			testutil.Equal(t, must(settle(a, nil, "", "", nil))["done"], true)
			testutil.Equal(t, outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil)["outcome"], "stop")
			diagnostic := "C1 needs a structure change; src/entry.ext:1 is the evidence."
			must(settle(a, nil, "", "", []string{diagnostic}))
			outcome := outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil)
			testutil.Equal(t, outcome["outcome"], "stop")
			testutil.Contains(t, strings.Join(stringsOf(object(outcome["stop"])["unresolved"]), "\n"), diagnostic)
		})
	}
}
func TestStructureDoesNotAdvancePastQuestionsOrBlockingFindings(t *testing.T) {
	for _, extra := range []document{{"needs_decision": true}, {"blocking": true}} {
		ctx, a := startStructure(t)
		state := must(read(filepath.Join(rounds(a), "latest.json")))
		state["judges"] = []string{"structure"}
		testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
		output := string(must(json.Marshal(judgment(document{"findings": []document{finding(extra)}}))))
		must(settle(a, map[string]string{"structure": output}, "", "", nil))
		testutil.Equal(t, must(advance(ctx, a))["proceed"], false)
	}
}
func TestDeferredPlanFindingsNeverBlockOrAskAndRetainMechanical(t *testing.T) {
	ctx, a := startStructure(t)
	state := must(read(filepath.Join(rounds(a), "latest.json")))
	state["judges"] = []string{"structure"}
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
	output := string(must(json.Marshal(judgment(document{"findings": []document{finding(document{"deferred": true, "mechanical": true, "blocking": false, "needs_decision": false})}}))))
	r := must(settle(a, map[string]string{"structure": output}, "", "", nil))
	testutil.Equal(t, r["fixable_count"], 0)
	testutil.Equal(t, r["needs_decision_count"], 0)
	testutil.Equal(t, must(advance(ctx, a))["proceed"], true)
	saved := must(read(filepath.Join(a, "plan/structure.json")))
	f := entries(object(object(saved["judgments"])["structure"])["findings"])[0]
	testutil.Equal(t, f["blocking"], false)
	testutil.Equal(t, f["mechanical"], true)
}
func TestPrepareWritesUsableDraftSchema(t *testing.T) {
	ctx, a := repository(t, document{})
	testutil.Standards(t, ctx.Cwd, ".devkit/standards")
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "standards")
	ctx.Dir = t.TempDir()
	must(record.Append(ctx.Dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), "run_id": "r1", "recorded_at": "t"}))
	requirements(ctx.Dir, "r1")
	must(prepare(ctx, a, "display"))
	file := filepath.Join(a, "plan/draft.schema.json")
	testutil.Equal(t, must(record.ValidateSchema(file, draft())), []string{})
	testutil.Equal(t, must(record.ValidateSchema(file, structureDraft())), []string{})
	d := draft()
	delete(entries(d["decisions"])[0], "nature")
	if len(must(record.ValidateSchema(file, d))) == 0 {
		t.Fatal("exported schema missed nature")
	}
	_, err := os.Stat(filepath.Join(a, "plan/common.json"))
	testutil.OK(t, err)
}

func TestTwoPhaseFinishRecordsBothJudgmentsAndLoopOutcomes(t *testing.T) {
	ctx, a := startStructure(t)
	ctx.RunID = "plan-phases"
	ctx.Branch = "feature/x"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), draft()))
	must(checkCommand(ctx, a, nil, "", "detail"))
	j := string(must(json.Marshal(judgment(nil))))
	must(settle(a, map[string]string{"delivery": j}, "", "", nil))
	head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	result := must(finish(ctx, a, record.ResultOptions{Base: head, Commit: head}))
	testutil.Equal(t, result["outcome"], "pass")
	saved := must(record.Latest(ctx.Dir, "plan"))
	testutil.Equal(t, saved["skeleton_commit"], testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"))
	judgments := must(record.List(ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, judgments[0]["judge"], "structure")
	testutil.Equal(t, judgments[1]["judge"], "delivery")
	testutil.Equal(t, must(record.Latest(ctx.Dir, "result"))["loops"], []document{{"name": "structure", "ended": "cleared", "rounds": 1}, {"name": "detail", "ended": "cleared", "rounds": 1}})
}
func TestExhaustedStructureNeverProducesAnAcceptedPartialPlan(t *testing.T) {
	ctx, a := startStructure(t)
	ctx.RunID = "plan-phases"
	ctx.Branch = "feature/x"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	output := string(must(json.Marshal(judgment(document{"verdict": "fail", "findings": []document{finding(nil)}}))))
	must(settle(a, map[string]string{"structure": output}, "", "", nil))
	head := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	result := must(finish(ctx, a, record.ResultOptions{Base: head, Commit: head}))
	testutil.Equal(t, result["outcome"], "stop")
	testutil.Contains(t, strings.Join(stringsOf(result["reasons"]), "\n"), "Wire the entry point")
}
func TestAdvanceRequiresSettledStructure(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Dir = t.TempDir()
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	must(checkCommand(ctx, a, nil, "", "structure"))
	_, err := advance(ctx, a)
	testutil.Fails(t, err, "not settled")
}
