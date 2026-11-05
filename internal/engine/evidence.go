package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Labels name the comparison base a question shows the human.
const (
	stageStartBase   = "Comparison base (tree at the start of this stage run)"
	lastApprovedBase = "Comparison base (tree at the last approved loop edit)"
	workStartBase    = "Comparison base (commit the work started from)"
)

func (e *Engine) comparison(ctx context.Context, label, before, after string) (string, error) {
	base, err := e.expanded(ctx, before)
	if err != nil {
		return "", err
	}
	paths, err := e.workspace.ChangedFiles(ctx, gitReference(before), gitReference(after))
	if err != nil {
		return "", err
	}
	patch, err := e.workspace.Diff(ctx, gitReference(before), gitReference(after))
	if err != nil {
		return "", err
	}
	name := "brief/" + before + "-" + after + ".diff"
	if err := e.store().Publish(name, patch); err != nil {
		return "", err
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%s: %s\nChanged files:\n", label, base)
	for _, path := range paths {
		fmt.Fprintf(&text, "- %s\n", path)
	}
	fmt.Fprintf(&text, "Diff file: %s\n", filepath.Join(e.workRoot(), name))
	if len(patch) <= 3000 {
		fmt.Fprintf(&text, "\n```diff\n%s\n```\n", patch)
	}
	return text.String(), nil
}

func (e *Engine) loopEditText(ctx context.Context, label, before, after, reason string) (string, error) {
	paths, err := e.workspace.ChangedFiles(ctx, gitReference(before), gitReference(after))
	if err != nil {
		return "", err
	}
	hook, err := e.workspace.PreCommitPath(ctx)
	if err != nil {
		return "", err
	}
	selected := map[string]bool{}
	for _, ref := range []string{before, after} {
		settings, err := e.settings(ctx, ref)
		if err != nil {
			return "", err
		}
		for _, path := range paths {
			if path == hook || slices.Contains(differenceCategories(e.Root(), []string{path}, settings), config.MetaLoopEdit) {
				selected[path] = true
			}
		}
	}
	var text strings.Builder
	text.WriteString(reason + "\n\nChanged loop files:\n")
	for _, path := range paths {
		if selected[path] {
			fmt.Fprintf(&text, "- %s\n", path)
		}
	}
	sources, err := e.sourceChangeText(ctx, before, after)
	if err != nil {
		return "", err
	}
	text.WriteString(sources)
	comparison, err := e.comparison(ctx, label, before, after)
	if err != nil {
		return "", err
	}
	text.WriteString("\n" + comparison)
	return text.String(), nil
}

func (e *Engine) stageQuestionText(ctx context.Context, stage plan.Stage, round *ledger.Round, reason string) (string, error) {
	var progress form.Progress
	records, err := e.records()
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if value, ok := record.Payload.(*ledger.Progress); ok && value.RoundID == round.ID {
			progress = value.Progress
		}
	}
	var text strings.Builder
	fmt.Fprintf(&text, "%s\n\nGate reason: %s\n\n%sWork performed:\n", reason, stage.Gate.Reason, gateReportText(progress.Report))
	for _, item := range progress.WorkPerformed {
		fmt.Fprintf(&text, "- %s\n", item)
	}
	text.WriteString("\nConfirmed matters:\n")
	for _, item := range progress.Confirmed {
		fmt.Fprintf(&text, "- %s\n", item)
	}
	text.WriteString("\nUnconfirmed matters:\n")
	for _, item := range progress.Unconfirmed {
		fmt.Fprintf(&text, "- %s\n", item)
	}
	findings, err := e.decisionFindings(round)
	if err != nil {
		return "", err
	}
	text.WriteString(findings)
	comparison, err := e.stageComparison(ctx, round)
	if err != nil {
		return "", err
	}
	text.WriteString("\n" + comparison)
	return text.String(), nil
}

// gateReportText shows the worker's stage report so the human sees its decisions and observations first.
func gateReportText(report json.RawMessage) string {
	if len(report) == 0 {
		return ""
	}
	return "Stage report (worker-reported):\n" + reportText(report, nil) + "\n\n"
}

// decisionFindings renders the findings of the judged round that need a human decision.
func (e *Engine) decisionFindings(round *ledger.Round) (string, error) {
	if round.ReturnedTarget == nil {
		return "", nil
	}
	judgments, err := e.roundJudgments(round)
	if err != nil {
		return "", err
	}
	var text strings.Builder
	for _, judgment := range judgments {
		for _, finding := range judgment.Findings {
			if finding.Blocking || finding.Kind == form.NeedsDecision {
				fmt.Fprintf(&text, "- %s (%s, %s): %s\n", judgment.Aspect, judgment.Runner, finding.Kind, finding.Scenario)
			}
		}
	}
	if text.Len() == 0 {
		return "", nil
	}
	return "\nFindings that require this decision:\n" + text.String(), nil
}

func (e *Engine) refreshQuestionEvidence(ctx context.Context, state *ledger.State, q *form.Question) error {
	work, _ := state.Work()
	before, label := work.BaseCommit, workStartBase
	if q.StopCategory == config.MetaLoopEdit {
		latest := ""
		answers := state.AnsweredQuestions()
		for _, previous := range state.Questions() {
			if previous.StopCategory != config.MetaLoopEdit {
				continue
			}
			answer, ok := answers[previous.ID]
			effective, valid := state.ResolveAnswer(previous.ApprovalTarget)
			if ok && valid && effective.RecordID == answer.RecordID && answer.Answer.Interpretation == ledger.AnswerApproved && answer.RecordID > latest {
				before, latest, label = approvalReference(previous.ApprovalTarget), answer.RecordID, lastApprovedBase
			}
		}
	}
	var stageText string
	for _, run := range state.StageRuns() {
		for _, round := range state.Rounds(run.ID) {
			if !slices.Contains(q.Evidence, form.Reference{RecordID: round.ID}) {
				continue
			}
			bound, ok, err := e.boundPlan(run.PlanVersion)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("bound plan is missing")
			}
			stage, _ := bound.Stage(run.StageID)
			copy := *round
			target := form.TargetVersion{Kind: form.TargetTree, Tree: q.ApprovalTarget.Tree}
			copy.ReturnedTarget = &target
			stageText, err = e.stageQuestionText(ctx, stage, &copy, "Confirm the updated stage target.")
			if err != nil {
				return err
			}
			before, label = state.Rounds(run.ID)[0].StartTarget.Tree, stageStartBase
		}
	}
	if q.StopCategory == config.MetaLoopEdit {
		text, err := e.loopEditText(ctx, label, before, approvalReference(q.ApprovalTarget), "Approve the updated development loop edits? If rejected, the worker must undo the edits.\n\n"+stageText)
		if err != nil {
			return err
		}
		q.Text = text
	} else if q.StopCategory == "stage_confirmation" && stageText != "" {
		q.Text = stageText
	}
	q.StopReason = q.Text
	return nil
}

func (e *Engine) stageDiffBase(round *ledger.Round) (string, error) {
	state, err := e.load()
	if err != nil {
		return "", err
	}
	rounds := state.Rounds(round.StageRunID)
	if len(rounds) == 0 {
		return "", fmt.Errorf("stage run has no rounds")
	}
	return rounds[0].StartTarget.Tree, nil
}

func (e *Engine) stageComparison(ctx context.Context, round *ledger.Round) (string, error) {
	base, err := e.stageDiffBase(round)
	if err != nil {
		return "", err
	}
	return e.comparison(ctx, stageStartBase, base, round.ReturnedTarget.Tree)
}
