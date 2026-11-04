package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Resolver returns document bodies selected at the caller's fixed instruction reference.
type Resolver func(Selection) ([]string, error)

// ValidatePatternReference checks the referenced pattern's existence and original content hash.
func ValidatePatternReference(ref PatternRef, resolve Resolver) error {
	if ref.Name == "" || strings.ContainsAny(ref.Name, "/\\*?[]\x00") || ref.Name == "." || ref.Name == ".." {
		return fmt.Errorf("invalid pattern name: %q", ref.Name)
	}
	documents, err := resolve(Selection{Source: "plans", Select: ref.Name + ".json"})
	if err != nil {
		return fmt.Errorf("pattern %q: %w", ref.Name, err)
	}
	if len(documents) != 1 {
		return fmt.Errorf("pattern %q must resolve to one document", ref.Name)
	}
	if ContentHash([]byte(documents[0])) != ref.ContentHash {
		return fmt.Errorf("pattern %q content hash does not match", ref.Name)
	}
	return nil
}

// ValidateReferences checks definitions and every selected document without reading files.
// Check definitions come from the instruction's fixed configuration.
func ValidateReferences(p Plan, checks map[string]CheckDefinition, detectionRequired bool, resolve Resolver) error {
	if err := ValidatePatternReference(p.Pattern, resolve); err != nil {
		return err
	}
	for _, stage := range p.Stages {
		if err := validateStageReferences(stage, checks, detectionRequired, resolve); err != nil {
			return fmt.Errorf("%s: %w", stage.ID, err)
		}
	}
	return nil
}

func validateStageReferences(stage Stage, checks map[string]CheckDefinition, detectionRequired bool, resolve Resolver) error {
	definitionID := stage.DefinitionID()
	if definitionID == "" || definitionID == "." || definitionID == ".." || strings.ContainsAny(definitionID, "/\\*?[]\x00") {
		return fmt.Errorf("invalid stage definition: %q", definitionID)
	}
	selections := []Selection{{Source: "stages", Select: definitionID + ".md"}}
	selections = append(selections, stage.WorkerInputs()...)
	selections = append(selections, stage.Delegations...)
	for _, selection := range []*Selection{stage.Report, stage.UnitReport, stage.HumanPrompt} {
		if selection != nil {
			selections = append(selections, *selection)
		}
	}
	if err := ValidateSelections(selections, resolve); err != nil {
		return err
	}
	for _, check := range stage.Checks {
		definition, ok := checks[check.Aspect]
		if !ok {
			return fmt.Errorf("check is not configured: %q", check.Aspect)
		}
		if err := validateCheckReferences(check, definition, detectionRequired, resolve); err != nil {
			return err
		}
	}
	return nil
}

func validateCheckReferences(check Check, definition CheckDefinition, detectionRequired bool, resolve Resolver) error {
	for _, selection := range definition.Criteria {
		if _, err := resolveDocuments(selection, resolve); err != nil {
			return fmt.Errorf("%s criteria: %w", check.Aspect, err)
		}
	}
	if definition.Examples == nil {
		if detectionRequired && check.Required && !definition.Runner.IsCommand() {
			return fmt.Errorf("%s: required judgment check has no examples", check.Aspect)
		}
		return nil
	}
	documents, err := resolveDocuments(*definition.Examples, resolve)
	if err != nil {
		return fmt.Errorf("%s examples: %w", check.Aspect, err)
	}
	for _, body := range documents {
		var specification struct {
			Aspect string `json:"aspect"`
		}
		if err := json.Unmarshal([]byte(body), &specification); err != nil {
			return fmt.Errorf("%s examples: %w", check.Aspect, err)
		}
		if specification.Aspect != check.Aspect {
			return fmt.Errorf("%s: examples aspect does not match: %q", check.Aspect, specification.Aspect)
		}
	}
	return nil
}

func resolveDocuments(selection Selection, resolve Resolver) ([]string, error) {
	documents, err := resolve(selection)
	if err != nil {
		return nil, fmt.Errorf("%s:%s: %w", selection.Source, selection.Select, err)
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("%s:%s did not resolve to any documents", selection.Source, selection.Select)
	}
	return documents, nil
}

// ValidateSelections requires every input selection to resolve at the fixed reference.
// Callers also use it for planning.inputs and plan_check.inputs.
func ValidateSelections(selections []Selection, resolve Resolver) error {
	for _, selection := range selections {
		if _, err := resolveDocuments(selection, resolve); err != nil {
			return err
		}
	}
	return nil
}
