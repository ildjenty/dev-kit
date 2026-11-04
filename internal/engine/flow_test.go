package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

const sessionWorker = "[roles.worker]\nkind = 'work'\nagent = ''\n"

// flowWork configures devkit.toml and the test stage, starts work, and returns the first
// instruction after planning.
func flowWork(t *testing.T, e *Engine, settings string, change func(*plan.Stage), instruct func(*plan.Plan)) Result {
	t.Helper()
	write(t, e.Root(), "devkit.toml", withCatalog("kit_version = 'v3'\n[approval]\nplan = 'deviation'\n[detection]\nrequired = false\n"+settings))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure flow")
	if change != nil {
		configureStage(t, e, change)
	}
	start, err := e.Start(t.Context(), StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.readInstruction(start.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := e.asset(context.Background(), data.Reference, "plans/test.json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	p := newPlan(template, plan.ContentHash(source), "Update documentation", []plan.CompletionCondition{{ID: "C1", Description: "The documentation is updated"}})
	for i := range p.Stages[0].Checks {
		p.Stages[0].Checks[i].CompletionConditionIDs = []string{"C1"}
		p.Stages[0].Checks[i].Target = "Documentation"
	}
	if instruct != nil {
		instruct(&p)
	}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	return roleNext(t, e)
}

func lastCommitSubject(t *testing.T, e *Engine) string {
	t.Helper()
	return git(t, e.Root(), "log", "-1", "--format=%s")
}

func TestSessionWorkRoleStillWorksInCheckout(t *testing.T) {
	e, ctx := fixture(t)
	branch := git(t, e.Root(), "branch", "--show-current")
	work := flowWork(t, e, sessionWorker, nil, nil)
	if work.Instruction == nil || work.Instruction.Kind != form.InstructionWork || work.Instruction.Runner != "" {
		t.Fatalf("a work role without an agent must give the session a checkout work instruction: %+v", work)
	}
	write(t, e.Root(), "README.md", "# Session work\n")
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, CommitMessage: "docs: update the readme"})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.WorkStatus() != ledger.WorkCompleted || lastCommitSubject(t, e) != "docs: update the readme" || git(t, e.Root(), "branch", "--show-current") != branch {
		t.Fatalf("session work did not commit with the worker message on the start branch: %s, %s", state.WorkStatus(), lastCommitSubject(t, e))
	}
}

func TestDefaultWorkerRunsAsSubagentInWorktree(t *testing.T) {
	e, ctx := defaultFixture(t)
	branch := git(t, e.Root(), "branch", "--show-current")
	work := flowWork(t, e, "", nil, nil)
	if len(work.Instructions) != 1 {
		t.Fatalf("expected one worker instruction: %+v", work)
	}
	unit := work.Instructions[0]
	if unit.Kind != form.InstructionUnitWork || unit.Runner != "devkit-worker" || !strings.HasPrefix(unit.TargetPath, filepath.Join(e.Root(), ".devkit", "worktrees")) {
		t.Fatalf("the default worker must be a subagent in a devkit worktree: %+v", unit)
	}
	write(t, unit.TargetPath, "worker.md", "Written by the worker subagent.\n")
	collect(t, e, &unit, form.Progress{InstructionID: unit.ID, TerminationKind: form.TerminationCompleted, CommitMessage: "feat: add the worker document"})
	for range 5 {
		if _, err := e.Next(ctx); err != nil {
			t.Fatal(err)
		}
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.WorkStatus() != ledger.WorkCompleted || lastCommitSubject(t, e) != "feat: add the worker document" {
		t.Fatalf("worktree work was not merged and committed: %s, %s", state.WorkStatus(), lastCommitSubject(t, e))
	}
	if _, err := os.Stat(filepath.Join(e.Root(), "worker.md")); err != nil || git(t, e.Root(), "branch", "--show-current") != branch {
		t.Fatalf("merged work is missing from the start branch: %v", err)
	}
}

func threeJudges(stage *plan.Stage) {
	stage.Checks = []plan.Check{judgeCheck("wording", false), judgeCheck("diff", false), judgeCheck("test", false)}
}

func TestParallelJudgesRespectGlobalAndRoleLimits(t *testing.T) {
	for name, tc := range map[string]struct {
		settings string
		want     int
	}{
		"global": {"[parallel]\nmax_concurrent = 2\n", 2},
		"role":   {"[roles.judge]\nmax_concurrent = 1\n", 1},
		"both":   {"[parallel]\nmax_concurrent = 3\n[roles.judge]\nmax_concurrent = 2\n", 2},
	} {
		t.Run(name, func(t *testing.T) {
			e, ctx := fixture(t)
			work := flowWork(t, e, sessionWorker+tc.settings, threeJudges, nil)
			submitWork(t, e, work)
			judges := nextResult(t, e, ctx)
			if len(judges.Instructions) != tc.want {
				t.Fatalf("issued %d judges; want %d: %+v", len(judges.Instructions), tc.want, judges)
			}
			for _, issued := range judges.Instructions {
				if issued.Kind != form.InstructionJudgment || !strings.HasSuffix(issued.TargetKey, ":judge") {
					t.Fatalf("judgment key must combine round, aspect, and runner: %+v", issued)
				}
			}
			again := nextResult(t, e, ctx)
			if len(again.Instructions) != tc.want || again.Instructions[0].ID != judges.Instructions[0].ID {
				t.Fatalf("outstanding judges were not reissued unchanged: %+v", again)
			}
			first := judges.Instructions[0]
			data, err := e.readInstruction(first.ID)
			if err != nil {
				t.Fatal(err)
			}
			collect(t, e, &first, pass(*data.Request))
			after := nextResult(t, e, ctx)
			if len(after.Instructions) != min(tc.want, 2) || slices.ContainsFunc(after.Instructions, func(i form.Instruction) bool { return i.ID == first.ID }) {
				t.Fatalf("a freed slot was not used for the next judge: %+v", after)
			}
		})
	}
}

func withReport(stage *plan.Stage) {
	stage.Report = &plan.Selection{Source: "templates", Select: "reports/implement.json"}
}

func TestReportIsValidatedOnlyForCompletionClaims(t *testing.T) {
	e, ctx := fixture(t)
	work := flowWork(t, e, sessionWorker, withReport, nil)
	if !strings.Contains(briefText(t, work.Instruction), "Selected report JSON Schema") {
		t.Fatal("work brief omitted the selected report schema")
	}
	for _, report := range []json.RawMessage{nil, json.RawMessage(`{"unexpected": true}`)} {
		_, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, Report: report}))
		if err == nil || !strings.Contains(err.Error(), "report") {
			t.Fatalf("accepted a completion claim with report %s: %v", report, err)
		}
	}
	partial := form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Which test layer should cover C1?"), Report: json.RawMessage(`{}`)}
	if _, err := e.Submit(ctx, marshal(t, partial)); err != nil {
		t.Fatalf("rejected a partial report on a confirmation request: %v", err)
	}
}

func TestConfirmationIsRenderedVerbatimIntoTheQuestion(t *testing.T) {
	e, ctx := fixture(t)
	work := flowWork(t, e, sessionWorker, nil, nil)
	confirmation := &form.Confirmation{Text: "Keep the legacy flag for one release?", Options: []string{"keep", "remove"}, Recommendation: "keep", RecommendationReason: "Two callers still pass it.", Evidence: []form.Reference{{External: "callers.txt"}}}
	progress := form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: confirmation, ResumeConditions: []string{"The human chooses an option"}}
	if _, err := e.Submit(ctx, marshal(t, progress)); err != nil {
		t.Fatal(err)
	}
	asked := nextResult(t, e, ctx)
	q := asked.Instruction.Question
	if q == nil || !strings.HasPrefix(q.Text, confirmation.Text) || !slices.Equal(q.Options, confirmation.Options) || q.Recommendation != "keep" || q.RecommendationReason != confirmation.RecommendationReason {
		t.Fatalf("question did not carry the confirmation verbatim: %+v", q)
	}
	if !strings.Contains(q.Text, "The human chooses an option") || !slices.Contains(q.Evidence, form.Reference{External: "callers.txt"}) {
		t.Fatalf("question omitted the materials: %+v", q)
	}
}

func TestHistoryScopeNeverDropsRequiredEvidence(t *testing.T) {
	e, ctx := fixture(t)
	settings := sessionWorker + "[commands.verify]\nexecutable = '/bin/sh'\nargs = ['-c', 'echo verified-output-marker']\ntimeout_seconds = 5\n" + judgeEntry("wording", "history = 'none'")
	work := flowWork(t, e, settings, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:verify", Required: true}, judgeCheck("wording", false)}
	}, nil)
	write(t, e.Root(), "README.md", "# Judged change\n")
	submitWork(t, e, work)
	judgment := nextResult(t, e, ctx)
	if judgment.Instruction == nil || judgment.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected a judgment: %+v", judgment)
	}
	body := briefText(t, judgment.Instruction)
	for _, want := range []string{"verified-output-marker", "Judged round report", "Stage comparison", "README.md"} {
		if !strings.Contains(body, want) {
			t.Fatalf("judge brief omitted required evidence %q", want)
		}
	}
	history, err := os.ReadFile(filepath.Join(e.workRoot(), "brief", judgment.Instruction.ID+"-history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(history), "scope: none") || strings.Contains(string(history), " judgment\n") {
		t.Fatalf("history ignored the none scope: %s", history)
	}
}

func TestBriefByPlannerAddsSupplementaryInstructionBeforeJudging(t *testing.T) {
	e, ctx := fixture(t)
	work := flowWork(t, e, sessionWorker+judgeEntry("wording", "brief_by_planner = true"), func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
	}, nil)
	submitWork(t, e, work)
	planned := nextResult(t, e, ctx)
	if planned.Instruction == nil || planned.Instruction.Kind != form.InstructionJudgmentBrief || planned.Instruction.Runner != "devkit-planner" {
		t.Fatalf("expected a pre-judgment planner instruction: %+v", planned)
	}
	collect(t, e, planned.Instruction, map[string]any{"instruction_id": planned.Instruction.ID, "instructions": []string{"Read the README heading first."}})
	judgment := nextResult(t, e, ctx)
	if judgment.Instruction == nil || judgment.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected the judgment after the planner instruction: %+v", judgment)
	}
	body := briefText(t, judgment.Instruction)
	if !strings.Contains(body, "Read the README heading first.") || !strings.Contains(body, "cannot narrow") {
		t.Fatal("judgment brief omitted the supplementary planner instruction")
	}
}

func TestPlanInstructionsAndDelegationsReachBriefsVerbatim(t *testing.T) {
	e, ctx := fixture(t)
	work := flowWork(t, e, sessionWorker, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{judgeCheck("wording", false)}
		stage.Delegations = []plan.Selection{{Source: "rules", Select: ""}}
	}, func(p *plan.Plan) {
		p.Stages[0].Instructions = []string{"Keep the README under ten lines."}
		p.Stages[0].Checks[0].Instructions = []string{"Check the heading wording."}
	})
	if !strings.Contains(briefText(t, work.Instruction), "Keep the README under ten lines.") {
		t.Fatal("work brief omitted the plan instruction")
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	p, _ := latestPlan(state)
	approval, err := e.planApprovalText(ctx, state, p.Plan, "HEAD")
	if err != nil || !strings.Contains(approval, "Keep the README under ten lines.") || !strings.Contains(approval, "Check the heading wording.") {
		t.Fatalf("plan approval omitted the instructions: %v", err)
	}
	submitWork(t, e, work)
	judgment := nextResult(t, e, ctx)
	body := briefText(t, judgment.Instruction)
	if !strings.Contains(body, "Check the heading wording.") || !strings.Contains(body, "Source: rules/") || !strings.Contains(body, "distinct from approvals") {
		t.Fatal("judgment brief omitted check instructions or source-attributed delegations")
	}
}

func TestWorkWideRoundLimitAsksAndExtendsFromCurrentTotal(t *testing.T) {
	e, ctx := fixture(t)
	settings := sessionWorker + "[limits]\nmax_total_rounds = 1\nmax_rounds_ceiling = 5\nmax_no_progress_ceiling = 5\n[commands.fail]\nexecutable = '/usr/bin/false'\ntimeout_seconds = 5\n"
	work := flowWork(t, e, settings, func(stage *plan.Stage) {
		stage.Checks = []plan.Check{{Aspect: "command:fail", Required: true}}
		stage.Limits = plan.Limits{MaxRounds: 5, MaxNoProgress: 5}
	}, nil)
	submitWork(t, e, work)
	if _, err := e.Next(ctx); err != nil {
		t.Fatal(err)
	}
	limit := nextResult(t, e, ctx)
	if limit.Instruction == nil || limit.Instruction.Question == nil || limit.Instruction.Question.StopCategory != totalRoundLimit {
		t.Fatalf("expected the work-wide round limit question: %+v", limit)
	}
	answerResult(t, e, limit, ledger.AnswerApproved)
	next := nextResult(t, e, ctx)
	if next.Instruction == nil || next.Instruction.Kind != form.InstructionWork {
		t.Fatalf("approval did not continue the work: %+v", next)
	}
	status, err := e.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Metrics.TotalRounds != 2 || status.Metrics.HumanStops != 2 || status.Metrics.Briefs == 0 || len(status.Metrics.Stages) != 1 {
		t.Fatalf("status metrics do not match the records: %+v", status.Metrics)
	}
}

func TestBriefOverflowMovesWholeBodiesToFiles(t *testing.T) {
	e, _ := fixture(t)
	work := flowWork(t, e, sessionWorker+"[briefs]\nmax_bytes = 4000\n", nil, nil)
	body := briefText(t, work.Instruction)
	if !strings.Contains(body, "Moved to ") {
		t.Fatal("an oversized brief was not split into files")
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	var overflow []string
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.ID == work.Instruction.ID && instruction.Context != nil {
			overflow = instruction.Context.Overflow
		}
	}
	if len(overflow) == 0 {
		t.Fatal("the overflow was not recorded")
	}
	_, path, _ := strings.Cut(overflow[0], ": ")
	moved, err := os.ReadFile(path)
	if err != nil || len(moved) <= 200 {
		t.Fatalf("moved body is missing or truncated: %v", err)
	}
}

func TestDetectionIdentityIncludesDeclaredModelOnly(t *testing.T) {
	e, ctx := fixture(t)
	item := catalogJudge("plan", true)
	inherited, _, err := e.detectionInput(ctx, "HEAD", item)
	if err != nil {
		t.Fatal(err)
	}
	if inherited.Identity.Model != "" || inherited.Identity.Effort != "" {
		t.Fatalf("an inherited model was recorded as declared: %+v", inherited.Identity)
	}
	path := filepath.Join(e.Root(), ".claude/agents/devkit-judge.md")
	definition, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), ".claude/agents/devkit-judge.md", strings.Replace(string(definition), "model: inherit", "model: opus\neffort: high", 1))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Declare the judge model")
	declared, _, err := e.detectionInput(ctx, "HEAD", item)
	if err != nil {
		t.Fatal(err)
	}
	if declared.Identity.Model != "opus" || declared.Identity.Effort != "high" || declared.Identity == inherited.Identity {
		t.Fatalf("the declared model and effort are not part of the pass identity: %+v", declared.Identity)
	}
}
