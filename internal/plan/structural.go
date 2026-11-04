package plan

import (
	"reflect"
	"slices"
)

// StructuralChange is a symmetric display difference with original values.
// Field is start_stage, stages, stage, or a Stage JSON field name.
type StructuralChange struct {
	StageID string
	Field   string
	Before  any
	After   any
}

// StructuralDifference compares plan structure without requirements, versions, or pattern identity.
func StructuralDifference(before, after Plan) []StructuralChange {
	var result []StructuralChange
	if before.StartStage != after.StartStage {
		result = append(result, StructuralChange{Field: "start_stage", Before: before.StartStage, After: after.StartStage})
	}
	beforeIDs, afterIDs := []string{}, []string{}
	for _, stage := range before.Stages {
		beforeIDs = append(beforeIDs, stage.ID)
		next, ok := after.Stage(stage.ID)
		if !ok {
			result = append(result, StructuralChange{stage.ID, "stage", cloneStage(stage), nil})
			continue
		}
		result = append(result, structuralStageDifference(stage, next)...)
	}
	for _, stage := range after.Stages {
		afterIDs = append(afterIDs, stage.ID)
		if _, ok := before.Stage(stage.ID); !ok {
			result = append(result, StructuralChange{stage.ID, "stage", nil, cloneStage(stage)})
		}
	}
	if !slices.Equal(beforeIDs, afterIDs) {
		result = append(result, StructuralChange{Field: "stages", Before: beforeIDs, After: afterIDs})
	}
	return result
}

func structuralStageDifference(before, after Stage) []StructuralChange {
	before, after = cloneStage(before), cloneStage(after)
	var result []StructuralChange
	for _, field := range []struct {
		name          string
		before, after any
	}{
		{"definition", before.DefinitionID(), after.DefinitionID()},
		{"assignee", before.Assignee, after.Assignee},
		{"checks", before.Checks, after.Checks},
		{"gate", before.Gate, after.Gate},
		{"limits", before.Limits, after.Limits},
		{"transitions", before.Transitions, after.Transitions},
		{"report", before.Report, after.Report},
		{"unit_report", before.UnitReport, after.UnitReport},
		{"human_prompt", before.HumanPrompt, after.HumanPrompt},
		{"delegations", before.Delegations, after.Delegations},
		{"outputs", before.Outputs, after.Outputs},
		{"history", before.WorkerHistory(), after.WorkerHistory()},
		{"inputs", before.WorkerInputs(), after.WorkerInputs()},
		{"instructions", before.Instructions, after.Instructions},
		{"split", before.Split, after.Split},
		{"units", before.Units, after.Units},
	} {
		if !reflect.DeepEqual(field.before, field.after) {
			result = append(result, StructuralChange{after.ID, field.name, field.before, field.after})
		}
	}
	return result
}

// RemainingPlan copies a plan without adopted completed instances for approval comparisons.
// An empty remainder has no start stage and is display material, not an executable plan.
func RemainingPlan(p Plan, completedIDs map[string]bool) Plan {
	result := clonePlan(p)
	result.Stages = slices.DeleteFunc(result.Stages, func(stage Stage) bool { return completedIDs[stage.ID] })
	for completedIDs[result.StartStage] {
		stage, ok := p.Stage(result.StartStage)
		if !ok || stage.Transitions.Done == nil || *stage.Transitions.Done == "replan" {
			result.StartStage = ""
			break
		}
		result.StartStage = *stage.Transitions.Done
	}
	return result
}
