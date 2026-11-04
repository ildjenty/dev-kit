package plan

import (
	"fmt"
	"reflect"
	"slices"
)

// NewPlan clones the default stages. The caller fills in check targets and completion condition mappings, which are validated on acceptance.
func NewPlan(template PlanTemplate, contentHash, task string, conditions []CompletionCondition) Plan {
	p := Plan{
		Version: 1, Template: TemplateRef{template.Name, contentHash}, Task: task,
		CompletionConditions: conditions, StartStage: template.StartStage,
		Stages:             template.Stages,
		ExternalOperations: []ExternalOperation{},
	}
	return clonePlan(p)
}

// SelectStages removes template stages and inserts optional stages in the specified order. The first stage becomes the start stage.
// Configuration is copied from the template, so check targets and completion condition mappings must be set after selection.
func (p Plan) SelectStages(template PlanTemplate, ids []string) (Plan, error) {
	if len(ids) == 0 {
		return Plan{}, fmt.Errorf("at least one stage must be selected")
	}
	known := make(map[string]Stage)
	for _, s := range append(slices.Clone(template.Stages), template.OptionalStages...) {
		known[s.ID] = s
	}
	selected := make(map[string]bool)
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			return Plan{}, fmt.Errorf("stage is neither a default nor an optional template stage: %q", id)
		}
		if selected[id] {
			return Plan{}, fmt.Errorf("duplicate stage ID: %q", id)
		}
		selected[id] = true
	}
	result := clonePlan(p)
	result.Stages = nil
	for _, stage := range template.Stages {
		result.Stages = append(result.Stages, cloneStage(stage))
	}
	for _, stage := range template.OptionalStages {
		if selected[stage.ID] {
			result.Stages = append(result.Stages, cloneStage(stage))
		}
	}
	for _, stage := range template.Stages {
		if !selected[stage.ID] {
			result.Stages = removeStage(result.Stages, stage.ID)
		}
	}
	result.insertOptionalStages(template, ids)
	ordered := make([]Stage, 0, len(ids))
	for _, id := range ids {
		stage, _ := result.Stage(id)
		for _, action := range []Transition{Fix, Ask, Back, Done} {
			target, _ := stage.Transitions.Next(action)
			if target != nil && !(action == Done && *target == "replan") && !selected[*target] {
				return Plan{}, fmt.Errorf("%s: %s transition target not found: %q", id, action, *target)
			}
		}
		ordered = append(ordered, stage)
	}
	result.Stages = ordered
	result.StartStage = ids[0]
	return result, nil
}

// Redirect edges to an omitted stage along its edge of the same action; a self-reference returns to the source stage.
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
	RemovedStage         DeviationKind = "removed_stage"
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
	result := make([]Deviation, 0)
	for _, stage := range template.Stages {
		if !slices.Contains(ids, stage.ID) {
			result = append(result, Deviation{Kind: RemovedStage, StageID: stage.ID})
		}
	}
	// Changing a retained start stage is a deviation, except when omitting leading stages or inserting optional stages.
	start := template.StartStage
	if !slices.Contains(ids, start) {
		for _, stage := range template.Stages {
			if slices.Contains(ids, stage.ID) {
				start = stage.ID
				break
			}
		}
	}
	if len(ids) > 0 && slices.ContainsFunc(template.OptionalStages, func(s Stage) bool { return s.ID == ids[0] }) {
		start = ids[0]
	}
	if _, exists := known[p.StartStage]; exists && p.StartStage != start {
		result = append(result, Deviation{Kind: ChangedStartStage, StageID: p.StartStage})
	}
	for _, stage := range p.Stages {
		original, ok := known[stage.ID]
		if !ok {
			result = append(result, Deviation{Kind: AddedStage, StageID: stage.ID})
			continue
		}
		result = append(result, stageDeviations(original, stage)...)
	}
	return result
}

// RequiresApproval compares the plan with the last human-approved requirements.
// The caller supplies a validated approval policy.
func (p Plan) RequiresApproval(template PlanTemplate, policy string, lastApproved *Plan) bool {
	if len(p.OpenQuestions) > 0 || policy == "always" || len(p.ExternalOperations) > 0 || len(Deviations(template, p)) > 0 {
		return true
	}
	if policy == "deviation" {
		return false
	}
	if lastApproved == nil || len(p.CompletionConditions) != len(lastApproved.CompletionConditions) {
		return true
	}
	for _, condition := range p.CompletionConditions {
		if !slices.Contains(lastApproved.CompletionConditions, condition) {
			return true
		}
	}
	return false
}

func (p *Plan) insertOptionalStages(template PlanTemplate, ids []string) {
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		if slices.ContainsFunc(template.Stages, func(s Stage) bool { return s.ID == id }) {
			continue
		}
		index := slices.IndexFunc(p.Stages, func(s Stage) bool { return s.ID == id })
		stage := p.Stages[index]
		if i+1 < len(ids) {
			before := ids[i+1]
			stage.Transitions.Done = &before
			for j := range p.Stages {
				done := p.Stages[j].Transitions.Done
				if done != nil && *done == before {
					p.Stages[j].Transitions.Done = &id
				}
			}
		} else {
			stage.Transitions.Done = nil
			for j := range p.Stages {
				if p.Stages[j].Transitions.Done == nil {
					p.Stages[j].Transitions.Done = &id
				}
			}
		}
		p.Stages[index] = stage
	}
}

func stageDeviations(original, stage Stage) []Deviation {
	var result []Deviation
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
	return result
}
