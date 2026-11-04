package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func closeForNextRound(t *testing.T, e *Engine) *ledger.Round {
	t.Helper()
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	round, _ := state.OpenRound()
	if err := e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "Review the next fixed inputs"}); err != nil {
		t.Fatal(err)
	}
	return round
}

func TestOutsidePinsSurviveRefreshAndRequireNewApproval(t *testing.T) {
	e, _, guides, work := outsideSourceWork(t)
	ctx := t.Context()
	originalBrief := briefText(t, work.Instruction)
	round := closeForNextRound(t, e)
	if !strings.Contains(round.NormReference, "@") {
		t.Fatal("round did not directly reference an immutable source pin")
	}
	write(t, guides, "style.md", "Second outside guide.\n")
	approval := checkedNext(t, e)
	q := approval.Instruction.Question
	if q == nil || q.StopCategory != "meta_loop_edit" || q.ApprovalTarget.SourcePin == "" {
		t.Fatalf("outside version has no approval identity: %+v", approval)
	}
	secondPin := q.ApprovalTarget.SourcePin
	answerResult(t, e, approval, ledger.AnswerApproved)
	// Keep git unchanged while changing the original again after approval.
	write(t, guides, "style.md", "Third outside guide.\n")
	next := checkedNext(t, e)
	q = next.Instruction.Question
	if q == nil || q.StopCategory != "meta_loop_edit" || q.ApprovalTarget.SourcePin == secondPin {
		t.Fatalf("an earlier approval authorized a new outside version: %+v", next)
	}
	if q.ApprovalTarget.Tree != approval.Instruction.Question.ApprovalTarget.Tree {
		t.Fatal("test changed the git target")
	}
	fixed, err := e.document(ctx, roundReference(round), "guides", "style.md")
	if err != nil || fixed.Body != "Initial outside guide.\n" {
		t.Fatalf("refresh overwrote the old pin: %+v, %v", fixed, err)
	}
	second, err := e.document(ctx, approvalReference(approval.Instruction.Question.ApprovalTarget), "guides", "style.md")
	if err != nil || second.Body != "Second outside guide.\n" {
		t.Fatalf("approval no longer references its fixed body: %+v, %v", second, err)
	}
	if err := os.Remove(work.Instruction.Brief); err != nil {
		t.Fatal(err)
	}
	regenerated, err := e.instructionBrief(work.Instruction.ID)
	if err != nil || string(regenerated) != originalBrief {
		t.Fatalf("past instruction changed after refreshing sources: %v", err)
	}
}

func TestReadOnlyOutsideProductsIdentifyEveryRound(t *testing.T) {
	e, ctx := fixture(t)
	outside := t.TempDir()
	write(t, outside, "requirements.md", "First read-only requirements.\n")
	configure(t, e, "[sources.requirements]\npath = "+strconv.Quote(outside)+"\nwritable = false\n")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work := checkedNext(t, e)
	first := closeForNextRound(t, e)
	if first.StartTarget.Sources == "" {
		t.Fatal("read-only outside products were omitted from target identity")
	}
	if _, err := os.Stat(filepath.Join(e.workRoot(), "sources", "edit")); !os.IsNotExist(err) {
		t.Fatalf("read-only source has an editable copy: %v", err)
	}
	write(t, outside, "requirements.md", "Second read-only requirements.\n")
	next := checkedNext(t, e)
	if next.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected the next work round: %+v", next)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := state.OpenRound()
	if first.StartTarget.Tree != second.StartTarget.Tree || first.StartTarget.Sources == second.StartTarget.Sources {
		t.Fatalf("outside product changes were not distinguished: %+v, %+v", first.StartTarget, second.StartTarget)
	}
	if !strings.Contains(briefText(t, work.Instruction), "First read-only requirements.") || !strings.Contains(briefText(t, next.Instruction), "Second read-only requirements.") {
		t.Fatal("round inputs do not match their fixed outside product identities")
	}
	submitWork(t, e, next)
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	submitted, _ := state.OpenRound()
	if submitted.ReturnedTarget.Sources != second.StartTarget.Sources {
		t.Fatal("submission lost the fixed read-only product identity")
	}
}

func TestTemplateRoleInputsReachPlannerWorkerAndJudge(t *testing.T) {
	e, ctx := fixture(t)
	configure(t, e, "[sources.context]\npath = 'context'\nnorm = true\n[planning]\ninputs = [{ source = 'context', select = 'planner.md' }]\n[plan_check]\ninputs = [{ source = 'context', select = 'judge.md' }]\n")
	for _, role := range []string{"planner", "worker", "judge"} {
		write(t, e.Root(), "context/"+role+".md", "Only the "+role+" receives this context.")
	}
	template, _, err := e.pattern(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Inputs = []plan.Selection{{Source: "context", Select: "worker.md"}}
	template.Stages[0].Checks = []plan.Check{judgeCheck("wording", false)}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	write(t, e.Root(), ".devkit/requirements.md", "Default requirements body must not be selected.")
	write(t, e.Root(), ".devkit/rules.md", "Default rules body must not be selected.")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure role inputs")
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	collect(t, e, review.Instruction, pass(*data.Request))
	work, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	work = approvedNext(t, e, work)
	submitWork(t, e, work)
	judgment, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		role        string
		instruction *form.Instruction
	}{{"planner", start.Instruction}, {"worker", work.Instruction}, {"judge", review.Instruction}, {"worker", judgment.Instruction}} {
		body := briefText(t, item.instruction)
		if strings.Contains(body, "## Repository requirements") || strings.Contains(body, "## Rules and enforcement") {
			t.Fatalf("unselected default inputs are shown for %s", item.role)
		}
		for _, role := range []string{"planner", "worker", "judge"} {
			if strings.Contains(body, "Only the "+role+" receives this context.") != (role == item.role) {
				t.Fatalf("wrong role inputs for %s: %s", item.role, role)
			}
		}
		for _, omitted := range []string{"Default requirements body must not be selected.", "Default rules body must not be selected."} {
			if strings.Contains(body, omitted) {
				t.Fatalf("explicit inputs did not replace defaults for %s", item.role)
			}
		}
	}
}

type capturedJudgeInput struct {
	check.Request
	CriteriaBodies []brief.Document `json:"criteria_bodies"`
	Brief          string           `json:"brief"`
}

func TestR10JudgeCommandProcess(t *testing.T) {
	if os.Getenv("DEVKIT_R10_JUDGE") != "1" {
		return
	}
	var input capturedJudgeInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		os.Exit(2)
	}
	name := filepath.Join(".devkit/run/work", os.Getenv("DEVKIT_WORK"), "captured-"+string(input.Kind)+".json")
	data, err := json.Marshal(input)
	if err != nil || os.WriteFile(name, data, 0o644) != nil {
		os.Exit(3)
	}
	if err := json.NewEncoder(os.Stdout).Encode(pass(input.Request)); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestRegisteredJudgesReceiveFixedBodiesRulesAndContracts(t *testing.T) {
	e, ctx := fixture(t)
	t.Setenv("DEVKIT_R10_JUDGE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	const criteriaBody = "Fixed outside criteria body for the registered judgment command."
	write(t, outside, "criteria.md", criteriaBody)
	write(t, e.Root(), "judge-schema.json", string(e.schemas["judgment"]))
	configure(t, e, "[sources.guides]\npath = "+strconv.Quote(outside)+"\nnorm = true\n[plan_check]\nrunner = 'capture'\ncriteria = [{source='principles',select='review/plan.md'}, {source='plans',select='README.md'}, {source='guides',select='criteria.md'}]\n[judges.capture]\nexecutable = "+strconv.Quote(executable)+"\nargs = ['-test.run=^TestR10JudgeCommandProcess$']\ntimeout_seconds = 5\noutput_schema = 'judge-schema.json'\n[checks.wording]\nrunner = 'capture'\ncriteria = [{ source = 'principles', select = 'review/wording.md' }, { source = 'guides', select = 'criteria.md' }]\n")
	configureStage(t, e, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	})
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work, err := e.Next(ctx)
	work = approvedNext(t, e, work)
	if err != nil || work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("registered command did not complete plan judgment: %+v, %v", work, err)
	}
	submitWork(t, e, work)
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []check.RequestKind{check.PlanRequest, check.RoundRequest} {
		data, err := os.ReadFile(filepath.Join(e.workRoot(), "captured-"+string(kind)+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var input capturedJudgeInput
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(input.Brief, "## Judgment rules") || !strings.Contains(input.Brief, "## Returning the result") || !json.Valid(input.OutputSchema) {
			t.Fatalf("%s command omitted rules or output contract", kind)
		}
		if !strings.Contains(input.Brief, criteriaBody) || !slices.ContainsFunc(input.CriteriaBodies, func(document brief.Document) bool { return document.Content == criteriaBody }) {
			t.Fatalf("%s command omitted fixed criteria bodies", kind)
		}
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if instruction, ok := record.Payload.(*ledger.Instruction); ok && instruction.Request != nil {
			for _, criterion := range instruction.Request.Criteria {
				if criterion.Content != "" {
					t.Fatal("runtime criteria bodies leaked into recorded identities")
				}
			}
		}
	}
}

func TestPlanCheckIncludesRequiredCriteriaAndBundledSchemas(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := briefText(t, review.Instruction)
	data, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	var identities []string
	for _, criterion := range data.Request.Criteria {
		identities = append(identities, criterion.Source+"/"+criterion.ID)
	}
	if !reflect.DeepEqual(identities, []string{"principles/review/plan.md", "plans/README.md"}) {
		t.Fatalf("missing plan criteria: %v", identities)
	}
	for _, name := range SchemaNames() {
		schema, err := e.asset(ctx, data.Reference, "schemas/"+name+".json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := form.CompileSchema(schema); err != nil {
			t.Fatalf("%s is not self-contained: %v", name, err)
		}
		selected := slices.Contains([]string{"plan", "plan-template", "finding"}, name)
		if strings.Contains(body, "### schemas/"+name+".json") != selected {
			t.Fatalf("plan check input contract selection differs for %s", name)
		}
	}
}

func TestPlannerUsesPinnedOutsideTemplateAtIssuance(t *testing.T) {
	e, ctx := fixture(t)
	outside := t.TempDir()
	template, _, err := e.pattern(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	write(t, outside, "test.json", string(marshal(t, template)))
	configure(t, e, "[sources.plans]\npath = "+strconv.Quote(outside)+"\n")
	newWork := e.newWork
	e.newWork = func() (string, string) {
		template.Stages[0].Gate.Reason = "The template fixed at instruction issue time"
		write(t, outside, "test.json", string(marshal(t, template)))
		return newWork()
	}
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	original := briefText(t, start.Instruction)
	if !strings.Contains(original, template.Stages[0].Gate.Reason) {
		t.Fatal("planner received a template read before its source pin was fixed")
	}
	write(t, outside, "test.json", "A later outside edit must not change the old instruction.")
	if err := os.Remove(start.Instruction.Brief); err != nil {
		t.Fatal(err)
	}
	restored, err := e.instructionBrief(start.Instruction.ID)
	if err != nil || string(restored) != original {
		t.Fatalf("planner instruction changed after an outside refresh: %v", err)
	}
}
