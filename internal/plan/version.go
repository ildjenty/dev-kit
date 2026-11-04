package plan

import (
	"reflect"
	"slices"
)

type Diff struct {
	AddedStages   []string
	RemovedStages []string
	ChangedStages []string
	ChangedFields []string
}

// Replan assigns a higher version to the remaining plan. Pass the highest version across all proposals as current.
func Replan(current Plan, remaining Plan) (Plan, Diff) {
	next := clonePlan(remaining)
	next.Version = current.Version + 1
	return next, Difference(current, next)
}

func Difference(before, after Plan) Diff {
	diff := Diff{AddedStages: []string{}, RemovedStages: []string{}, ChangedStages: []string{}, ChangedFields: []string{}}
	for _, stage := range before.Stages {
		next, exists := after.Stage(stage.ID)
		if !exists {
			diff.RemovedStages = append(diff.RemovedStages, stage.ID)
		} else if !reflect.DeepEqual(stage, next) {
			diff.ChangedStages = append(diff.ChangedStages, stage.ID)
		}
	}
	for _, stage := range after.Stages {
		if _, exists := before.Stage(stage.ID); !exists {
			diff.AddedStages = append(diff.AddedStages, stage.ID)
		}
	}
	for _, field := range []struct {
		name   string
		before any
		after  any
	}{
		{"template", before.Template, after.Template},
		{"task", before.Task, after.Task},
		{"completion_conditions", before.CompletionConditions, after.CompletionConditions},
		{"start_stage", before.StartStage, after.StartStage},
		{"open_questions", before.OpenQuestions, after.OpenQuestions},
		{"irreversible_decisions", before.IrreversibleDecisions, after.IrreversibleDecisions},
		{"external_operations", before.ExternalOperations, after.ExternalOperations},
	} {
		if !reflect.DeepEqual(field.before, field.after) {
			diff.ChangedFields = append(diff.ChangedFields, field.name)
		}
	}
	beforeIDs, afterIDs := []string{}, []string{}
	for _, stage := range before.Stages {
		beforeIDs = append(beforeIDs, stage.ID)
	}
	for _, stage := range after.Stages {
		afterIDs = append(afterIDs, stage.ID)
	}
	if !slices.Equal(beforeIDs, afterIDs) {
		diff.ChangedFields = append(diff.ChangedFields, "stages")
	}
	return diff
}

type PlanJudgment string

const (
	PlanPending     PlanJudgment = ""
	PlanPass        PlanJudgment = "PASS"
	PlanFail        PlanJudgment = "FAIL"
	PlanNotAssessed PlanJudgment = "NOT ASSESSED"
)

type ProposalOrigin string

const (
	InitialProposal ProposalOrigin = "initial"
	HumanProposal   ProposalOrigin = "human"
	BackProposal    ProposalOrigin = "back"
)

// VersionEvaluation takes judgments, approvals, and withdrawals derived from records. It is not stored in Plan JSON.
type VersionEvaluation struct {
	Plan      Plan
	Judgment  PlanJudgment
	Origin    ProposalOrigin
	Approved  bool
	Rejected  bool
	Withdrawn bool
}

func (v VersionEvaluation) Executable(template PlanTemplate, policy string, lastApproved *Plan) bool {
	return v.Judgment == PlanPass && !v.Rejected && !v.Withdrawn &&
		(!v.Plan.RequiresApproval(template, policy, lastApproved) || v.Approved)
}

// ExecutableVersion is used only at stage start. The caller maintains the binding during execution.
// replanAfterVersion is the old plan version awaiting replanning; normally pass zero.
// It prevents returning to old plans and skips only voluntarily withdrawn human proposals.
func ExecutableVersion(template PlanTemplate, versions []VersionEvaluation, replanAfterVersion int, policy string) (Plan, bool) {
	ordered := slices.Clone(versions)
	slices.SortFunc(ordered, func(a, b VersionEvaluation) int {
		switch {
		case a.Plan.Version > b.Plan.Version:
			return -1
		case a.Plan.Version < b.Plan.Version:
			return 1
		default:
			return 0
		}
	})
	for _, version := range ordered {
		if version.Withdrawn && version.Origin == HumanProposal {
			continue
		}
		if version.Plan.Version <= replanAfterVersion {
			return Plan{}, false
		}
		var lastApproved *Plan
		for _, previous := range ordered {
			if previous.Plan.Version < version.Plan.Version && previous.Approved && !previous.Rejected && !previous.Withdrawn {
				lastApproved = &previous.Plan
				break
			}
		}
		if version.Executable(template, policy, lastApproved) {
			return clonePlan(version.Plan), true
		}
		return Plan{}, false
	}
	return Plan{}, false
}
