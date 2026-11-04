package router

import (
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// CompletionInput holds plan rules and accepted records for the current target.
// Targets resolves explicit check targets; checks without one use TargetVersion.
// Runners maps each check aspect to the runner fixed by the check catalog.
type CompletionInput struct {
	Rule            plan.CompletionRule
	Runners         map[string]plan.Runner
	TargetVersion   form.TargetVersion
	Targets         map[string]form.TargetVersion
	Norms           []form.NormReference
	Judgments       []check.Judgment
	CommitSucceeded bool
	TargetMatches   bool
}

type Completion struct {
	ChecksPassed    bool
	CommitSucceeded bool
	TargetMatches   bool
	ReadyToCommit   bool
	Complete        bool
	Coverage        []check.Coverage
}

// CheckCompletion separately checks pre-judgment conditions and final completion after commit.
// ReadyToCommit permits engine to request completion processing; Complete permits finalization.
func CheckCompletion(input CompletionInput) Completion {
	required := make([]check.Requirement, 0, len(input.Rule.Checks))
	resolvedTargets := true
	for _, item := range input.Rule.Checks {
		runner := string(input.Runners[item.Aspect])
		target := input.TargetVersion
		if item.Target != "" {
			var ok bool
			target, ok = input.Targets[item.Target]
			resolvedTargets = resolvedTargets && ok
		}
		required = append(required, check.Requirement{
			Aspect: item.Aspect, Runner: runner, TargetVersion: target, Norms: input.Norms,
			CompletionConditionIDs: item.CompletionConditionIDs,
		})
	}
	result := Completion{
		ChecksPassed: resolvedTargets, Coverage: check.CheckRequired(required, input.Judgments),
		CommitSucceeded: input.CommitSucceeded, TargetMatches: input.TargetMatches,
	}
	for _, coverage := range result.Coverage {
		result.ChecksPassed = result.ChecksPassed && coverage.Judgment == check.Pass
	}
	result.ReadyToCommit = result.ChecksPassed
	result.Complete = result.ReadyToCommit && (!input.Rule.RequireCommit || input.CommitSucceeded) && (!input.Rule.RequireTargetMatch || input.TargetMatches)
	return result
}
