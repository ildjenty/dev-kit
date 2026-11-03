package plan

import (
	"fmt"
	"reflect"
	"slices"
)

// NewPlan は既定ステージを複製する。check の対象と完了条件の対応は呼び出し側で埋め、受理時に検証する。
func NewPlan(template PlanTemplate, contentHash, task string, conditions []CompletionCondition) Plan {
	p := Plan{
		Version: 1, Template: TemplateRef{template.Name, contentHash}, Task: task,
		CompletionConditions: conditions, StartStage: template.StartStage,
		MetaLoopStages: template.MetaLoopStages, Stages: template.Stages,
		ExternalOperations: []ExternalOperation{},
	}
	return clonePlan(p)
}

// SelectStages は雛形のステージを外し、候補を指定順の位置に足す。先頭を開始ステージとする。
// 構成は雛形から取り直すため、check の対象と完了条件の対応は選択後に設定する。
func (p Plan) SelectStages(template PlanTemplate, ids []string) (Plan, error) {
	if len(ids) == 0 {
		return Plan{}, fmt.Errorf("ステージを1つ以上選ぶ必要がある")
	}
	known := make(map[string]Stage)
	for _, s := range append(slices.Clone(template.Stages), template.OptionalStages...) {
		known[s.ID] = s
	}
	selected := make(map[string]bool)
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			return Plan{}, fmt.Errorf("雛形にも候補にもないステージ: %q", id)
		}
		if selected[id] {
			return Plan{}, fmt.Errorf("ステージ ID の重複: %q", id)
		}
		selected[id] = true
	}
	result := clonePlan(p)
	result.Stages = nil
	for _, stage := range template.Stages {
		result.Stages = append(result.Stages, cloneStage(stage))
	}
	for _, stage := range template.Stages {
		if !selected[stage.ID] {
			result.Stages = removeStage(result.Stages, stage.ID)
		}
	}
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		if slices.ContainsFunc(result.Stages, func(s Stage) bool { return s.ID == id }) {
			continue
		}
		stage := cloneStage(known[id])
		if i+1 < len(ids) {
			before := ids[i+1]
			stage.Transitions.Done = &before
			for j := range result.Stages {
				done := result.Stages[j].Transitions.Done
				if done != nil && *done == before {
					result.Stages[j].Transitions.Done = &id
				}
			}
		} else {
			stage.Transitions.Done = nil
			for j := range result.Stages {
				if result.Stages[j].Transitions.Done == nil {
					result.Stages[j].Transitions.Done = &id
				}
			}
		}
		result.Stages = append(result.Stages, stage)
	}
	ordered := make([]Stage, 0, len(ids))
	for _, id := range ids {
		stage, _ := result.Stage(id)
		ordered = append(ordered, stage)
	}
	result.Stages = ordered
	result.StartStage = ids[0]
	result.MetaLoopStages = make([]string, 0)
	for _, id := range template.MetaLoopStages {
		if selected[id] {
			result.MetaLoopStages = append(result.MetaLoopStages, id)
		}
	}
	return result, nil
}

// 省略したステージへの辺を同じ分類の次の辺につなぎ、自己参照なら元のステージに戻す。
func removeStage(stages []Stage, id string) []Stage {
	index := slices.IndexFunc(stages, func(s Stage) bool { return s.ID == id })
	removed := stages[index]
	stages = slices.Delete(stages, index, index+1)
	for i := range stages {
		for _, action := range []Transition{Fix, Ask, Back, Done} {
			target, _ := stages[i].Transitions.Next(action)
			if target == nil || *target != id {
				continue
			}
			next, _ := removed.Transitions.Next(action)
			if next != nil && *next == id {
				self := stages[i].ID
				next = &self
			}
			setTransition(&stages[i].Transitions, action, next)
		}
	}
	return stages
}

func setTransition(t *Transitions, action Transition, target *string) {
	switch action {
	case Fix:
		t.Fix = *target
	case Ask:
		t.Ask = *target
	case Back:
		t.Back = *target
	case Done:
		t.Done = target
	}
}

type DeviationKind string

const (
	AddedStage           DeviationKind = "added_stage"
	RemovedGate          DeviationKind = "removed_gate"
	ChangedTransition    DeviationKind = "changed_transition"
	ChangedStartStage    DeviationKind = "changed_start_stage"
	RemovedRequiredCheck DeviationKind = "removed_required_check"
)

type Deviation struct {
	Kind       DeviationKind
	StageID    string
	Aspect     string
	Runner     Runner
	Transition Transition
}

func Deviations(template PlanTemplate, p Plan) []Deviation {
	known := make(map[string]Stage)
	for _, stage := range append(slices.Clone(template.Stages), template.OptionalStages...) {
		known[stage.ID] = stage
	}
	ids := make([]string, 0, len(p.Stages))
	for _, stage := range p.Stages {
		if _, ok := known[stage.ID]; ok {
			ids = append(ids, stage.ID)
		}
	}
	expected := Plan{}
	if len(ids) > 0 {
		expected, _ = p.SelectStages(template, ids)
	}
	result := make([]Deviation, 0)
	// 先頭の省略と候補の挿入を除き、採用済みの開始ステージを変更することは逸脱とする。
	start := ""
	for _, stage := range template.Stages {
		if slices.Contains(ids, stage.ID) {
			start = stage.ID
			break
		}
	}
	if len(ids) > 0 && slices.ContainsFunc(template.OptionalStages, func(s Stage) bool { return s.ID == ids[0] }) {
		start = ids[0]
	}
	if _, exists := known[p.StartStage]; exists && p.StartStage != start {
		result = append(result, Deviation{Kind: ChangedStartStage, StageID: p.StartStage})
	}
	for _, stage := range p.Stages {
		original, ok := expected.Stage(stage.ID)
		if !ok {
			result = append(result, Deviation{Kind: AddedStage, StageID: stage.ID})
			continue
		}
		if original.Gate.Required && !stage.Gate.Required {
			result = append(result, Deviation{Kind: RemovedGate, StageID: stage.ID})
		}
		for _, action := range []Transition{Fix, Ask, Back, Done} {
			before, _ := original.Transitions.Next(action)
			after, _ := stage.Transitions.Next(action)
			if !reflect.DeepEqual(before, after) {
				result = append(result, Deviation{Kind: ChangedTransition, StageID: stage.ID, Transition: action})
			}
		}
		for _, required := range original.RequiredChecks() {
			if !slices.ContainsFunc(stage.Checks, func(c Check) bool {
				return c.Required && c.Aspect == required.Aspect && c.Runner == required.Runner
			}) {
				result = append(result, Deviation{Kind: RemovedRequiredCheck, StageID: stage.ID, Aspect: required.Aspect, Runner: required.Runner})
			}
		}
	}
	return result
}

func (p Plan) RequiresApproval(template PlanTemplate) bool {
	return len(p.ExternalOperations) > 0 || len(Deviations(template, p)) > 0
}
