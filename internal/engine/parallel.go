package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/workspace"
)

func (e *Engine) unitPath(id string) string {
	return filepath.Join(e.Root(), ".devkit", "worktrees", id)
}

func (e *Engine) roundStartNorms(ctx context.Context, state *ledger.State, run *ledger.StageRun, tree string) (string, *form.Question, error) {
	rounds := state.Rounds(run.ID)
	if len(rounds) > 0 {
		previous := rounds[len(rounds)-1]
		if merge, ok := state.RoundMerge(previous.ID); ok && merge.Status == "conflict" {
			// Conflicted norms must be repaired before they can be confirmed or used.
			return roundReference(previous), nil, nil
		}
	}
	if _, err := e.settings(ctx, tree); err != nil {
		ref := ""
		if len(rounds) > 0 {
			ref = roundReference(rounds[len(rounds)-1])
		} else {
			work, _ := state.Work()
			ref = work.BaseCommit
		}
		update := *run
		update.StopReason = configurationFix(err)
		if _, err := e.append(&update); err != nil {
			return "", nil, err
		}
		return ref, nil, nil
	}
	return e.confirmRoundNorms(ctx, state, run, tree)
}

func configurationFix(err error) string {
	return "Fix devkit.toml. Its snapshot configuration was not applied; the last valid norms remain in use.\n" + err.Error()
}

func (e *Engine) prepareUnitRetry(ctx context.Context, state *ledger.State, meta ledger.Instruction) error {
	for _, round := range state.Rounds(meta.StageRunID) {
		if round.ID == meta.RoundID {
			if err := e.active(); err != nil {
				return err
			}
			_, err := e.unitWorktree(ctx, state, round, meta.TargetPath)
			return err
		}
	}
	return errors.New("worker retry round is missing")
}

func (e *Engine) acceptUnitResult(state *ledger.State, instruction ledger.Instruction, obs observedResult) (bool, error) {
	var progress form.Progress
	if err := e.forms["progress"].Decode(obs.result.Content, &progress); err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	if progress.InstructionID != instruction.ID || instruction.Unit == nil {
		return e.rejectResult(instruction, obs, errors.New("worker instruction ID or unit does not match"))
	}
	if err := e.validateUnitReport(state, instruction, progress); err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	_, err := e.append(&ledger.UnitResult{Progress: progress, RoundID: instruction.RoundID, UnitID: instruction.Unit.ID, AgentID: obs.result.AgentID, ResultPath: obs.path})
	return true, err
}

func (e *Engine) advanceUnits(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	if len(state.UnitResults(round.ID)) == len(stage.Units) {
		return e.mergeUnits(ctx, state, p, stage, run, round)
	}
	settings, err := e.settings(ctx, roundReference(round))
	if err != nil {
		return Result{}, false, err
	}
	accepted := map[string]bool{}
	for _, result := range state.UnitResults(round.ID) {
		accepted[result.UnitID] = true
	}
	free := newSlots(state, settings)
	r := response(state)
	for _, unit := range stage.Units {
		if accepted[unit.ID] || !free.take(form.InstructionUnitWork, round.ID+":"+unit.ID, unit.WorkRole()) {
			continue
		}
		published, err := e.unitInstruction(ctx, state, p, stage, round, unit)
		if err != nil {
			return Result{}, false, err
		}
		if published.Paused {
			return published, true, nil
		}
		r.Instructions = append(r.Instructions, *published.Instruction)
	}
	r.Message = launchMessage(len(r.Instructions))
	return r, true, nil
}

// launchMessage tells the session how to handle the subagent instructions issued together.
func launchMessage(count int) string {
	if count == 0 {
		return "Wait for the outstanding subagent results, then call devkit next."
	}
	return "Launch each listed subagent instruction once, then wait for their results before calling devkit next."
}

func (e *Engine) unitInstruction(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, round *ledger.Round, unit plan.Unit) (Result, error) {
	meta, fresh, err := e.issuance(form.InstructionUnitWork, round.ID+":"+unit.ID, round, p.Version, "")
	if err != nil {
		return Result{}, err
	}
	if !fresh {
		data, err := e.readInstruction(meta.ID)
		if err != nil {
			return Result{}, err
		}
		return e.publishInstruction(meta, false, data, "")
	}
	meta.Unit = &unit
	meta.TargetPath = e.unitPath(meta.ID)
	for _, previous := range state.UnacceptedInstructions() {
		if previous.Kind == meta.Kind && previous.TargetKey == meta.TargetKey {
			meta.TargetPath, meta.Context = previous.TargetPath, previous.Context
		}
	}
	if err := e.active(); err != nil {
		return Result{}, err
	}
	if _, err := e.unitWorktree(ctx, state, round, meta.TargetPath); err != nil {
		return Result{}, err
	}
	b, err := e.workBrief(ctx, state, p, stage, round, &meta)
	if err != nil {
		return Result{}, err
	}
	runner, err := e.runnerAgent(ctx, roundReference(round), unit.WorkRole())
	if err != nil {
		return Result{}, err
	}
	data := instructionData{Reference: roundReference(round), Instruction: form.Instruction{Runner: runner, TargetPath: meta.TargetPath, Text: "Launch the worker with: devkit-instruction " + meta.ID}}
	return e.publishInstruction(meta, fresh, data, b.Text)
}

func (e *Engine) freezeUnits(ctx context.Context, state *ledger.State, stage plan.Stage, round *ledger.Round) (*ledger.Merge, error) {
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return nil, err
	}
	merge := &ledger.Merge{RoundID: round.ID, Head: head, Current: round.StartCommit, BeforeTree: round.StartTarget.Tree, Status: "ready", Repair: round.MergeFix}
	for _, unit := range stage.Units {
		results := state.UnitResults(round.ID)
		index := slices.IndexFunc(results, func(r *ledger.UnitResult) bool { return r.UnitID == unit.ID })
		if index < 0 {
			return nil, errors.New("unit result is missing")
		}
		result := results[index]
		data, err := e.readInstruction(result.InstructionID)
		if err != nil {
			return nil, err
		}
		w, err := e.workspace.AddWorktree(ctx, data.Instruction.TargetPath, round.StartCommit)
		if err != nil {
			return nil, err
		}
		tree, err := w.SnapshotTree(ctx)
		if err != nil {
			return nil, err
		}
		snapshot, err := e.workspace.CommitObject(ctx, e.workID, tree, round.StartCommit)
		if err != nil {
			return nil, err
		}
		merge.Inputs = append(merge.Inputs, ledger.MergeInput{InstructionID: result.InstructionID, UnitID: unit.ID, Commit: snapshot.Commit})
	}
	_, err = e.append(merge)
	return merge, err
}

func (e *Engine) mergeUnits(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, bool, error) {
	merge, ok := state.RoundMerge(round.ID)
	if !ok {
		var err error
		merge, err = e.freezeUnits(ctx, state, stage, round)
		if err != nil {
			return Result{}, false, err
		}
	}
	progress := repairProgress(state, round, mergedProgress(round.ID, state.UnitResults(round.ID)))
	_, accepted, err := e.submittedArtifact(round, progress)
	if err != nil {
		return Result{}, false, err
	}
	if !accepted {
		_, err := e.append(&ledger.Progress{Progress: progress, RoundID: round.ID})
		return Result{}, false, err
	}
	if merge.Status == "conflict" {
		r, err := e.mergeConflict(ctx, state, p, stage, run, round, merge)
		return r, r.Instruction != nil || r.Message != "", err
	}
	if merge.Status == "merged" {
		return Result{}, false, e.recordSubmission(ctx, state, run, round, progress)
	}
	if err := e.mergeUnit(ctx, round, merge); err != nil {
		return Result{}, false, err
	}
	return Result{}, false, nil
}

func (e *Engine) mergeUnit(ctx context.Context, round *ledger.Round, saved *ledger.Merge) error {
	if err := e.active(); err != nil {
		return err
	}
	merge := *saved
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return err
	}
	if head != merge.Head {
		return errors.New("HEAD changed during unit merging; restore the recorded merge HEAD before continuing")
	}
	tree, err := e.workspace.SnapshotTree(ctx)
	if err != nil {
		return err
	}
	if merge.Status == "ready" {
		if tree != merge.BeforeTree {
			return errors.New("work checkout changed before unit merge; restore the recorded merge tree before continuing")
		}
		preview, err := e.workspace.PreviewMerge(ctx, merge.Current, merge.Inputs[merge.Applied].Commit)
		if err != nil {
			return err
		}
		merge.ResultTree, merge.Conflicts, merge.Status = preview.Tree, preview.Conflicts, "applying"
		_, err = e.append(&merge)
		return err
	}
	if tree == merge.BeforeTree && tree != merge.ResultTree {
		if merge.Repair {
			if _, err := e.workspace.ResolveMerge(ctx); err != nil {
				return err
			}
		}
		if err := e.workspace.Merge(ctx, merge.Inputs[0].Commit+"^", merge.Current, merge.Inputs[merge.Applied].Commit); err != nil {
			return err
		}
	}
	return e.recordMergeStep(ctx, &merge)
}

func (e *Engine) recordMergeStep(ctx context.Context, merge *ledger.Merge) error {
	tree, err := e.workspace.SnapshotTree(ctx)
	if err != nil {
		return err
	}
	conflicts, err := e.workspace.Conflicts(ctx)
	if err != nil {
		return err
	}
	if tree != merge.ResultTree || !slices.Equal(conflicts, merge.Conflicts) {
		return errors.New("git merge state does not match the recorded unit merge; preserve the checkout and inspect the merge record")
	}
	if len(conflicts) > 0 {
		merge.Status = "conflict"
	} else {
		if err := e.completeMergeInput(ctx, merge, tree); err != nil {
			return err
		}
	}
	_, err = e.append(merge)
	return err
}

func (e *Engine) mergeConflict(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, merge *ledger.Merge) (Result, error) {
	// Preserve the conflicted tree as the returned target so the usual round limits apply.
	copy := *round
	target, err := e.treeTarget(merge.ResultTree)
	if err != nil {
		return Result{}, err
	}
	copy.ReturnedTarget = &target
	if _, err := e.append(&copy); err != nil {
		return Result{}, err
	}
	units := []string{}
	for _, input := range merge.Inputs {
		units = append(units, input.UnitID+" ("+input.Commit+")")
	}
	reason := fmt.Sprintf("Resolve only the unit merge conflicts in the work checkout, then submit. Devkit will record the resolution and merge the remaining fixed unit SHAs. %d units were merged before the conflicting input. Conflicting files: %s. Units in merge order: %s", merge.Applied, strings.Join(merge.Conflicts, ", "), strings.Join(units, ", "))
	return e.fixRound(ctx, state, p, stage, run, &copy, reason)
}

func mergedProgress(id string, results []*ledger.UnitResult) form.Progress {
	p := form.Progress{InstructionID: "merged-" + id, TerminationKind: form.TerminationCompleted}
	for _, result := range results {
		if result.TerminationKind == form.TerminationQuestion || p.TerminationKind != form.TerminationQuestion && result.TerminationKind == form.TerminationStopped {
			p.TerminationKind = result.TerminationKind
		}
		p.WorkPerformed = append(p.WorkPerformed, result.WorkPerformed...)
		p.Confirmed = append(p.Confirmed, result.Confirmed...)
		p.Unconfirmed = append(p.Unconfirmed, result.Unconfirmed...)
		p.Obstacles = append(p.Obstacles, result.Obstacles...)
		p.TemporaryState = append(p.TemporaryState, result.TemporaryState...)
		p.ResumeConditions = append(p.ResumeConditions, result.ResumeConditions...)
		p.OutOfScope = append(p.OutOfScope, result.OutOfScope...)
		if result.ChangeReason != "" {
			p.ChangeReason += result.UnitID + ": " + result.ChangeReason + "\n"
		}
	}
	if len(results) == 1 {
		// A one-unit round is whole-stage work: its report and commit message are the stage's.
		p.Report, p.CommitMessage = results[0].Report, results[0].CommitMessage
	}
	p.Confirmation = mergedConfirmation(results)
	return p
}

// mergedConfirmation keeps each unit's confirmation request verbatim under its unit ID.
func mergedConfirmation(results []*ledger.UnitResult) *form.Confirmation {
	var requests []*ledger.UnitResult
	for _, result := range results {
		if result.Confirmation != nil {
			requests = append(requests, result)
		}
	}
	if len(requests) == 0 {
		return nil
	}
	if len(results) == 1 {
		copy := *requests[0].Confirmation
		return &copy
	}
	merged := &form.Confirmation{Options: []string{}, Evidence: []form.Reference{}}
	texts := []string{}
	for _, result := range requests {
		texts = append(texts, "Unit "+result.UnitID+": "+result.Confirmation.Text)
		merged.Options = append(merged.Options, result.Confirmation.Options...)
		merged.Evidence = append(merged.Evidence, result.Confirmation.Evidence...)
	}
	merged.Text = strings.Join(texts, "\n\n")
	slices.Sort(merged.Options)
	merged.Options = slices.Compact(merged.Options)
	return merged
}

func (e *Engine) cleanupUnits(ctx context.Context, state *ledger.State) ([]string, error) {
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	retained := []string{}
	for _, record := range records {
		i, ok := record.Payload.(*ledger.Instruction)
		if !ok || i.Kind != form.InstructionUnitWork {
			continue
		}
		for _, round := range state.Rounds(i.StageRunID) {
			if round.ID != i.RoundID {
				continue
			}
			removed, err := e.workspace.RemoveWorktree(ctx, i.TargetPath, round.StartCommit)
			if err != nil {
				return nil, err
			}
			if !removed && !slices.Contains(retained, i.TargetPath) {
				retained = append(retained, i.TargetPath)
			}
		}
	}
	return retained, nil
}

func (e *Engine) acceptAmbiguous(state *ledger.State) (bool, error) {
	if changed, err := e.rejectAmbiguousAnswer(state); changed || err != nil {
		return changed, err
	}
	files, err := filepath.Glob(filepath.Join(e.workRoot(), "results", "unmatched-*.json"))
	if err != nil {
		return false, err
	}
	for _, name := range files {
		if slices.ContainsFunc(state.Questions(), func(q form.Question) bool {
			return slices.Contains(q.Evidence, form.Reference{External: name})
		}) {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return false, err
		}
		var result hook.SubagentResult
		if err := json.Unmarshal(data, &result); err != nil {
			return false, err
		}
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "ambiguous_result", fmt.Sprintf("A %s result without an instruction ID could not be associated with one of the outstanding instructions. Inspect %s and answer with the exact instruction ID to retry in the answer text.\n%s", result.Runner, name, result.Failure), "")
		q.Evidence = []form.Reference{{External: name}}
		q.Options = nil
		for _, instruction := range state.UnacceptedInstructions() {
			if instruction.Runner == result.Runner {
				q.Options = append(q.Options, instruction.ID)
			}
		}
		_, err = e.saveQuestion(state, q, "")
		return true, err
	}
	return false, nil
}

func (e *Engine) validateUnitReport(state *ledger.State, instruction ledger.Instruction, progress form.Progress) error {
	for _, round := range state.Rounds(instruction.StageRunID) {
		if round.ID == instruction.RoundID {
			return e.validateRoundReport(context.Background(), state, round, instruction.Unit, progress)
		}
	}
	return errors.New("worker round is missing")
}

func (e *Engine) unitWorktree(ctx context.Context, state *ledger.State, round *ledger.Round, path string) (*workspace.Workspace, error) {
	if round.MergeFix {
		if merge := previousMerge(state, round); merge != nil && merge.Status == "conflict" {
			return e.workspace.AddMergeWorktree(ctx, path, round.StartCommit)
		}
	}
	return e.workspace.AddWorktree(ctx, path, round.StartCommit)
}
