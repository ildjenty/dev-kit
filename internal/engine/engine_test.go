package engine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/host/event"
	"github.com/ildjenty/dev-kit/internal/install"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func write(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// fixture configures the session as the worker (a work role without an agent), the
// configuration kept for repositories that do not delegate work to subagents. The "agent"
// role keeps a devkit-worker subagent available for worktree units.
func fixture(t *testing.T) (*Engine, context.Context) {
	t.Helper()
	return fixtureWith(t, func(settings string) string {
		return strings.Replace(settings, `agent = "devkit-worker"`, `agent = ""`, 1) + "\n[roles.agent]\nkind = 'work'\nagent = 'devkit-worker'\n"
	})
}

// defaultFixture keeps the installed roles, where the worker is the devkit-worker subagent.
func defaultFixture(t *testing.T) (*Engine, context.Context) {
	t.Helper()
	return fixtureWith(t, func(settings string) string { return settings })
}

func fixtureWith(t *testing.T, roles func(string) string) (*Engine, context.Context) {
	t.Helper()
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(name, "GIT_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
	ctx := context.Background()
	root := t.TempDir()
	t.Cleanup(func() {
		if err := removeExpansion(filepath.Join(root, ".devkit/run/work")); err != nil {
			t.Error(err)
		}
	})
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "engine@example.invalid")
	git(t, root, "config", "user.name", "Engine Test")
	git(t, root, "config", "commit.gpgsign", "false")
	write(t, root, "README.md", "# Test repository\n")
	if _, err := install.Init(ctx, root, func([]install.Change) error { return nil }); err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(root, "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	configured := roles(withoutLoopRules(string(settings)))
	write(t, root, "devkit.toml", configured+planCatalog+"\n[approval]\nplan = 'deviation'\n")
	template := plan.PlanTemplate{Name: "test", StartStage: "implement", Stages: []plan.Stage{{ID: "implement", Assignee: plan.Worker, Gate: plan.Gate{Reason: "Documentation task"}, Checks: []plan.Check{}, Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}, Transitions: plan.Transitions{Fix: "implement", Ask: "implement", Back: "implement"}}}, OptionalStages: []plan.Stage{}}
	source, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, ".devkit/plans/test.json", string(source))
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "Install test loop")
	if err := hook.PublishSession(filepath.Join(root, ".devkit/run"), hook.SessionObservation{Host: event.ClaudeCode, SessionID: "session-one", ObservedAt: time.Now().UTC()}, "SessionStart"); err != nil {
		t.Fatal(err)
	}
	e, err := newTestEngine(ctx, root, "session-one")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.initialize(); err != nil {
		t.Fatal(err)
	}
	return e, ctx
}
func marshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func collect(t *testing.T, e *Engine, instruction *form.Instruction, value any) {
	t.Helper()
	h := hook.Handler{Root: e.Root(), WorkID: e.workID, Validator: e.validator, InstructionRunner: e.InstructionRunner}
	_, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: e.sessionID, AgentID: "agent-one", AgentType: instruction.Runner, LastAssistantMessage: string(marshal(t, value))})
	if err != nil {
		t.Fatal(err)
	}
}
func pass(request check.Request) form.JudgmentResult {
	return form.JudgmentResult{InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion, Norms: normRefs(request.Norms), CompletionConditionIDs: request.CompletionConditionIDs, Judgment: form.Pass, Findings: []form.Finding{}, CheckedScope: []string{"Requested scope"}, UncheckedScope: []string{}, Method: "Compared target with fixed norms", Environment: "Test fixture"}
}
func planResult(t *testing.T, e *Engine, start Result) plan.Plan {
	t.Helper()
	return planResultWith(t, e, start, nil)
}

// planResultWith returns the pattern's plan after change, which may select its stages.
func planResultWith(t *testing.T, e *Engine, start Result, change func(*plan.Plan)) plan.Plan {
	t.Helper()
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
	for i := range p.Stages {
		for j := range p.Stages[i].Checks {
			p.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
			p.Stages[i].Checks[j].Target = "Documentation"
		}
	}
	if change != nil {
		change(&p)
	}
	collect(t, e, start.Instruction, form.PlannerResult{InstructionID: start.Instruction.ID, Plan: marshal(t, p)})
	return p
}

// ask returns a confirmation request without a recommendation.
func ask(text string) *form.Confirmation {
	return &form.Confirmation{Text: text, Options: []string{}, Evidence: []form.Reference{}}
}

// observed reduces a judgment to the compact detection observation format.
func observed(id string, j form.JudgmentResult, reason string) form.DetectionObservation {
	observation := form.DetectionObservation{ID: id, Judgment: j.Judgment, Blocking: []form.DetectionFinding{}, Reason: reason}
	for _, finding := range j.Findings {
		if finding.Blocking {
			observation.Blocking = append(observation.Blocking, form.DetectionFinding{Scenario: finding.Scenario})
		}
	}
	return observation
}

func acceptDetectionResult(t *testing.T, e *Engine, instruction *form.Instruction) {
	t.Helper()
	data, err := e.readInstruction(instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := form.DetectionResult{InstructionID: instruction.ID}
	for _, example := range data.Detection.Spec.Cases {
		j := pass(*data.Request)
		j.Judgment = example.Expected
		if example.Kind == check.Violation {
			j.Findings = []form.Finding{{ID: "F1", Blocking: true, Kind: form.AutoFixable, Scenario: "The example violates the stated blocking condition"}}
		}
		result.Observations = append(result.Observations, observed(example.ID, j, "The actual finding matches the intended distinction"))
	}
	collect(t, e, instruction, result)
}
func working(t *testing.T, e *Engine, ctx context.Context) Result {
	t.Helper()
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	planResult(t, e, start)
	detection, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if detection.Instruction == nil || detection.Instruction.Kind != form.InstructionDetectionTest {
		t.Fatalf("expected detection: %+v", detection)
	}
	acceptDetectionResult(t, e, detection.Instruction)
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionPlanCheck {
		t.Fatalf("expected plan check: %+v", review)
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
	if work.Instruction == nil || work.Instruction.Kind != form.InstructionWork {
		t.Fatalf("expected work: %+v", work)
	}
	return work
}
func TestLifecycleAndIdempotentNext(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Instruction.ID != work.Instruction.ID || len(records) != len(after) {
		t.Fatal("repeated next changed instruction or records")
	}
	write(t, e.Root(), "README.md", "# Updated documentation\n")
	_, err = e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{"Updated documentation"}}))
	if err != nil {
		t.Fatal(err)
	}
	done, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != ledger.WorkCompleted || done.Instruction == nil || done.Instruction.Kind != form.InstructionCompleted {
		t.Fatalf("expected completion: %+v", done)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	run, _ := state.LatestStageRun()
	if run.Commit == "" {
		t.Fatal("completed stage has no commit")
	}
	records, err = e.records()
	if err != nil {
		t.Fatal(err)
	}
	before := len(records)
	_, err = e.Pause(ctx, "Already ended")
	if err != nil {
		t.Fatal(err)
	}
	records, err = e.records()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != before {
		t.Fatal("pause appended records after completion")
	}
	state, err = e.load()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, paused := state.Paused(); paused {
		t.Fatal("completed work was paused")
	}
}
func TestPauseWithoutLockAndBranchMismatch(t *testing.T) {
	e, ctx := fixture(t)
	started := git(t, e.Root(), "branch", "--show-current")
	work := working(t, e, ctx)
	lock, err := e.store().Lock()
	if err != nil {
		t.Fatal(err)
	}
	git(t, e.Root(), "switch", "-c", "unrelated")
	paused, err := e.Pause(ctx, "Human requested a pause")
	if err != nil {
		t.Fatal(err)
	}
	if !paused.Paused {
		t.Fatal("pause was not recorded")
	}
	result, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instruction != nil || !result.Paused {
		t.Fatal("next advanced paused work")
	}
	lock.Close()
	if _, err = e.Resume(ctx); err == nil {
		t.Fatal("resumed on a different branch")
	}
	git(t, e.Root(), "switch", started)
	if _, err = e.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	resumed, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Instruction.ID != work.Instruction.ID {
		t.Fatal("resume invalidated the work instruction")
	}
}
func TestCommandTargetMutation(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	round, _ := state.OpenRound()
	request := check.Request{RequestInput: check.RequestInput{InstructionID: work.Instruction.ID, Runner: "command:test", Aspect: check.Aspect{ID: "command:test"}, TargetVersion: round.StartTarget, Norms: []check.Norm{}, TargetPath: e.Root()}}
	// Use a real process to verify mutation detection after execution.
	command := config.Command{Executable: "/bin/sh", Args: []string{"-c", "printf 'Changed by command\\n' > README.md"}, TimeoutSeconds: 5}
	judgment, err := e.runCheck(ctx, request, command, "implement", round.StartTarget.Tree)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(e.Root(), "README.md"))
	if err != nil || string(content) != "Changed by command\n" {
		t.Fatalf("command did not change the target: %q, %v", content, err)
	}
	if judgment.Command == nil || judgment.Command.ExitCode != 0 || judgment.Command.TimedOut {
		t.Fatalf("actual command result was not preserved: %+v", judgment.Command)
	}
	if judgment.Judgment != check.Fail || !strings.Contains(judgment.Findings[0].Scenario, "changed its target") {
		t.Fatalf("mutation was accepted: %+v", judgment)
	}
}

func observeAnswer(t *testing.T, e *Engine, text string) {
	t.Helper()
	h := hook.Handler{Root: e.Root(), WorkID: e.workID}
	if _, err := h.Process(event.Event{Host: event.ClaudeCode, Name: event.UserPromptSubmit, SessionID: e.sessionID, Prompt: text}); err != nil {
		t.Fatal(err)
	}
}
func TestAnswerKeepsObservedInputAndCorrection(t *testing.T) {
	e, ctx := fixture(t)
	work := working(t, e, ctx)
	_, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationQuestion, Confirmation: ask("Confirm how to continue."), Obstacles: []string{"Choose the documentation scope"}}))
	if err != nil {
		t.Fatal(err)
	}
	asked, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := asked.Instruction.Question
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerApproved}); err == nil {
		t.Fatal("accepted an answer without a human input observation")
	}
	observeAnswer(t, e, "Please include the installation instructions.")
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Include installation instructions"}); err != nil {
		t.Fatal(err)
	}
	state, err := e.load()
	if err != nil {
		t.Fatal(err)
	}
	answer := state.AnsweredQuestions()[q.ID]
	if answer.Answer.Original != "Please include the installation instructions." || answer.Answer.SessionID != e.sessionID || answer.Answer.ObservedAt.IsZero() {
		t.Fatalf("missing answer provenance: %+v", answer)
	}
	observeAnswer(t, e, "Actually, keep the original scope.")
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: q.ID, Interpretation: ledger.AnswerText, Text: "Keep the original scope", Supersedes: answer.RecordID}); err != nil {
		t.Fatal(err)
	}
	next, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Instruction == nil || next.Instruction.Kind != form.InstructionWork || next.Instruction.ID == work.Instruction.ID {
		t.Fatalf("expected a new work round: %+v", next)
	}
}
func TestNewSessionReissuesPlannerAndWork(t *testing.T) {
	e, ctx := fixture(t)
	start, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"})
	if err != nil {
		t.Fatal(err)
	}
	if err = hook.PublishSession(e.runRoot(), hook.SessionObservation{Host: event.ClaudeCode, SessionID: "session-two", ObservedAt: time.Now().UTC()}, "SessionStart"); err != nil {
		t.Fatal(err)
	}
	second, err := newTestEngine(ctx, e.Root(), "session-two")
	if err != nil {
		t.Fatal(err)
	}
	reissued, err := second.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reissued.Instruction == nil || reissued.Instruction.ID == start.Instruction.ID || reissued.Instruction.Reissue != 1 {
		t.Fatalf("planner was not reissued: %+v", reissued)
	}
	old := second.sessionID
	second.sessionID = e.sessionID
	planResult(t, second, start)
	second.sessionID = old
	next, err := second.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Instruction.ID != reissued.Instruction.ID {
		t.Fatal("accepted the superseded planner result")
	}
}
func TestRequiredCommandFailsBeforeJudgeAndStopsAtLimit(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[commands.fail]\nexecutable = '/usr/bin/false'\nargs = []\ntimeout_seconds = 5\n")
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Checks = []plan.Check{judgeCheck("plan", false), {Aspect: "command:fail", Required: true}}
	template.Stages[0].Limits = plan.Limits{MaxRounds: 1, MaxNoProgress: 1}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Configure command check")
	work := working(t, e, ctx)
	if _, err = e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	result, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instruction == nil || result.Instruction.Kind != form.InstructionQuestion {
		t.Fatalf("expected a limit question: %+v", result)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range records {
		if j, ok := r.Payload.(*ledger.Judgment); ok && j.RoundID != "" {
			if j.Aspect != "command:fail" {
				t.Fatal("judge ran after a required verification failure")
			}
			found = true
			if j.Judgment.Judgment != check.Fail || j.Command == nil || j.Command.ExitCode != 1 || j.Command.TimedOut || len(j.Findings) != 1 || !strings.Contains(j.Findings[0].Scenario, "command:fail failed with exit code 1") {
				t.Fatalf("missing command failure evidence: %+v", j)
			}
		}
	}
	if !found {
		t.Fatal("required command result was not recorded")
	}
	observeAnswer(t, e, "Try one more round.")
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: result.Instruction.Question.ID, Interpretation: ledger.AnswerApproved, Text: "Try another round"}); err != nil {
		t.Fatal(err)
	}
	continued, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if continued.Instruction == nil || continued.Instruction.Kind != form.InstructionWork {
		t.Fatalf("did not continue after human input: %+v", continued)
	}
}

func TestGateRequiresCurrentObservedApproval(t *testing.T) {
	e, ctx := fixture(t)
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Gate.Required = true
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Require stage approval")
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# First target\n")
	if _, err = e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	asked, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if asked.Instruction == nil || asked.Instruction.Question == nil {
		t.Fatalf("gate was skipped: %+v", asked)
	}
	oldID := asked.Instruction.Question.ID
	write(t, e.Root(), "README.md", "# Revised target\n")
	revised, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if revised.Instruction.Question.ID == oldID {
		t.Fatal("changed target kept the old question")
	}
	observeAnswer(t, e, "Approve the revised documentation.")
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: oldID, Interpretation: ledger.AnswerApproved}); err == nil {
		t.Fatal("accepted approval of a superseded question")
	}
	if _, err = e.Answer(ctx, AnswerRequest{QuestionID: revised.Instruction.Question.ID, Interpretation: ledger.AnswerApproved}); err != nil {
		t.Fatal(err)
	}
	done, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("approved target did not complete: %+v", done)
	}
}
func TestCommitTargetRefreshAndReassessment(t *testing.T) {
	e, ctx := fixture(t)
	settings, err := os.ReadFile(filepath.Join(e.Root(), "devkit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, e.Root(), "devkit.toml", string(settings)+"\n[commands.verify]\nexecutable = '/bin/sh'\nargs = ['-c', 'cat README.md']\ntimeout_seconds = 5\n")
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Checks = []plan.Check{{Aspect: "command:verify", Required: true}}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Require verification after formatting")
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Submitted target\n")
	write(t, e.Root(), ".git/hooks/pre-commit", "#!/bin/sh\nprintf '# Formatted target\\n' > README.md\ngit add README.md\n")
	if err := os.Chmod(filepath.Join(e.Root(), ".git/hooks/pre-commit"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	result, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ledger.WorkCompleted {
		t.Fatalf("refreshed target did not complete: %+v", result)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	judgments := []*ledger.Judgment{}
	var refreshed form.TargetVersion
	refreshedJudgment := ""
	for _, r := range records {
		if artifact, ok := r.Payload.(*ledger.Artifact); ok && artifact.ChangeKind == "target_refresh" {
			refreshes++
			refreshed = artifact.After
		}
		if judgment, ok := r.Payload.(*ledger.Judgment); ok && judgment.RoundID != "" {
			judgments = append(judgments, judgment)
			if judgment.TargetVersion == refreshed {
				refreshedJudgment = r.ID
			}
		}
		if run, ok := r.Payload.(*ledger.StageRun); ok && run.State == ledger.StageCompleted && (refreshedJudgment == "" || refreshedJudgment >= r.ID) {
			t.Fatal("stage completed before reassessment of the refreshed target")
		}
	}
	if refreshes != 1 || len(judgments) != 2 || judgments[0].TargetVersion == judgments[1].TargetVersion || judgments[1].TargetVersion != refreshed {
		t.Fatalf("missing reassessment: refreshes=%d, judgments=%+v", refreshes, judgments)
	}
	for i, output := range []string{"# Submitted target\n", "# Formatted target\n"} {
		if judgments[i].Judgment.Judgment != check.Pass || judgments[i].Command == nil || judgments[i].Command.ExitCode != 0 || judgments[i].Command.Output != output {
			t.Fatalf("verification did not run for target %d: %+v", i, judgments[i])
		}
	}
}

func TestJudgeAcceptanceUsesFirstCollectedResult(t *testing.T) {
	e, ctx := fixture(t)
	source, err := os.ReadFile(filepath.Join(e.Root(), ".devkit/plans/test.json"))
	if err != nil {
		t.Fatal(err)
	}
	template, err := e.plans.ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	template.Stages[0].Checks = []plan.Check{judgeCheck("plan", true)}
	write(t, e.Root(), ".devkit/plans/test.json", string(marshal(t, template)))
	git(t, e.Root(), "add", ".")
	git(t, e.Root(), "commit", "-qm", "Require independent judgment")
	work := working(t, e, ctx)
	write(t, e.Root(), "README.md", "# Evaluated documentation\n")
	if _, err = e.Submit(ctx, marshal(t, form.Progress{InstructionID: work.Instruction.ID, TerminationKind: form.TerminationCompleted})); err != nil {
		t.Fatal(err)
	}
	review, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if review.Instruction == nil || review.Instruction.Kind != form.InstructionJudgment {
		t.Fatalf("expected round judgment: %+v", review)
	}
	request, err := e.readInstruction(review.Instruction.ID)
	if err != nil {
		t.Fatal(err)
	}
	hasPlan := strings.Contains(briefText(t, review.Instruction), "Agreed plan")
	if request.Request.Progress == nil || !hasPlan {
		t.Fatal("judgment omitted progress or the agreed plan")
	}
	collect(t, e, review.Instruction, pass(*request.Request))
	late := pass(*request.Request)
	late.Judgment = form.Fail
	late.Findings = []form.Finding{{ID: "late-finding", Blocking: true, Kind: form.AutoFixable, Scenario: "A later result disagrees"}}
	h := hook.Handler{Root: e.Root(), WorkID: e.workID, Validator: e.validator, InstructionRunner: e.InstructionRunner}
	if _, err = h.Process(event.Event{Host: event.ClaudeCode, Name: event.SubagentStop, SessionID: e.sessionID, AgentID: "late-agent", AgentType: review.Instruction.Runner, LastAssistantMessage: string(marshal(t, late))}); err != nil {
		t.Fatal(err)
	}
	done, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != ledger.WorkCompleted {
		t.Fatalf("late result affected progression: %+v", done)
	}
	records, err := e.records()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range records {
		if j, ok := r.Payload.(*ledger.Judgment); ok && j.InstructionID == review.Instruction.ID {
			count++
			if j.Judgment.Judgment != check.Pass {
				t.Fatal("accepted the later judgment")
			}
		}
	}
	if count != 1 {
		t.Fatalf("accepted %d results for one instruction", count)
	}
}

func TestAbortWhilePausedIsTerminal(t *testing.T) {
	e, ctx := fixture(t)
	if _, err := e.Start(ctx, StartRequest{Pattern: "test", Task: "Update documentation"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pause(ctx, "Stop for now"); err != nil {
		t.Fatal(err)
	}
	aborted, err := e.Abort(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if aborted.Status != ledger.WorkAborted || aborted.Paused || aborted.Instruction == nil || aborted.Instruction.Kind != form.InstructionAborted {
		t.Fatalf("abort did not end paused work: %+v", aborted)
	}
	next, err := e.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != ledger.WorkAborted || next.Paused {
		t.Fatalf("ended work still required resume: %+v", next)
	}
}
