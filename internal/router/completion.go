package router

import (
	"strings"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// CompletionInput は plan の規則と、現在の対象に対する受理済みの記録である。
// Targets は check の target 指定を解決した値で、指定のない check は TargetVersion を使う。
type CompletionInput struct {
	Rule            plan.CompletionRule
	TargetVersion   form.TargetVersion
	Targets         map[string]form.TargetVersion
	Norms           []form.NormReference
	Judgments       []check.Judgment
	Detection       []check.DetectionRequirement
	DetectionTests  []check.DetectionTest
	CommitSucceeded bool
	TargetMatches   bool
}

type Completion struct {
	ChecksPassed     bool
	DetectionPassed  bool
	CommitSucceeded  bool
	TargetMatches    bool
	ReadyToCommit    bool
	Complete         bool
	Coverage         []check.Coverage
	MissingDetection []check.DetectionRequirement
}

// CheckCompletion は判定前の条件と commit 後の完了確定を分けて照合する。
// ReadyToCommit が真なら engine は完了処理を依頼でき、Complete が真なら確定できる。
func CheckCompletion(input CompletionInput) Completion {
	required := make([]check.Requirement, 0, len(input.Rule.Checks))
	var detection []check.DetectionRequirement
	resolvedTargets := true
	for _, item := range input.Rule.Checks {
		target := input.TargetVersion
		if item.Target != "" {
			var ok bool
			target, ok = input.Targets[item.Target]
			resolvedTargets = resolvedTargets && ok
		}
		required = append(required, check.Requirement{
			Aspect: item.Aspect, Runner: string(item.Runner), TargetVersion: target, Norms: input.Norms,
			CompletionConditionIDs: item.CompletionConditionIDs,
		})
		if input.Rule.RequireDetectionTests && !strings.HasPrefix(string(item.Runner), "command:") {
			requirement := check.DetectionRequirement{Aspect: item.Aspect, Runner: string(item.Runner)}
			for _, candidate := range input.Detection {
				if candidate.Aspect == item.Aspect && candidate.Runner == string(item.Runner) {
					requirement = candidate
					break
				}
			}
			detection = append(detection, requirement)
		}
	}
	result := Completion{
		ChecksPassed: resolvedTargets, Coverage: check.CheckRequired(required, input.Judgments),
		MissingDetection: check.MissingDetection(detection, input.DetectionTests),
		CommitSucceeded:  input.CommitSucceeded, TargetMatches: input.TargetMatches,
	}
	for _, coverage := range result.Coverage {
		result.ChecksPassed = result.ChecksPassed && coverage.Judgment == check.Pass
	}
	result.DetectionPassed = len(result.MissingDetection) == 0
	result.ReadyToCommit = result.ChecksPassed && result.DetectionPassed
	result.Complete = result.ReadyToCommit && (!input.Rule.RequireCommit || input.CommitSucceeded) && (!input.Rule.RequireTargetMatch || input.TargetMatches)
	return result
}
