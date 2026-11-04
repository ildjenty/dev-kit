package engine

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// adoptedRuns returns the completed StageRuns on the current execution path in start order.
// A back decision supersedes earlier completed runs of the back target and the stages after it.
func (e *Engine) adoptedRuns(state *ledger.State) ([]*ledger.StageRun, error) {
	adopted := map[string]*ledger.StageRun{}
	order := []string{}
	for _, run := range state.StageRuns() {
		switch run.State {
		case ledger.StageCompleted:
			if _, ok := adopted[run.StageID]; !ok {
				order = append(order, run.StageID)
			}
			adopted[run.StageID] = run
		case ledger.StageInvalidated:
			superseded, err := e.backSuperseded(state, run)
			if err != nil {
				return nil, err
			}
			for _, id := range superseded {
				delete(adopted, id)
			}
		}
	}
	runs := []*ledger.StageRun{}
	for _, id := range order {
		if run, ok := adopted[id]; ok && !slices.Contains(runs, run) {
			runs = append(runs, run)
		}
	}
	slices.SortStableFunc(runs, func(a, b *ledger.StageRun) int { return strings.Compare(a.ID, b.ID) })
	return runs, nil
}

// backSuperseded returns the stage IDs whose adopted outputs a back decision of run invalidates.
func (e *Engine) backSuperseded(state *ledger.State, run *ledger.StageRun) ([]string, error) {
	rounds := state.Rounds(run.ID)
	if len(rounds) == 0 || rounds[len(rounds)-1].Decision == nil || rounds[len(rounds)-1].Decision.Action != plan.Back {
		return nil, nil
	}
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil || !ok {
		return nil, err
	}
	target, err := p.Next(run.StageID, plan.Back)
	if err != nil || target == nil {
		return nil, err
	}
	ids := []string{}
	found := false
	for _, stage := range p.Stages {
		found = found || stage.ID == *target
		if found {
			ids = append(ids, stage.ID)
		}
	}
	return ids, nil
}

// adoptedOutputs returns the report and fixed artifact references of the last adopted round
// of each stage on the current execution path. Superseded runs belong to history.
func (e *Engine) adoptedOutputs(state *ledger.State) ([]brief.StageOutput, error) {
	runs, err := e.adoptedRuns(state)
	if err != nil {
		return nil, err
	}
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	outputs := []brief.StageOutput{}
	for _, run := range runs {
		rounds := state.Rounds(run.ID)
		if len(rounds) == 0 {
			continue
		}
		outputs = append(outputs, brief.StageOutput{StageID: run.StageID, Content: roundOutput(state, records, run, rounds[len(rounds)-1])})
	}
	return outputs, nil
}

func roundOutput(state *ledger.State, records []ledger.Record, run *ledger.StageRun, round *ledger.Round) string {
	var text strings.Builder
	fmt.Fprintf(&text, "Stage run: %s\nRound: %s\nCommit: %s\n", run.ID, round.ID, run.Commit)
	if round.ReturnedTarget != nil {
		fmt.Fprintf(&text, "Tree: %s\n", round.ReturnedTarget.Tree)
	}
	if progress := roundProgress(records, round.ID); progress != nil {
		text.WriteString("\nReport:\n\n" + reportText(progress.Progress.Report, progress.Progress) + "\n")
	}
	units := roundUnitResults(state, round)
	for _, unit := range units {
		if len(units) == 1 && unit.UnitID == run.StageID && !round.MergeFix {
			continue
		}
		fmt.Fprintf(&text, "\nUnit %s report:\n\n%s\n", unit.UnitID, reportText(unit.Report, unit.Progress))
	}
	return text.String()
}

func roundProgress(records []ledger.Record, roundID string) *ledger.Progress {
	var found *ledger.Progress
	for _, record := range records {
		if progress, ok := record.Payload.(*ledger.Progress); ok && progress.RoundID == roundID {
			found = progress
		}
	}
	return found
}

// reportText renders a stage-specific report, or the termination block when none was selected.
func reportText(report json.RawMessage, progress any) string {
	var value any = progress
	if len(report) > 0 {
		value = report
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	return "```json\n" + string(data) + "\n```"
}

// selectOutputs keeps the adopted outputs named by selection; nil selects all of them.
func selectOutputs(outputs []brief.StageOutput, selection []string) []brief.StageOutput {
	if selection == nil {
		return slices.Clone(outputs)
	}
	return slices.DeleteFunc(slices.Clone(outputs), func(output brief.StageOutput) bool {
		return !slices.Contains(selection, output.StageID)
	})
}
