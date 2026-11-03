package router

import (
	"slices"

	"github.com/ildjenty/dev-kit/internal/plan"
)

// ConfirmationInput receives category names from the caller after inspecting actual differences.
// Include an empty category for each unclassified decision item.
// Exclude meta-loop edits in declared stages from UndeclaredMetaLoopEdit.
type ConfirmationInput struct {
	Categories         []string
	NeedsDecision      bool
	Gate               plan.Gate
	GateApproved       bool
	HumanStage         bool
	HumanStageAnswered bool
}

type Confirmation struct {
	Required    bool
	Provisional bool
	Categories  []string
}

// ClassifyConfirmation distinguishes immediate confirmation from provisional decisions reviewed at the next gate or PR.
// Unanswered explicit gates and human stages require confirmation regardless of the table.
func (t Table) ClassifyConfirmation(input ConfirmationInput) Confirmation {
	result := Confirmation{Categories: []string{}}
	unclassified := len(input.Categories) == 0
	for _, category := range input.Categories {
		mode, ok := t.ConfirmationRules[category]
		if !ok && slices.Contains(t.HumanConfirmation, category) {
			mode, ok = ConfirmNow, true
		}
		if !ok {
			unclassified = true
			continue
		}
		result.Categories = append(result.Categories, category)
		switch mode {
		case ConfirmNow:
			result.Required = true
		case Provisional:
			result.Provisional = true
		}
	}
	if input.NeedsDecision && unclassified || input.Gate.Required && !input.GateApproved || input.HumanStage && !input.HumanStageAnswered {
		result.Required = true
	}
	return result
}
