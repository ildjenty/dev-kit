package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator holds schemas supplied by kit and does not read files.
type Validator struct {
	template *jsonschema.Schema
	plan     *jsonschema.Schema
}

// NewValidator takes the contents of both schemas, which may come from embedded assets.
func NewValidator(templateSchema, planSchema []byte) (*Validator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	for _, resource := range []struct {
		path string
		data []byte
	}{{TemplateSchemaPath, templateSchema}, {PlanSchemaPath, planSchema}} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(resource.data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", resource.path, err)
		}
		if err := compiler.AddResource(schemaURL(resource.path), doc); err != nil {
			return nil, fmt.Errorf("%s: %w", resource.path, err)
		}
	}
	template, err := compiler.Compile(schemaURL(TemplateSchemaPath))
	if err != nil {
		return nil, fmt.Errorf("template schema: %w", err)
	}
	plan, err := compiler.Compile(schemaURL(PlanSchemaPath))
	if err != nil {
		return nil, fmt.Errorf("plan schema: %w", err)
	}
	return &Validator{template: template, plan: plan}, nil
}

func schemaURL(path string) string {
	return "https://devkit.invalid/schemas/v1/" + strings.TrimPrefix(path, "schemas/")
}

func decode(data []byte, schema *jsonschema.Schema, result any) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("JSON Schema validation: %w", err)
	}
	return json.Unmarshal(data, result)
}

func (v *Validator) ParseTemplate(data []byte) (PlanTemplate, error) {
	var template PlanTemplate
	if err := decode(data, v.template, &template); err != nil {
		return PlanTemplate{}, err
	}
	if err := validateStages(template.StartStage, template.Stages, template.OptionalStages, nil); err != nil {
		return PlanTemplate{}, err
	}
	return template, nil
}

func (v *Validator) ParsePlan(data []byte) (Plan, error) {
	var plan Plan
	if err := decode(data, v.plan, &plan); err != nil {
		return Plan{}, err
	}
	conditions := make(map[string]bool)
	for _, condition := range plan.CompletionConditions {
		if conditions[condition.ID] {
			return Plan{}, fmt.Errorf("duplicate completion condition ID: %q", condition.ID)
		}
		conditions[condition.ID] = true
	}
	if err := validateStages(plan.StartStage, plan.Stages, nil, conditions); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (v *Validator) ValidateTemplate(template PlanTemplate) error {
	data, err := json.Marshal(template)
	if err != nil {
		return err
	}
	_, err = v.ParseTemplate(data)
	return err
}

func (v *Validator) ValidatePlan(plan Plan) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	_, err = v.ParsePlan(data)
	return err
}

func validateStages(start string, stages, optional []Stage, conditions map[string]bool) error {
	all := append(slices.Clone(stages), optional...)
	ids := make(map[string]bool)
	for _, stage := range all {
		if stage.ID == "replan" {
			return fmt.Errorf("stage ID replan is reserved")
		}
		if ids[stage.ID] {
			return fmt.Errorf("duplicate stage ID: %q", stage.ID)
		}
		ids[stage.ID] = true
	}
	if !slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == start }) {
		return fmt.Errorf("start stage is not selected: %q", start)
	}
	for _, stage := range all {
		if err := validateStageTransitions(stage, stages, ids, conditions != nil); err != nil {
			return err
		}
		if err := validateStageChecks(stage, conditions); err != nil {
			return err
		}
	}
	return validateDoneChains(all)
}

func validateStageTransitions(stage Stage, stages []Stage, ids map[string]bool, plannedBack bool) error {
	for _, action := range []Transition{Fix, Ask, Back, Done} {
		target, _ := stage.Transitions.Next(action)
		if action == Done && target != nil && *target == "replan" {
			continue
		}
		// A back transition replans from its target, so in a plan it may name a stage that
		// already ran and is no longer selected. ValidateBackTargets checks it against the template.
		if action == Back && plannedBack && target != nil && *target != "replan" {
			continue
		}
		if target != nil && !ids[*target] {
			return fmt.Errorf("%s: %s transition target not found: %q", stage.ID, action, *target)
		}
		if target != nil && slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == stage.ID }) &&
			!slices.ContainsFunc(stages, func(s Stage) bool { return s.ID == *target }) {
			return fmt.Errorf("%s: %s references an unselected optional stage: %q", stage.ID, action, *target)
		}
	}
	return nil
}

func validateStageChecks(stage Stage, conditions map[string]bool) error {
	for i, check := range stage.Checks {
		if slices.ContainsFunc(stage.Checks[:i], func(previous Check) bool {
			return previous.Aspect == check.Aspect && previous.Runner == check.Runner &&
				(previous.Target != check.Target || !slices.Equal(previous.CompletionConditionIDs, check.CompletionConditionIDs))
		}) {
			return fmt.Errorf("%s: conflicting check mappings for aspect %q and runner %q", stage.ID, check.Aspect, check.Runner)
		}
		if check.Runner == "command:" {
			return fmt.Errorf("%s: command name is missing", stage.ID)
		}
		if (check.Runner.IsCommand() || strings.HasPrefix(check.Aspect, "command:")) && check.Aspect != string(check.Runner) {
			return fmt.Errorf("%s: command aspect and runner differ", stage.ID)
		}
		for _, id := range check.CompletionConditionIDs {
			if conditions != nil && !conditions[id] {
				return fmt.Errorf("%s: completion condition not found: %q", stage.ID, id)
			}
		}
	}
	return nil
}

func validateDoneChains(stages []Stage) error {
	next := make(map[string]*string, len(stages))
	for _, stage := range stages {
		next[stage.ID] = stage.Transitions.Done
	}
	finished := make(map[string]bool)
	for _, stage := range stages {
		seen := make(map[string]bool)
		for id := &stage.ID; id != nil && *id != "replan" && !finished[*id]; id = next[*id] {
			if seen[*id] {
				return fmt.Errorf("done transitions contain a cycle at stage %q", *id)
			}
			seen[*id] = true
		}
		for id := range seen {
			finished[id] = true
		}
	}
	return nil
}

// ValidateBackTargets checks that every back transition of the plan names a stage of the
// plan or of its template, including optional stages.
func ValidateBackTargets(template PlanTemplate, p Plan) error {
	known := map[string]bool{}
	for _, stage := range append(append(slices.Clone(template.Stages), template.OptionalStages...), p.Stages...) {
		known[stage.ID] = true
	}
	for _, stage := range p.Stages {
		if target, _ := stage.Transitions.Next(Back); target != nil && !known[*target] {
			return fmt.Errorf("%s: back transition target not found: %q", stage.ID, *target)
		}
	}
	return nil
}
