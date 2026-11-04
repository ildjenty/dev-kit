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
	return e.adoptedBefore(state, "")
}

// adoptedBefore replays the adopted path up to stop, excluding that run.
func (e *Engine) adoptedBefore(state *ledger.State, stop string) ([]*ledger.StageRun, error) {
	adopted := []*ledger.StageRun{}
	for _, run := range state.StageRuns() {
		if run.ID == stop {
			break
		}
		switch run.State {
		case ledger.StageCompleted:
			adopted = slices.DeleteFunc(adopted, func(old *ledger.StageRun) bool { return old.StageID == run.StageID })
			adopted = append(adopted, run)
		case ledger.StageInvalidated:
			target, err := e.backTarget(state, run)
			if err != nil {
				return nil, err
			}
			if index := slices.IndexFunc(adopted, func(old *ledger.StageRun) bool { return old.StageID == target }); index >= 0 {
				adopted = adopted[:index]
			}
		}
	}
	return adopted, nil
}

func (e *Engine) backTarget(state *ledger.State, run *ledger.StageRun) (string, error) {
	if !backDecided(state, run) {
		return "", nil
	}
	p, ok, err := e.boundPlan(run.PlanVersion)
	if err != nil || !ok {
		return "", err
	}
	target, err := p.Next(run.StageID, plan.Back)
	if err != nil || target == nil {
		return "", err
	}
	return *target, nil
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
		bound, ok, err := e.boundPlan(run.PlanVersion)
		if err != nil {
			return nil, err
		}
		stage, found := bound.Stage(run.StageID)
		if !ok || !found {
			return nil, fmt.Errorf("bound stage %q is missing from plan %d", run.StageID, run.PlanVersion)
		}
		outputs = append(outputs, brief.StageOutput{StageID: run.StageID, Definition: stage.DefinitionID(), Content: roundOutput(state, records, run, rounds[len(rounds)-1])})
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
		return !slices.Contains(selection, output.StageID) && (output.Definition == "" || !slices.Contains(selection, output.Definition))
	})
}
