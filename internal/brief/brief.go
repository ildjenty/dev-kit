package brief

import (
	"encoding/json"

	"github.com/ildjenty/dev-kit/internal/form"
)

// BuildWork assembles a work request without changing previous stage or verification outputs.
func BuildWork(input WorkInput) (Brief, error) {
	stage, err := jsonText(input.Stage)
	if err != nil {
		return Brief{}, err
	}
	p, err := jsonText(input.Plan)
	if err != nil {
		return Brief{}, err
	}
	checks, err := jsonText(input.Repository.AvailableChecks)
	if err != nil {
		return Brief{}, err
	}
	findings, err := jsonText(input.PreviousFindings)
	if err != nil {
		return Brief{}, err
	}
	progress, err := jsonText(form.Progress{
		InstructionID:   input.InstructionID,
		TerminationKind: form.TerminationCompleted,
	})
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, input.RequestTemplate, struct {
		WorkInput
		StageJSON, PlanJSON, ChecksJSON, FindingsJSON, ProgressJSON   string
		TerminationCompleted, TerminationQuestion, TerminationStopped form.TerminationKind
	}{input, stage, p, checks, findings, progress, form.TerminationCompleted, form.TerminationQuestion, form.TerminationStopped})
}

// BuildJudgment wraps the content selected by check and requires the instruction ID in the result.
func BuildJudgment(input JudgmentInput) (Brief, error) {
	id, err := jsonText(input.InstructionID)
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, input.RequestTemplate, struct {
		JudgmentInput
		InstructionIDJSON string
	}{input, id})
}

// BuildPlanner assembles a planning request from the loop template, task, and revision or replanning inputs.
func BuildPlanner(input PlannerInput) (Brief, error) {
	template, err := jsonText(input.Template)
	if err != nil {
		return Brief{}, err
	}
	current, err := jsonText(input.CurrentPlan)
	if err != nil {
		return Brief{}, err
	}
	remaining, err := jsonText(input.RemainingPlan)
	if err != nil {
		return Brief{}, err
	}
	checks, err := jsonText(input.Repository.AvailableChecks)
	if err != nil {
		return Brief{}, err
	}
	findings, err := jsonText(input.PreviousFindings)
	if err != nil {
		return Brief{}, err
	}
	id, err := jsonText(input.InstructionID)
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, input.RequestTemplate, struct {
		PlannerInput
		TemplateJSON, CurrentJSON, RemainingJSON, ChecksJSON, FindingsJSON, InstructionIDJSON string
	}{input, template, current, remaining, checks, findings, id})
}

func jsonText(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return string(data), err
}

func render(id, source string, value any) (Brief, error) {
	text, err := form.ApplyTemplate(source, value)
	if err != nil {
		return Brief{}, err
	}
	return Brief{InstructionID: id, Text: text}, nil
}
