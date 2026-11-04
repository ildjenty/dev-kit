package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

func normRefs(norms []check.Norm) []form.NormReference {
	refs := []form.NormReference{}
	for _, n := range norms {
		refs = append(refs, n.Reference)
	}
	return refs
}
func (e *Engine) judgmentSchema(ctx context.Context, ref, runner string) ([]byte, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	source, err := e.asset(ctx, ref, "schemas/judgment.json")
	if err != nil {
		return nil, err
	}
	if command, ok := settings.Judges[runner]; ok {
		source, err = e.workspace.ReadFile(ctx, ref, command.OutputSchema)
		if err != nil {
			return nil, err
		}
	}
	return e.bundledSchema(source)
}
func (e *Engine) outputSchema(ctx context.Context, ref, runner string) ([]byte, *form.Schema, error) {
	source, err := e.judgmentSchema(ctx, ref, runner)
	if err != nil {
		return nil, nil, err
	}
	resources := map[string][]byte{}
	for name := range e.schemas {
		frozen, err := e.asset(ctx, ref, "schemas/"+name+".json")
		if err != nil {
			return nil, nil, err
		}
		resources["urn:devkit:form:"+name] = frozen
		if name == "plan" || name == "plan-template" {
			resources["https://devkit.invalid/schemas/v1/"+name+".json"] = frozen
		}
	}
	schema, err := form.CompileSchemaWithResources(source, resources)
	return source, schema, err
}
func (e *Engine) judgmentInstruction(ctx context.Context, state *ledger.State, item plan.Check, kind form.InstructionKind, key, ref string, target form.TargetVersion, path string, round *ledger.Round, version int) (ledger.Instruction, instructionData, bool, error) {
	meta, fresh, err := e.issuance(kind, key, round, version, "")
	if err != nil {
		return meta, instructionData{}, false, err
	}
	if !fresh {
		previousID := meta.ID
		data, reusable, err := e.reuseJudgment(ctx, &meta, item, target, ref)
		if err != nil {
			return meta, instructionData{}, false, err
		}
		if reusable {
			return meta, data, false, nil
		}
		fresh = meta.ID != previousID
	}
	norms, _, err := e.normDocuments(ctx, ref)
	if err != nil {
		return meta, instructionData{}, false, err
	}
	productRef := ref
	if round != nil {
		productRef = target.Tree
	}
	if err := e.requestNorms(ctx, ref, norms, productRef); err != nil {
		return meta, instructionData{}, false, err
	}
	aspect := check.Aspect{ID: item.Aspect}
	criteria := []check.Criterion{}
	if !item.Runner.IsCommand() {
		if criteria, err = e.criteria(ctx, ref, item.Criteria); err != nil {
			return meta, instructionData{}, false, err
		}
	}
	schema, _, err := e.outputSchema(ctx, ref, string(item.Runner))
	if err != nil {
		return meta, instructionData{}, false, err
	}
	requestKind := check.PlanRequest
	var progress *form.Progress
	findings := []form.Finding{}
	if round != nil {
		if err := e.roundNorms(ctx, state, round.ID, version, norms); err != nil {
			return meta, instructionData{}, false, err
		}
		requestKind = check.RoundRequest
		records, err := e.records()
		if err != nil {
			return meta, instructionData{}, false, err
		}
		for _, r := range records {
			if p, ok := r.Payload.(*ledger.Progress); ok && p.RoundID == round.ID {
				copy := p.Progress
				progress = &copy
			}
		}
		findings = check.FindingsForAspect(item.Aspect, state.UnresolvedBlocking(round.StageRunID))
	}
	evidence, err := e.judgmentEvidence(ctx, &meta, state, round)
	if err != nil {
		return meta, instructionData{}, false, err
	}
	if len(norms) > 0 {
		norms[0].Content += "\n" + evidence
	}
	input := check.RequestInput{Kind: requestKind, InstructionID: meta.ID, Runner: string(item.Runner), Aspect: aspect, Criteria: criteria, TargetVersion: target, TargetPath: path, Norms: norms, OutputSchema: schema, CompletionConditionIDs: append([]string{}, item.CompletionConditionIDs...), Progress: progress, PreviousFindings: findings, Authorizations: []check.Authorization{}}
	input.Authorizations = judgmentAuthorizations(state, target, version)
	if requestKind == check.PlanRequest {
		if err := e.planRequestInputs(state, &input); err != nil {
			return meta, instructionData{}, false, err
		}
	}
	request, err := check.PrepareRequest(input)
	if err != nil {
		return meta, instructionData{}, false, err
	}
	data := instructionData{Reference: ref, Request: &request, Context: meta.Context, Instruction: form.Instruction{ID: meta.ID, Kind: kind, TargetPath: path, Runner: string(item.Runner), Text: "Launch the assigned judge with: devkit-instruction " + meta.ID}}
	return meta, data, fresh, nil
}
func (e *Engine) roundNorms(ctx context.Context, state *ledger.State, roundID string, version int, norms []check.Norm) error {
	bound, ok, err := e.boundPlan(version)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("bound plan is missing")
	}
	agreed, err := json.MarshalIndent(bound, "", "  ")
	if err != nil {
		return err
	}
	additional := "\n\n## Agreed plan\n\n" + string(agreed)
	for _, run := range state.StageRuns() {
		for _, round := range state.Rounds(run.ID) {
			if round.ID == roundID {
				definition, err := e.asset(ctx, roundReference(round), "stages/"+run.StageID+".md")
				if err != nil {
					return err
				}
				additional += "\n\n## Stage definition\n\n" + string(definition)
			}
		}
	}
	for index := range norms {
		if index == 0 {
			norms[index].Content += additional
		}
	}
	return nil
}

func (e *Engine) requestNorms(ctx context.Context, ref string, norms []check.Norm, productRef ...string) error {
	facts, err := e.repository(ctx, ref, productRef...)
	if err != nil {
		return err
	}
	docs := append([]brief.Document{facts.Rules, facts.Requirements}, facts.LayoutGuides...)
	if len(norms) > 0 {
		for _, doc := range docs {
			norms[0].Content += "\nReference: " + doc.Path + "\n\n" + doc.Content + "\n"
		}
	}
	return nil
}

func (e *Engine) hydrateRequest(ctx context.Context, data instructionData) error {
	if data.Request == nil || data.Request.Kind == check.DetectionRequest || len(data.Request.Norms) == 0 || data.Request.Norms[0].Content != "" {
		return nil
	}
	norms, _, err := e.normDocuments(ctx, data.Reference)
	if err != nil {
		return err
	}
	productRef := data.Reference
	if data.Request.Kind == check.RoundRequest {
		productRef = data.Request.TargetVersion.Tree
	}
	if err := e.requestNorms(ctx, data.Reference, norms, productRef); err != nil {
		return err
	}
	if data.Request.Kind == check.RoundRequest {
		if err := e.hydrateRoundNorms(ctx, data.Instruction.ID, norms); err != nil {
			return err
		}
	}
	records, err := e.records()
	if err != nil {
		return err
	}
	state, err := e.load()
	if err != nil {
		return err
	}
	for _, record := range records {
		meta, ok := record.Payload.(*ledger.Instruction)
		if !ok || meta.ID != data.Instruction.ID {
			continue
		}
		var round *ledger.Round
		for _, candidate := range state.Rounds(meta.StageRunID) {
			if candidate.ID == meta.RoundID {
				round = candidate
			}
		}
		evidence, err := e.judgmentEvidence(ctx, meta, state, round)
		if err != nil {
			return err
		}
		if len(norms) > 0 {
			norms[0].Content += "\n" + evidence
		}
		break
	}
	data.Request.Norms = norms
	return nil
}

func (e *Engine) publishJudgment(ctx context.Context, meta ledger.Instruction, data instructionData, fresh bool) (Result, error) {
	b, err := e.judgmentBrief(ctx, data)
	if err != nil {
		return Result{}, err
	}
	return e.publishInstruction(meta, fresh, data, b.Text)
}

func (e *Engine) judgmentBrief(ctx context.Context, data instructionData) (brief.Brief, error) {
	if err := e.hydrateRequest(ctx, data); err != nil {
		return brief.Brief{}, err
	}
	template, err := e.asset(ctx, data.Reference, "templates/brief/judgment.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	productRef := data.Reference
	if data.Request.Kind == check.RoundRequest {
		productRef = data.Request.TargetVersion.Tree
	}
	facts, err := e.repository(ctx, data.Reference, productRef)
	if err != nil {
		return brief.Brief{}, err
	}
	criteria, err := e.criteriaDocuments(ctx, data.Reference, data.Request.Criteria)
	if err != nil {
		return brief.Brief{}, err
	}
	paths, err := e.normPaths(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	content, err := json.MarshalIndent(data.Request, "", "  ")
	if err != nil {
		return brief.Brief{}, err
	}
	schema, err := e.judgmentSchema(ctx, data.Reference, data.Instruction.Runner)
	if err != nil {
		return brief.Brief{}, err
	}
	return brief.BuildJudgment(brief.JudgmentInput{RequestTemplate: string(template), InstructionID: data.Instruction.ID, Content: "```json\n" + string(content) + "\n```", Criteria: criteria, Repository: facts, Norms: []brief.Document{}, NormPaths: paths, OutputSchema: string(schema)})
}

func (e *Engine) planCheck(ctx context.Context, state *ledger.State, p plan.Plan) (Result, error) {
	ref, err := e.workspace.Head(ctx)
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	selected := settings.PlanCheckSettings()
	item := plan.Check{Aspect: selected.Aspect, Runner: plan.Runner(selected.Runner), Required: true, Criteria: planSelections(selected.Criteria)}
	if selected.Examples != nil {
		item.Examples = &plan.Selection{Source: selected.Examples.Source, Select: selected.Examples.Select}
	}
	if err := e.refreshSources(ctx, ref); err != nil {
		return Result{}, err
	}
	if r, pending, err := e.ensureDetection(ctx, state, []plan.Check{item}, ref); pending || err != nil {
		return r, err
	}
	// A plan check is tied to the proposal and the norms at issuance.
	key := fmt.Sprintf("%d:%s:%s", p.Version, item.Aspect, item.Runner)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return Result{}, err
	}
	name := fmt.Sprintf("brief/plan-%d.json", p.Version)
	if err = e.store().Publish(name, data); err != nil {
		return Result{}, err
	}
	meta, request, fresh, err := e.judgmentInstruction(ctx, state, item, form.InstructionPlanCheck, key, ref, form.TargetVersion{Kind: form.TargetPlan, Plan: p.Version}, filepath.Join(e.workRoot(), name), nil, p.Version)
	if err != nil {
		return Result{}, err
	}
	issued, err := e.publishJudgment(ctx, meta, request, fresh)
	if err != nil {
		return Result{}, err
	}
	if command, ok := settings.Judges[string(item.Runner)]; ok {
		tree, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, err
		}
		judgment, err := e.runJudge(ctx, request, command, "", tree)
		if err != nil {
			return Result{}, err
		}
		if _, err := e.append(&ledger.Judgment{Judgment: judgment}); err != nil {
			return Result{}, err
		}
		return Result{}, nil
	}
	return issued, nil
}
func (e *Engine) acceptPending(ctx context.Context, state *ledger.State) (bool, error) {
	if changed, err := e.acceptAmbiguous(state); changed || err != nil {
		return changed, err
	}
	if changed, err := e.reissueRejected(ctx, state); changed || err != nil {
		return changed, err
	}
	observations, err := e.pendingObservations(state)
	if err != nil || observations == nil {
		return false, err
	}
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.Kind != form.InstructionPlan && instruction.Kind != form.InstructionJudgment && instruction.Kind != form.InstructionPlanCheck && instruction.Kind != form.InstructionDetectionTest && instruction.Kind != form.InstructionUnitWork {
			continue
		}
		request, err := e.readInstruction(instruction.ID)
		if err != nil {
			return false, err
		}
		current, blocked, err := e.pendingRequestCurrent(ctx, state, instruction, request)
		if err != nil || blocked {
			return false, err
		}
		if !current {
			continue
		}
		for _, obs := range observations {
			if obs.result.InstructionID != instruction.ID {
				continue
			}
			accepted, err := e.acceptObservedResult(ctx, state, instruction, request, obs)
			if accepted || err != nil {
				return accepted, err
			}
		}
	}
	return false, nil
}
func (e *Engine) judgeRound(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, error) {
	if run.State == ledger.StageAnswerWait || run.State == ledger.StageApprovalWait || run.State == ledger.StageExternalWait {
		update := *run
		update.State = ledger.StageJudgmentWait
		update.NextActor = "judge"
		if _, err := e.append(&update); err != nil {
			return Result{}, err
		}
	}
	if roundReference(round) != round.StartTarget.Tree {
		ref, question, err := e.confirmRoundNorms(ctx, state, run, round.ReturnedTarget.Tree)
		if err != nil {
			return Result{}, err
		}
		if question != nil {
			return e.issueQuestion(state, *question)
		}
		if ref != round.ReturnedTarget.Tree {
			return e.fixRound(ctx, state, p, stage, run, round, "Undo the rejected development loop edits listed in the previous human question.")
		}
	}
	target := *round.ReturnedTarget
	path, err := e.expanded(ctx, target.Tree)
	if err != nil {
		return Result{}, err
	}
	items := slices.Clone(stage.Checks)
	slices.SortStableFunc(items, func(a, b plan.Check) int {
		if a.Runner.IsCommand() && !b.Runner.IsCommand() {
			return -1
		}
		if !a.Runner.IsCommand() && b.Runner.IsCommand() {
			return 1
		}
		return 0
	})
	for _, item := range items {
		state, err = e.load()
		if err != nil {
			return Result{}, err
		}
		r, ready, err := e.judgeRoundCheck(ctx, state, p, stage, run, round, item, path)
		if err != nil || ready {
			return r, err
		}
	}
	return e.decideRound(ctx, p, stage, run, round)
}

func (e *Engine) progress(state *ledger.State, runID string, current *ledger.Round, summary check.Summary) (bool, int, error) {
	rounds, err := e.roundWindow(state, runID, current)
	if err != nil {
		return false, 0, err
	}
	previous := []check.Judgment{}
	noProgress := 0
	for _, round := range rounds {
		if round.ID == current.ID {
			break
		}
		if round.ReturnedTarget != nil {
			js, err := e.roundJudgments(round)
			if err != nil {
				return false, 0, err
			}
			before := check.Aggregate(previous)
			after := check.Aggregate(js)
			if summaryProgress(before, after) {
				noProgress = 0
			} else {
				noProgress++
			}
			previous = js
		}
	}
	return summaryProgress(check.Aggregate(previous), summary), noProgress, nil
}
func summaryProgress(before, after check.Summary) bool {
	for _, id := range after.CompletionConditionIDs {
		if !slices.Contains(before.CompletionConditionIDs, id) {
			return true
		}
	}
	for _, f := range after.Findings {
		for _, resolved := range f.ResolvedFindingIDs {
			if slices.ContainsFunc(before.Findings, func(old check.Finding) bool { return old.ID == resolved }) {
				return true
			}
		}
	}
	return false
}
func (e *Engine) runCommand(ctx context.Context, command config.Command, stage string, input []byte) (check.CommandResult, []byte, error) {
	if err := e.active(); err != nil {
		return check.CommandResult{}, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, command.Timeout())
	defer cancel()
	executable := command.Executable
	if strings.ContainsRune(executable, filepath.Separator) && !filepath.IsAbs(executable) {
		executable = filepath.Join(e.Root(), executable)
	}
	cmd := exec.CommandContext(ctx, executable, command.Args...)
	cmd.Dir = e.Root()
	cmd.Env = append(os.Environ(), "DEVKIT_STAGE="+stage, "DEVKIT_WORK="+e.workID)
	cmd.Stdin = bytes.NewReader(input)
	cmd.WaitDelay = command.Timeout()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := check.CommandResult{ExitCode: 0, Output: stdout.String() + stderr.String(), Method: command.Executable, Environment: e.Root()}
	if err != nil {
		result.ExitCode = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			result.ExitCode = exit.ExitCode()
		} else if ctx.Err() == nil {
			return result, stdout.Bytes(), err
		}
	}
	result.TimedOut = ctx.Err() != nil
	return result, stdout.Bytes(), nil
}
func (e *Engine) matches(ctx context.Context, tree string) (bool, error) {
	actual, err := e.workspace.SnapshotTree(ctx)
	return actual == tree, err
}
func changedCommand(request check.Request, output string, result *check.CommandResult) check.Judgment {
	return check.Judgment{JudgmentResult: form.JudgmentResult{InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion, Norms: normRefs(request.Norms), CompletionConditionIDs: []string{}, Judgment: check.Fail, Findings: []form.Finding{{ID: check.FindingID(request.Aspect.ID, request.InstructionID+":target-changed"), Blocking: true, Kind: form.AutoFixable, Scenario: "The command changed its target, or the working tree did not match before execution. Configure generated command outputs to be ignored.\n" + output}}, CheckedScope: []string{}, UncheckedScope: []string{request.TargetPath}, Method: "Working tree comparison before and after command execution", Environment: "work checkout"}, Runner: request.Runner, Provenance: check.Provenance{Source: check.JudgeCommand, InstructionID: request.InstructionID, Runner: request.Runner}, Command: result}
}
func (e *Engine) runCheck(ctx context.Context, request check.Request, command config.Command, stage, tree string) (check.Judgment, error) {
	match, err := e.matches(ctx, tree)
	if err != nil {
		return check.Judgment{}, err
	}
	if !match {
		return changedCommand(request, "The target differed before command execution.", nil), nil
	}
	result, _, err := e.runCommand(ctx, command, stage, nil)
	if err != nil {
		return check.Judgment{}, err
	}
	match, err = e.matches(ctx, tree)
	if err != nil {
		return check.Judgment{}, err
	}
	if !match {
		return changedCommand(request, result.Output, &result), nil
	}
	return check.FromCommand(request, result)
}
func (e *Engine) runJudge(ctx context.Context, data instructionData, command config.Judge, stage, tree string) (check.Judgment, error) {
	request := *data.Request
	match, err := e.matches(ctx, tree)
	if err != nil {
		return check.Judgment{}, err
	}
	if !match {
		return changedCommand(request, "The target differed before command execution.", nil), nil
	}
	input, err := json.Marshal(request)
	if err != nil {
		return check.Judgment{}, err
	}
	result, stdout, err := e.runCommand(ctx, command.Command, stage, input)
	if err != nil {
		return check.Judgment{}, err
	}
	match, err = e.matches(ctx, tree)
	if err != nil {
		return check.Judgment{}, err
	}
	if !match {
		return changedCommand(request, result.Output, &result), nil
	}
	if result.ExitCode != 0 || result.TimedOut {
		return unassessed(request, result.Output), nil
	}
	_, schema, err := e.outputSchema(ctx, data.Reference, request.Runner)
	if err != nil {
		return check.Judgment{}, err
	}
	judgment, _, err := check.AcceptResult(request, stdout, schema, check.Provenance{Source: check.JudgeCommand, InstructionID: request.InstructionID, Runner: request.Runner}, nil)
	return judgment, err
}
func unassessed(request check.Request, reason string) check.Judgment {
	return check.Judgment{JudgmentResult: form.JudgmentResult{InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion, Norms: normRefs(request.Norms), CompletionConditionIDs: []string{}, Judgment: check.NotAssessed, Findings: []form.Finding{}, CheckedScope: []string{}, UncheckedScope: []string{request.TargetPath}, Method: "Judgment command did not return an assessment: " + reason, Environment: "work checkout"}, Runner: request.Runner, Provenance: check.Provenance{Source: check.JudgeCommand, InstructionID: request.InstructionID, Runner: request.Runner}}
}

func (e *Engine) roundWindow(state *ledger.State, runID string, current *ledger.Round) ([]*ledger.Round, error) {
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	stage := ""
	for _, run := range state.StageRuns() {
		if run.ID == runID {
			stage = run.StageID
		}
	}
	questions := map[string]form.Question{}
	for _, q := range state.Questions() {
		questions[q.ID] = q
	}
	rounds := []*ledger.Round{}
	indices := map[string]int{}
	for _, record := range records {
		questionID := ""
		switch p := record.Payload.(type) {
		case *ledger.Answer:
			questionID = p.QuestionID
		case *ledger.Approval:
			questionID = p.QuestionID
		}
		if questionID != "" {
			q := questions[questionID]
			if q.StoppedStage == stage || q.ApprovalTarget.Kind == form.ApprovalWork {
				rounds = nil
				indices = map[string]int{}
			}
		}
		if round, ok := record.Payload.(*ledger.Round); ok && round.StageRunID == runID {
			if i, ok := indices[round.ID]; ok {
				rounds[i] = round
			} else {
				indices[round.ID] = len(rounds)
				rounds = append(rounds, round)
			}
		}
	}
	if _, ok := indices[current.ID]; !ok {
		rounds = append(rounds, current)
	}
	return rounds, nil
}
func (e *Engine) fixRound(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, reason string) (Result, error) {
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	judgments, err := e.roundJudgments(round)
	if err != nil {
		return Result{}, err
	}
	summary := check.Aggregate(judgments)
	made, noProgress, err := e.progress(state, run.ID, round, summary)
	if err != nil {
		return Result{}, err
	}
	window, err := e.roundWindow(state, run.ID, round)
	if err != nil {
		return Result{}, err
	}
	if made {
		noProgress = 0
	} else {
		noProgress++
	}
	if stage.Limits.Reached(len(window), noProgress) {
		tree, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, err
		}
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: tree}, "round_limit", reason+" The round or consecutive no-progress limit was reached.", stage.ID)
		next, err := p.Next(stage.ID, plan.Ask)
		if err != nil {
			return Result{}, err
		}
		q.NextStage = *next
		q, err = e.saveQuestion(state, q, "")
		if err != nil {
			return Result{}, err
		}
		if err = e.closeRound(state, run, round, router.Decision{Action: plan.Ask, Reason: q.Text}); err != nil {
			return Result{}, err
		}
		return e.issueQuestion(state, q)
	}
	err = e.closeRound(state, run, round, router.Decision{Action: plan.Fix, Reason: reason})
	return Result{}, err
}

func gateApproved(state *ledger.State, run *ledger.StageRun, round *ledger.Round, target form.ApprovalTarget) bool {
	var gate *form.Question
	for _, q := range state.Questions() {
		if q.StopCategory != "stage_confirmation" && q.StopCategory != config.MetaLoopEdit || q.StoppedStage != run.StageID {
			continue
		}
		if slices.Contains(q.Evidence, form.Reference{RecordID: run.ID}) && slices.Contains(q.Evidence, form.Reference{RecordID: round.ID}) {
			copy := q
			gate = &copy
		}
	}
	if gate == nil || gate.ApprovalTarget != target {
		return false
	}
	answer, ok := state.AnsweredQuestions()[gate.ID]
	return ok && answer.Answer.Interpretation == ledger.AnswerApproved && answer.Answer.Target == target
}

func commandTargetChanged(judgment check.Judgment) bool {
	return slices.ContainsFunc(judgment.Findings, func(f form.Finding) bool {
		return f.ID == check.FindingID(judgment.Aspect, judgment.InstructionID+":target-changed")
	})
}

func (e *Engine) roundJudgments(round *ledger.Round) ([]check.Judgment, error) {
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	judgments := []check.Judgment{}
	for _, record := range records {
		j, ok := record.Payload.(*ledger.Judgment)
		if !ok || j.RoundID != round.ID || j.TargetVersion != *round.ReturnedTarget {
			continue
		}
		index := slices.IndexFunc(judgments, func(old check.Judgment) bool { return old.Aspect == j.Aspect && old.Runner == j.Runner })
		if index < 0 {
			judgments = append(judgments, j.Judgment)
		} else {
			judgments[index] = j.Judgment
		}
	}
	return slices.DeleteFunc(judgments, func(j check.Judgment) bool { return !j.ValidFor(*round.ReturnedTarget, round.Norms) }), nil
}

func humanStageAnswer(state *ledger.State, run *ledger.StageRun, round *ledger.Round) (ledger.Answer, bool) {
	var question *form.Question
	for _, q := range state.Questions() {
		if q.StopCategory == "human_stage" && q.StoppedStage == run.StageID && slices.Contains(q.Evidence, form.Reference{RecordID: run.ID}) && slices.Contains(q.Evidence, form.Reference{RecordID: round.ID}) {
			copy := q
			question = &copy
		}
	}
	if question == nil {
		return ledger.Answer{}, false
	}
	answer, ok := state.AnsweredQuestions()[question.ID]
	return answer.Answer, ok && answer.Answer.Target == question.ApprovalTarget && answer.Answer.Interpretation != ledger.AnswerDeferred
}

func (e *Engine) reuseJudgment(ctx context.Context, meta *ledger.Instruction, item plan.Check, target form.TargetVersion, ref string) (instructionData, bool, error) {
	data, err := e.readInstruction(meta.ID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return instructionData{}, false, err
	}
	if err == nil && data.Request != nil {
		norms, err := e.norms(ctx, ref)
		if err != nil {
			return instructionData{}, false, err
		}
		previous := check.Judgment{JudgmentResult: form.JudgmentResult{TargetVersion: data.Request.TargetVersion, Norms: normRefs(data.Request.Norms)}}
		if data.Request.Runner == string(item.Runner) && previous.ValidFor(target, norms) {
			return data, true, nil
		}
		meta.Reissue++
		meta.ID = instructionID(e.workID, meta.Kind, meta.TargetKey, meta.Reissue)
	}
	return instructionData{}, false, nil
}

func judgmentAuthorizations(state *ledger.State, target form.TargetVersion, version int) []check.Authorization {
	authorizations := []check.Authorization{}
	answers := state.AnsweredQuestions()
	for _, q := range state.Questions() {
		answer, ok := answers[q.ID]
		if !ok || answer.Answer.Interpretation != ledger.AnswerApproved && answer.Answer.Interpretation != ledger.AnswerText {
			continue
		}
		effective, ok := state.ResolveAnswer(q.ApprovalTarget)
		if !ok || effective.RecordID != answer.RecordID {
			continue
		}
		applicable := q.ApprovalTarget.Kind == form.ApprovalTree && q.ApprovalTarget.Tree == target.Tree || q.ApprovalTarget.Kind == form.ApprovalPlan && q.ApprovalTarget.Plan == version || q.ApprovalTarget.Kind == form.ApprovalDecision || q.ApprovalTarget.Kind == form.ApprovalWork
		if applicable {
			authorizations = append(authorizations, check.Authorization{Target: q.ApprovalTarget, Scope: q.Text + "\n" + answer.Answer.Text + "\nOriginal human input: " + answer.Answer.Original, Evidence: []form.Reference{{RecordID: answer.RecordID}}})
		}
	}
	return authorizations
}

func (e *Engine) hydrateRoundNorms(ctx context.Context, instructionID string, norms []check.Norm) error {
	records, err := e.records()
	if err != nil {
		return err
	}
	for _, record := range records {
		if meta, ok := record.Payload.(*ledger.Instruction); ok && meta.ID == instructionID {
			state, err := e.load()
			if err != nil {
				return err
			}
			if err := e.roundNorms(ctx, state, meta.RoundID, meta.PlanVersion, norms); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

type observedResult struct {
	result hook.SubagentResult
	path   string
}

func (e *Engine) pendingObservations(state *ledger.State) ([]observedResult, error) {
	entries, err := os.ReadDir(filepath.Join(e.workRoot(), "results"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	observations := []observedResult{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		relevant := false
		for _, instruction := range state.UnacceptedInstructions() {
			if strings.HasPrefix(entry.Name(), instruction.ID+"-") {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		name := filepath.Join(e.workRoot(), "results", entry.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var result hook.SubagentResult
		if err = json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("result %s: %w", name, err)
		}
		observations = append(observations, observedResult{result, name})
	}
	slices.SortStableFunc(observations, func(a, b observedResult) int { return a.result.ObservedAt.Compare(b.result.ObservedAt) })
	return observations, nil
}

func (e *Engine) pendingRequestCurrent(ctx context.Context, state *ledger.State, instruction ledger.Instruction, request instructionData) (bool, bool, error) {
	if instruction.Kind != form.InstructionPlanCheck && instruction.Kind != form.InstructionDetectionTest {
		return true, false, nil
	}
	ref, err := e.workspace.Head(ctx)
	if err != nil {
		return false, false, err
	}
	if instruction.Kind == form.InstructionDetectionTest && instruction.StageRunID != "" {
		tree, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return false, false, err
		}
		run, ok := state.LatestStageRun()
		if !ok || run.ID != instruction.StageRunID {
			return false, false, nil
		}
		confirmed, question, err := e.confirmRoundNorms(ctx, state, run, tree)
		if err != nil {
			return false, false, err
		}
		if question != nil {
			return false, true, nil
		}
		ref = confirmed
	}
	if request.Detection != nil {
		current, _, err := e.detectionInput(ctx, ref, request.Detection.Check)
		if err != nil {
			return false, false, err
		}
		if current.Identity != request.Detection.Identity {
			return false, false, nil
		}
	} else {
		norms, err := e.norms(ctx, ref)
		if err != nil {
			return false, false, err
		}
		previous := check.Judgment{JudgmentResult: form.JudgmentResult{TargetVersion: request.Request.TargetVersion, Norms: normRefs(request.Request.Norms)}}
		if !previous.ValidFor(request.Request.TargetVersion, norms) {
			return false, false, nil
		}
	}
	return true, false, nil
}

func (e *Engine) acceptObservedResult(ctx context.Context, state *ledger.State, instruction ledger.Instruction, request instructionData, obs observedResult) (bool, error) {
	result := obs.result
	if result.Failure != "" {
		return e.rejectResult(instruction, obs, errors.New(result.Failure))
	}
	if result.AgentID == "" || result.Runner != request.Instruction.Runner {
		return e.rejectResult(instruction, obs, errors.New("runner or agent identity does not match"))
	}
	if instruction.Kind == form.InstructionUnitWork {
		return e.acceptUnitResult(instruction, obs)
	}
	provenance := check.Provenance{Source: check.SubagentResult, InstructionID: instruction.ID, Runner: result.Runner, AgentID: result.AgentID, Effort: result.Effort, Model: result.Model, ResultPath: obs.path}
	if instruction.Kind == form.InstructionPlan {
		return e.acceptPlannerResult(ctx, state, instruction, request, obs)
	}
	if request.Detection != nil {
		test, err := e.validateDetection(instruction, request, result.Content, provenance)
		if err != nil {
			return e.rejectResult(instruction, obs, err)
		}
		return true, e.recordDetection(ctx, state, instruction, request, test)
	}
	if request.Request == nil {
		return false, errors.New("judgment request is missing")
	}
	_, schema, err := e.outputSchema(ctx, request.Reference, result.Runner)
	if err != nil {
		return false, err
	}
	judgment, accepted, err := check.AcceptResult(*request.Request, result.Content, schema, provenance, state.LatestJudgments(request.Request.TargetVersion, normRefs(request.Request.Norms)))
	if err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	if accepted {
		_, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: instruction.RoundID})
		return true, err
	}
	return false, nil
}

func (e *Engine) acceptPlannerResult(ctx context.Context, state *ledger.State, instruction ledger.Instruction, request instructionData, obs observedResult) (bool, error) {
	p, err := e.parsePlannerResult(instruction, obs.result.Content)
	if err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	source, err := e.asset(ctx, request.Reference, "plans/"+request.TemplateName+".json")
	if err != nil {
		return false, err
	}
	if p.Template.Name != request.TemplateName || p.Template.ContentHash != plan.ContentHash(source) {
		return e.rejectResult(instruction, obs, errors.New("planner template does not match its instruction"))
	}
	if template, err := e.plans.ParseTemplate(source); err != nil {
		return false, err
	} else if err := plan.ValidateBackTargets(template, p); err != nil {
		return e.rejectResult(instruction, obs, err)
	}
	return true, e.acceptPlan(ctx, state, p, request.Origin, instruction.ID)
}

func (e *Engine) parsePlannerResult(instruction ledger.Instruction, content []byte) (plan.Plan, error) {
	var output form.PlannerResult
	if err := e.forms["planner-result"].Decode(content, &output); err != nil {
		return plan.Plan{}, err
	}
	if output.InstructionID != instruction.ID {
		return plan.Plan{}, errors.New("planner instruction ID does not match")
	}
	p, err := e.plans.ParsePlan(output.Plan)
	if err != nil {
		return plan.Plan{}, err
	}
	if p.Version != instruction.PlanVersion {
		return plan.Plan{}, errors.New("planner version does not match the requested version")
	}
	return p, nil
}

func (e *Engine) judgeRoundCheck(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, item plan.Check, path string) (Result, bool, error) {
	target := *round.ReturnedTarget
	judgments, err := e.roundJudgments(round)
	if err != nil {
		return Result{}, false, err
	}
	var existing *check.Judgment
	for _, j := range judgments {
		if j.Aspect == item.Aspect && j.Runner == string(item.Runner) {
			copy := j
			existing = &copy
		}
	}
	if existing != nil {
		if commandTargetChanged(*existing) {
			r, err := e.fixRound(ctx, state, p, stage, run, round, "A command changed its target.")
			return r, true, err
		}
		if item.Required && item.Runner.IsCommand() && existing.Judgment == check.Fail {
			r, err := e.fixRound(ctx, state, p, stage, run, round, "A required command check failed.")
			return r, true, err
		}
		return Result{}, false, nil
	}
	key := round.ID + ":" + item.Aspect
	meta, request, fresh, err := e.judgmentInstruction(ctx, state, item, form.InstructionJudgment, key, roundReference(round), target, path, round, p.Version)
	if err != nil {
		return Result{}, false, err
	}
	issued, err := e.publishJudgment(ctx, meta, request, fresh)
	if err != nil {
		return Result{}, false, err
	}
	settings, err := e.settings(ctx, roundReference(round))
	if err != nil {
		return Result{}, false, err
	}
	if item.Runner.IsCommand() {
		return e.judgeCommandCheck(ctx, state, p, stage, run, round, item, request, settings)
	}
	if command, ok := settings.Judges[string(item.Runner)]; ok {
		judgment, err := e.runJudge(ctx, request, command, stage.ID, target.Tree)
		if err != nil {
			return Result{}, false, err
		}
		if _, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: round.ID}); err != nil {
			return Result{}, false, err
		}
		if commandTargetChanged(judgment) {
			r, err := e.fixRound(ctx, state, p, stage, run, round, "A judgment command changed its target.")
			return r, true, err
		}
		return Result{}, false, nil
	}
	if !strings.HasPrefix(string(item.Runner), "devkit-") {
		return Result{}, false, fmt.Errorf("judge %q is neither a registered command nor a devkit subagent", item.Runner)
	}
	return issued, true, nil
}

func (e *Engine) judgeCommandCheck(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, item plan.Check, request instructionData, settings config.Config) (Result, bool, error) {
	target := *round.ReturnedTarget
	var err error
	command, ok := settings.Commands[strings.TrimPrefix(string(item.Runner), "command:")]
	var judgment check.Judgment
	if !ok {
		judgment, err = check.UnregisteredCommand(*request.Request)
	} else {
		judgment, err = e.runCheck(ctx, *request.Request, command, stage.ID, target.Tree)
	}
	if err != nil {
		return Result{}, false, err
	}
	if _, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: round.ID}); err != nil {
		return Result{}, false, err
	}
	if commandTargetChanged(judgment) {
		r, err := e.fixRound(ctx, state, p, stage, run, round, "A command check changed its target.")
		return r, true, err
	}
	if judgment.Judgment == check.Fail && item.Required {
		result, err := e.fixRound(ctx, state, p, stage, run, round, "A required command check failed.")
		if !ok && err == nil && result.Instruction == nil {
			result = response(state)
			result.Message = judgment.Findings[0].Scenario + "\nCorrect this finding, then run devkit next to open the fix round."
		}
		return result, true, err
	}
	return Result{}, false, nil
}

func (e *Engine) askRoundApproval(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, decision router.Decision, table router.Table, categories []string, summary check.Summary, completion router.Completion, termination form.TerminationKind) (Result, error) {
	target := *round.ReturnedTarget
	approvalTarget := approvalTree(target)
	category := "stage_confirmation"
	if table.ClassifyConfirmation(router.ConfirmationInput{Categories: []string{config.MetaLoopEdit}}).Required && slices.Contains(categories, config.MetaLoopEdit) {
		category = config.MetaLoopEdit
	}
	text, err := e.stageQuestionText(ctx, stage, round, decision.Reason)
	if err != nil {
		return Result{}, err
	}
	if category == config.MetaLoopEdit {
		base, baseErr := e.stageDiffBase(round)
		if baseErr != nil {
			return Result{}, baseErr
		}
		text, err = e.loopEditText(ctx, base, target.Tree, "Approve development loop edits before committing this stage? If rejected, the worker must undo the edits.\n\n"+text)
		if err != nil {
			return Result{}, err
		}
	}
	q := e.question(approvalTarget, category, text, stage.ID)
	q.Evidence = []form.Reference{{RecordID: run.ID}, {RecordID: round.ID}}
	next, err := p.Next(stage.ID, plan.Ask)
	if err != nil {
		return Result{}, err
	}
	q.NextStage = *next
	q, err = e.saveQuestion(state, q, "")
	if err != nil {
		return Result{}, err
	}
	// Keep a gate's evaluated round open so approval can complete the exact target.
	if summary.Judgment == check.Pass && completion.ReadyToCommit && termination == form.TerminationCompleted {
		update := *run
		update.State = ledger.StageApprovalWait
		update.NextActor = "human"
		if _, err = e.append(&update); err != nil {
			return Result{}, err
		}
	} else if err = e.closeRound(state, run, round, decision); err != nil {
		return Result{}, err
	}
	return e.issueQuestion(state, q)
}

func (e *Engine) roundConfirmationCategories(ctx context.Context, state *ledger.State, round *ledger.Round, settings config.Config) ([]string, error) {
	target := *round.ReturnedTarget
	base, err := e.stageDiffBase(round)
	if err != nil {
		return nil, err
	}
	paths, err := e.workspace.ChangedFiles(ctx, base, target.Tree)
	if err != nil {
		return nil, err
	}
	categories := differenceCategories(e.Root(), paths, settings)
	categories = slices.DeleteFunc(categories, func(category string) bool { return category == config.MetaLoopEdit })
	if edits, err := e.sourceChanges(ctx, roundReference(round), true); err != nil {
		return nil, err
	} else if len(edits) > 0 {
		categories = append(categories, config.MetaLoopEdit)
	}
	// Loop edits a human already approved before this round need no second confirmation.
	work, _ := state.Work()
	if confirmed, _ := confirmedLoopEdits(state); confirmed != work.BaseCommit {
		base = confirmed
	}
	if changed, err := e.metaLoopChanged(ctx, base, target.Tree); err != nil {
		return nil, err
	} else if changed && !slices.Contains(categories, config.MetaLoopEdit) {
		categories = append(categories, config.MetaLoopEdit)
	}
	return categories, nil
}

func (e *Engine) decideRound(ctx context.Context, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round) (Result, error) {
	target := *round.ReturnedTarget
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	judgments, err := e.roundJudgments(round)
	if err != nil {
		return Result{}, err
	}
	targets := map[string]form.TargetVersion{}
	for _, item := range stage.Checks {
		targets[item.Target] = target
	}
	requirements, tests, err := e.detectionContext(ctx, stage.RequiredChecks(), roundReference(round))
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, roundReference(round))
	if err != nil {
		return Result{}, err
	}
	if r, blocked, err := e.rejectReadOnlyEdits(ctx, state, p, stage, run, round, settings); blocked || err != nil {
		return r, err
	}
	rule := stage.CompletionRule()
	rule.RequireDetectionTests = settings.DetectionRequired()
	completion := router.CheckCompletion(router.CompletionInput{Rule: rule, TargetVersion: target, Targets: targets, Norms: round.Norms, Judgments: judgments, Detection: requirements, DetectionTests: tests})
	summary := check.Aggregate(judgments)
	categories, err := e.roundConfirmationCategories(ctx, state, round, settings)
	if err != nil {
		return Result{}, err
	}
	approvalTarget := approvalTree(target)
	if gateApproved(state, run, round, approvalTarget) {
		categories = nil
	}
	_, humanAnswered := humanStageAnswer(state, run, round)
	termination := form.TerminationCompleted
	records, err := e.records()
	if err != nil {
		return Result{}, err
	}
	for _, r := range records {
		if progress, ok := r.Payload.(*ledger.Progress); ok && progress.RoundID == round.ID {
			termination = progress.TerminationKind
		}
	}
	progressMade, noProgress, err := e.progress(state, run.ID, round, summary)
	if err != nil {
		return Result{}, err
	}
	window, err := e.roundWindow(state, run.ID, round)
	if err != nil {
		return Result{}, err
	}
	table, err := e.table(ctx, roundReference(round))
	if err != nil {
		return Result{}, err
	}
	decision, err := table.Decide(router.Input{Summary: summary, Stage: stage, CompletedRounds: len(window), PreviousNoProgressRounds: noProgress, ProgressMade: progressMade, TerminationKind: termination, Completion: completion, Confirmation: router.ConfirmationInput{Categories: categories, GateApproved: gateApproved(state, run, round, approvalTarget), HumanStageAnswered: humanAnswered}})
	if err != nil {
		return Result{}, err
	}
	if decision.Action == plan.Done {
		return e.finishCommit(ctx, state, p, stage, run, round)
	}
	if decision.Action == plan.Ask {
		return e.askRoundApproval(ctx, state, p, stage, run, round, decision, table, categories, summary, completion, termination)
	}
	if err = e.closeRound(state, run, round, decision); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}

func (e *Engine) planRequestInputs(state *ledger.State, input *check.RequestInput) error {
	work, _ := state.Work()
	task, err := os.ReadFile(filepath.Join(e.workRoot(), work.TaskRef))
	if err != nil {
		return err
	}
	input.OriginalRequest = work.OriginalRequest
	input.StartTask = string(task)
	return nil
}

// rejectReadOnlyEdits returns the round for a fix when it changed a read-only source.
func (e *Engine) rejectReadOnlyEdits(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, run *ledger.StageRun, round *ledger.Round, settings config.Config) (Result, bool, error) {
	base, err := e.stageDiffBase(round)
	if err != nil {
		return Result{}, false, err
	}
	paths, err := e.workspace.ChangedFiles(ctx, base, round.ReturnedTarget.Tree)
	if err != nil {
		return Result{}, false, err
	}
	blocked := e.readOnlyChanges(settings, paths)
	if len(blocked) == 0 {
		return Result{}, false, nil
	}
	reason := "Read-only sources were edited: " + strings.Join(blocked, ", ") + ". Undo these edits and record each improvement as a lesson with devkit note lesson so that the source maintainers can apply it."
	r, err := e.fixRound(ctx, state, p, stage, run, round, reason)
	return r, true, err
}
