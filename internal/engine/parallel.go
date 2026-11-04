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
	return e.confirmRoundNorms(ctx, state, run, tree)
}

func (e *Engine) prepareUnitRetry(ctx context.Context, state *ledger.State, meta ledger.Instruction) error {
	for _, round := range state.Rounds(meta.StageRunID) {
		if round.ID == meta.RoundID {
			if err := e.active(); err != nil {
				return err
			}
			_, err := e.workspace.AddWorktree(ctx, e.unitPath(meta.ID), round.StartCommit)
			return err
		}
	}
	return errors.New("worker retry round is missing")
}

func (e *Engine) acceptUnitResult(instruction ledger.Instruction, obs observedResult) (bool, error) {
	var progress form.Progress
	if err := e.forms["progress"].Decode(obs.result.Content, &progress); err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	if progress.InstructionID != instruction.ID || instruction.Unit == nil {
		return e.rejectResult(instruction, obs, errors.New("worker instruction ID or unit does not match"))
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
	pending := map[string]ledger.Instruction{}
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.Kind == form.InstructionUnitWork && instruction.RoundID == round.ID && instruction.Unit != nil {
			pending[instruction.Unit.ID] = instruction
		}
	}
	r := response(state)
	for _, unit := range stage.Units {
		if accepted[unit.ID] {
			continue
		}
		if previous, ok := pending[unit.ID]; ok {
			data, err := e.readInstruction(previous.ID)
			if err != nil {
				return Result{}, false, err
			}
			published, err := e.publishInstruction(previous, false, data, "")
			if err != nil {
				return Result{}, false, err
			}
			if published.Paused {
				return published, true, nil
			}
			r.Instructions = append(r.Instructions, *published.Instruction)
			continue
		}
		if len(pending) >= settings.MaxWorkers() {
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
		pending[unit.ID] = ledger.Instruction{}
	}
	r.Message = "Launch each issued devkit-worker once, then wait for its result before calling devkit next."
	return r, true, nil
}

func (e *Engine) unitInstruction(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, round *ledger.Round, unit plan.Unit) (Result, error) {
	meta, fresh, err := e.issuance(form.InstructionUnitWork, round.ID+":"+unit.ID, round, p.Version, "")
	if err != nil {
		return Result{}, err
	}
	meta.Unit = &unit
	if err := e.active(); err != nil {
		return Result{}, err
	}
	if _, err := e.workspace.AddWorktree(ctx, e.unitPath(meta.ID), round.StartCommit); err != nil {
		return Result{}, err
	}
	b, err := e.workBrief(ctx, state, p, stage, round, &meta)
	if err != nil {
		return Result{}, err
	}
	data := instructionData{Reference: roundReference(round), Instruction: form.Instruction{Runner: "devkit-worker", TargetPath: e.unitPath(meta.ID), Text: "Launch the worker with: devkit-instruction " + meta.ID}}
	return e.publishInstruction(meta, fresh, data, b.Text)
}

func (e *Engine) freezeUnits(ctx context.Context, state *ledger.State, stage plan.Stage, round *ledger.Round) (*ledger.Merge, error) {
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return nil, err
	}
	merge := &ledger.Merge{RoundID: round.ID, Head: head, Current: round.StartCommit, BeforeTree: round.StartTarget.Tree, Status: "ready"}
	for _, unit := range stage.Units {
		results := state.UnitResults(round.ID)
		index := slices.IndexFunc(results, func(r *ledger.UnitResult) bool { return r.UnitID == unit.ID })
		if index < 0 {
			return nil, errors.New("unit result is missing")
		}
		result := results[index]
		w, err := e.workspace.AddWorktree(ctx, e.unitPath(result.InstructionID), round.StartCommit)
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
	if merge.Status == "conflict" {
		r, err := e.mergeConflict(ctx, state, p, stage, run, round, merge)
		return r, r.Instruction != nil || r.Message != "", err
	}
	if merge.Status == "merged" {
		progress := mergedProgress(round.ID, state.UnitResults(round.ID))
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
		if err := e.workspace.Merge(ctx, round.StartCommit, merge.Current, merge.Inputs[merge.Applied].Commit); err != nil {
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
		commit, err := e.workspace.CommitObject(ctx, e.workID, tree, merge.Current, merge.Inputs[merge.Applied].Commit)
		if err != nil {
			return err
		}
		merge.Current, merge.BeforeTree = commit.Commit, tree
		merge.Applied++
		merge.Status = "ready"
		if merge.Applied == len(merge.Inputs) {
			merge.Status = "merged"
		}
	}
	_, err = e.append(merge)
	return err
}

func (e *Engine) mergeConflict(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, merge *ledger.Merge) (Result, error) {
	// Preserve the conflicted tree as the returned target so the usual round limits apply.
	copy := *round
	target := form.TargetVersion{Kind: form.TargetTree, Tree: merge.ResultTree}
	copy.ReturnedTarget = &target
	if _, err := e.append(&copy); err != nil {
		return Result{}, err
	}
	units := []string{}
	for _, input := range merge.Inputs {
		units = append(units, input.UnitID+" ("+input.Commit+")")
	}
	reason := fmt.Sprintf("Resolve unit merge conflicts in the work checkout and integrate any remaining fixed unit SHAs. %d units were merged before the conflicting input. Conflicting files: %s. Units in merge order: %s", merge.Applied, strings.Join(merge.Conflicts, ", "), strings.Join(units, ", "))
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
	return p
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
			removed, err := e.workspace.RemoveWorktree(ctx, e.unitPath(i.ID), round.StartCommit)
			if err != nil {
				return nil, err
			}
			if !removed {
				retained = append(retained, e.unitPath(i.ID))
			}
		}
	}
	return retained, nil
}

func (e *Engine) acceptAmbiguous(state *ledger.State) (bool, error) {
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
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: e.workID}, "ambiguous_result", fmt.Sprintf("A %s result without an instruction ID could not be associated with one of the outstanding instructions. Inspect %s and provide instructions.\n%s", result.Runner, name, result.Failure), "")
		q.Evidence = []form.Reference{{External: name}}
		_, err = e.saveQuestion(state, q, "")
		return true, err
	}
	return false, nil
}
