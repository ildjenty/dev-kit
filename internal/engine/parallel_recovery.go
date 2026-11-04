package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func validateAmbiguousAnswer(state *ledger.State, question *form.Question, request AnswerRequest) error {
	if question.StopCategory != "ambiguous_result" || request.Interpretation == ledger.AnswerDeferred {
		return nil
	}
	id := strings.TrimSpace(request.Text)
	if slices.Contains(question.Options, id) {
		for _, instruction := range state.UnacceptedInstructions() {
			if instruction.ID == id {
				return nil
			}
		}
	}
	return errors.New("answer text must identify one outstanding instruction ID from the ambiguous result question")
}

func (e *Engine) rejectAmbiguousAnswer(state *ledger.State) (bool, error) {
	records, err := e.records()
	if err != nil {
		return false, err
	}
	rejected := map[string]bool{}
	for _, record := range records {
		if failure, ok := record.Payload.(*ledger.ResultRejection); ok {
			rejected[failure.InstructionID] = true
		}
	}
	answers := state.AnsweredQuestions()
	for _, question := range state.Questions() {
		answer, ok := answers[question.ID]
		if question.StopCategory != "ambiguous_result" || !ok || answer.Answer.Interpretation == ledger.AnswerDeferred {
			continue
		}
		id := strings.TrimSpace(answer.Answer.Text)
		for _, instruction := range state.UnacceptedInstructions() {
			if instruction.ID == id && !rejected[id] {
				_, err := e.append(&ledger.ResultRejection{InstructionID: id, ResultPath: question.Evidence[0].External, Reason: "The human identified this instruction for retry after an ambiguous result. Answer: " + answer.RecordID})
				return true, err
			}
		}
	}
	return false, nil
}

func unitResultRound(state *ledger.State, runID string) *ledger.Round {
	var source *ledger.Round
	for _, round := range state.Rounds(runID) {
		if !round.MergeFix && len(state.UnitResults(round.ID)) > 0 {
			source = round
		}
	}
	return source
}

func unitConfirmationAnswered(state *ledger.State, source *ledger.Round) bool {
	answers := state.AnsweredQuestions()
	for _, question := range state.Questions() {
		if question.StopCategory != "worker_confirmation" || !slices.Contains(question.Evidence, form.Reference{RecordID: source.ID}) {
			continue
		}
		if answer, ok := answers[question.ID]; ok && answer.Answer.Interpretation != ledger.AnswerDeferred {
			return true
		}
	}
	return false
}

func repairProgress(state *ledger.State, round *ledger.Round, progress form.Progress) form.Progress {
	source := unitResultRound(state, round.StageRunID)
	if !round.MergeFix || source == nil {
		return progress
	}
	units := mergedProgress(round.ID, state.UnitResults(source.ID))
	if units.TerminationKind == form.TerminationQuestion {
		if unitConfirmationAnswered(state, source) {
			units.TerminationKind = form.TerminationCompleted
		}
	} else {
		for _, previous := range state.Rounds(round.StageRunID) {
			merge, ok := state.RoundMerge(previous.ID)
			if previous.ID >= source.ID && previous.Decision != nil && ok && merge.Status == "merged" {
				units.TerminationKind = form.TerminationCompleted
			}
		}
	}
	combined := mergedProgress(round.ID, []*ledger.UnitResult{{Progress: units, UnitID: "units"}, {Progress: progress, UnitID: "repair"}})
	combined.InstructionID = progress.InstructionID
	return combined
}

func (e *Engine) completeMergeInput(ctx context.Context, merge *ledger.Merge, tree string) error {
	commit, err := e.workspace.CommitObject(ctx, e.workID, tree, merge.Current, merge.Inputs[merge.Applied].Commit)
	if err != nil {
		return err
	}
	merge.Current, merge.BeforeTree, merge.ResultTree = commit.Commit, tree, tree
	merge.Conflicts = nil
	merge.Applied++
	merge.Status = "ready"
	if merge.Applied == len(merge.Inputs) {
		merge.Status = "merged"
	}
	return nil
}

func (e *Engine) repairedMerge(ctx context.Context, state *ledger.State, round *ledger.Round) (*ledger.Merge, error) {
	if merge, ok := state.RoundMerge(round.ID); ok && (!merge.Repair || merge.Status != "merged") {
		return merge, nil
	}
	if !round.MergeFix {
		return nil, nil
	}
	previous := previousMerge(state, round)
	if previous == nil {
		return nil, errors.New("merge repair is missing its recorded inputs")
	}
	merge := *previous
	merge.RoundID = round.ID
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return nil, err
	}
	if merge.Status == "conflict" {
		if head != merge.Head {
			return nil, errors.New("HEAD changed during conflict resolution; restore the recorded merge HEAD before submitting")
		}
		tree, err := e.workspace.ResolveMerge(ctx)
		if err != nil {
			return nil, err
		}
		if err := e.completeMergeInput(ctx, &merge, tree); err != nil {
			return nil, err
		}
	} else if merge.Status == "merged" {
		tree, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return nil, err
		}
		commit, err := e.workspace.CommitObject(ctx, e.workID, tree, merge.Current)
		if err != nil {
			return nil, err
		}
		merge.Head, merge.Current, merge.BeforeTree, merge.ResultTree = head, commit.Commit, tree, tree
	} else {
		return nil, errors.New("merge repair requires a conflict or a completed merge")
	}
	_, err = e.append(&merge)
	return &merge, err
}

func (e *Engine) continueUnitSubmission(ctx context.Context, state *ledger.State, run *ledger.StageRun, round *ledger.Round, progress form.Progress, accepted bool) (bool, error) {
	merge, err := e.repairedMerge(ctx, state, round)
	if err != nil || merge == nil {
		return false, err
	}
	if merge.Status == "merged" {
		return e.checkMergedSubmission(ctx, state, run, round, merge)
	}
	if !accepted {
		if _, err := e.append(&ledger.Progress{Progress: progress, RoundID: round.ID}); err != nil {
			return true, err
		}
	}
	if merge.Status == "conflict" {
		p, _, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return true, err
		}
		stage, _ := p.Stage(run.StageID)
		_, err = e.mergeConflict(ctx, state, p, stage, run, round, merge)
		return true, err
	}
	return true, e.mergeUnit(ctx, round, merge)
}

func (e *Engine) checkMergedSubmission(ctx context.Context, state *ledger.State, run *ledger.StageRun, round *ledger.Round, merge *ledger.Merge) (bool, error) {
	if merge.Applied != len(merge.Inputs) {
		return true, errors.New("unit submission requires a merge record that applied every input")
	}
	head, err := e.workspace.Head(ctx)
	if err != nil {
		return true, err
	}
	tree, err := e.workspace.SnapshotTree(ctx)
	if err != nil {
		return true, err
	}
	conflicts, err := e.workspace.Conflicts(ctx)
	if err != nil {
		return true, err
	}
	if head == merge.Head && tree == merge.ResultTree && len(conflicts) == 0 {
		return false, nil
	}
	paths, err := e.workspace.ChangedFiles(ctx, merge.ResultTree, tree)
	if err != nil {
		return true, err
	}
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil {
		return true, err
	}
	if !ok {
		return true, errors.New("bound plan is missing")
	}
	stage, _ := p.Stage(run.StageID)
	copy := *round
	copy.MergeFix = true
	target, err := e.treeTarget(merge.ResultTree)
	if err != nil {
		return true, err
	}
	copy.ReturnedTarget = &target
	if _, err := e.append(&copy); err != nil {
		return true, err
	}
	reason := fmt.Sprintf("The work checkout differs from the recorded unit merge. Inspect and fix the differences before submitting. Recorded HEAD: %s. Current HEAD: %s. Differing files: %s. Unresolved files: %s", merge.Head, head, strings.Join(paths, ", "), strings.Join(conflicts, ", "))
	_, err = e.fixRound(ctx, state, p, stage, run, &copy, reason)
	return true, err
}

func previousMerge(state *ledger.State, round *ledger.Round) *ledger.Merge {
	var previous *ledger.Merge
	for _, prior := range state.Rounds(round.StageRunID) {
		if prior.ID < round.ID {
			if merge, ok := state.RoundMerge(prior.ID); ok && !merge.Repair {
				previous = merge
			}
		}
	}
	return previous
}

// roundUnitResults includes the fixed original unit reports throughout merge repair rounds.
func roundUnitResults(state *ledger.State, round *ledger.Round) []*ledger.UnitResult {
	var results []*ledger.UnitResult
	for _, prior := range state.Rounds(round.StageRunID) {
		if prior.ID > round.ID {
			break
		}
		if !round.MergeFix && prior.ID != round.ID {
			continue
		}
		if !prior.MergeFix {
			results = nil
		}
		results = append(results, state.UnitResults(prior.ID)...)
	}
	return results
}
