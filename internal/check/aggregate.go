package check

import (
	"fmt"
	"slices"

	"github.com/ildjenty/dev-kit/internal/form"
)

type Summary struct {
	Judgment               JudgmentValue `json:"judgment"`
	Findings               []Finding     `json:"findings"`
	CompletionConditionIDs []string      `json:"completion_condition_ids"`
}

// Aggregate は FAIL、NOT ASSESSED、PASS の優先順で集約する。
// 対象と規範の有効性は呼び出し前に照合する。空の集合は違反なしとして扱う。
func Aggregate(judgments []Judgment) Summary {
	result := Summary{Judgment: Pass, Findings: []Finding{}, CompletionConditionIDs: []string{}}
	for _, judgment := range judgments {
		if judgment.Judgment == Fail {
			result.Judgment = Fail
		} else if judgment.Judgment != Pass && result.Judgment != Fail {
			result.Judgment = NotAssessed
		}
		result.Findings = append(result.Findings, judgment.Findings...)
		for _, id := range judgment.CompletionConditionIDs {
			if !slices.Contains(result.CompletionConditionIDs, id) {
				result.CompletionConditionIDs = append(result.CompletionConditionIDs, id)
			}
		}
	}
	return result
}

// ExemptionDecision は主張と根拠を照合した判断である。
// CoversFinding は指摘が扱う要求がすべて適用除外になる場合だけ真にする。
// 一部だけ外れる場合は指摘の blocking と判定を維持する。
type ExemptionDecision struct {
	FindingID            string
	ExcludedRequirements []string
	CoversFinding        bool
	Reason               string
}

// ApplyExemptions は要求の照合結果を適用し、元の指摘を参考情報として残す。
// 意味上の適用可否を文章の一致から推測しない。
func ApplyExemptions(judgment Judgment, decisions []ExemptionDecision) (Judgment, error) {
	judgment.Findings = slices.Clone(judgment.Findings)
	removedBlocking := false
	for _, decision := range decisions {
		index := slices.IndexFunc(judgment.Findings, func(f Finding) bool { return f.ID == decision.FindingID })
		if index < 0 {
			return Judgment{}, fmt.Errorf("適用除外の指摘がない: %q", decision.FindingID)
		}
		finding := &judgment.Findings[index]
		if len(decision.ExcludedRequirements) == 0 {
			continue
		}
		if finding.Exemption == nil || finding.Exemption.Claim == "" || len(finding.Exemption.Evidence) == 0 || decision.Reason == "" {
			return Judgment{}, fmt.Errorf("適用除外には主張・根拠・照合理由が必要です: %q", finding.ID)
		}
		if decision.CoversFinding {
			removedBlocking = removedBlocking || finding.Blocking
			finding.Blocking = false
		}
	}
	if removedBlocking && judgment.Judgment == Fail && !slices.ContainsFunc(judgment.Findings, func(f Finding) bool { return f.Blocking }) {
		judgment.Judgment = Pass
		if len(judgment.UncheckedScope) != 0 {
			judgment.Judgment = NotAssessed
		}
	}
	return judgment, nil
}

// Requirement は必須 check と、呼び出し側が解決した判定対象である。
type Requirement struct {
	Aspect                 string
	Runner                 string
	TargetVersion          form.TargetVersion
	Norms                  []form.NormReference
	CompletionConditionIDs []string
}

type Coverage struct {
	Requirement         Requirement
	Judgment            JudgmentValue
	Missing             bool
	MissingConditionIDs []string
}

// CheckRequired は必須 check ごとに有効な結果と完了条件 ID の網羅を照合する。
func CheckRequired(required []Requirement, judgments []Judgment) []Coverage {
	coverage := make([]Coverage, 0, len(required))
	for _, requirement := range required {
		var matches []Judgment
		for _, judgment := range judgments {
			if judgment.Aspect == requirement.Aspect && judgment.Runner == requirement.Runner && judgment.ValidFor(requirement.TargetVersion, requirement.Norms) {
				matches = append(matches, judgment)
			}
		}
		summary := Aggregate(matches)
		item := Coverage{Requirement: requirement, Judgment: summary.Judgment, Missing: len(matches) == 0, MissingConditionIDs: []string{}}
		for _, id := range requirement.CompletionConditionIDs {
			if !slices.Contains(summary.CompletionConditionIDs, id) {
				item.MissingConditionIDs = append(item.MissingConditionIDs, id)
			}
		}
		if (item.Missing || len(item.MissingConditionIDs) != 0) && item.Judgment == Pass {
			item.Judgment = NotAssessed
		}
		coverage = append(coverage, item)
	}
	return coverage
}
