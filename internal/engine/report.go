package engine

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"github.com/ildjenty/dev-kit/internal/ledger"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// reportSchema returns the body of a selected report schema fixed for ref, or "" without a selection.
func (e *Engine) reportSchema(ctx context.Context, ref string, selection *plan.Selection) (string, error) {
	if selection == nil {
		return "", nil
	}
	documents, err := e.selectDocuments(ctx, ref, selection.Source, selection.Select)
	if err != nil {
		return "", err
	}
	if len(documents) != 1 {
		return "", fmt.Errorf("report selection %q from source %q must select one schema; it selected %d", selection.Select, selection.Source, len(documents))
	}
	return documents[0].Body, nil
}

// checkReport validates a report against its schema. Completion claims require the full shape;
// questions and stops may carry a partial report that conforms to the fields it supplies.
// Conformance only shows the shape; it is never evidence of completion.
func checkReport(schema string, progress form.Progress) error {
	if schema == "" {
		return nil
	}
	complete := progress.TerminationKind == form.TerminationCompleted
	if len(progress.Report) == 0 {
		if complete {
			return errors.New("a completion claim requires report following the selected report schema")
		}
		return nil
	}
	source := []byte(schema)
	if !complete {
		var value any
		if err := json.Unmarshal(source, &value); err != nil {
			return err
		}
		partial, err := json.Marshal(withoutRequired(value))
		if err != nil {
			return err
		}
		source = partial
	}
	compiled, err := form.CompileSchema(source)
	if err != nil {
		return fmt.Errorf("report schema: %w", err)
	}
	if err := compiled.Validate(progress.Report); err != nil {
		return fmt.Errorf("report does not match the selected report schema: %w", err)
	}
	return nil
}

func withoutRequired(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, item := range v {
			if _, list := item.([]any); key == "required" && list {
				continue
			}
			result[key] = withoutRequired(item)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = withoutRequired(item)
		}
		return result
	}
	return value
}

// stageReportSelection returns the report schema selection for work in a round: split units use
// unit_report, and whole-stage work (session or one agent unit) uses the stage report.
func stageReportSelection(stage plan.Stage, unit *plan.Unit) *plan.Selection {
	if unit != nil && stage.Split && unit.ID != stage.ID {
		return stage.UnitReport
	}
	return stage.Report
}

// validateRoundReport checks a submitted or returned termination block against the report schema
// selected for the round's work.
func (e *Engine) validateRoundReport(ctx context.Context, state *ledger.State, round *ledger.Round, unit *plan.Unit, progress form.Progress) error {
	stage, _, err := e.roundPlanStage(state, round)
	if err != nil {
		return err
	}
	schema, err := e.reportSchema(ctx, roundReference(round), stageReportSelection(stage, unit))
	if err != nil {
		return err
	}
	return checkReport(schema, progress)
}
