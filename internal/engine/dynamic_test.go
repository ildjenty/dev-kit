package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

const passingCommands = "\n[commands.test]\nexecutable = '/usr/bin/true'\ntimeout_seconds = 5\n\n[commands.e2e]\nexecutable = '/usr/bin/true'\ntimeout_seconds = 5\n"

// withInstalledRules restores the loop rules written by init and registers passing commands.
func withInstalledRules(t *testing.T, e *Engine) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := kit.HostFragment("roles.toml")
	if err != nil {
		t.Fatal(err)
	}
	rules := string(fragment)[strings.Index(string(fragment), "[[loop."):]
	write(t, e.Root(), "devkit.toml", string(data)+passingCommands+"\n"+rules)
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Use the installed loop rules")
}

// instance returns a worker stage instance of definition with required checks bound to C1.
func instance(id, definition string, done *string, aspects ...string) plan.Stage {
	stage := plan.Stage{ID: id, Definition: definition, Assignee: plan.Worker, Checks: []plan.Check{}, Gate: plan.Gate{Reason: "Review the result"},
		Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}, Transitions: plan.Transitions{Fix: id, Ask: id, Back: id, Done: done}}
	for _, aspect := range aspects {
		stage.Checks = append(stage.Checks, plan.Check{Aspect: aspect, Required: true, CompletionConditionIDs: []string{"C1"}, Target: "The change"})
	}
	return stage
}

func doneTo(id string) *string { return &id }

func implementInstance(id string, done *string) plan.Stage {
	return instance(id, "implement", done, "command:test", "test", "diff")
}

func verificationInstance(id string, done *string) plan.Stage {
	return instance(id, "verification", done, "command:e2e", "behavior")
}

func retrospectiveInstance(id string) plan.Stage {
	return instance(id, "retrospective", nil, "harness")
}

// composedPlan cites the default pattern and composes the given instances from start.
func composedPlan(t *testing.T, e *Engine, version int, stages ...plan.Stage) plan.Plan {
	t.Helper()
	_, source, err := e.pattern(t.Context(), "HEAD", "default")
	if err != nil {
		t.Fatal(err)
	}
	return plan.Plan{Version: version, Pattern: plan.PatternRef{Name: "default", ContentHash: plan.ContentHash(source)}, Task: "Update documentation",
		CompletionConditions: []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}}, StartStage: stages[0].ID,
		Stages: stages, ExternalOperations: []plan.ExternalOperation{}}
}

// drive answers checks, approvals, and work until a planner instruction or completion is ready.
func drive(t *testing.T, e *Engine) Result {
	t.Helper()
	for range 120 {
		result, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		instructions := result.Instructions
		if len(instructions) == 0 && result.Instruction != nil {
			instructions = []form.Instruction{*result.Instruction}
		}
		for i := range instructions {
			if driveInstruction(t, e, &instructions[i]) {
				return result
			}
		}
	}
	t.Fatal("the work did not reach a planner instruction or completion")
	return Result{}
}

func driveInstruction(t *testing.T, e *Engine, instruction *form.Instruction) bool {
	t.Helper()
	switch instruction.Kind {
	case form.InstructionPlan, form.InstructionCompleted:
		return true
	case form.InstructionPlanCheck, form.InstructionJudgment:
		data, err := e.readInstruction(instruction.ID)
		if err != nil {
			t.Fatal(err)
		}
		collect(t, e, instruction, pass(*data.Request))
	case form.InstructionQuestion:
		observeAnswer(t, e, "Approve and continue.")
		if _, err := e.Answer(t.Context(), AnswerRequest{QuestionID: instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
			t.Fatal(err)
		}
	case form.InstructionWork:
		write(t, e.Root(), "README.md", "# Updated by "+instruction.ID+"\n")
		progress := form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated the README"}, Report: currentReport(t, e)}
		if _, err := e.Submit(t.Context(), marshal(t, progress)); err != nil {
			t.Fatal(err)
		}
	case form.InstructionUnitWork:
		write(t, instruction.TargetPath, "README.md", "# Updated by "+instruction.ID+"\n")
		collect(t, e, instruction, form.Progress{InstructionID: instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated the README"}, Report: currentReport(t, e)})
	default:
		t.Fatalf("unexpected instruction: %+v", instruction)
	}
	return false
}

func currentReport(t *testing.T, e *Engine) []byte {
	t.Helper()
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.OpenRound(); !ok {
		return nil
	}
	return stageReport(t, e, "")
}

// propose submits a planner result and returns the next result.
func propose(t *testing.T, e *Engine, instruction *form.Instruction, p plan.Plan) Result {
	t.Helper()
	collect(t, e, instruction, form.PlannerResult{InstructionID: instruction.ID, Plan: marshal(t, p)})
	result, err := e.Next(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// rejection returns the reason devkit gave the reissued planner instruction.
func rejection(t *testing.T, previous *form.Instruction, result Result) string {
	t.Helper()
	if result.Instruction == nil || result.Instruction.Kind != form.InstructionPlan || result.Instruction.ID == previous.ID {
		t.Fatalf("the plan was not rejected: %+v", result)
	}
	body := briefText(t, result.Instruction)
	_, reason, found := strings.Cut(body, "## Previous result rejected")
	if !found {
		t.Fatalf("the reissued planner omitted the rejection: %s", body)
	}
	return reason
}

func startRules(t *testing.T, factory func(*testing.T) (*Engine, context.Context)) (*Engine, Result) {
	t.Helper()
	e, ctx := factory(t)
	withInstalledRules(t, e)
	start, err := e.Start(ctx, StartRequest{Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	return e, start
}

func TestComposedIncrementsRunToCompletionUnderDefaultRules(t *testing.T) {
	e, start := startRules(t, fixture)
	first := composedPlan(t, e, 1, implementInstance("implement-1", doneTo("verification-1")), verificationInstance("verification-1", doneTo("replan")))
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, first)})
	replanning := drive(t, e)
	if replanning.Instruction == nil || replanning.Instruction.Kind != form.InstructionPlan {
		t.Fatalf("the first increment did not replan: %+v", replanning)
	}
	second := composedPlan(t, e, 2, implementInstance("implement-2", doneTo("verification-2")), verificationInstance("verification-2", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	collect(t, e, replanning.Instruction, form.PlannerResult{InstructionID: replanning.Instruction.ID, Plan: marshal(t, second)})
	approval := checkedResult(t, e)
	if approval.Instruction.Question == nil || !strings.Contains(approval.Instruction.Question.Text, "Structural difference from the last approved plan (version 1)") ||
		!strings.Contains(approval.Instruction.Question.Text, `"StageID": "implement-2"`) {
		t.Fatalf("the replanned approval omitted the difference from the approved plan: %+v", approval.Instruction)
	}
	answerResult(t, e, approval, ledger.AnswerApproved)
	done := drive(t, e)
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("the composed plan did not complete: %+v", done)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	ran := []string{}
	for _, run := range state.StageRuns() {
		ran = append(ran, run.StageID)
	}
	if strings.Join(ran, ",") != "implement-1,verification-1,implement-2,verification-2,retrospective" {
		t.Fatalf("instances ran out of order: %v", ran)
	}
	for _, definition := range state.Metrics().Definitions {
		if definition.Definition == "implement" && (definition.Instances != 2 || definition.Runs != 2) {
			t.Fatalf("definition counts do not group instances: %+v", definition)
		}
	}
	if subject := lastCommitSubject(t, e); !strings.Contains(git(t, e.Root(), "log", "--format=%s"), "implement-2") {
		t.Fatalf("commit messages do not name instances: %s", subject)
	}
}

func TestPlanAcceptanceRejectsRuleAndStructureViolations(t *testing.T) {
	cases := []struct {
		name   string
		stages func() []plan.Stage
		reason string
	}{
		{"verification dropped after implement", func() []plan.Stage {
			return []plan.Stage{implementInstance("implement-1", doneTo("retrospective")), retrospectiveInstance("retrospective")}
		}, "followed in the same segment by verification"},
		{"replan bypasses verification", func() []plan.Stage {
			return []plan.Stage{implementInstance("implement-1", doneTo("replan"))}
		}, "followed in the same segment by verification"},
		{"required stage unreachable", func() []plan.Stage {
			return []plan.Stage{implementInstance("implement-1", doneTo("retrospective")), retrospectiveInstance("retrospective"), verificationInstance("verification-1", doneTo("retrospective"))}
		}, "unreachable from start_stage"},
		{"ask targets another stage", func() []plan.Stage {
			stage := implementInstance("implement-1", doneTo("verification-1"))
			stage.Transitions.Ask = "verification-1"
			return []plan.Stage{stage, verificationInstance("verification-1", doneTo("retrospective")), retrospectiveInstance("retrospective")}
		}, "ask must target the current stage"},
		{"limit above the ceiling", func() []plan.Stage {
			stage := implementInstance("implement-1", doneTo("verification-1"))
			stage.Limits.MaxRounds = 5
			return []plan.Stage{stage, verificationInstance("verification-1", doneTo("retrospective")), retrospectiveInstance("retrospective")}
		}, "stage limits exceed global ceilings"},
		{"reference resolves to nothing", func() []plan.Stage {
			stage := instance("draft", "missing-definition", doneTo("retrospective"))
			return []plan.Stage{stage, retrospectiveInstance("retrospective")}
		}, "stages:missing-definition.md"},
		{"retrospective not last", func() []plan.Stage {
			return []plan.Stage{instance("retrospective-1", "retrospective", doneTo("edit-1"), "harness"), instance("edit-1", "edit", nil, "wording")}
		}, "last definition must be retrospective"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, start := startRules(t, fixture)
			result := propose(t, e, start.Instruction, composedPlan(t, e, 1, tc.stages()...))
			if reason := rejection(t, start.Instruction, result); !strings.Contains(reason, tc.reason) {
				t.Fatalf("rejection omitted %q: %s", tc.reason, reason)
			}
		})
	}
}

func TestHumanPlanIsRejectedByTheSameRules(t *testing.T) {
	e, _ := startRules(t, fixture)
	p := composedPlan(t, e, 1, implementInstance("implement-1", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	if _, err := e.Plan(t.Context(), marshal(t, p)); err == nil || !strings.Contains(err.Error(), "followed in the same segment by verification") {
		t.Fatalf("devkit plan accepted a plan without verification: %v", err)
	}
}

func TestInstanceIDsRerunOnlyTheirDefinitionAndBackTargetsRan(t *testing.T) {
	e, start := startRules(t, fixture)
	first := composedPlan(t, e, 1, implementInstance("implement-1", doneTo("verification-1")), verificationInstance("verification-1", doneTo("replan")))
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, first)})
	replanning := drive(t, e)
	reused := composedPlan(t, e, 2, instance("implement-1", "edit", doneTo("verification-2"), "wording"), verificationInstance("verification-2", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	result := propose(t, e, replanning.Instruction, reused)
	if reason := rejection(t, replanning.Instruction, result); !strings.Contains(reason, `instance ID ran with definition "implement"`) {
		t.Fatalf("a reused ID changed its definition: %s", reason)
	}
	ghost := composedPlan(t, e, 2, implementInstance("implement-2", doneTo("verification-2")), verificationInstance("verification-2", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	ghost.Stages[0].Transitions.Back = "ghost"
	retried := propose(t, e, result.Instruction, ghost)
	if reason := rejection(t, result.Instruction, retried); !strings.Contains(reason, `back transition target not found: "ghost"`) {
		t.Fatalf("a back target outside the work was accepted: %s", reason)
	}
	rerun := composedPlan(t, e, 2, implementInstance("implement-1", doneTo("verification-2")), verificationInstance("verification-2", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	rerun.Stages[0].Transitions.Back = "verification-1"
	accepted := propose(t, e, retried.Instruction, rerun)
	if accepted.Instruction != nil && accepted.Instruction.Kind == form.InstructionPlan {
		t.Fatalf("rerunning an instance with its definition was rejected: %s", briefText(t, accepted.Instruction))
	}
}

func TestRulesAreRecheckedAtStageStartAfterConfirmedLoopEdit(t *testing.T) {
	e, start := startRules(t, fixture)
	p := composedPlan(t, e, 1, implementInstance("implement-1", doneTo("verification-1")), verificationInstance("verification-1", doneTo("retrospective")), retrospectiveInstance("retrospective"))
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	work := checkedNext(t, e)
	if work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected implement work: %+v", work)
	}
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[[loop.stage]]\ndefinition = \"verification\"\ngate = true\n")
	if _, err := e.Submit(t.Context(), marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Required a gate for verification"}})); err != nil {
		t.Fatal(err)
	}
	asked := false
	for range 60 {
		result, err := e.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		instructions := result.Instructions
		if len(instructions) == 0 && result.Instruction != nil {
			instructions = []form.Instruction{*result.Instruction}
		}
		for i := range instructions {
			instruction := &instructions[i]
			if instruction.Question != nil && instruction.Question.StopCategory == "meta_loop_edit" {
				asked = true
			}
			if instruction.Kind == form.InstructionPlan {
				assertStageStartRejection(t, e, instruction, asked)
				return
			}
			if driveInstruction(t, e, instruction) {
				t.Fatalf("the work ended without rechecking the rules: %+v", result)
			}
		}
	}
	t.Fatal("the stage start never rechecked the rules")
}

func assertStageStartRejection(t *testing.T, e *Engine, instruction *form.Instruction, asked bool) {
	t.Helper()
	body := briefText(t, instruction)
	if !asked || !strings.Contains(body, "Current plan rejected at stage start") || !strings.Contains(body, "a human gate is required") {
		t.Fatalf("the stage start did not reject the plan under the confirmed rule: %v %s", asked, body)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	if run, _ := state.LatestStageRun(); run.StageID != "implement-1" || run.State != ledger.StageCompleted {
		t.Fatalf("verification started under the violated rule: %+v", run)
	}
}

func TestShippedDefaultPatternRunsWithDefaultRoles(t *testing.T) {
	e, start := startRules(t, defaultFixture)
	template, source, err := e.pattern(t.Context(), "HEAD", "default")
	if err != nil {
		t.Fatal(err)
	}
	full := newPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}})
	for i := range full.Stages {
		for j := range full.Stages[i].Checks {
			full.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
			full.Stages[i].Checks[j].Target = "The change"
		}
	}
	first := chainStages(full, "purpose")
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, first)})
	replanning := drive(t, e)
	if replanning.Instruction == nil || replanning.Instruction.Kind != form.InstructionPlan {
		t.Fatalf("purpose did not replan: %+v", replanning)
	}
	rest := chainStages(full, "scope", "structure", "contract", "steps", "implement", "verification", "retrospective")
	rest.Version = 2
	collect(t, e, replanning.Instruction, form.PlannerResult{InstructionID: replanning.Instruction.ID, Plan: marshal(t, rest)})
	done := drive(t, e)
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("the default pattern did not complete with the default roles: %+v", done)
	}
}
