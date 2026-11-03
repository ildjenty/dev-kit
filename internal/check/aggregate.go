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

// Aggregate combines judgments in FAIL, NOT ASSESSED, PASS priority order.
// Validate targets and norms before calling. An empty set has no violations.
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

// ExemptionDecision records a decision based on the claim and evidence.
// Set CoversFinding only when all requirements covered by the finding are exempt.
// A partial exemption preserves the blocking flag and judgment.
type ExemptionDecision struct {
	FindingID            string
	ExcludedRequirements []string
	CoversFinding        bool
	Reason               string
}

// ApplyExemptions applies requirement assessments and retains findings as reference information.
// It does not infer semantic applicability from matching text.
func ApplyExemptions(judgment Judgment, decisions []ExemptionDecision) (Judgment, error) {
	judgment.Findings = slices.Clone(judgment.Findings)
	removedBlocking := false
	for _, decision := range decisions {
		index := slices.IndexFunc(judgment.Findings, func(f Finding) bool { return f.ID == decision.FindingID })
		if index < 0 {
			return Judgment{}, fmt.Errorf("exemption finding not found: %q", decision.FindingID)
		}
		finding := &judgment.Findings[index]
		if len(decision.ExcludedRequirements) == 0 {
			continue
		}
		if finding.Exemption == nil || finding.Exemption.Claim == "" || len(finding.Exemption.Evidence) == 0 || decision.Reason == "" {
			return Judgment{}, fmt.Errorf("exemption requires a claim, evidence, and assessment reason: %q", finding.ID)
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

// Requirement describes a required check and its caller-resolved target.
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

// CheckRequired checks valid results and completion condition ID coverage for each required check.
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
