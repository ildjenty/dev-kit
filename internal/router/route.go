package router

import (
	"fmt"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Input の CompletedRounds は今回を含む完了済みのラウンド数である。
// PreviousNoProgressRounds は前回までの連続回数であり、今回の進展は engine が計算する。
type Input struct {
	Summary                  check.Summary
	Stage                    plan.Stage
	CompletedRounds          int
	PreviousNoProgressRounds int
	ProgressMade             bool
	TerminationKind          form.TerminationKind
	PremiseBroken            bool
	Confirmation             ConfirmationInput
	// FindingCategories は人の判断を要する指摘 ID と分類名の対応である。
	FindingCategories map[string]string
	Completion        Completion
}

type Decision struct {
	Action       plan.Transition
	Reason       string
	Confirmation Confirmation
}

// Decide は分類だけを行う。遷移・問い・暫定判断の記録は engine に委ねる。
// Done は完了処理の依頼であり、完了確定には CheckCompletion の Complete が必要である。
func (t Table) Decide(input Input) (Decision, error) {
	confirmationInput := input.Confirmation
	confirmationInput.HumanStage = confirmationInput.HumanStage || input.Stage.Assignee == plan.Human
	// ゲートは修正を終えて完了処理へ進む時点で確認する。
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
		decision.Action, decision.Reason = plan.Ask, "人の確認が必要です"
		decision.Confirmation.Provisional = false
		return decision, nil
	}
	if input.PremiseBroken {
		decision.Action, decision.Reason = plan.Back, "前提を決めたステージの見直しが必要です"
		decision.Confirmation.Provisional = false
		return decision, nil
	}
	action, ok := t.CheckResults[input.Summary.Judgment]
	if !ok {
		return Decision{}, fmt.Errorf("判定結果の分類がない: %q", input.Summary.Judgment)
	}
	if action != plan.Fix && action != plan.Ask && action != plan.Back && action != plan.Done {
		return Decision{}, fmt.Errorf("不明な分類: %q", action)
	}
	if action == plan.Done && !input.Completion.ReadyToCommit {
		action = plan.Ask
		decision.Reason = "必須 check または検出試験の完了条件が満たされていません"
	}
	noProgress := 0
	if !input.ProgressMade {
		noProgress = input.PreviousNoProgressRounds + 1
	}
	if action == plan.Fix && input.Stage.Limits.Reached(input.CompletedRounds, noProgress) {
		action = plan.Ask
		decision.Reason = "ラウンド数または進展のない回数が上限に達しました"
	}
	if confirmation.Provisional && action != plan.Fix && action != plan.Done {
		// 未評価や上限による停止は、暫定判断だけで続行しない。
		decision.Confirmation.Provisional = false
	}
	decision.Action = action
	return decision, nil
}
