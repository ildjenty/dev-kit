package router

import (
	"fmt"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Input.CompletedRounds counts completed rounds including the current round.
// PreviousNoProgressRounds counts preceding consecutive rounds; engine computes progress for this round.
type Input struct {
	Summary                  check.Summary
	Stage                    plan.Stage
	CompletedRounds          int
	PreviousNoProgressRounds int
	ProgressMade             bool
	TerminationKind          form.TerminationKind
	Confirmation             ConfirmationInput
	// FindingCategories maps IDs of findings requiring human decisions to category names.
	FindingCategories map[string]string
	Completion        Completion
}

type Decision struct {
	Action       plan.Transition
	Reason       string
	Confirmation Confirmation
}

// Decide only classifies. Engine handles transitions and records questions and provisional decisions.
// Done requests completion processing; final completion requires CheckCompletion.Complete.
func (t Table) Decide(input Input) (Decision, error) {
	confirmationInput := input.Confirmation
	confirmationInput.HumanStage = confirmationInput.HumanStage || input.Stage.Assignee == plan.Human
	// Check gates after corrections when proceeding to completion processing.
	if input.Summary.Judgment == check.Pass && input.Completion.ReadyToCommit {
		confirmationInput.Gate = input.Stage.Gate
	}
	confirmation := t.ClassifyConfirmation(confirmationInput)
	for _, finding := range input.Summary.Findings {
		if !finding.Blocking || finding.Kind != check.NeedsDecision {
			continue
		}
		item := t.ClassifyConfirmation(ConfirmationInput{
			Categories: []string{input.FindingCategories[finding.ID]}, NeedsDecision: true,
		})
		confirmation.Required = confirmation.Required || item.Required
		confirmation.Provisional = confirmation.Provisional || item.Provisional
		confirmation.Categories = append(confirmation.Categories, item.Categories...)
	}
	decision := Decision{Confirmation: confirmation}
	if input.TerminationKind == form.TerminationQuestion || input.TerminationKind == form.TerminationStopped || confirmation.Required {
		decision.Action, decision.Reason = plan.Ask, "human confirmation is required"
		decision.Confirmation.Provisional = false
		return decision, nil
	}
	action, ok := t.CheckResults[input.Summary.Judgment]
	if !ok {
		return Decision{}, fmt.Errorf("judgment classification not found: %q", input.Summary.Judgment)
	}
	if action != plan.Fix && action != plan.Ask && action != plan.Back && action != plan.Done {
		return Decision{}, fmt.Errorf("unknown classification: %q", action)
	}
	if action == plan.Done && !input.Completion.ReadyToCommit {
		action = plan.Ask
		decision.Reason = "completion conditions for required checks or detection tests are not met"
	}
	noProgress := 0
	if !input.ProgressMade {
		noProgress = input.PreviousNoProgressRounds + 1
	}
	if action == plan.Fix && input.Stage.Limits.Reached(input.CompletedRounds, noProgress) {
		action = plan.Ask
		decision.Reason = "round or consecutive no-progress limit reached"
	}
	if confirmation.Provisional && action != plan.Fix && action != plan.Done {
		// Provisional decisions alone do not allow continuation after an unassessed result or a limit stop.
		decision.Confirmation.Provisional = false
	}
	decision.Action = action
	return decision, nil
}
