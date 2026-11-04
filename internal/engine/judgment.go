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

func (e *Engine) aspect(ctx context.Context, ref, id string) (check.Aspect, error) {
	if !localName(id) {
		return check.Aspect{}, errors.New("invalid aspect ID")
	}
	data, err := e.asset(ctx, ref, "checks/"+id+".md")
	if err != nil {
		return check.Aspect{}, err
	}
	aspect, err := check.ParseAspect(string(data))
	if err != nil {
		return aspect, err
	}
	if aspect.ID != id {
		return aspect, errors.New("aspect ID does not match its file name")
	}
	return aspect, nil
}
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
	return source, nil
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
		data, err := e.readInstruction(meta.ID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return meta, instructionData{}, false, err
		}
		if err == nil && data.Request != nil {
			norms, err := e.norms(ctx, ref)
			if err != nil {
				return meta, instructionData{}, false, err
			}
			previous := check.Judgment{JudgmentResult: form.JudgmentResult{TargetVersion: data.Request.TargetVersion, Norms: normRefs(data.Request.Norms)}}
			if data.Request.Runner == string(item.Runner) && previous.ValidFor(target, norms) {
				return meta, data, false, nil
			}
			meta.Reissue++
			meta.ID = instructionID(e.workID, kind, key, meta.Reissue)
			fresh = true
		}
	}
	norms, _, err := e.normDocuments(ctx, ref)
	if err != nil {
		return meta, instructionData{}, false, err
	}
	if err := e.requestNorms(ctx, ref, item, norms); err != nil {
		return meta, instructionData{}, false, err
	}
	aspect := check.Aspect{ID: item.Aspect}
	if !item.Runner.IsCommand() {
		aspect, err = e.aspect(ctx, ref, item.Aspect)
		if err != nil {
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
	input := check.RequestInput{Kind: requestKind, InstructionID: meta.ID, Runner: string(item.Runner), Aspect: aspect, TargetVersion: target, TargetPath: path, Norms: norms, OutputSchema: schema, CompletionConditionIDs: append([]string{}, item.CompletionConditionIDs...), Progress: progress, PreviousFindings: findings, Authorizations: []check.Authorization{}}
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
			input.Authorizations = append(input.Authorizations, check.Authorization{Target: q.ApprovalTarget, Scope: q.Text + "\n" + answer.Answer.Text + "\nOriginal human input: " + answer.Answer.Original, Evidence: []form.Reference{{RecordID: answer.RecordID}}})
		}
	}
	request, err := check.PrepareRequest(input)
	if err != nil {
		return meta, instructionData{}, false, err
	}
	data := instructionData{Reference: ref, Request: &request, Instruction: form.Instruction{ID: meta.ID, Kind: kind, TargetPath: path, Runner: string(item.Runner), Text: "Launch the assigned judge with: devkit-instruction " + meta.ID}}
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
				definition, err := e.asset(ctx, round.StartTarget.Tree, "stages/"+run.StageID+".md")
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

func (e *Engine) requestNorms(ctx context.Context, ref string, item plan.Check, norms []check.Norm) error {
	docs, err := e.aspectDocuments(ctx, ref, []plan.Check{item})
	if err != nil {
		return err
	}
	facts, err := e.repository(ctx, ref)
	if err != nil {
		return err
	}
	docs = append(docs, facts.Rules)
	if item.Target != "detection" {
		docs = append(docs, facts.Requirements)
		docs = append(docs, facts.LayoutGuides...)
	}
	if len(norms) > 0 {
		for _, doc := range docs {
			norms[0].Content += "\nReference: " + doc.Path + "\n\n" + doc.Content + "\n"
		}
	}
	return nil
}

func (e *Engine) hydrateRequest(ctx context.Context, data instructionData) error {
	if data.Request == nil || len(data.Request.Norms) == 0 || data.Request.Norms[0].Content != "" {
		return nil
	}
	norms, _, err := e.normDocuments(ctx, data.Reference)
	if err != nil {
		return err
	}
	item := plan.Check{Aspect: data.Request.Aspect.ID, Runner: plan.Runner(data.Request.Runner)}
	if data.Request.Kind == check.DetectionRequest {
		item.Target = "detection"
	}
	if err := e.requestNorms(ctx, data.Reference, item, norms); err != nil {
		return err
	}
	if data.Request.Kind == check.RoundRequest {
		records, err := e.records()
		if err != nil {
			return err
		}
		for _, record := range records {
			if meta, ok := record.Payload.(*ledger.Instruction); ok && meta.ID == data.Instruction.ID {
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
	facts, err := e.repository(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	docs, err := e.aspectDocuments(ctx, data.Reference, []plan.Check{{Aspect: data.Request.Aspect.ID, Runner: plan.Runner(data.Request.Runner)}})
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
	return brief.BuildJudgment(brief.JudgmentInput{RequestTemplate: string(template), InstructionID: data.Instruction.ID, Content: "```json\n" + string(content) + "\n```", Repository: facts, Norms: docs, NormPaths: paths, OutputSchema: string(schema)})
}

func (e *Engine) planCheck(ctx context.Context, state *ledger.State, p plan.Plan) (Result, error) {
	item := plan.Check{Aspect: "plan", Runner: "devkit-judge", Required: true}
	snapshot, err := e.workspace.Snapshot(ctx, e.workID)
	if err != nil {
		return Result{}, err
	}
	if r, pending, err := e.ensureDetection(ctx, state, []plan.Check{item}, snapshot.Tree); pending || err != nil {
		return r, err
	}
	// A plan check is tied to the proposal and the norms at issuance.
	key := fmt.Sprintf("%d:plan", p.Version)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return Result{}, err
	}
	name := fmt.Sprintf("brief/plan-%d.json", p.Version)
	if err = e.store().Publish(name, data); err != nil {
		return Result{}, err
	}
	meta, request, fresh, err := e.judgmentInstruction(ctx, state, item, form.InstructionPlanCheck, key, snapshot.Tree, form.TargetVersion{Kind: form.TargetPlan, Plan: p.Version}, filepath.Join(e.workRoot(), name), nil, p.Version)
	if err != nil {
		return Result{}, err
	}
	return e.publishJudgment(ctx, meta, request, fresh)
}
func (e *Engine) acceptPending(ctx context.Context, state *ledger.State) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(e.workRoot(), "results"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	type observed struct {
		result hook.SubagentResult
		path   string
	}
	observations := []observed{}
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
			return false, err
		}
		var result hook.SubagentResult
		if err = json.Unmarshal(data, &result); err != nil {
			return false, fmt.Errorf("result %s: %w", name, err)
		}
		observations = append(observations, observed{result, name})
	}
	slices.SortStableFunc(observations, func(a, b observed) int { return a.result.ObservedAt.Compare(b.result.ObservedAt) })
	for _, instruction := range state.UnacceptedInstructions() {
		if instruction.Kind != form.InstructionPlan && instruction.Kind != form.InstructionJudgment && instruction.Kind != form.InstructionPlanCheck && instruction.Kind != form.InstructionDetectionTest {
			continue
		}
		request, err := e.readInstruction(instruction.ID)
		if err != nil {
			return false, err
		}
		for _, obs := range observations {
			result := obs.result
			if result.InstructionID != instruction.ID {
				continue
			}
			if result.AgentID == "" || result.Runner != request.Instruction.Runner {
				return false, fmt.Errorf("result %s: runner or agent identity does not match", obs.path)
			}
			provenance := check.Provenance{Source: check.SubagentResult, InstructionID: instruction.ID, Runner: result.Runner, AgentID: result.AgentID, Effort: result.Effort, Model: result.Model, ResultPath: obs.path}
			if instruction.Kind == form.InstructionPlan {
				var output form.PlannerResult
				if err = e.forms["planner-result"].Decode(result.Content, &output); err != nil {
					return false, err
				}
				if output.InstructionID != instruction.ID {
					return false, errors.New("planner instruction ID does not match")
				}
				p, err := e.plans.ParsePlan(output.Plan)
				if err != nil {
					return false, err
				}
				if p.Version != instruction.PlanVersion {
					return false, errors.New("planner version does not match the requested version")
				}
				return true, e.acceptPlan(ctx, state, p, request.Origin, instruction.ID)
			}
			if request.Detection != nil {
				return true, e.acceptDetection(ctx, state, instruction, request, result.Content, provenance)
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
				return false, err
			}
			if accepted {
				_, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: instruction.RoundID})
				return true, err
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
		judgments, err := e.roundJudgments(round)
		if err != nil {
			return Result{}, err
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
				return e.fixRound(ctx, state, p, stage, run, round, "A command changed its target.")
			}
			if item.Required && item.Runner.IsCommand() && existing.Judgment == check.Fail {
				return e.fixRound(ctx, state, p, stage, run, round, "A required command check failed.")
			}
			continue
		}
		key := round.ID + ":" + item.Aspect
		meta, request, fresh, err := e.judgmentInstruction(ctx, state, item, form.InstructionJudgment, key, round.StartTarget.Tree, target, path, round, p.Version)
		if err != nil {
			return Result{}, err
		}
		issued, err := e.publishJudgment(ctx, meta, request, fresh)
		if err != nil {
			return Result{}, err
		}
		settings, err := e.settings(ctx, round.StartTarget.Tree)
		if err != nil {
			return Result{}, err
		}
		if item.Runner.IsCommand() {
			command, ok := settings.Commands[strings.TrimPrefix(string(item.Runner), "command:")]
			var judgment check.Judgment
			if !ok {
				judgment, err = check.UnregisteredCommand(*request.Request)
			} else {
				judgment, err = e.runCheck(ctx, *request.Request, command, stage.ID, target.Tree)
			}
			if err != nil {
				return Result{}, err
			}
			if _, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: round.ID}); err != nil {
				return Result{}, err
			}
			if commandTargetChanged(judgment) {
				return e.fixRound(ctx, state, p, stage, run, round, "A command check changed its target.")
			}
			if judgment.Judgment == check.Fail && item.Required {
				result, err := e.fixRound(ctx, state, p, stage, run, round, "A required command check failed.")
				if !ok && err == nil && result.Instruction == nil {
					result = response(state)
					result.Message = judgment.Findings[0].Scenario + "\nCorrect this finding, then run devkit next to open the fix round."
				}
				return result, err
			}
			continue
		}
		if command, ok := settings.Judges[string(item.Runner)]; ok {
			judgment, err := e.runJudge(ctx, request, command, stage.ID, target.Tree)
			if err != nil {
				return Result{}, err
			}
			if _, err = e.append(&ledger.Judgment{Judgment: judgment, RoundID: round.ID}); err != nil {
				return Result{}, err
			}
			if commandTargetChanged(judgment) {
				return e.fixRound(ctx, state, p, stage, run, round, "A judgment command changed its target.")
			}
			continue
		}
		if !strings.HasPrefix(string(item.Runner), "devkit-") {
			return Result{}, fmt.Errorf("judge %q is neither a registered command nor a devkit subagent", item.Runner)
		}
		return issued, nil
	}
	state, err = e.load()
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
	requirements, tests, err := e.detectionContext(ctx, stage.RequiredChecks(), round.StartTarget.Tree)
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, round.StartTarget.Tree)
	if err != nil {
		return Result{}, err
	}
	rule := stage.CompletionRule()
	rule.RequireDetectionTests = settings.DetectionRequired()
	completion := router.CheckCompletion(router.CompletionInput{Rule: rule, TargetVersion: target, Targets: targets, Norms: round.Norms, Judgments: judgments, Detection: requirements, DetectionTests: tests})
	summary := check.Aggregate(judgments)
	paths, err := e.workspace.ChangedFiles(ctx, round.StartTarget.Tree, target.Tree)
	if err != nil {
		return Result{}, err
	}
	categories := differenceCategories(paths, settings)
	if changed, err := e.metaLoopChanged(ctx, round.StartTarget.Tree, target.Tree); err != nil {
		return Result{}, err
	} else if changed && !slices.Contains(categories, config.MetaLoopEdit) {
		categories = append(categories, config.MetaLoopEdit)
	}
	approvalTarget := form.ApprovalTarget{Kind: form.ApprovalTree, Tree: target.Tree}
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
	table, err := e.table(ctx, round.StartTarget.Tree)
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
		category := "stage_confirmation"
		if slices.Contains(categories, config.MetaLoopEdit) {
			category = config.MetaLoopEdit
		}
		text := decision.Reason
		if category == config.MetaLoopEdit {
			text = "Approve development loop edits before committing this stage? If rejected, the worker must undo the edits."
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
	if err = e.closeRound(state, run, round, decision); err != nil {
		return Result{}, err
	}
	return Result{}, nil
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
