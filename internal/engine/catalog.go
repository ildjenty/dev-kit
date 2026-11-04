package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// catalogCheck is a plan check resolved through the [checks.<aspect>] catalog. The plan binds
// the check to the task; the catalog fixes how it runs.
type catalogCheck struct {
	plan.Check
	plan.CheckDefinition
}

// checkCatalog converts the configured check catalog to plan check definitions.
func checkCatalog(settings config.Config) map[string]plan.CheckDefinition {
	catalog := map[string]plan.CheckDefinition{}
	for aspect, item := range settings.Checks {
		definition := plan.CheckDefinition{Runner: plan.Runner(item.Runner), Criteria: planSelections(item.Criteria), Schemas: item.Schemas,
			History: plan.History(item.History), BriefByPlanner: item.BriefByPlanner}
		if item.Examples != nil {
			definition.Examples = &plan.Selection{Source: item.Examples.Source, Select: item.Examples.Select}
		}
		catalog[aspect] = definition
	}
	return catalog
}

// resolveChecks binds each plan check to its catalog entry in the given settings.
func resolveChecks(settings config.Config, items []plan.Check) ([]catalogCheck, error) {
	catalog := checkCatalog(settings)
	result := make([]catalogCheck, 0, len(items))
	for _, item := range items {
		definition, ok := catalog[item.Aspect]
		if !ok {
			return nil, fmt.Errorf("check is not configured: %q", item.Aspect)
		}
		result = append(result, catalogCheck{item, definition})
	}
	return result, nil
}

// catalogChecks resolves plan checks with the check catalog fixed for ref.
func (e *Engine) catalogChecks(ctx context.Context, ref string, items []plan.Check) ([]catalogCheck, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	return resolveChecks(settings, items)
}

// checkRunners maps each aspect of the given checks to its catalog runner.
func checkRunners(items []catalogCheck) map[string]plan.Runner {
	runners := map[string]plan.Runner{}
	for _, item := range items {
		runners[item.Aspect] = item.Runner
	}
	return runners
}

// checkDescription returns a one-line description of a catalog check: the first sentence line
// after the title of its first criteria document, or the command it runs.
func (e *Engine) checkDescription(ctx context.Context, ref, aspect string, definition plan.CheckDefinition) (string, error) {
	if definition.Runner.IsCommand() || len(definition.Criteria) == 0 {
		return fmt.Sprintf("Runs %s.", definition.Runner), nil
	}
	selection := definition.Criteria[0]
	documents, err := e.selectDocuments(ctx, ref, selection.Source, selection.Select)
	if err != nil {
		return "", fmt.Errorf("checks.%s criteria: %w", aspect, err)
	}
	for _, document := range documents {
		for _, line := range strings.Split(document.Body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				return line, nil
			}
		}
	}
	return "Judged by " + string(definition.Runner) + ".", nil
}
