package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

// workMaterial collects what a worker needs for this round: the previous round's report in the
// same StageRun and the effective human answers that apply to it. Findings and command output
// are rendered by their own brief sections.
func (e *Engine) workMaterial(state *ledger.State, round *ledger.Round) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	var text strings.Builder
	rounds := state.Rounds(round.StageRunID)
	for i, candidate := range rounds {
		if candidate.ID != round.ID || i == 0 {
			continue
		}
		if progress := roundProgress(records, rounds[i-1].ID); progress != nil {
			fmt.Fprintf(&text, "### Previous round report (%s)\n\n%s\n\n", rounds[i-1].ID, reportText(progress.Progress.Report, progress.Progress))
		}
	}
	text.WriteString(humanAnswers(state, round.StageRunID))
	if text.Len() == 0 {
		return "None", nil
	}
	return text.String(), nil
}

// humanAnswers renders effective answers to questions about this StageRun and work-wide guidance.
func humanAnswers(state *ledger.State, runID string) string {
	var text strings.Builder
	answers := state.AnsweredQuestions()
	for _, q := range state.Questions() {
		answer, ok := answers[q.ID]
		if !ok {
			continue
		}
		if effective, valid := state.ResolveAnswer(q.ApprovalTarget); !valid || effective.RecordID != answer.RecordID {
			continue
		}
		if !slices.Contains(q.Evidence, form.Reference{RecordID: runID}) && q.ApprovalTarget.Kind != form.ApprovalWork && q.ApprovalTarget.Kind != form.ApprovalDecision {
			continue
		}
		fmt.Fprintf(&text, "### Human answer (%s)\n\nQuestion: %s\nInterpretation: %s\n%s\nOriginal human input: %s\n\n", answer.RecordID, firstLine(q.Text), answer.Answer.Interpretation, answer.Answer.Text, answer.Answer.Original)
	}
	return text.String()
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// judgmentMaterial collects evidence always supplied to a judge, independently of the history
// scope: the judged report, unit reports by unit ID, applicable command results, valid approvals,
// cited evidence, previous findings, and the stage comparison.
func (e *Engine) judgmentMaterial(ctx context.Context, state *ledger.State, request check.Request, round *ledger.Round) (string, error) {
	var text strings.Builder
	if request.OriginalRequest != nil || request.StartTask != "" {
		original := "No original user request was observed."
		if request.OriginalRequest != nil {
			original = *request.OriginalRequest
		}
		fmt.Fprintf(&text, "### Original user request\n\n%s\n\n### Task supplied to start\n\n%s\n\n", original, request.StartTask)
	}
	if request.Progress != nil {
		fmt.Fprintf(&text, "### Judged round report\n\n%s\n\n", jsonBlock(request.Progress))
	}
	if round != nil {
		evidence, err := e.roundEvidence(ctx, state, request, round)
		if err != nil {
			return "", err
		}
		text.WriteString(evidence)
	}
	for _, authorization := range request.Authorizations {
		fmt.Fprintf(&text, "### Valid human approval (%s)\n\n%s\n\n", referenceList(authorization.Evidence), authorization.Scope)
	}
	if len(request.PreviousFindings) > 0 {
		fmt.Fprintf(&text, "### Previous findings for this aspect\n\n%s\n\n", jsonBlock(request.PreviousFindings))
	}
	return text.String(), nil
}

func (e *Engine) roundEvidence(ctx context.Context, state *ledger.State, request check.Request, round *ledger.Round) (string, error) {
	records, err := e.records()
	if err != nil {
		return "", err
	}
	var text strings.Builder
	units := state.UnitResults(round.ID)
	for _, unit := range units {
		if len(units) == 1 {
			// A one-unit round's report is the judged round report.
			continue
		}
		fmt.Fprintf(&text, "### Unit %s report\n\n%s\n\n", unit.UnitID, reportText(unit.Report, unit.Progress))
	}
	cited := citedRecords(request.Progress)
	for _, record := range records {
		switch p := record.Payload.(type) {
		case *ledger.Judgment:
			if p.RoundID == round.ID && p.Command != nil && p.TargetVersion == request.TargetVersion {
				fmt.Fprintf(&text, "### Verification command %s (%s)\n\nExit code: %d\n\n%s\n\n", p.Aspect, p.Runner, p.Command.ExitCode, p.Command.Output)
			}
		case *ledger.Evidence:
			if cited[record.ID] || record.ID > round.ID {
				fmt.Fprintf(&text, "### Cited evidence (%s)\n\n%s\n\n", record.ID, jsonBlock(p.Evidence))
			}
		}
	}
	if round.ReturnedTarget != nil {
		comparison, err := e.stageComparison(ctx, round)
		if err != nil {
			return "", err
		}
		text.WriteString("### Stage comparison\n\n" + comparison + "\n")
	}
	return text.String(), nil
}

func citedRecords(progress *form.Progress) map[string]bool {
	cited := map[string]bool{}
	if progress != nil && progress.Confirmation != nil {
		for _, reference := range progress.Confirmation.Evidence {
			cited[reference.RecordID] = reference.RecordID != ""
		}
	}
	return cited
}

func referenceList(references []form.Reference) string {
	items := []string{}
	for _, reference := range references {
		items = append(items, reference.RecordID+reference.External)
	}
	return strings.Join(items, ", ")
}

func jsonBlock(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return "```json\n" + string(data) + "\n```"
}
