package ledger

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// State is a snapshot derived from an ordered record scan.
// Reload after appending records. Stop observations are supplied by the caller.
type State struct {
	records []Record
	stops   []StopObservation
}

// Load reads the work records and includes unincorporated stop questions in queries.
// The caller reads generated observations; fsstore remains responsible for record I/O.
func Load(store *fsstore.Store, stops []StopObservation, validator *Validator) (*State, error) {
	records, err := List(store, validator)
	if err != nil {
		return nil, err
	}
	s := &State{records: records, stops: slices.Clone(stops)}
	if work, ok := s.Work(); ok {
		for _, stop := range stops {
			if stop.WorkID != work.ID {
				return nil, fmt.Errorf("stop observation %s: work ID does not match %s", stop.ID, work.ID)
			}
		}
	}
	return s, nil
}

func (s *State) Work() (*Work, bool) {
	for _, record := range s.records {
		if work, ok := record.Payload.(*Work); ok {
			return work, true
		}
	}
	return nil, false
}

func (s *State) LastRecordID() string {
	if len(s.records) == 0 {
		return ""
	}
	return s.records[len(s.records)-1].ID
}

func (s *State) WorkStatus() WorkStatus {
	status := WorkActive
	for _, record := range s.records {
		if change, ok := record.Payload.(*StateChange); ok && change.WorkStatus != "" {
			status = change.WorkStatus
		}
	}
	return status
}

// Paused is independent of results and stage updates arriving during interruption.
// A resume clears only the pause record it references.
func (s *State) Paused() (*Pause, string, bool) {
	resumed := make(map[string]bool)
	for _, record := range s.records {
		if resume, ok := record.Payload.(*Resume); ok {
			resumed[resume.PauseID] = true
		}
	}
	for i := len(s.records) - 1; i >= 0; i-- {
		record := s.records[i]
		if pause, ok := record.Payload.(*Pause); ok {
			if resumed[record.ID] {
				return nil, "", false
			}
			return pause, record.ID, true
		}
	}
	return nil, "", false
}

// StageRuns returns the latest appended value of each run in start order.
func (s *State) StageRuns() []*StageRun {
	var runs []*StageRun
	indices := make(map[string]int)
	for _, record := range s.records {
		if run, ok := record.Payload.(*StageRun); ok {
			if index, exists := indices[run.ID]; exists {
				runs[index] = run
			} else {
				indices[run.ID] = len(runs)
				runs = append(runs, run)
			}
		}
	}
	return runs
}

func (s *State) IncompleteStageRuns() []*StageRun {
	return slices.DeleteFunc(s.StageRuns(), func(run *StageRun) bool { return run.State == StageCompleted })
}

func (s *State) LatestStageRun() (*StageRun, bool) {
	runs := s.StageRuns()
	if len(runs) == 0 {
		return nil, false
	}
	return runs[len(runs)-1], true
}

func (s *State) Rounds(stageRunID string) []*Round {
	var rounds []*Round
	indices := make(map[string]int)
	for _, record := range s.records {
		round, ok := record.Payload.(*Round)
		if !ok || round.StageRunID != stageRunID {
			continue
		}
		if index, exists := indices[round.ID]; exists {
			rounds[index] = round
		} else {
			indices[round.ID] = len(rounds)
			rounds = append(rounds, round)
		}
	}
	return rounds
}

// OpenRound remains open after progress is submitted, until the next action is recorded.
func (s *State) OpenRound() (*Round, bool) {
	run, ok := s.LatestStageRun()
	if !ok || run.State == StageCompleted || run.State == StageInvalidated || s.WorkStatus() != WorkActive {
		return nil, false
	}
	rounds := s.Rounds(run.ID)
	if len(rounds) == 0 || rounds[len(rounds)-1].Decision != nil {
		return nil, false
	}
	return rounds[len(rounds)-1], true
}

func (s *State) LatestProgress() (*Progress, bool) {
	for i := len(s.records) - 1; i >= 0; i-- {
		if progress, ok := s.records[i].Payload.(*Progress); ok {
			return progress, true
		}
	}
	return nil, false
}

// Questions includes stop questions that CLI has not yet incorporated, once per question ID.
// Superseded questions remain available here for historical answer lookup.
func (s *State) Questions() []form.Question {
	questions := []form.Question{}
	seen := make(map[string]bool)
	for _, record := range s.records {
		if question, ok := record.Payload.(*Question); ok && !seen[question.ID] {
			questions = append(questions, question.Question)
			seen[question.ID] = true
		}
	}
	stops := slices.Clone(s.stops)
	slices.SortFunc(stops, func(a, b StopObservation) int { return a.ObservedAt.Compare(b.ObservedAt) })
	for _, stop := range stops {
		if stop.LimitReached && stop.Question != nil && !seen[stop.Question.ID] {
			question := *stop.Question
			if question.StopObservation == nil {
				question.StopObservation = &form.Reference{External: "stops/" + stop.ID + ".json"}
			}
			questions = append(questions, question)
			seen[question.ID] = true
		}
	}
	return questions
}

type AcceptedAnswer struct {
	RecordID string `json:"record_id"`
	Answer   Answer `json:"answer"`
}

// AnsweredQuestions returns the effective answer per question, including corrections.
func (s *State) AnsweredQuestions() map[string]AcceptedAnswer {
	answers := make(map[string]AcceptedAnswer)
	for _, record := range s.records {
		var answer Answer
		switch p := record.Payload.(type) {
		case *Answer:
			answer = *p
		case *Approval:
			answer = p.Answer
		default:
			continue
		}
		answers[answer.QuestionID] = AcceptedAnswer{RecordID: record.ID, Answer: answer}
	}
	return answers
}

func (s *State) supersededQuestions() map[string]bool {
	superseded := make(map[string]bool)
	for _, record := range s.records {
		if question, ok := record.Payload.(*Question); ok && question.Supersedes != "" {
			superseded[question.Supersedes] = true
		}
	}
	return superseded
}

func (s *State) UnansweredQuestions() []form.Question {
	answers := s.AnsweredQuestions()
	superseded := s.supersededQuestions()
	questions := []form.Question{}
	for _, question := range s.Questions() {
		answer, answered := answers[question.ID]
		if !superseded[question.ID] && (!answered || !reflect.DeepEqual(answer.Answer.Target, question.ApprovalTarget)) {
			questions = append(questions, question)
		}
	}
	return questions
}

// ResolveAnswer finds the latest effective answer for the exact approval target.
// Rejection or deferral supersedes earlier approval of the same target.
func (s *State) ResolveAnswer(target form.ApprovalTarget) (AcceptedAnswer, bool) {
	answers := s.AnsweredQuestions()
	superseded := s.supersededQuestions()
	var latest AcceptedAnswer
	for _, question := range s.Questions() {
		answer, ok := answers[question.ID]
		if !ok || superseded[question.ID] || !reflect.DeepEqual(question.ApprovalTarget, target) || !reflect.DeepEqual(answer.Answer.Target, target) {
			continue
		}
		if latest.RecordID == "" || answer.RecordID > latest.RecordID {
			latest = answer
		}
	}
	return latest, latest.RecordID != ""
}

func (s *State) ResolveApproval(target form.ApprovalTarget) (AcceptedAnswer, bool) {
	answer, ok := s.ResolveAnswer(target)
	return answer, ok && answer.Answer.Interpretation == AnswerApproved
}

// ResolveExternalApproval also accepts an exact declaration in an approved plan.
// Supply the executable plan selected for the operation, or nil for individual approval only.
func (s *State) ResolveExternalApproval(target form.ExternalOperationTarget, executable *plan.Plan) (AcceptedAnswer, bool) {
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalExternal, External: &target}
	if answer, exists := s.ResolveAnswer(approvalTarget); exists {
		return answer, answer.Answer.Interpretation == AnswerApproved
	}
	if executable == nil || !executable.Declares(plan.ExternalOperation{
		Kind: target.Kind, Target: target.Target, InputHash: target.InputHash,
		DefinitionHash: target.DefinitionHash, ExecutableHash: target.ExecutableHash,
	}) {
		return AcceptedAnswer{}, false
	}
	return s.ResolveApproval(form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: executable.Version})
}

// LatestJudgments selects the latest result per aspect and runner for a target,
// then checks norm identity. Older results cannot hide a newer invalid result.
func (s *State) LatestJudgments(target form.TargetVersion, norms []form.NormReference) []check.Judgment {
	type key struct{ aspect, runner string }
	indices := make(map[key]int)
	latest := []check.Judgment{}
	for _, record := range s.records {
		judgment, ok := record.Payload.(*Judgment)
		if !ok || judgment.TargetVersion != target {
			continue
		}
		k := key{judgment.Aspect, judgment.Runner}
		if index, exists := indices[k]; exists {
			latest[index] = judgment.Judgment
		} else {
			indices[k] = len(latest)
			latest = append(latest, judgment.Judgment)
		}
	}
	return slices.DeleteFunc(latest, func(j check.Judgment) bool { return !j.ValidFor(target, norms) })
}

// UnresolvedBlocking follows explicit finding resolutions across rounds of a stage run.
func (s *State) UnresolvedBlocking(stageRunID string) []check.Finding {
	rounds := make(map[string]bool)
	for _, round := range s.Rounds(stageRunID) {
		rounds[round.ID] = true
	}
	findings := []check.Finding{}
	indices := make(map[string]int)
	resolved := make(map[string]bool)
	for _, record := range s.records {
		judgment, ok := record.Payload.(*Judgment)
		if !ok || !rounds[judgment.RoundID] {
			continue
		}
		for _, finding := range judgment.Findings {
			for _, id := range finding.ResolvedFindingIDs {
				resolved[id] = true
			}
			if index, exists := indices[finding.ID]; exists {
				findings[index] = finding
			} else {
				indices[finding.ID] = len(findings)
				findings = append(findings, finding)
			}
		}
	}
	return slices.DeleteFunc(findings, func(f check.Finding) bool { return !f.Blocking || resolved[f.ID] })
}

func (s *State) ExternalOperation(operationID string) (*ExternalOp, bool) {
	for i := len(s.records) - 1; i >= 0; i-- {
		if operation, ok := s.records[i].Payload.(*ExternalOp); ok && operation.OperationID == operationID {
			return operation, true
		}
	}
	return nil, false
}

// ExternalOperations returns the latest execution record per operation in first-seen order.
func (s *State) ExternalOperations() []*ExternalOp {
	operations := []*ExternalOp{}
	indices := make(map[string]int)
	for _, record := range s.records {
		if operation, ok := record.Payload.(*ExternalOp); ok {
			if index, exists := indices[operation.OperationID]; exists {
				operations[index] = operation
			} else {
				indices[operation.OperationID] = len(operations)
				operations = append(operations, operation)
			}
		}
	}
	return operations
}

func (s *State) PlanEvaluations(norms []form.NormReference) []plan.VersionEvaluation {
	versions := []plan.VersionEvaluation{}
	withdrawn := make(map[int]bool)
	for _, record := range s.records {
		if withdrawal, ok := record.Payload.(*Withdrawal); ok {
			withdrawn[withdrawal.PlanVersion] = true
		}
	}
	for _, record := range s.records {
		proposal, ok := record.Payload.(*PlanVersion)
		if !ok {
			continue
		}
		version := plan.VersionEvaluation{Plan: proposal.Plan, Origin: proposal.Origin, Withdrawn: withdrawn[proposal.Plan.Version]}
		judgments := s.LatestJudgments(form.TargetVersion{Kind: form.TargetPlan, Plan: proposal.Plan.Version}, norms)
		if len(judgments) != 0 {
			version.Judgment = plan.PlanJudgment(check.Aggregate(judgments).Judgment)
		}
		if answer, ok := s.ResolveAnswer(form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: proposal.Plan.Version}); ok {
			version.Approved = answer.Answer.Interpretation == AnswerApproved
			version.Rejected = answer.Answer.Interpretation == AnswerRejected || answer.Answer.Interpretation == AnswerText
		}
		versions = append(versions, version)
	}
	return versions
}

// ExecutablePlan applies plan's version selection only when opening a new stage run.
func (s *State) ExecutablePlan(template plan.PlanTemplate, norms []form.NormReference, policy string) (plan.Plan, bool) {
	replanAfter := 0
	for _, record := range s.records {
		if round, ok := record.Payload.(*Round); ok && round.Decision != nil && round.Decision.Action == plan.Back {
			for _, run := range s.StageRuns() {
				if run.ID == round.StageRunID {
					replanAfter = max(replanAfter, run.PlanVersion)
				}
			}
		}
	}
	for _, run := range s.StageRuns() {
		if run.State != StageCompleted {
			continue
		}
		for _, record := range s.records {
			if proposal, ok := record.Payload.(*PlanVersion); ok && proposal.Plan.Version == run.PlanVersion {
				next, _ := proposal.Plan.Next(run.StageID, plan.Done)
				if next != nil && *next == "replan" {
					replanAfter = max(replanAfter, run.PlanVersion)
				}
			}
		}
	}
	versions := s.PlanEvaluations(norms)
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i].Withdrawn && versions[i].Origin == plan.HumanProposal {
			continue
		}
		template = s.ApprovalTemplate(template, versions[i].Plan)
		break
	}
	return plan.ExecutableVersion(template, versions, replanAfter, policy)
}
