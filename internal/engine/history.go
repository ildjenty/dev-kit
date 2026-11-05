package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// historyScope selects the optional history of a fixed index deterministically from records.
// Required judgment evidence is supplied as current material regardless of the scope.
type historyScope struct {
	WorkerStage string
	Kind        plan.History
	RunID       string
	Aspect      string
	Runner      string
}

// historyIndex writes the fixed index for an instruction: every adopted output followed by the
// history selected by scope. The text is fixed at issue time in the instruction context.
func (e *Engine) historyIndex(state *ledger.State, meta *ledger.Instruction, scope historyScope, adopted []brief.StageOutput) (string, error) {
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{}
	}
	if meta.Context.WorkHistory == "" {
		text, err := e.historyText(state, scope, adopted)
		if err != nil {
			return "", err
		}
		meta.Context.WorkHistory = text
	}
	name := "brief/" + meta.ID + "-history.md"
	if err := e.store().Publish(name, []byte(meta.Context.WorkHistory)); err != nil {
		return "", err
	}
	return filepath.Join(e.workRoot(), name), nil
}

func (e *Engine) historyText(state *ledger.State, scope historyScope, adopted []brief.StageOutput) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	var text strings.Builder
	text.WriteString("# Adopted outputs\n\n")
	for _, output := range adopted {
		fmt.Fprintf(&text, "## %s\n\n%s\n\n", output.StageID, output.Content)
	}
	if scope.WorkerStage != "" {
		instructions, err := e.mainInstructions("worker", scope.WorkerStage)
		if err != nil {
			return "", err
		}
		if instructions != "" {
			text.WriteString("# Main agent instruction records\n" + instructions + "\n")
		}
	}
	if scope.Kind == plan.HistoryAll {
		inputs, err := humanInputsText(e.workRoot(), "Human inputs", "")
		if err != nil {
			return "", err
		}
		text.WriteString(inputs)
	}
	fmt.Fprintf(&text, "# History (scope: %s)\n\n", scope.Kind)
	if scope.Kind == plan.HistoryNone {
		return text.String(), nil
	}
	selected := historyRecords(state, records, scope)
	for _, record := range records {
		if !selected[record.ID] {
			continue
		}
		data, err := json.MarshalIndent(record.Payload, "", "  ")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&text, "%s %s\n%s\n\n", record.ID, record.Kind(), data)
	}
	for _, run := range state.StageRuns() {
		if scope.Kind == plan.HistoryAll || run.ID == scope.RunID {
			fmt.Fprintf(&text, "Stage %s (%s): %d rounds, state: %s, plan version: %d\n", run.StageID, run.ID, len(state.Rounds(run.ID)), run.State, run.PlanVersion)
		}
	}
	return text.String(), nil
}

// humanInputsText lists inputs after the latest one included in the fixed history, oldest first.
func humanInputsText(root, heading, history string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "prompts"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	type input struct {
		name string
		at   time.Time
	}
	var inputs []input
	var through time.Time
	for _, entry := range entries {
		base, ok := strings.CutSuffix(entry.Name(), ".txt")
		if entry.IsDir() || !ok {
			continue
		}
		observed, err := time.Parse(promptTimestamp, base[strings.LastIndex(base, "-")+1:])
		if err != nil {
			return "", fmt.Errorf("prompt observation %s: %w", entry.Name(), err)
		}
		inputs = append(inputs, input{entry.Name(), observed})
		if strings.Contains(history, fmt.Sprintf("## %s (%s)\n\n", observed.Format(time.RFC3339Nano), entry.Name())) && observed.After(through) {
			through = observed
		}
	}
	slices.SortStableFunc(inputs, func(a, b input) int { return a.at.Compare(b.at) })
	var text strings.Builder
	fmt.Fprintf(&text, "# %s\n\n", heading)
	if len(inputs) == 0 || !inputs[len(inputs)-1].at.After(through) {
		text.WriteString("None recorded.\n\n")
	}
	for _, item := range inputs {
		if !item.at.After(through) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "prompts", item.name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&text, "## %s (%s)\n\n%s\n\n", item.at.Format(time.RFC3339Nano), item.name, data)
	}
	return text.String(), nil
}

// historyRecords returns the IDs of records in scope. "all" includes reports of superseded runs.
func historyRecords(state *ledger.State, records []ledger.Record, scope historyScope) map[string]bool {
	rounds := map[string]bool{}
	for _, round := range state.Rounds(scope.RunID) {
		rounds[round.ID] = true
	}
	questions := map[string]bool{}
	for _, q := range state.Questions() {
		if slices.Contains(q.Evidence, form.Reference{RecordID: scope.RunID}) {
			questions[q.ID] = true
		}
	}
	selected := map[string]bool{}
	for _, record := range records {
		if scope.Kind == plan.HistoryAll {
			selected[record.ID] = allHistoryRecord(record)
			continue
		}
		selected[record.ID] = runHistoryRecord(record, rounds, questions, scope)
	}
	return selected
}

func allHistoryRecord(record ledger.Record) bool {
	switch record.Payload.(type) {
	case *ledger.Judgment, *ledger.Question, *ledger.Answer, *ledger.Approval, *ledger.Decision, *ledger.Evidence,
		*ledger.Lesson, *ledger.ResultRejection, *ledger.Progress, *ledger.UnitResult, *ledger.JudgmentBrief:
		return true
	}
	return false
}

func runHistoryRecord(record ledger.Record, rounds, questions map[string]bool, scope historyScope) bool {
	switch p := record.Payload.(type) {
	case *ledger.Judgment:
		return rounds[p.RoundID] && (scope.Kind != plan.HistoryCheck || p.Aspect == scope.Aspect && p.Runner == scope.Runner)
	case *ledger.Progress:
		return rounds[p.RoundID]
	case *ledger.UnitResult:
		return rounds[p.RoundID] && scope.Kind == plan.HistoryStageRun
	case *ledger.JudgmentBrief:
		return rounds[p.RoundID] && (scope.Kind != plan.HistoryCheck || p.Aspect == scope.Aspect && p.Runner == scope.Runner)
	case *ledger.Question:
		return questions[p.ID]
	case *ledger.Answer:
		return questions[p.QuestionID]
	case *ledger.Approval:
		return questions[p.QuestionID]
	}
	return false
}
