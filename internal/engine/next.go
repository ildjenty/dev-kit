package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/external"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
	"github.com/ildjenty/dev-kit/internal/workspace"
	"github.com/oklog/ulid/v2"
)

func ulidString() string { return ulid.Make().String() }

// Next accepts available hook results and advances until one next action is ready.
// A branch mismatch is a read-only query, and pause is checked before taking a lock.
func (e *Engine) Next(ctx context.Context) (result Result, err error) {
	state, warning, err := e.resolve(ctx, false)
	if err != nil {
		return result, err
	}
	if _, _, paused := state.Paused(); paused && state.WorkStatus() == ledger.WorkActive {
		return response(state), nil
	}
	if warning != "" {
		return e.branchWarning(state, warning)
	}
	if err := e.initialize(); err != nil {
		return result, err
	}
	if _, _, err = e.resolve(ctx, true); err != nil {
		return result, err
	}
	if err = e.checkSession(); err != nil {
		return result, err
	}
	lock, err := e.store().Lock()
	if err != nil {
		return result, err
	}
	defer lock.Close()
	defer func() {
		s, loadErr := e.load()
		if loadErr == nil {
			if _, _, paused := s.Paused(); paused && s.WorkStatus() == ledger.WorkActive {
				result = response(s)
				err = nil
			}
		}
	}()
	for {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		state, err = e.load()
		if err != nil {
			return result, err
		}
		if _, _, paused := state.Paused(); paused && state.WorkStatus() == ledger.WorkActive {
			return response(state), nil
		}
		if state.WorkStatus() != ledger.WorkActive {
			return e.terminal(ctx, state)
		}
		r, ready, stepErr := e.advanceNext(ctx, state)
		if stepErr != nil || ready {
			return r, stepErr
		}
	}
}

func findPlan(versions []plan.VersionEvaluation, n int) (plan.Plan, bool) {
	for _, v := range versions {
		if v.Plan.Version == n {
			return v.Plan, true
		}
	}
	return plan.Plan{}, false
}
func (e *Engine) boundPlan(n int) (plan.Plan, bool, error) {
	records, err := e.records()
	if err != nil {
		return plan.Plan{}, false, err
	}
	for _, r := range records {
		if p, ok := r.Payload.(*ledger.PlanVersion); ok && p.Plan.Version == n {
			return p.Plan, true, nil
		}
	}
	return plan.Plan{}, false, nil
}
func (e *Engine) planRounds() (int, error) {
	records, err := e.records()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range records {
		switch p := r.Payload.(type) {
		case *ledger.Answer:
			if p.Target.Kind == form.ApprovalPlan {
				n = 0
			}
		case *ledger.Approval:
			if p.Target.Kind == form.ApprovalPlan {
				n = 0
			}
		case *ledger.Judgment:
			if p.TargetVersion.Kind == form.TargetPlan && p.Judgment.Judgment == check.Fail {
				n++
			}
		}
	}
	return n, nil
}
func (e *Engine) importStops(state *ledger.State) error {
	records, err := e.records()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range records {
		if q, ok := r.Payload.(*ledger.Question); ok {
			seen[q.ID] = true
		}
	}
	for _, q := range state.Questions() {
		if q.StopObservation != nil && !seen[q.ID] {
			if _, err = e.append(&ledger.Question{Question: q}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *Engine) terminal(ctx context.Context, state *ledger.State) (Result, error) {
	if state.WorkStatus() == ledger.WorkCompleted {
		for _, q := range state.UnansweredQuestions() {
			if externalQuestion(&q) {
				return e.issueQuestion(state, q)
			}
		}
	}
	retained, err := e.cleanupUnits(ctx, state)
	if err != nil {
		return Result{}, err
	}
	if err := removeExpansion(filepath.Join(e.workRoot(), "snap")); err != nil {
		return Result{}, err
	}
	if state.WorkStatus() == ledger.WorkCompleted {
		if err := e.workspace.DeleteSnapshots(ctx, e.workID); err != nil {
			return Result{}, err
		}
	}
	kind := form.InstructionCompleted
	if state.WorkStatus() == ledger.WorkAborted {
		kind = form.InstructionAborted
	}
	meta, fresh, err := e.issuance(kind, e.workID, nil, 0, "")
	if err != nil {
		return Result{}, err
	}
	if fresh {
		if _, err = e.append(&meta); err != nil {
			return Result{}, err
		}
	}
	r := response(state)
	r.Instruction = &form.Instruction{ID: meta.ID, Kind: kind, TargetKey: e.workID, Reissue: meta.Reissue, Text: "Work is " + string(state.WorkStatus()) + "."}
	if len(retained) > 0 {
		r.Warning = "Changed unit worktrees were retained:\n" + strings.Join(retained, "\n")
	}
	return r, nil
}

// Submit validates a progress block and records the returned target and artifacts.
func (e *Engine) Submit(ctx context.Context, data []byte) (Result, error) {
	lock, err := e.mutation(ctx)
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	round, ok := state.OpenRound()
	if !ok {
		return Result{}, errors.New("there is no open round")
	}
	var progress form.Progress
	if err = e.forms["progress"].Decode(data, &progress); err != nil {
		return Result{}, err
	}
	progress = repairProgress(state, round, progress)
	current := ""
	for _, i := range state.UnacceptedInstructions() {
		if i.Kind == form.InstructionWork && i.RoundID == round.ID {
			current = i.ID
		}
	}
	records, err := e.records()
	if err != nil {
		return Result{}, err
	}
	for _, record := range records {
		if saved, ok := record.Payload.(*ledger.Progress); ok && saved.RoundID == round.ID && saved.InstructionID == progress.InstructionID {
			if !reflect.DeepEqual(saved.Progress, progress) {
				return Result{}, errors.New("resubmission differs from the recorded progress")
			}
			current = saved.InstructionID
		}
	}
	if current == "" || progress.InstructionID != current {
		return Result{}, errors.New("progress instruction ID does not match the current work instruction")
	}
	if len(e.currentQuestions(state)) != 0 {
		return Result{}, errors.New("an unanswered question blocks submission")
	}
	if err := e.validateRoundReport(ctx, state, round, nil, progress); err != nil {
		return Result{}, err
	}
	run, _ := state.LatestStageRun()
	if err = e.recordSubmission(ctx, state, run, round, progress); err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}
func (e *Engine) recordSubmission(ctx context.Context, state *ledger.State, run *ledger.StageRun, round *ledger.Round, progress form.Progress) error {
	if err := e.active(); err != nil {
		return err
	}
	artifact, accepted, err := e.submittedArtifact(round, progress)
	if err != nil {
		return err
	}
	if handled, err := e.continueUnitSubmission(ctx, state, run, round, progress, accepted); handled || err != nil {
		return err
	}
	if artifact == nil {
		snapshot, err := e.workspace.Snapshot(ctx, e.workID)
		if err != nil {
			return err
		}
		after, err := e.treeTarget(snapshot.Tree)
		if err != nil {
			return err
		}
		artifact = &ledger.Artifact{Before: round.StartTarget, After: after, CompletionConditionIDs: []string{}, ChangeKind: "submission", Decisions: []form.Reference{}}
		if _, err = e.append(artifact); err != nil {
			return err
		}
	}
	if !accepted {
		if _, err = e.append(&ledger.Progress{Progress: progress, RoundID: round.ID}); err != nil {
			return err
		}
	}
	target := artifact.After
	copy := *round
	if copy.ReturnedTarget == nil {
		copy.ReturnedTarget = &target
		if _, err = e.append(&copy); err != nil {
			return err
		}
	}
	if run.State != ledger.StageWorking && !(run.State == ledger.StageAnswerWait && strings.HasPrefix(progress.InstructionID, "human-")) {
		return nil
	}
	update := *run
	update.State = ledger.StageJudgmentWait
	update.NextActor = "judge"
	if progress.TerminationKind == form.TerminationQuestion {
		text, err := e.saveWorkerConfirmation(state, run, round, target, progress)
		if err != nil {
			return err
		}
		update.State = ledger.StageAnswerWait
		update.NextActor = "human"
		copy.Decision = &router.Decision{Action: plan.Ask, Reason: text}
		if _, err = e.append(&copy); err != nil {
			return err
		}
	}
	_, err = e.append(&update)
	return err
}

func (e *Engine) saveWorkerConfirmation(state *ledger.State, run *ledger.StageRun, round *ledger.Round, target form.TargetVersion, progress form.Progress) (string, error) {
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("bound plan is missing")
	}
	next, err := p.Next(run.StageID, plan.Ask)
	if err != nil {
		return "", err
	}
	text := confirmationText(progress)
	q := e.question(approvalTree(target), "worker_confirmation", text, run.StageID)
	q.NextStage = *next
	q.Evidence = []form.Reference{{RecordID: run.ID}, {RecordID: round.ID}}
	if c := progress.Confirmation; c != nil {
		if len(c.Options) > 0 {
			q.Options = slices.Clone(c.Options)
		}
		q.Recommendation, q.RecommendationReason = c.Recommendation, c.RecommendationReason
		q.Evidence = append(q.Evidence, c.Evidence...)
	}
	if source := unitResultRound(state, run.ID); source != nil {
		if source.ID != round.ID {
			q.Evidence = append(q.Evidence, form.Reference{RecordID: source.ID})
		}
	}
	if _, err = e.saveQuestion(state, q, ""); err != nil {
		return "", err
	}
	return text, nil
}

func (e *Engine) recoverSubmission(ctx context.Context, state *ledger.State) (bool, error) {
	run, ok := state.LatestStageRun()
	if !ok || run.State != ledger.StageWorking && run.State != ledger.StageAnswerWait {
		return false, nil
	}
	rounds := state.Rounds(run.ID)
	if len(rounds) == 0 {
		return false, nil
	}
	round := rounds[len(rounds)-1]
	if round.Decision != nil && round.Decision.Action != plan.Ask {
		return false, nil
	}
	records, err := e.records()
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if progress, ok := record.Payload.(*ledger.Progress); ok && progress.RoundID == round.ID {
			if run.State == ledger.StageAnswerWait && !strings.HasPrefix(progress.InstructionID, "human-") {
				continue
			}
			return true, e.recordSubmission(ctx, state, run, round, progress.Progress)
		}
	}
	return false, nil
}

func (e *Engine) closeRound(state *ledger.State, run *ledger.StageRun, round *ledger.Round, decision router.Decision) error {
	if err := e.active(); err != nil {
		return err
	}
	if decision.Action == plan.Fix {
		current, err := e.load()
		if err != nil {
			return err
		}
		p, ok, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("bound plan is missing")
		}
		stage, _ := p.Stage(run.StageID)
		decision, err = e.correctionDecision(current, stage, round, decision)
		if err != nil {
			return err
		}
	}
	copy := *round
	copy.Decision = &decision
	if _, err := e.append(&copy); err != nil {
		return err
	}
	update := *run
	update.StopReason = decision.Reason
	update.State = ledger.StageWorking
	update.NextActor = "worker"
	switch decision.Action {
	case plan.Ask:
		update.State = ledger.StageAnswerWait
		update.NextActor = "human"
	case plan.Back:
		update.State = ledger.StageInvalidated
		update.NextActor = "planner"
	}
	_, err := e.append(&update)
	return err
}

func (e *Engine) refreshTarget(ctx context.Context, state *ledger.State, run *ledger.StageRun, round *ledger.Round, commit string) (Result, error) {
	records, err := e.records()
	if err != nil {
		return Result{}, err
	}
	count := 0
	for _, r := range records {
		if a, ok := r.Payload.(*ledger.Artifact); ok && r.ID > round.ID && a.Before == round.StartTarget && a.ChangeKind == "target_refresh" {
			count++
		}
	}
	if count > 0 {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: round.ReturnedTarget.Tree}, config.TargetChangedAgain, "The target changed again after its one permitted refresh. Confirm how to continue.", run.StageID)
		q, err = e.saveQuestion(state, q, "")
		if err != nil {
			return Result{}, err
		}
		if err = e.closeRound(state, run, round, router.Decision{Action: plan.Ask, Reason: q.Text}); err != nil {
			return Result{}, err
		}
		return e.issueQuestion(state, q)
	}
	snapshot, err := e.workspace.Snapshot(ctx, e.workID)
	if commit != "" {
		snapshot, err = e.workspace.SnapshotFromCommit(ctx, e.workID, commit)
	}
	if err != nil {
		return Result{}, err
	}
	target, err := e.treeTarget(snapshot.Tree)
	if err != nil {
		return Result{}, err
	}
	if _, err = e.append(&ledger.Artifact{Before: round.StartTarget, After: target, ChangeKind: "target_refresh", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}, Commit: commit}); err != nil {
		return Result{}, err
	}
	copy := *round
	copy.ReturnedTarget = &target
	if _, err = e.append(&copy); err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, errors.New("bound plan is missing")
	}
	stage, _ := p.Stage(run.StageID)
	return e.judgeRound(ctx, state, p, stage, run, &copy)
}

func differenceCategories(root string, paths []string, settings config.Config) []string {
	categories := []string{}
	roots := append(repositoryNorms(root, settings), ".claude/settings.json", ".codex/hooks.json", ".codex/config.toml", ".codex/rules", ".agents/skills", ".husky", ".pre-commit-config.yaml")
	for _, path := range paths {
		if settings.IsDependencyFile(path) {
			categories = append(categories, config.DependencyChanged)
		}
		for _, root := range roots {
			if path == root || strings.HasPrefix(path, root+"/") {
				categories = append(categories, config.MetaLoopEdit)
				break
			}
		}
	}
	slices.Sort(categories)
	return slices.Compact(categories)
}

func (e *Engine) metaLoopChanged(ctx context.Context, before, after string) (bool, error) {
	paths, err := e.workspace.ChangedFiles(ctx, gitReference(before), gitReference(after))
	if err != nil {
		return false, err
	}
	for _, ref := range []string{before, after} {
		settings, err := e.settings(ctx, ref)
		if err != nil {
			return false, err
		}
		if slices.Contains(differenceCategories(e.Root(), paths, settings), config.MetaLoopEdit) {
			return true, nil
		}
	}
	hookPath, err := e.workspace.PreCommitPath(ctx)
	return slices.Contains(paths, hookPath), err
}

// startNormsChanged reports loop edits before a round: changes to norms inside the repository
// and changes to the fixed documents of norm sources outside it.
func (e *Engine) startNormsChanged(ctx context.Context, before, after string) (bool, error) {
	changed, err := e.metaLoopChanged(ctx, before, after)
	if err != nil || changed {
		return changed, err
	}
	changes, err := e.externalNormChanges(ctx, before, after)
	return len(changes) > 0, err
}

// confirmedLoopEdits returns the tree whose loop edits a human last approved (the work's base
// commit when none were approved) and the latest rejection made after that approval, if any.
func confirmedLoopEdits(state *ledger.State) (string, *form.Question) {
	work, _ := state.Work()
	base := work.BaseCommit
	var rejected *form.Question
	lastApproval, lastRejection := "", ""
	answers := state.AnsweredQuestions()
	for _, q := range state.Questions() {
		if q.StopCategory != config.MetaLoopEdit || q.ApprovalTarget.Kind != form.ApprovalTree {
			continue
		}
		answer, ok := answers[q.ID]
		if !ok {
			continue
		}
		if effective, ok := state.ResolveAnswer(q.ApprovalTarget); !ok || effective.RecordID != answer.RecordID {
			continue
		}
		if answer.Answer.Interpretation == ledger.AnswerApproved {
			if answer.RecordID > lastApproval {
				base, lastApproval = approvalReference(q.ApprovalTarget), answer.RecordID
			}
		} else if answer.RecordID > lastRejection {
			copy := q
			rejected, lastRejection = &copy, answer.RecordID
		}
	}
	if lastRejection < lastApproval {
		rejected = nil
	}
	return base, rejected
}

func (e *Engine) confirmRoundNorms(ctx context.Context, state *ledger.State, run *ledger.StageRun, tree string) (string, *form.Question, error) {
	base, rejected := confirmedLoopEdits(state)
	settings, err := e.settings(ctx, base)
	if err != nil {
		return "", nil, err
	}
	if tree, err = e.refreshSources(ctx, tree); err != nil {
		return "", nil, err
	}
	if settings.MetaLoopEditApproval() == "allow" {
		return tree, nil, nil
	}
	changed, err := e.startNormsChanged(ctx, base, tree)
	if err != nil || !changed {
		return tree, nil, err
	}
	if rejected != nil {
		changed, err := e.startNormsChanged(ctx, approvalReference(rejected.ApprovalTarget), tree)
		if err != nil {
			return "", nil, err
		}
		if !changed {
			return base, nil, nil
		}
	}
	text, err := e.loopEditText(ctx, base, tree, "Approve development loop edits before opening the next round? If rejected, the worker must undo the edits.")
	if err != nil {
		return "", nil, err
	}
	q := e.question(normApproval(tree), config.MetaLoopEdit, text, run.StageID)
	q.Evidence = []form.Reference{{RecordID: run.ID}}
	q, err = e.saveQuestion(state, q, "")
	return tree, &q, err
}

func (e *Engine) planApprovalText(ctx context.Context, state *ledger.State, p plan.Plan, ref string) (string, error) {
	work, _ := state.Work()
	original := originalRequest(work)
	if work.OriginalRequest == nil {
		original = "No original user request was observed."
	}
	task, err := os.ReadFile(filepath.Join(e.workRoot(), work.TaskRef))
	if err != nil {
		return "", err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return "", err
	}
	before, label, err := e.approvalBaseline(ctx, state, p, ref)
	if err != nil {
		return "", err
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Approve this checked plan?\n\nOriginal user request:\n%s\n\nTask supplied to start:\n%s\n\nCompletion conditions:\n", original, task)
	for _, condition := range p.CompletionConditions {
		fmt.Fprintf(&text, "- %s: %s\n", condition.ID, condition.Description)
	}
	text.WriteString(planChainText(p, checkCatalog(settings)))
	text.WriteString(planInstructionsText(p))
	changes := plan.StructuralDifference(before, p)
	fmt.Fprintf(&text, "\nStructural difference from %s (shown for review, not a reason for approval):\n%s\n\nIrreversible items:\n%s\n\nExternal operations:\n%s\n\nOpen questions (answer with revision instructions):\n%s",
		label, approvalJSON(changes, len(changes)), approvalList(p.IrreversibleDecisions), approvalJSON(p.ExternalOperations, len(p.ExternalOperations)), approvalList(p.OpenQuestions))
	return text.String(), nil
}

// approvalBaseline returns the remaining part of the last approved plan, or the cited pattern
// for the first plan, as the structure the proposal is compared with.
func (e *Engine) approvalBaseline(ctx context.Context, state *ledger.State, p plan.Plan, ref string) (plan.Plan, string, error) {
	var last *plan.Plan
	for _, previous := range state.PlanEvaluations(nil) {
		if previous.Plan.Version < p.Version && previous.Approved && !previous.Rejected && !previous.Withdrawn {
			approved := previous.Plan
			last = &approved
		}
	}
	if last != nil {
		completed, err := e.approvalCompleted(state)
		return plan.RemainingPlan(*last, completed), fmt.Sprintf("the last approved plan (version %d)", last.Version), err
	}
	pattern, _, err := e.pattern(ctx, ref, p.Pattern.Name)
	if err != nil {
		return plan.Plan{}, "", err
	}
	return plan.Plan{StartStage: pattern.StartStage, Stages: pattern.Stages}, "the cited pattern " + pattern.Name, nil
}

// planChainText lists the plan's stages along done transitions with their definition, assignee,
// gate, and required checks; stages off the chain follow.
func planChainText(p plan.Plan, catalog map[string]plan.CheckDefinition) string {
	var text strings.Builder
	fmt.Fprintf(&text, "\nPlan chain from %s:\n", p.StartStage)
	listed := map[string]bool{}
	for id := &p.StartStage; id != nil && *id != "replan" && !listed[*id]; {
		stage, ok := p.Stage(*id)
		if !ok {
			break
		}
		listed[stage.ID] = true
		writeChainStage(&text, stage, catalog)
		id = stage.Transitions.Done
	}
	for _, stage := range p.Stages {
		if !listed[stage.ID] {
			writeChainStage(&text, stage, catalog)
		}
	}
	return text.String()
}

func writeChainStage(text *strings.Builder, stage plan.Stage, catalog map[string]plan.CheckDefinition) {
	gate := "none"
	if stage.Gate.Required {
		gate = "required (" + stage.Gate.Reason + ")"
	}
	done := "null"
	if stage.Transitions.Done != nil {
		done = *stage.Transitions.Done
	}
	fmt.Fprintf(text, "- %s: definition %s, assignee %s, gate %s, done %s\n", stage.ID, stage.DefinitionID(), stage.Assignee, gate, done)
	for _, item := range stage.RequiredChecks() {
		fmt.Fprintf(text, "  - required check %s (%s), conditions: %s; target: %s\n", item.Aspect, catalog[item.Aspect].Runner, strings.Join(item.CompletionConditionIDs, ", "), item.Target)
	}
}

// approvalList renders strings as a bulleted list, or "None" when empty.
func approvalList(items []string) string {
	if len(items) == 0 {
		return "None"
	}
	return "- " + strings.Join(items, "\n- ")
}

// approvalJSON renders structured items as indented JSON, or "None" when empty.
func approvalJSON(value any, count int) string {
	if count == 0 {
		return "None"
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func (e *Engine) commitBase(round *ledger.Round) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if record.ID > round.ID {
			if artifact, ok := record.Payload.(*ledger.Artifact); ok && artifact.ChangeKind == "commit_base" {
				return artifact.Commit, nil
			}
		}
	}
	return "", nil
}

func (e *Engine) recoverCommit(ctx context.Context, state *ledger.State, run *ledger.StageRun) (bool, Result, error) {
	rounds := state.Rounds(run.ID)
	if len(rounds) == 0 {
		return false, Result{}, nil
	}
	round := rounds[len(rounds)-1]
	if round.ReturnedTarget == nil || round.Decision != nil && round.Decision.Action != plan.Done {
		return false, Result{}, nil
	}
	if run.State != ledger.StageCompleted && round.Decision == nil {
		before, err := e.commitBase(round)
		if err != nil {
			return false, Result{}, err
		}
		if before == "" {
			return false, Result{}, nil
		}
		head, err := e.workspace.Head(ctx)
		if err != nil {
			return false, Result{}, err
		}
		if head == before {
			return false, Result{}, nil
		}
	}
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return false, Result{}, err
	}
	if !ok {
		return false, Result{}, errors.New("bound plan is missing")
	}
	if run.State == ledger.StageCompleted {
		next, err := p.Next(run.StageID, plan.Done)
		if err != nil || next != nil {
			return false, Result{}, err
		}
		if err := e.active(); err != nil {
			return false, Result{}, err
		}
		if _, err := e.append(&ledger.StateChange{WorkStatus: ledger.WorkCompleted}); err != nil {
			return false, Result{}, err
		}
		return true, Result{}, nil
	}
	stage, _ := p.Stage(run.StageID)
	if round.Decision == nil {
		r, err := e.judgeRound(ctx, state, p, stage, run, round)
		return true, r, err
	}
	r, err := e.finishCommit(ctx, state, p, stage, run, round)
	return true, r, err
}

func (e *Engine) finishCommit(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, error) {
	if r, handled, err := e.prepareCommit(ctx, state, p, stage, run, round); handled || err != nil {
		return r, err
	}
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return Result{}, err
	}
	before, err := e.recordCommitBase(ctx, round, head)
	if err != nil {
		return Result{}, err
	}
	var commit workspace.CommitResult
	if head != before {
		tree, err := e.workspace.CommitTree(ctx, head)
		if err != nil {
			return Result{}, err
		}
		if tree != round.ReturnedTarget.Tree {
			return e.refreshTarget(ctx, state, run, round, head)
		}
		commit = workspace.CommitResult{Commit: head, Tree: tree}
	} else {
		if err := e.active(); err != nil {
			return Result{}, err
		}
		settings, settingsErr := e.settings(ctx, roundReference(round))
		if settingsErr != nil {
			return Result{}, settingsErr
		}
		tree, treeErr := e.workspace.CommitTree(ctx, head)
		if treeErr != nil {
			return Result{}, treeErr
		}
		if !settings.AllowEmptyCommit() && tree == round.ReturnedTarget.Tree {
			matches, matchErr := e.matches(ctx, tree)
			if matchErr != nil {
				return Result{}, matchErr
			}
			if !matches {
				return e.refreshTarget(ctx, state, run, round, "")
			}
			commit = workspace.CommitResult{Commit: head, Tree: tree}
		} else {
			commit, err = e.commitStage(ctx, p, stage, round, settings)
		}
	}
	if errors.Is(err, workspace.ErrTargetChanged) {
		return e.refreshTarget(ctx, state, run, round, commit.Commit)
	}
	if err != nil {
		reason := fmt.Sprintf("Commit failed: %v\n%s", err, commit.Output)
		return e.fixRound(ctx, state, p, stage, run, round, reason)
	}
	if commit.Tree != round.ReturnedTarget.Tree {
		return e.refreshTarget(ctx, state, run, round, commit.Commit)
	}
	return e.recordStageCommit(p, stage, run, round, commit)
}

// prepareCommit checks the session before commit.
func (e *Engine) prepareCommit(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	if err := e.active(); err != nil {
		return Result{}, false, err
	}
	if err := e.checkCommitSession(ctx, state); err != nil {
		return Result{}, false, err
	}
	return Result{}, false, nil
}

// RecoverExternal queries an interrupted external operation without repeating its write.
func (e *Engine) RecoverExternal(ctx context.Context, operationID string) (Result, error) {
	_, warning, err := e.resolve(ctx, false)
	if err != nil {
		return Result{}, err
	}
	if warning != "" {
		return Result{}, errors.New(warning)
	}
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	ref, err := e.reference(ctx, state)
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	outcome, err := external.New(e.Root(), e.store(), e.validator, settings.External).Recover(ctx, operationID)
	if err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	r := response(state)
	r.External = &outcome
	return r, nil
}

func removeExpansion(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := filepath.WalkDir(path, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(name, 0755)
		}
		return nil
	}); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (e *Engine) currentQuestions(state *ledger.State) []form.Question {
	questions := state.UnansweredQuestions()
	versions := state.PlanEvaluations(nil)
	latest := 0
	for _, v := range versions {
		if !(v.Withdrawn && v.Origin == plan.HumanProposal) {
			latest = max(latest, v.Plan.Version)
		}
	}
	return slices.DeleteFunc(questions, func(q form.Question) bool {
		return q.ApprovalTarget.Kind == form.ApprovalPlan && q.ApprovalTarget.Plan != latest
	})
}

func (e *Engine) replaceQuestionTarget(ctx context.Context, round *ledger.Round, tree string) error {
	snapshot, err := e.workspace.Snapshot(ctx, e.workID)
	if err != nil {
		return err
	}
	if snapshot.Tree != tree {
		return errors.New("the question target changed while it was being captured")
	}
	target, err := e.treeTarget(tree)
	if err != nil {
		return err
	}
	if _, err = e.append(&ledger.Artifact{Before: round.StartTarget, After: target, ChangeKind: "question_target_change", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}}); err != nil {
		return err
	}
	copy := *round
	copy.ReturnedTarget = &target
	_, err = e.append(&copy)
	return err
}

func (e *Engine) advanceNext(ctx context.Context, state *ledger.State) (Result, bool, error) {
	if err := e.importStops(state); err != nil {
		return Result{}, false, err
	}
	state, err := e.load()
	if err != nil {
		return Result{}, false, err
	}
	if recovered, err := e.recoverSubmission(ctx, state); err != nil {
		return Result{}, false, err
	} else if recovered {
		return Result{}, false, nil
	}

	if questions := e.currentQuestions(state); len(questions) > 0 {
		return e.advanceQuestion(ctx, state, questions[0])
	}
	changed, err := e.acceptPending(ctx, state)
	if err != nil {
		return Result{}, false, err
	}
	if changed {
		return Result{}, false, nil
	}
	if run, ok := state.LatestStageRun(); ok && run.State != ledger.StageInvalidated {
		if recovered, r, err := e.recoverCommit(ctx, state, run); err != nil || recovered {
			if err != nil || r.Instruction != nil || r.Paused || r.Message != "" {
				return r, true, err
			}
			return Result{}, false, nil
		}
	}
	if roundLimitWaiting(state) {
		r := response(state)
		r.Message = "The stage round limit was reached and continuation was not approved. Submit a new plan with devkit plan, run devkit abort, or correct the answer to continue."
		return r, true, nil
	}
	return e.advancePlan(ctx, state)
}

func (e *Engine) advanceQuestion(ctx context.Context, state *ledger.State, q form.Question) (Result, bool, error) {
	if q.ApprovalTarget.Kind == form.ApprovalTree {
		tree, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, false, err
		}
		current, err := e.currentApprovalTarget(ctx, q.ApprovalTarget, tree)
		if err != nil {
			return Result{}, false, err
		}
		if current != q.ApprovalTarget {
			replacement := q
			replacement.ID = ulidString()
			replacement.ApprovalTarget = current
			if err := e.refreshQuestionEvidence(ctx, state, &replacement); err != nil {
				return Result{}, false, err
			}
			if round, ok := state.OpenRound(); ok && round.ReturnedTarget != nil && q.StopCategory != "detection_failed" {
				if err = e.replaceQuestionTarget(ctx, round, tree); err != nil {
					return Result{}, false, err
				}
			}
			if _, err = e.append(&ledger.Question{Question: replacement, Supersedes: q.ID}); err != nil {
				return Result{}, false, err
			}
			return Result{}, false, nil
		}
	}
	r, err := e.issueQuestion(state, q)
	return r, true, err
}

func (e *Engine) pendingPlanner(ctx context.Context, state *ledger.State, proposals []plan.VersionEvaluation) (Result, bool, error) {
	highest := 0
	for _, proposal := range proposals {
		highest = max(highest, proposal.Plan.Version)
	}
	for _, issued := range state.UnacceptedInstructions() {
		if issued.Kind == form.InstructionPlan && issued.PlanVersion > highest {
			data, err := e.readInstruction(issued.ID)
			if err != nil {
				return Result{}, true, err
			}
			r, err := e.planner(ctx, data.PatternName, issued.PlanVersion, data.Origin, "")
			return r, true, err
		}
	}
	if len(proposals) == 0 {
		for _, i := range state.UnacceptedInstructions() {
			if i.Kind == form.InstructionPlan {
				data, err := e.readInstruction(i.ID)
				if err != nil {
					return Result{}, true, err
				}
				if i.SessionID != e.sessionID {
					r, err := e.planner(ctx, data.PatternName, i.PlanVersion, data.Origin, "")
					return r, true, err
				}
				r, err := e.publishInstruction(i, false, data, "")
				return r, true, err
			}
		}
		return Result{}, true, errors.New("initial planning instruction is missing; retry devkit start with the original task and pattern")
	}
	return Result{}, false, nil
}

func (e *Engine) reviseFailedPlan(ctx context.Context, state *ledger.State, latest plan.VersionEvaluation, head string) (Result, error) {
	rounds, err := e.planRounds()
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, head)
	if err != nil {
		return Result{}, err
	}
	if !latest.Rejected && !latest.Withdrawn && rounds >= settings.PlanningMaxRounds() {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: latest.Plan.Version}, "plan_round_limit", "The plan check round limit was reached. Explain how to continue.", "")
		if _, answered := state.ResolveAnswer(q.ApprovalTarget); !answered {
			q, err = e.saveQuestion(state, q, "")
			if err != nil {
				return Result{}, err
			}
			return e.issueQuestion(state, q)
		}
	}
	n, err := e.highestPlan()
	if err != nil {
		return Result{}, err
	}
	return e.planner(ctx, latest.Plan.Pattern.Name, n+1, latest.Origin, "")
}

func (e *Engine) advancePlan(ctx context.Context, state *ledger.State) (Result, bool, error) {
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return Result{}, false, err
	}
	head, err = e.refreshSources(ctx, head)
	if err != nil {
		return Result{}, false, err
	}
	norms, err := e.norms(ctx, head)
	if err != nil {
		return Result{}, false, err
	}
	proposals := state.PlanEvaluations(norms)
	if r, ready, err := e.pendingPlanner(ctx, state, proposals); err != nil || ready {
		return r, ready, err
	}
	latest := proposals[len(proposals)-1]
	for latest.Withdrawn && latest.Origin == plan.HumanProposal && len(proposals) > 1 {
		proposals = proposals[:len(proposals)-1]
		latest = proposals[len(proposals)-1]
	}
	return e.advanceProposal(ctx, state, latest, head, norms)
}

func (e *Engine) advanceProposal(ctx context.Context, state *ledger.State, latest plan.VersionEvaluation, head string, norms []form.NormReference) (Result, bool, error) {
	if latest.Withdrawn || latest.Rejected || latest.Judgment == plan.PlanFail {
		r, err := e.reviseFailedPlan(ctx, state, latest, head)
		return r, true, err
	}
	if latest.Judgment == plan.PlanPending {
		r, err := e.planCheck(ctx, state, latest.Plan)
		if err != nil || r.Instruction != nil || r.Paused {
			return r, true, err
		}
		return Result{}, false, nil
	}
	if latest.Judgment == plan.PlanNotAssessed {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: latest.Plan.Version}, "plan_not_assessed", "The plan could not be assessed. Provide revision instructions.", "")
		q, err := e.saveQuestion(state, q, "")
		if err != nil {
			return Result{}, false, err
		}
		r, err := e.issueQuestion(state, q)
		return r, true, err
	}
	return e.advanceApprovedPlan(ctx, state, latest, head, norms)
}

func (e *Engine) advanceApprovedPlan(ctx context.Context, state *ledger.State, latest plan.VersionEvaluation, head string, norms []form.NormReference) (Result, bool, error) {
	settings, err := e.settings(ctx, head)
	if err != nil {
		return Result{}, false, err
	}
	completed, err := e.approvalCompleted(state)
	if err != nil {
		return Result{}, false, err
	}
	var lastApproved *plan.Plan
	for _, previous := range state.PlanEvaluations(nil) {
		if previous.Plan.Version < latest.Plan.Version && previous.Approved && !previous.Rejected && !previous.Withdrawn {
			remaining := plan.RemainingPlan(previous.Plan, completed)
			lastApproved = &remaining
		}
	}
	if !latest.Executable(settings.PlanApproval(), lastApproved) {
		text, err := e.planApprovalText(ctx, state, latest.Plan, head)
		if err != nil {
			return Result{}, false, err
		}
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: latest.Plan.Version}, "plan_approval", text, latest.Plan.StartStage)
		q, err = e.saveQuestion(state, q, "")
		if err != nil {
			return Result{}, false, err
		}
		r, err := e.issueQuestion(state, q)
		return r, true, err
	}
	p, executable := state.ExecutablePlan(norms, settings.PlanApproval(), completed)
	if !executable {
		r, err := e.replan(ctx, latest.Plan.Pattern.Name, "")
		return r, true, err
	}
	return e.advanceStages(ctx, state, p, settings)
}

// replan issues the planner for the next plan version to rebuild the remaining work.
func (e *Engine) replan(ctx context.Context, pattern, rejection string) (Result, error) {
	n, err := e.highestPlan()
	if err != nil {
		return Result{}, err
	}
	return e.planner(ctx, pattern, n+1, plan.BackProposal, rejection)
}

// advanceStages continues the active stage run or starts the instance where the work continues.
// Before a new run starts, the fixed loop rules are evaluated again on the work path.
func (e *Engine) advanceStages(ctx context.Context, state *ledger.State, p plan.Plan, settings config.Config) (Result, bool, error) {
	run, ok := state.LatestStageRun()
	if ok && run.State != ledger.StageInvalidated && run.State != ledger.StageCompleted {
		return e.advanceActiveStage(ctx, state, run)
	}
	if ok && run.State == ledger.StageCompleted {
		finished, err := e.finishedWork(run)
		if err != nil || finished {
			return Result{}, false, err
		}
	}
	stageID, err := e.pathStart(state, p)
	if err != nil {
		return Result{}, false, err
	}
	violations, err := e.ruleViolations(state, p, settings)
	if err != nil {
		return Result{}, false, err
	}
	if len(violations) > 0 {
		r, err := e.replan(ctx, p.Pattern.Name, violationText(violations))
		return r, true, err
	}
	return Result{}, false, e.startStage(p, stageID)
}

// finishedWork completes the work when the completed run's bound plan ends after it.
func (e *Engine) finishedWork(run *ledger.StageRun) (bool, error) {
	old, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, errors.New("bound plan is missing")
	}
	target, err := old.Next(run.StageID, plan.Done)
	if err != nil || target != nil {
		return false, err
	}
	if err := e.active(); err != nil {
		return false, err
	}
	_, err = e.append(&ledger.StateChange{WorkStatus: ledger.WorkCompleted})
	return true, err
}

func (e *Engine) startStage(p plan.Plan, stageID string) error {
	if _, ok := p.Stage(stageID); !ok {
		return fmt.Errorf("stage %q is missing from plan %d", stageID, p.Version)
	}
	if err := e.active(); err != nil {
		return err
	}
	run := &ledger.StageRun{StageID: stageID, PlanVersion: p.Version, State: ledger.StageWorking, NextActor: "worker"}
	_, err := e.append(run)
	return err
}

func (e *Engine) advanceActiveStage(ctx context.Context, state *ledger.State, run *ledger.StageRun) (Result, bool, error) {
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return Result{}, false, err
	}
	if !ok {
		return Result{}, false, errors.New("bound plan is missing")
	}
	stageID := run.StageID
	rounds := state.Rounds(run.ID)
	if len(rounds) > 0 && rounds[len(rounds)-1].Decision != nil {
		target, err := p.Next(stageID, rounds[len(rounds)-1].Decision.Action)
		if err != nil {
			return Result{}, false, err
		}
		if target != nil && *target != stageID {
			return Result{}, false, e.startStage(p, *target)
		}
	}
	stage, ok := p.Stage(stageID)
	if !ok {
		return Result{}, false, fmt.Errorf("stage %q is missing from plan %d", stageID, p.Version)
	}
	if round, ok := state.OpenRound(); ok {
		return e.advanceOpenRound(ctx, state, p, stage, run, round)
	}
	return e.openStageRound(ctx, state, stage, run)
}

func (e *Engine) advanceOpenRound(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	stage, err := e.roundStage(ctx, stage, round)
	if err != nil {
		return Result{}, false, err
	}
	if merge, ok := state.RoundMerge(round.ID); ok && merge.Status == "conflict" {
		return e.mergeUnits(ctx, state, p, stage, run, round)
	}
	if round.ReturnedTarget != nil {
		r, err := e.judgeRound(ctx, state, p, stage, run, round)
		if err != nil || r.Instruction != nil || r.Paused || r.Message != "" {
			return r, true, err
		}
		return Result{}, false, nil
	}
	if stage.Assignee == plan.Human {
		return e.advanceHumanRound(ctx, state, stage, run, round)
	}
	if stage.Split && len(stage.Units) > 0 {
		return e.advanceUnits(ctx, state, p, stage, run, round)
	}
	if run.State == ledger.StageExternalWait {
		update := *run
		update.State, update.NextActor, update.StopReason = ledger.StageWorking, "worker", ""
		if _, err := e.append(&update); err != nil {
			return Result{}, false, err
		}
	}
	r, err := e.workInstruction(ctx, state, p, stage, round)
	return r, true, err
}

func (e *Engine) advanceHumanRound(ctx context.Context, state *ledger.State, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	target := form.ApprovalTarget{Kind: form.ApprovalTree, Tree: round.StartTarget.Tree}
	if answer, ok := humanStageAnswer(state, run, round); ok {
		progress := form.Progress{InstructionID: "human-" + round.ID, TerminationKind: form.TerminationCompleted, WorkPerformed: []string{answer.Text}}
		if err := e.recordSubmission(ctx, state, run, round, progress); err != nil {
			return Result{}, false, err
		}
		return Result{}, false, nil
	}
	text, err := e.humanStageText(ctx, state, stage, round)
	if err != nil {
		return Result{}, false, err
	}
	q := e.question(target, "human_stage", text, stage.ID)
	q.Evidence = []form.Reference{{RecordID: run.ID}, {RecordID: round.ID}}
	q, err = e.saveQuestion(state, q, "")
	if err != nil {
		return Result{}, false, err
	}
	r, err := e.issueQuestion(state, q)
	return r, true, err
}

// humanStageText asks for a human stage's output with the template's human_prompt document,
// which states the purpose, inputs, and what to return, and the fixed adopted outputs.
func (e *Engine) humanStageText(ctx context.Context, state *ledger.State, stage plan.Stage, round *ledger.Round) (string, error) {
	text := "Provide the output for human stage " + stage.ID + "."
	if stage.HumanPrompt != nil {
		documents, err := e.selectionDocuments(ctx, roundReference(round), round.StartTarget.Tree, []plan.Selection{*stage.HumanPrompt})
		if err != nil {
			return "", err
		}
		for _, document := range documents {
			text += "\n\n" + document.Content + "\n\nSource: " + document.Path
		}
	}
	adopted, err := e.adoptedOutputs(state)
	if err != nil {
		return "", err
	}
	for _, output := range selectOutputs(adopted, stage.Outputs) {
		text += "\n\nAdopted output " + output.StageID + ":\n" + output.Content
	}
	return text, nil
}

func (e *Engine) openStageRound(ctx context.Context, state *ledger.State, stage plan.Stage, run *ledger.StageRun) (Result, bool, error) {
	if err := e.active(); err != nil {
		return Result{}, false, err
	}
	snapshot, err := e.workspace.Snapshot(ctx, e.workID)
	if err != nil {
		return Result{}, false, err
	}
	startRef, question, err := e.roundStartNorms(ctx, state, run, snapshot.Tree)
	if err != nil {
		return Result{}, false, err
	}
	if question != nil {
		r, err := e.issueQuestion(state, *question)
		return r, true, err
	}
	if r, ready, err := e.checkRoundBudget(ctx, state, startRef, snapshot.Tree); err != nil || ready {
		return r, ready, err
	}
	required, err := e.catalogChecks(ctx, startRef, stage.RequiredChecks())
	if err != nil {
		return Result{}, false, err
	}
	if r, pending, err := e.ensureDetection(ctx, state, required, startRef, run.ID); err != nil || pending {
		if err != nil || r.Instruction != nil || r.Paused {
			return r, true, err
		}
		return Result{}, false, nil
	}
	norms, err := e.norms(ctx, startRef)
	if err != nil {
		return Result{}, false, err
	}
	sources, err := e.sourceIdentityAt(ctx, startRef)
	start := form.TargetVersion{Kind: form.TargetTree, Tree: snapshot.Tree, Sources: sources}
	if err != nil {
		return Result{}, false, err
	}
	round := &ledger.Round{Assignee: stage.Assignee, StageRunID: run.ID, SessionID: e.sessionID, StartTarget: start, NormReference: startRef, Norms: norms}

	previous := state.Rounds(run.ID)
	if len(previous) > 0 {
		last := previous[len(previous)-1]
		round.Assignee = roundAssignee(stage, last)
		if last.Decision != nil && last.Decision.Assignee != "" {
			round.Assignee = last.Decision.Assignee
		}
	}
	effective, err := e.roundStage(ctx, stage, round)
	if err != nil {
		return Result{}, false, err
	}
	settings, err := e.settings(ctx, startRef)
	if err != nil {
		return Result{}, false, err
	}
	if err := validateRoles([]plan.Stage{stage}, settings); err != nil {
		return Result{}, false, err
	}
	if effective.Split && len(effective.Units) > 0 {
		if err := e.prepareUnitRound(ctx, state, stage, run, round, snapshot.Tree); err != nil {
			return Result{}, false, err
		}
	}
	if _, err = e.append(round); err != nil {
		return Result{}, false, err
	}
	return Result{}, false, nil
}

// prepareUnitRound fixes the unit base commit and marks merge-conflict repair rounds.
func (e *Engine) prepareUnitRound(ctx context.Context, state *ledger.State, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, tree string) error {
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return err
	}
	base, err := e.workspace.CommitObject(ctx, e.workID, tree, head)
	if err != nil {
		return err
	}
	round.StartCommit = base.Commit
	previous := state.Rounds(run.ID)
	if len(previous) > 0 {
		merge, ok := state.RoundMerge(previous[len(previous)-1].ID)
		round.MergeFix = stage.Split && (previous[len(previous)-1].MergeFix || ok && merge.Status == "conflict")
	}
	return nil
}

func (e *Engine) branchWarning(state *ledger.State, warning string) (Result, error) {
	r := response(state)
	r.Warning = warning
	pending := state.UnacceptedInstructions()
	if len(pending) > 0 {
		data, err := e.readInstructionFacts(pending[len(pending)-1].ID)
		if err != nil {
			return r, err
		}
		r.Instruction = &data.Instruction
	}
	return r, nil
}

func (e *Engine) submittedArtifact(round *ledger.Round, progress form.Progress) (*ledger.Artifact, bool, error) {
	records, err := e.records()
	if err != nil {
		return nil, false, err
	}
	var artifact *ledger.Artifact
	accepted := false
	for _, record := range records {
		if record.ID <= round.ID {
			continue
		}
		if a, ok := record.Payload.(*ledger.Artifact); ok && a.Before == round.StartTarget && a.ChangeKind == "submission" {
			artifact = a
		}
		if saved, ok := record.Payload.(*ledger.Progress); ok && saved.RoundID == round.ID {
			if !reflect.DeepEqual(saved.Progress, progress) {
				return nil, false, errors.New("resubmission differs from the recorded progress")
			}
			accepted = true
		}
	}
	return artifact, accepted, nil
}

func (e *Engine) recordStageCommit(p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, commit workspace.CommitResult) (Result, error) {
	if err := e.active(); err != nil {
		return Result{}, err
	}
	judgments, err := e.roundJudgments(round)
	if err != nil {
		return Result{}, err
	}
	summary := check.Aggregate(judgments)
	records, err := e.records()
	if err != nil {
		return Result{}, err
	}
	recorded := false
	for _, record := range records {
		if artifact, ok := record.Payload.(*ledger.Artifact); ok && record.ID > round.ID && artifact.ChangeKind == "commit" && artifact.Commit == commit.Commit {
			recorded = true
		}
	}
	if !recorded {
		if _, err = e.append(&ledger.Artifact{Before: round.StartTarget, After: *round.ReturnedTarget, CompletionConditionIDs: summary.CompletionConditionIDs, ChangeKind: "commit", Decisions: []form.Reference{}, Commit: commit.Commit}); err != nil {
			return Result{}, err
		}
	}
	if round.Decision == nil {
		copy := *round
		copy.Decision = &router.Decision{Action: plan.Done, Reason: "All completion conditions passed and the stage completion was recorded at HEAD."}
		if _, err = e.append(&copy); err != nil {
			return Result{}, err
		}
	}
	update := *run
	update.State = ledger.StageCompleted
	update.Commit = commit.Commit
	update.NextActor = ""
	update.StopReason = commit.Output
	if _, err = e.append(&update); err != nil {
		return Result{}, err
	}
	next, err := p.Next(stage.ID, plan.Done)
	if err != nil {
		return Result{}, err
	}
	if next == nil {
		if _, err = e.append(&ledger.StateChange{WorkStatus: ledger.WorkCompleted}); err != nil {
			return Result{}, err
		}
	}
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}

func (e *Engine) recordCommitBase(ctx context.Context, round *ledger.Round, head string) (string, error) {
	before, err := e.commitBase(round)
	if err != nil {
		return "", err
	}
	if before == "" {
		before = head
		tree, err := e.workspace.CommitTree(ctx, head)
		if err != nil {
			return "", err
		}
		target := form.TargetVersion{Kind: form.TargetTree, Tree: tree}
		if _, err := e.append(&ledger.Artifact{Before: target, After: target, ChangeKind: "commit_base", Commit: head, CompletionConditionIDs: []string{}, Decisions: []form.Reference{}}); err != nil {
			return "", err
		}
	}
	return before, nil
}

func (e *Engine) checkCommitSession(ctx context.Context, state *ledger.State) error {
	if err := e.checkSession(); err != nil {
		return err
	}
	branch, err := e.workspace.Branch(ctx)
	if err != nil {
		branch = ""
	}
	work, _ := state.Work()
	if branch != work.Branch {
		return errors.New("current branch does not match the branch at work start")
	}
	return nil
}

// confirmationText puts the worker's question verbatim first, followed by the materials of the
// termination block a human needs: the partial report, obstacles, unconfirmed items, and resume
// conditions.
func confirmationText(progress form.Progress) string {
	var text strings.Builder
	if progress.Confirmation != nil {
		text.WriteString(progress.Confirmation.Text)
	} else {
		text.WriteString("The worker stopped to request confirmation.")
	}
	if len(progress.Report) > 0 {
		text.WriteString("\n\nReport supplied with the request:\n" + reportText(progress.Report, nil))
	}
	for _, section := range []struct {
		title string
		items []string
	}{{"Obstacles", progress.Obstacles}, {"Unconfirmed", progress.Unconfirmed}, {"Resume conditions", progress.ResumeConditions}} {
		if len(section.items) > 0 {
			text.WriteString("\n\n" + section.title + ":\n- " + strings.Join(section.items, "\n- "))
		}
	}
	return text.String()
}

// commitStage commits the judged target on the current branch.
func (e *Engine) commitStage(ctx context.Context, p plan.Plan, stage plan.Stage, round *ledger.Round, settings config.Config) (workspace.CommitResult, error) {
	message, err := e.commitMessage(round, settings.CommitMessage(stage.ID, e.workID, p.Task))
	if err != nil {
		return workspace.CommitResult{}, err
	}
	return e.workspace.Commit(ctx, round.ReturnedTarget.Tree, e.workID, stage.ID, message)
}

// commitMessage uses the worker's commit_message, written following the git norm, and falls back
// to the [git] commit_message template.
func (e *Engine) commitMessage(round *ledger.Round, fallback string) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	if progress := roundProgress(records, round.ID); progress != nil && strings.TrimSpace(progress.CommitMessage) != "" {
		return progress.CommitMessage, nil
	}
	return fallback, nil
}

// planInstructionsText lists the planner's stage, check, and unit instructions verbatim.
func planInstructionsText(p plan.Plan) string {
	var text strings.Builder
	for _, stage := range p.Stages {
		for _, instruction := range stage.Instructions {
			fmt.Fprintf(&text, "- %s: %s\n", stage.ID, instruction)
		}
		for _, item := range stage.Checks {
			for _, instruction := range item.Instructions {
				fmt.Fprintf(&text, "- %s check %s: %s\n", stage.ID, item.Aspect, instruction)
			}
		}
		for _, unit := range stage.Units {
			for _, instruction := range unit.Instructions {
				fmt.Fprintf(&text, "- %s unit %s: %s\n", stage.ID, unit.ID, instruction)
			}
		}
	}
	if text.Len() == 0 {
		return "\nPlan instructions:\nNone\n"
	}
	return "\nPlan instructions (passed verbatim to the workers and judges):\n" + text.String()
}

func (e *Engine) checkRoundBudget(ctx context.Context, state *ledger.State, ref, tree string) (Result, bool, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, false, err
	}
	question, message, err := e.roundBudget(state, settings, tree)
	if err != nil {
		return Result{}, false, err
	}
	if question != nil {
		r, err := e.issueQuestion(state, *question)
		return r, true, err
	}
	if message != "" {
		r := response(state)
		r.Message = message
		return r, true, nil
	}
	return Result{}, false, nil
}
