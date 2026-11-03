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

// Replan は現在より大きい番号を残りの計画に付ける。すべての提案を含む最大の番号を渡す。
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
		{"meta_loop_stages", before.MetaLoopStages, after.MetaLoopStages},
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

// VersionEvaluation は記録から導いた判定・承認・撤回を受け取る。Plan の JSON には保存しない。
type VersionEvaluation struct {
	Plan      Plan
	Judgment  PlanJudgment
	Origin    ProposalOrigin
	Approved  bool
	Rejected  bool
	Withdrawn bool
}

func (v VersionEvaluation) Executable(template PlanTemplate) bool {
	return v.Judgment == PlanPass && !v.Rejected && !v.Withdrawn &&
		(!(v.Origin == BackProposal || v.Plan.RequiresApproval(template)) || v.Approved)
}

// ExecutableVersion はステージ開始時だけ使う。実行中の束縛は呼び出し側が維持する。
// replanAfterVersion は再計画待ちになった旧計画の番号とし、通常は0を渡す。
// 旧計画への復帰を許さず、人が自発的に撤回した提案だけを読み飛ばす。
func ExecutableVersion(template PlanTemplate, versions []VersionEvaluation, replanAfterVersion int) (Plan, bool) {
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
		if version.Withdrawn && version.Origin == HumanProposal && replanAfterVersion == 0 {
			continue
		}
		if version.Plan.Version <= replanAfterVersion {
			return Plan{}, false
		}
		if version.Executable(template) && (replanAfterVersion == 0 || version.Approved) {
			return clonePlan(version.Plan), true
		}
		return Plan{}, false
	}
	return Plan{}, false
}
