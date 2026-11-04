package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

// validateRoles checks role references against the fixed configuration.
func validateRoles(stages []plan.Stage, settings config.Config) error {
	roles := settings.RoleSettings()
	for _, stage := range stages {
		assignee, exists := roles[string(stage.Assignee)]
		if !exists || assignee.Kind != "work" && assignee.Kind != "human" {
			return fmt.Errorf("%s: assignee must name a work or human role", stage.ID)
		}
		if stage.Split && assignee.Kind != "work" {
			return fmt.Errorf("%s: split stages require a work assignee", stage.ID)
		}
		if err := validateCheckRoles(stage, settings); err != nil {
			return err
		}
		for _, unit := range stage.Units {
			role, exists := roles[unit.WorkRole()]
			if !exists || role.Kind != "work" || role.Agent == "" {
				return fmt.Errorf("%s: unit role %q must name a work role with an agent", stage.ID, unit.WorkRole())
			}
		}
		for _, rule := range stage.Transitions.FixRules {
			role, exists := roles[string(rule.Assignee)]
			if !exists || role.Kind != "work" {
				return fmt.Errorf("%s: fix assignee %q must name an eligible work role", stage.ID, rule.Assignee)
			}
		}
	}
	return nil
}

func (e *Engine) runnerAgent(ctx context.Context, ref, runner string) (string, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return "", err
	}
	if role, ok := settings.RoleSettings()[runner]; ok {
		return role.Agent, nil
	}
	if _, ok := settings.Judges[runner]; ok || strings.HasPrefix(runner, "command:") {
		return runner, nil
	}
	return "", fmt.Errorf("unknown runner role: %q", runner)
}

func roundAssignee(stage plan.Stage, round *ledger.Round) plan.Assignee {
	if round.Assignee != "" {
		return round.Assignee
	}
	return stage.Assignee
}

func (e *Engine) roundStage(ctx context.Context, stage plan.Stage, round *ledger.Round) (plan.Stage, error) {
	stage.Assignee = roundAssignee(stage, round)
	settings, err := e.settings(ctx, roundReference(round))
	if err != nil {
		return plan.Stage{}, err
	}
	role, ok := settings.RoleSettings()[string(stage.Assignee)]
	if !ok {
		return plan.Stage{}, fmt.Errorf("unknown assignee role: %q", stage.Assignee)
	}
	if role.Kind == "human" {
		stage.Assignee = plan.Human
	}
	if round.MergeFix && role.Kind == "work" && role.Agent == "" {
		stage.Units = nil
	}
	if role.Kind == "work" && role.Agent != "" && (!stage.Split || len(stage.Units) == 0 || round.MergeFix) {
		stage.Units = []plan.Unit{{ID: stage.ID, Description: "Perform the stage work.", Scope: "The agreed stage scope.", Role: string(stage.Assignee)}}
		stage.Split = true
	}
	return stage, nil
}

func (e *Engine) correctionDecision(state *ledger.State, stage plan.Stage, round *ledger.Round, decision router.Decision) (router.Decision, error) {
	if decision.Assignee != "" {
		return decision, nil
	}
	if decision.PreserveAssignee {
		decision.Assignee = roundAssignee(stage, round)
		return decision, nil
	}
	facts := plan.FixFacts{}
	for _, previous := range state.Rounds(round.StageRunID) {
		if previous.ReturnedTarget == nil {
			continue
		}
		judgments, err := e.roundJudgments(previous)
		if err != nil {
			return decision, err
		}
		failed := false
		for _, judgment := range judgments {
			if previous.ID == round.ID && judgment.Judgment != check.Pass {
				facts.Checks = append(facts.Checks, judgment.Aspect)
			}
			for _, item := range stage.RequiredChecks() {
				if item.Aspect == judgment.Aspect && string(item.Runner) == judgment.Runner && judgment.Judgment == check.Fail {
					failed = true
				}
			}
		}
		if failed {
			facts.Failures++
		}
	}
	for _, question := range state.Questions() {
		if question.StoppedStage != stage.ID || !slices.Contains(question.Evidence, form.Reference{RecordID: round.StageRunID}) {
			continue
		}
		if answer, ok := state.AnsweredQuestions()[question.ID]; ok && answer.Answer.Interpretation == ledger.AnswerRejected {
			facts.HumanRejected = true
		}
	}
	decision.Assignee, decision.FixRule = stage.FixAssignee(facts)
	return decision, nil
}

func roleContinuationOptions(q *form.Question, stage plan.Stage) {
	seen := map[plan.Assignee]bool{}
	for _, rule := range stage.Transitions.FixRules {
		if seen[rule.Assignee] {
			continue
		}
		seen[rule.Assignee] = true
		q.Options = append(q.Options, "continue:"+string(rule.Assignee))
	}
}

func roleDifferences(template plan.PlanTemplate, p plan.Plan, deviations []plan.Deviation) string {
	var text strings.Builder
	originals := append(slices.Clone(template.Stages), template.OptionalStages...)
	for _, deviation := range deviations {
		if deviation.Kind != plan.ChangedAssignee && deviation.Kind != plan.ChangedFixRules && deviation.Kind != plan.ChangedUnitRole {
			continue
		}
		after, _ := p.Stage(deviation.StageID)
		var before plan.Stage
		for _, stage := range originals {
			if stage.ID == deviation.StageID {
				before = stage
			}
		}
		var oldValue, newValue any
		switch deviation.Kind {
		case plan.ChangedAssignee:
			oldValue, newValue = before.Assignee, after.Assignee
		case plan.ChangedFixRules:
			oldValue, newValue = before.Transitions.FixRules, after.Transitions.FixRules
		case plan.ChangedUnitRole:
			oldValue, newValue = before.Units, after.Units
		}
		fmt.Fprintf(&text, "\n%s in %s:\n- Template: %s\n- Plan: %s\n", deviation.Kind, deviation.StageID, approvalJSON(oldValue, 1), approvalJSON(newValue, 1))
	}
	return text.String()
}

func validateCheckRoles(stage plan.Stage, settings config.Config) error {
	roles := settings.RoleSettings()
	for _, item := range stage.Checks {
		if item.Runner.IsCommand() {
			continue
		}
		if _, command := settings.Judges[string(item.Runner)]; command {
			continue
		}
		role, exists := roles[string(item.Runner)]
		if !exists || role.Kind != "judge" {
			return fmt.Errorf("%s: check runner %q must name a judge role or registered judgment command", stage.ID, item.Runner)
		}
		if string(stage.Assignee) == string(item.Runner) {
			return fmt.Errorf("%s: assignee cannot judge its own work", stage.ID)
		}
	}

	return nil
}
