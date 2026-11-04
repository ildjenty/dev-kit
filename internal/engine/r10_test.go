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
	configure(t, e, "[detection]\nrequired = false\n[sources.requirements]\npath = "+strconv.Quote(outside)+"\nwritable = false\n")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
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
	configure(t, e, "[detection]\nrequired = false\n[sources.context]\npath = 'context'\nnorm = true\n")
	for _, role := range []string{"planner", "worker", "judge"} {
		write(t, e.Root(), "context/"+role+".md", "Only the "+role+" receives this context.")
	}
	template, _, err := e.template(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	template.PlannerInputs = []plan.Selection{{Source: "context", Select: "planner.md"}}
	template.JudgeInputs = []plan.Selection{{Source: "context", Select: "judge.md"}}
	template.Stages[0].Inputs = []plan.Selection{{Source: "context", Select: "worker.md"}}
	template.Stages[0].Checks = []plan.Check{judgeCheck("wording", false)}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	write(t, e.Root(), ".devkit/requirements.md", "Default requirements body must not be selected.")
	write(t, e.Root(), ".devkit/rules.md", "Default rules body must not be selected.")
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure role inputs")
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
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
	submitWork(t, e, work)
	judgment, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		role        string
		instruction *form.Instruction
	}{{"planner", start.Instruction}, {"worker", work.Instruction}, {"judge", review.Instruction}, {"judge", judgment.Instruction}} {
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
	if input.Kind == check.DetectionRequest {
		result := form.DetectionResult{InstructionID: input.InstructionID}
		for _, example := range input.DetectionExamples {
			judgment := pass(input.Request)
			if example.ID == "e1" || example.ID == "e4" || example.ID == "e5" {
				judgment.Judgment = form.Fail
				judgment.Findings = []form.Finding{{ID: "F1", Blocking: true, Kind: form.AutoFixable, Scenario: "The proposed plan violates the fixed requirements"}}
			}
			result.Observations = append(result.Observations, observed(example.ID, judgment, "Compared the example against the supplied criteria"))
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			os.Exit(4)
		}
	} else if err := json.NewEncoder(os.Stdout).Encode(pass(input.Request)); err != nil {
		os.Exit(5)
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
	configure(t, e, "[sources.guides]\npath = "+strconv.Quote(outside)+"\nnorm = true\n[plan_check]\nrunner = 'capture'\ncriteria = [{source='principles',select='review/plan.md'}, {source='plans',select='README.md'}, {source='guides',select='criteria.md'}]\n[judges.capture]\nexecutable = "+strconv.Quote(executable)+"\nargs = ['-test.run=^TestR10JudgeCommandProcess$']\ntimeout_seconds = 5\noutput_schema = 'judge-schema.json'\n")
	configureStage(t, e, func(stage *plan.Stage) {
		item := judgeCheck("wording", false)
		item.Runner = "capture"
		item.Criteria = append(item.Criteria, plan.Selection{Source: "guides", Select: "criteria.md"})
		stage.Checks = []plan.Check{item}
	})
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	work, err := e.Next(ctx)
	if err != nil || work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("registered command did not complete detection and plan judgment: %+v, %v", work, err)
	}
	submitWork(t, e, work)
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []check.RequestKind{check.DetectionRequest, check.PlanRequest, check.RoundRequest} {
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
		if kind == check.DetectionRequest {
			if len(input.Norms) != 0 || strings.Contains(string(data), `"expected"`) || strings.Contains(input.Brief, "-history.md") {
				t.Fatal("detection command received private expectations or history")
			}
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

func TestPlanDetectionIncludesRequiredCriteriaAndBundledSchemas(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	detection, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := briefText(t, detection.Instruction)
	data, err := e.readInstruction(detection.Instruction.ID)
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
		// Detection requests carry only the bundled output contract, not other schemas.
		if strings.Contains(body, "### schemas/"+name+".json") {
			t.Fatalf("detection supplied an unselected supporting schema %s", name)
		}
	}
}

func TestPlannerUsesPinnedOutsideTemplateAtIssuance(t *testing.T) {
	e, ctx := fixture(t)
	outside := t.TempDir()
	template, _, err := e.template(ctx, "HEAD", "test")
	if err != nil {
		t.Fatal(err)
	}
	write(t, outside, "test.json", string(marshal(t, template)))
	configure(t, e, "[sources.plans]\npath = "+strconv.Quote(outside)+"\n[detection]\nrequired = false\n")
	newWork := e.newWork
	e.newWork = func() (string, string) {
		template.Stages[0].Gate.Reason = "The template fixed at instruction issue time"
		write(t, outside, "test.json", string(marshal(t, template)))
		return newWork()
	}
	start, err := e.Start(ctx, StartRequest{Template: "test", Task: "Update documentation"})
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
