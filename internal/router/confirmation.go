package router

import (
	"slices"

	"github.com/ildjenty/dev-kit/internal/plan"
)

// ConfirmationInput の分類名は、実際の差分を調べた呼び出し側が渡す。
// 宣言済みステージのメタループ編集は UndeclaredMetaLoopEdit に含めない。
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

// ClassifyConfirmation は即時確認と、次のゲートまたは PR で確認する暫定判断を区別する。
// 明示したゲートと人のステージは、未回答なら分類表にかかわらず確認する。
func (t Table) ClassifyConfirmation(input ConfirmationInput) Confirmation {
	result := Confirmation{Categories: []string{}}
	matched := false
	for _, category := range input.Categories {
		mode, ok := t.ConfirmationRules[category]
		if !ok && slices.Contains(t.HumanConfirmation, category) {
			mode, ok = ConfirmNow, true
		}
		if !ok {
			continue
		}
		matched = true
		result.Categories = append(result.Categories, category)
		switch mode {
		case ConfirmNow:
			result.Required = true
		case Provisional:
			result.Provisional = true
		}
	}
	if input.NeedsDecision && !matched || input.Gate.Required && !input.GateApproved || input.HumanStage && !input.HumanStageAnswered {
		result.Required = true
	}
	return result
}
