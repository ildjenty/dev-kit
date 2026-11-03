package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
	"github.com/oklog/ulid/v2"
)

// AnswerRequest supplies the agent's interpretation of an observed human input.
// Observation is an optional prompts/ basename; when empty the latest input in
// this session is used. Supersedes identifies the answer being corrected.
type AnswerRequest struct {
	QuestionID     string
	Interpretation ledger.Interpretation
	Text           string
	Observation    string
	Supersedes     string
}

// Answer preserves the original input, source session, and observation time.
// It does not infer approval from the human's words.
func (e *Engine) Answer(ctx context.Context, request AnswerRequest) (Result, error) {
	_, warning, err := e.resolve(ctx, false)
	if err != nil {
		return Result{}, err
	}
	if warning != "" {
		return Result{}, errors.New(warning)
	}
	if err = e.checkSession(); err != nil {
		return Result{}, err
	}
	lock, err := e.store().Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	if _, _, paused := state.Paused(); paused && state.WorkStatus() == ledger.WorkActive {
		return Result{}, errors.New("work is paused; use devkit resume")
	}
	if err = e.importStops(state); err != nil {
		return Result{}, err
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	var question *form.Question
	for _, q := range state.Questions() {
		if q.ID == request.QuestionID {
			copy := q
			question = &copy
		}
	}
	if question == nil {
		return Result{}, fmt.Errorf("question %q was not found", request.QuestionID)
	}
	records, err := e.records()
	if err != nil {
		return Result{}, err
	}
	var issuedAt time.Time
	for _, r := range records {
		if q, ok := r.Payload.(*ledger.Question); ok {
			if q.Supersedes == question.ID {
				return Result{}, errors.New("the question has been superseded")
			}
			if q.ID == question.ID {
				id, err := ulid.ParseStrict(r.ID)
				if err != nil {
					return Result{}, err
				}
				issuedAt = ulid.Time(id.Time())
			}
		}
	}
	if question.ApprovalTarget.Kind == form.ApprovalPlan {
		latest := 0
		for _, v := range state.PlanEvaluations(nil) {
			if !(v.Withdrawn && v.Origin == plan.HumanProposal) {
				latest = max(latest, v.Plan.Version)
			}
		}
		if question.ApprovalTarget.Plan != latest {
			return Result{}, errors.New("the question refers to an obsolete plan version")
		}
	}

	if question.ApprovalTarget.Kind == form.ApprovalTree {
		actual, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, err
		}
		if actual != question.ApprovalTarget.Tree {
			replacement := *question
			replacement.ID = ulidString()
			replacement.ApprovalTarget.Tree = actual
			if round, ok := state.OpenRound(); ok && round.ReturnedTarget != nil && question.StopCategory != "detection_failed" {
				if err = e.replaceQuestionTarget(ctx, round, actual); err != nil {
					return Result{}, err
				}
			}
			if _, err = e.append(&ledger.Question{Question: replacement, Supersedes: question.ID}); err != nil {
				return Result{}, err
			}
			return Result{}, fmt.Errorf("approval target changed; answer the new question %s", replacement.ID)
		}
	}
	original, observedAt, err := e.prompt(request.Observation)
	if err != nil {
		return Result{}, err
	}
	if observedAt.Before(issuedAt) {
		return Result{}, errors.New("the observed human input predates the question")
	}
	answers := state.AnsweredQuestions()
	previous, answered := answers[question.ID]
	if answered {
		if request.Supersedes == "" {
			if previous.Answer.Original == original && previous.Answer.Interpretation == request.Interpretation && previous.Answer.Text == request.Text && reflect.DeepEqual(previous.Answer.Target, question.ApprovalTarget) {
				return response(state), nil
			}
			return Result{}, fmt.Errorf("answer correction must reference %s", previous.RecordID)
		}
		if request.Supersedes != previous.RecordID {
			return Result{}, errors.New("correction does not reference the effective answer")
		}
		if !observedAt.After(previous.Answer.ObservedAt) {
			return Result{}, errors.New("an answer correction requires a newer human input")
		}
	} else if request.Supersedes != "" {
		return Result{}, errors.New("there is no earlier answer to correct")
	}
	answer := ledger.Answer{QuestionID: question.ID, Target: question.ApprovalTarget, Interpretation: request.Interpretation, Text: request.Text, Original: original, SessionID: e.sessionID, ObservedAt: observedAt, Supersedes: request.Supersedes}
	for _, i := range state.UnacceptedInstructions() {
		if i.QuestionID == question.ID {
			answer.InstructionID = i.ID
		}
	}
	var id string
	if request.Interpretation == ledger.AnswerApproved {
		id, err = e.append(&ledger.Approval{Answer: answer})
	} else {
		id, err = e.append(&answer)
	}
	if err != nil {
		return Result{}, err
	}
	if question.ApprovalTarget.Kind == form.ApprovalDecision && request.Interpretation != ledger.AnswerText {
		status := ledger.DecisionDeferred
		switch request.Interpretation {
		case ledger.AnswerApproved:
			status = ledger.DecisionApproved
		case ledger.AnswerRejected:
			status = ledger.DecisionRejected
		}
		if _, err = e.append(&ledger.StateChange{DecisionID: question.ApprovalTarget.DecisionID, DecisionStatus: status, AnswerID: id}); err != nil {
			return Result{}, err
		}
	}
	if state.WorkStatus() != ledger.WorkActive {
		state, err = e.load()
		if err != nil {
			return Result{}, err
		}
		return response(state), nil
	}
	if request.Interpretation == ledger.AnswerDeferred {
		replacement := *question
		replacement.ID = ulidString()
		if _, err = e.append(&ledger.Question{Question: replacement, Supersedes: question.ID}); err != nil {
			return Result{}, err
		}
		state, err = e.load()
		if err != nil {
			return Result{}, err
		}
		return e.issueQuestion(state, replacement)
	}
	if question.ApprovalTarget.Kind == form.ApprovalPlan && question.StopCategory != "plan_approval" {
		// An assessment that could not complete is revised through the planner.
		if request.Interpretation == ledger.AnswerText {
			if err := e.initialize(); err != nil {
				return Result{}, err
			}
			n, err := e.highestPlan()
			if err != nil {
				return Result{}, err
			}
			p, ok, err := e.boundPlan(n)
			if err != nil {
				return Result{}, err
			}
			if !ok {
				return Result{}, errors.New("plan is missing")
			}
			head, err := e.workspace.Head(ctx)
			if err != nil {
				return Result{}, err
			}
			t, source, err := e.template(ctx, head, p.Template.Name)
			if err != nil {
				return Result{}, err
			}
			return e.planner(ctx, t, source, n+1, plan.InitialProposal)
		}
	}
	if run, ok := state.LatestStageRun(); ok && question.StoppedStage == run.StageID {
		if round, open := state.OpenRound(); open && run.State == ledger.StageApprovalWait && request.Interpretation != ledger.AnswerApproved {
			if err = e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: "The human requested changes: " + request.Text}); err != nil {
				return Result{}, err
			}
		}
	}
	state, err = e.load()
	if err != nil {
		return Result{}, err
	}
	return response(state), nil
}
func (e *Engine) prompt(name string) (string, time.Time, error) {
	if name != "" && !localName(name) {
		return "", time.Time{}, errors.New("prompt observation must be a basename")
	}
	entries, err := os.ReadDir(filepath.Join(e.workRoot(), "prompts"))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("human input observation is missing: %w", err)
	}
	prefix := e.sessionID + "-"
	selected := ""
	var at time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".txt") || name != "" && entry.Name() != name {
			continue
		}
		timestamp := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), ".txt")
		observed, err := time.Parse("20060102T150405.000000000Z", timestamp)
		if err != nil {
			return "", at, fmt.Errorf("prompt observation %s: %w", entry.Name(), err)
		}
		if selected == "" || observed.After(at) {
			selected = entry.Name()
			at = observed
		}
	}
	if selected == "" {
		return "", at, errors.New("no human input was observed for the current session")
	}
	data, err := os.ReadFile(filepath.Join(e.workRoot(), "prompts", selected))
	return string(data), at, err
}
