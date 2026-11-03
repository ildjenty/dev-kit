package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
)

// instructionData is a regenerable request artifact, never a ledger record.
type instructionData struct {
	Instruction  form.Instruction    `json:"instruction"`
	Reference    string              `json:"reference"`
	Request      *check.Request      `json:"request,omitempty"`
	Origin       plan.ProposalOrigin `json:"origin,omitempty"`
	TemplateName string              `json:"template_name,omitempty"`
	Detection    *detectionInput     `json:"detection,omitempty"`
	BriefText    string              `json:"brief_text,omitempty"`
}

func (e *Engine) kitPath(name string) string { return ".devkit/kit/" + e.version + "/" + name }
func (e *Engine) asset(ctx context.Context, ref, name string) ([]byte, error) {
	return e.workspace.ReadFile(ctx, ref, e.kitPath(name))
}
func (e *Engine) expanded(ctx context.Context, ref string) (string, error) {
	tree, err := e.workspace.CommitTree(ctx, ref)
	if err != nil {
		tree = ref
	}
	path := filepath.Join(e.workRoot(), "snap", tree)
	marker := path + ".complete"
	if _, err := os.Stat(marker); err == nil {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.Remove(marker); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := e.active(); err != nil {
		return "", err
	}
	if err := removeExpansion(path); err != nil {
		return "", err
	}
	if err := e.workspace.Expand(ctx, tree, path); err != nil {
		return "", err
	}
	err = filepath.WalkDir(path, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := fs.FileMode(0444)
		if entry.IsDir() || info.Mode()&0111 != 0 {
			mode = 0555
		}
		return os.Chmod(name, mode)
	})
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(marker, nil, 0444); err != nil {
		return "", err
	}
	return path, nil
}
func (e *Engine) normDocuments(ctx context.Context, ref string) ([]check.Norm, []brief.Document, error) {
	refs, err := e.norms(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	path, err := e.expanded(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	norms := []check.Norm{}
	docs := []brief.Document{}
	for _, norm := range refs {
		var content strings.Builder
		absolute := filepath.Join(path, filepath.FromSlash(norm.Path))
		err = filepath.WalkDir(absolute, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "run" && name == filepath.Join(path, ".devkit/run") {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(path, name)
			if err != nil {
				return err
			}
			doc := brief.Document{Path: filepath.ToSlash(rel), Content: string(data)}
			docs = append(docs, doc)
			fmt.Fprintf(&content, "\nReference: %s\n\n%s\n", doc.Path, doc.Content)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
		norms = append(norms, check.Norm{Reference: norm, Content: content.String()})
	}
	return norms, docs, nil
}
func (e *Engine) repository(ctx context.Context, ref string) (brief.RepositoryFacts, error) {
	path, err := e.expanded(ctx, ref)
	if err != nil {
		return brief.RepositoryFacts{}, err
	}
	facts := brief.RepositoryFacts{LayoutGuides: []brief.Document{}, AvailableChecks: []plan.Check{}}
	err = filepath.WalkDir(path, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || name == filepath.Join(path, ".devkit/run") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(path, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Name() == "README.md" || entry.Name() == "doc.go" || rel == ".devkit/rules.md" {
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			doc := brief.Document{Path: rel, Content: string(data)}
			if rel == ".devkit/rules.md" {
				facts.Rules = doc
				facts.IrreversibleDecisions = []string{doc.Content}
			} else {
				facts.LayoutGuides = append(facts.LayoutGuides, doc)
			}
		}
		return nil
	})
	return facts, err
}
func (e *Engine) outputs(state *ledger.State) ([]brief.StageOutput, error) {
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	outputs := []brief.StageOutput{}
	questions := map[string]form.Question{}
	for _, q := range state.Questions() {
		questions[q.ID] = q
	}
	effective := state.AnsweredQuestions()
	for _, record := range records {
		if q, ok := record.Payload.(*ledger.Question); ok && q.Supersedes != "" {
			delete(effective, q.Supersedes)
		}
	}
	for _, record := range records {
		var answer *ledger.Answer
		switch p := record.Payload.(type) {
		case *ledger.Decision:
			data, err := json.MarshalIndent(p.Decision, "", "  ")
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, brief.StageOutput{StageID: "Decision " + p.ID, Content: string(data)})
		case *ledger.Answer:
			answer = p
		case *ledger.Approval:
			answer = &p.Answer
		}
		if answer != nil && effective[answer.QuestionID].RecordID == record.ID {
			q := questions[answer.QuestionID]
			outputs = append(outputs, brief.StageOutput{StageID: q.StoppedStage + " human guidance", Content: "Interpretation: " + string(answer.Interpretation) + "\n" + answer.Text + "\nOriginal human input: " + answer.Original})
		}
	}

	for _, run := range state.StageRuns() {
		if run.State != ledger.StageCompleted {
			continue
		}
		for _, r := range records {
			if progress, ok := r.Payload.(*ledger.Progress); ok {
				for _, round := range state.Rounds(run.ID) {
					if progress.RoundID == round.ID {
						data, _ := json.MarshalIndent(progress.Progress, "", "  ")
						outputs = append(outputs, brief.StageOutput{StageID: run.StageID, Content: string(data)})
					}
				}
			}
		}
	}
	return outputs, nil
}
func instructionID(workID string, kind form.InstructionKind, key string, reissue int) string {
	return fmt.Sprintf("%s-%s-%s-%d", workID, kind, hash([]byte(key))[:24], reissue)
}
func (e *Engine) issuance(kind form.InstructionKind, key string, round *ledger.Round, version int, questionID string) (ledger.Instruction, bool, error) {
	records, err := e.records()
	if err != nil {
		return ledger.Instruction{}, false, err
	}
	var previous *ledger.Instruction
	for _, r := range records {
		if i, ok := r.Payload.(*ledger.Instruction); ok && i.Kind == kind && i.TargetKey == key {
			previous = i
		}
	}
	if previous != nil && previous.SessionID == e.sessionID {
		return *previous, false, nil
	}
	i := ledger.Instruction{Kind: kind, TargetKey: key, SessionID: e.sessionID, PlanVersion: version, QuestionID: questionID}
	if previous != nil {
		i.Reissue = previous.Reissue + 1
	}
	i.ID = instructionID(e.workID, kind, key, i.Reissue)
	if round != nil {
		i.RoundID = round.ID
		i.StageRunID = round.StageRunID
	}
	return i, true, nil
}
func (e *Engine) publishInstruction(meta ledger.Instruction, fresh bool, data instructionData, text string) (Result, error) {
	if err := e.active(); err != nil {
		return Result{}, err
	}
	i := data.Instruction
	i.ID = meta.ID
	i.Kind = meta.Kind
	i.TargetKey = meta.TargetKey
	i.Reissue = meta.Reissue
	if text == "" && i.Brief != "" {
		if _, err := os.Stat(i.Brief); errors.Is(err, os.ErrNotExist) {
			text = data.BriefText
			if text == "" {
				body, err := e.regenerateBrief(context.Background(), meta, data)
				if err != nil {
					return Result{}, err
				}
				text = body.Text
			}
		} else if err != nil {
			return Result{}, err
		}
	}
	if text != "" {
		data.BriefText = text
		i.Brief = filepath.Join(e.workRoot(), "brief", i.ID+".md")
		if err := e.store().Publish("brief/"+i.ID+".md", []byte(text)); err != nil {
			return Result{}, err
		}
	}
	data.Instruction = i
	body, err := json.Marshal(data)
	if err != nil {
		return Result{}, err
	}
	if err = e.store().Publish("brief/"+i.ID+".json", body); err != nil {
		return Result{}, err
	}
	if fresh {
		meta.Reference, meta.Runner, meta.TargetPath = data.Reference, i.Runner, i.TargetPath
		meta.TemplateName, meta.Origin = data.TemplateName, data.Origin
		meta.Request, meta.Detection = data.Request, data.Detection
		if data.Request != nil {
			request := *data.Request
			request.Norms = slices.Clone(request.Norms)
			for i := range request.Norms {
				request.Norms[i].Content = ""
			}
			meta.Request = &request
		}
		if meta.RoundID != "" {
			state, err := e.load()
			if err != nil {
				return Result{}, err
			}
			if round, ok := state.OpenRound(); ok && round.ID == meta.RoundID && round.SessionID != e.sessionID {
				copy := *round
				copy.SessionID = e.sessionID
				if _, err = e.append(&copy); err != nil {
					return Result{}, err
				}
			}
		}
		if err = e.active(); err != nil {
			return Result{}, err
		}
		if _, err = e.append(&meta); err != nil {
			return Result{}, err
		}
	}
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	r := response(state)
	if r.Paused {
		return r, nil
	}
	r.Instruction = &i
	return r, nil
}
func (e *Engine) readInstruction(id string) (instructionData, error) {
	data, err := e.readInstructionFacts(id)
	if err != nil {
		return data, err
	}
	return data, e.hydrateRequest(context.Background(), data)
}

func (e *Engine) readInstructionFacts(id string) (instructionData, error) {
	records, err := e.records()
	if err != nil {
		return instructionData{}, err
	}
	for _, record := range records {
		meta, ok := record.Payload.(*ledger.Instruction)
		if !ok || meta.ID != id {
			continue
		}
		i := form.Instruction{ID: meta.ID, Kind: meta.Kind, TargetKey: meta.TargetKey, Reissue: meta.Reissue, Runner: meta.Runner, TargetPath: meta.TargetPath, Brief: filepath.Join(e.workRoot(), "brief", id+".md")}
		switch meta.Kind {
		case form.InstructionPlan:
			i.Text = "Launch the planner with: devkit-instruction " + id
		case form.InstructionWork:
			i.Text = "Perform the work described in the brief, then run devkit submit."
		case form.InstructionJudgment, form.InstructionPlanCheck:
			i.Text = "Launch the assigned judge with: devkit-instruction " + id
		case form.InstructionDetectionTest:
			i.Text = "Run the detection test with the assigned judge: devkit-instruction " + id
		case form.InstructionQuestion:
			for _, r := range records {
				if question, ok := r.Payload.(*ledger.Question); ok && question.ID == meta.QuestionID {
					q := question.Question
					i.Question, i.Text = &q, q.Text
				}
			}
		}
		return instructionData{Instruction: i, Reference: meta.Reference, Origin: meta.Origin, TemplateName: meta.TemplateName, Request: meta.Request, Detection: meta.Detection}, nil
	}
	return instructionData{}, fmt.Errorf("instruction not found: %s", id)
}

func (e *Engine) instructionBrief(id string) ([]byte, error) {
	name := "brief/" + id + ".md"
	body, err := os.ReadFile(filepath.Join(e.workRoot(), name))
	if !errors.Is(err, os.ErrNotExist) {
		return body, err
	}
	data, err := e.readInstruction(id)
	if err != nil {
		return nil, err
	}
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if meta, ok := record.Payload.(*ledger.Instruction); ok && meta.ID == id {
			b, err := e.regenerateBrief(context.Background(), *meta, data)
			if err != nil {
				return nil, err
			}
			body = []byte(b.Text)
			return body, e.store().Publish(name, body)
		}
	}
	return nil, fmt.Errorf("instruction not found: %s", id)
}

func (e *Engine) regenerateBrief(ctx context.Context, meta ledger.Instruction, data instructionData) (brief.Brief, error) {
	state, err := e.load()
	if err != nil {
		return brief.Brief{}, err
	}
	switch meta.Kind {
	case form.InstructionPlan:
		template, source, err := e.template(ctx, data.Reference, data.TemplateName)
		if err != nil {
			return brief.Brief{}, err
		}
		return e.plannerBrief(ctx, state, template, source, meta.PlanVersion, data.Origin, data.Reference, &meta)
	case form.InstructionWork:
		p, ok, err := e.boundPlan(meta.PlanVersion)
		if err != nil {
			return brief.Brief{}, err
		}
		if !ok {
			return brief.Brief{}, errors.New("bound plan is missing")
		}
		for _, round := range state.Rounds(meta.StageRunID) {
			if round.ID != meta.RoundID {
				continue
			}
			for _, run := range state.StageRuns() {
				if run.ID == meta.StageRunID {
					stage, ok := p.Stage(run.StageID)
					if !ok {
						return brief.Brief{}, errors.New("bound stage is missing")
					}
					return e.workBrief(ctx, state, p, stage, round, &meta)
				}
			}
		}
		return brief.Brief{}, errors.New("instruction round is missing")
	case form.InstructionJudgment, form.InstructionPlanCheck:
		return e.judgmentBrief(ctx, data)
	case form.InstructionDetectionTest:
		return e.detectionBrief(ctx, data)
	case form.InstructionQuestion:
		text, err := form.RenderMarkdown(*data.Instruction.Question)
		return brief.Brief{Text: text}, err
	default:
		return brief.Brief{}, fmt.Errorf("unsupported instruction kind: %s", meta.Kind)
	}
}

// InstructionRunner resolves the runner from an issued request for hook collection.
// The caller supplies the resolved work and loads its record validator before collection.
func (e *Engine) InstructionRunner(instruction ledger.Instruction) (string, error) {
	data, err := e.readInstructionFacts(instruction.ID)
	if err != nil {
		return "", err
	}
	return data.Instruction.Runner, nil
}

func (e *Engine) planner(ctx context.Context, t plan.PlanTemplate, source []byte, version int, origin plan.ProposalOrigin) (Result, error) {
	state, err := e.load()
	if err != nil {
		return Result{}, err
	}
	ref, err := e.workspace.Head(ctx)
	if err != nil {
		return Result{}, err
	}
	meta, fresh, err := e.issuance(form.InstructionPlan, fmt.Sprint(version), nil, version, "")
	if err != nil {
		return Result{}, err
	}
	if !fresh {
		old, err := e.readInstruction(meta.ID)
		if err != nil {
			return Result{}, err
		}
		return e.publishInstruction(meta, false, old, "")
	}
	b, err := e.plannerBrief(ctx, state, t, source, version, origin, ref, &meta)
	if err != nil {
		return Result{}, err
	}
	return e.publishInstruction(meta, fresh, instructionData{Instruction: form.Instruction{Runner: "devkit-planner", Text: "Launch the planner with: devkit-instruction " + meta.ID}, Reference: ref, Origin: origin, TemplateName: t.Name}, b.Text)
}

func (e *Engine) plannerBrief(ctx context.Context, state *ledger.State, t plan.PlanTemplate, source []byte, version int, origin plan.ProposalOrigin, ref string, meta *ledger.Instruction) (brief.Brief, error) {
	template, err := e.asset(ctx, ref, "templates/brief/planner.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	_, docs, err := e.normDocuments(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	definitions := []brief.Document{}
	for _, stage := range append(slices.Clone(t.Stages), t.OptionalStages...) {
		data, err := e.asset(ctx, ref, "stages/"+stage.ID+".md")
		if err != nil {
			return brief.Brief{}, err
		}
		definitions = append(definitions, brief.Document{Path: e.kitPath("stages/" + stage.ID + ".md"), Content: string(data)})
		facts.AvailableChecks = append(facts.AvailableChecks, stage.Checks...)
	}
	work, _ := state.Work()
	task, err := os.ReadFile(filepath.Join(e.workRoot(), work.TaskRef))
	if err != nil {
		return brief.Brief{}, err
	}
	outputs, err := e.outputs(state)
	if err != nil {
		return brief.Brief{}, err
	}
	outputSchema, err := e.asset(ctx, ref, "schemas/planner-result.json")
	if err != nil {
		return brief.Brief{}, err
	}
	input := brief.PlannerInput{RequestTemplate: string(template), InstructionID: meta.ID, Version: version, Template: t, TemplateHash: plan.ContentHash(source), Task: string(task), StageDefinitions: definitions, Repository: facts, Norms: docs, PreviousOutputs: outputs, OutputSchema: string(outputSchema)}
	if p, ok := latestPlan(state); ok {
		input.CurrentPlan = &p.Plan
		if origin == plan.BackProposal {
			remaining := p.Plan
			remaining.Stages = nil
			start := p.Plan.StartStage
			if run, ok := state.LatestStageRun(); ok {
				if target, err := p.Plan.Next(run.StageID, plan.Back); err == nil && target != nil {
					start = *target
				}
			}
			remaining.StartStage = start
			retained := map[string]bool{}
			id := start
			for !retained[id] {
				retained[id] = true
				stage, ok := p.Plan.Stage(id)
				if !ok || stage.Transitions.Done == nil {
					break
				}
				id = *stage.Transitions.Done
			}
			for _, stage := range p.Plan.Stages {
				if retained[stage.ID] {
					remaining.Stages = append(remaining.Stages, stage)
				}
			}
			input.RemainingPlan = &remaining
		}
	}
	records, err := e.records()
	if err != nil {
		return brief.Brief{}, err
	}
	effective := state.AnsweredQuestions()
	for _, record := range records {
		if q, ok := record.Payload.(*ledger.Question); ok && q.Supersedes != "" {
			delete(effective, q.Supersedes)
		}
	}
	for _, r := range records {
		switch p := r.Payload.(type) {
		case *ledger.Answer:
			if effective[p.QuestionID].RecordID == r.ID {
				input.HumanInstructions += p.Text + "\n"
			}
		case *ledger.Approval:
			if effective[p.QuestionID].RecordID == r.ID {
				input.HumanInstructions += p.Text + "\n"
			}
		case *ledger.Judgment:
			if p.TargetVersion.Kind == form.TargetPlan {
				input.PreviousFindings = append(input.PreviousFindings, p.Findings...)
			}
		}
	}
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{Findings: input.PreviousFindings, CurrentPlan: input.CurrentPlan, RemainingPlan: input.RemainingPlan, HumanInstructions: input.HumanInstructions}
		for _, output := range input.PreviousOutputs {
			meta.Context.Outputs = append(meta.Context.Outputs, ledger.InstructionOutput{StageID: output.StageID, Content: output.Content})
		}
	} else {
		input.PreviousOutputs = instructionOutputs(meta.Context)
		input.PreviousFindings = meta.Context.Findings
		input.CurrentPlan, input.RemainingPlan = meta.Context.CurrentPlan, meta.Context.RemainingPlan
		input.HumanInstructions = meta.Context.HumanInstructions
	}
	return brief.BuildPlanner(input)
}

func (e *Engine) workInstruction(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, round *ledger.Round) (Result, error) {
	meta, fresh, err := e.issuance(form.InstructionWork, round.ID, round, p.Version, "")
	if err != nil {
		return Result{}, err
	}
	if !fresh {
		old, err := e.readInstruction(meta.ID)
		if err != nil {
			return Result{}, err
		}
		return e.publishInstruction(meta, false, old, "")
	}
	b, err := e.workBrief(ctx, state, p, stage, round, &meta)
	if err != nil {
		return Result{}, err
	}
	if round.SessionID != e.sessionID {
		copy := *round
		copy.SessionID = e.sessionID
		if _, err = e.append(&copy); err != nil {
			return Result{}, err
		}
	}
	return e.publishInstruction(meta, fresh, instructionData{Instruction: form.Instruction{Text: "Perform the work described in the brief, then run devkit submit."}, Reference: round.StartTarget.Tree}, b.Text)
}

func (e *Engine) workBrief(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, round *ledger.Round, meta *ledger.Instruction) (brief.Brief, error) {
	ref := round.StartTarget.Tree
	template, err := e.asset(ctx, ref, "templates/brief/work.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	definition, err := e.asset(ctx, ref, "stages/"+stage.ID+".md")
	if err != nil {
		return brief.Brief{}, err
	}
	_, docs, err := e.normDocuments(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	facts.AvailableChecks = stage.Checks
	outputs, err := e.outputs(state)
	if err != nil {
		return brief.Brief{}, err
	}
	outputSchema, err := e.asset(ctx, ref, "schemas/progress.json")
	if err != nil {
		return brief.Brief{}, err
	}
	input := brief.WorkInput{RequestTemplate: string(template), InstructionID: meta.ID, Stage: stage, StageDefinition: string(definition), Plan: p, Purpose: p.Task, Repository: facts, Norms: docs, PreviousOutputs: outputs, PreviousFindings: state.UnresolvedBlocking(round.StageRunID), OutputSchema: string(outputSchema)}
	for _, condition := range p.CompletionConditions {
		input.Verification = append(input.Verification, condition.ID+": "+condition.Description)
	}
	commands := []check.Judgment{}
	records, err := e.records()
	if err != nil {
		return brief.Brief{}, err
	}
	for _, r := range records {
		switch value := r.Payload.(type) {
		case *ledger.Judgment:
			if value.Command != nil {
				for _, previous := range state.Rounds(round.StageRunID) {
					if previous.ID == value.RoundID {
						input.CommandOutputs = append(input.CommandOutputs, brief.CommandOutput{Check: plan.Check{Aspect: value.Aspect, Runner: plan.Runner(value.Runner)}, ExitCode: value.Command.ExitCode, Output: value.Command.Output})
						commands = append(commands, value.Judgment)
					}
				}
			}
		case *ledger.StageRun:
			if value.ID == round.StageRunID && value.StopReason != "" {
				input.PreCommitOutput = value.StopReason
			}
		}
	}
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{Findings: input.PreviousFindings, Commands: commands, PreCommitOutput: input.PreCommitOutput}
		for _, output := range input.PreviousOutputs {
			meta.Context.Outputs = append(meta.Context.Outputs, ledger.InstructionOutput{StageID: output.StageID, Content: output.Content})
		}
	} else {
		input.PreviousOutputs = instructionOutputs(meta.Context)
		input.PreviousFindings = meta.Context.Findings
		input.PreCommitOutput = meta.Context.PreCommitOutput
		input.CommandOutputs = nil
		for _, command := range meta.Context.Commands {
			input.CommandOutputs = append(input.CommandOutputs, brief.CommandOutput{Check: plan.Check{Aspect: command.Aspect, Runner: plan.Runner(command.Runner)}, ExitCode: command.Command.ExitCode, Output: command.Command.Output})
		}
	}
	return brief.BuildWork(input)
}

func instructionOutputs(context *ledger.InstructionContext) []brief.StageOutput {
	var outputs []brief.StageOutput
	for _, output := range context.Outputs {
		outputs = append(outputs, brief.StageOutput{StageID: output.StageID, Content: output.Content})
	}
	return outputs
}

func (e *Engine) table(ctx context.Context, ref string) (router.Table, error) {
	data, err := e.asset(ctx, ref, "router/default.toml")
	if err != nil {
		return router.Table{}, err
	}
	var table router.Table
	if err = toml.Unmarshal(data, &table); err != nil {
		return table, err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return table, err
	}
	return router.MergeTables(table, router.Table{HumanConfirmation: settings.Router.HumanConfirmation}), nil
}
func (e *Engine) question(target form.ApprovalTarget, category, text, stage string) form.Question {
	return form.Question{ID: ulidString(), Text: text, ApprovalTarget: target, Options: []string{"approve", "reject", "defer"}, Recommendation: "defer", StopCategory: category, NextStage: stage, StoppedStage: stage, StopReason: text, Evidence: []form.Reference{}}
}
func (e *Engine) saveQuestion(state *ledger.State, q form.Question, supersedes string) (form.Question, error) {
	for _, current := range state.UnansweredQuestions() {
		a, _ := json.Marshal(current.ApprovalTarget)
		b, _ := json.Marshal(q.ApprovalTarget)
		if string(a) == string(b) && current.StopCategory == q.StopCategory && current.StoppedStage == q.StoppedStage && slices.Equal(current.Evidence, q.Evidence) {
			return current, nil
		}
	}
	if supersedes == "" && q.ApprovalTarget.Kind == form.ApprovalExternal && q.ApprovalTarget.External != nil {
		for _, old := range state.UnansweredQuestions() {
			if old.ApprovalTarget.Kind == form.ApprovalExternal && old.ApprovalTarget.External != nil && old.ApprovalTarget.External.OperationID == q.ApprovalTarget.External.OperationID {
				supersedes = old.ID
			}
		}
	}
	if supersedes == "" && q.ApprovalTarget.Kind == form.ApprovalPlan {
		for _, old := range state.UnansweredQuestions() {
			if old.ApprovalTarget.Kind == form.ApprovalPlan && old.ApprovalTarget.Plan != q.ApprovalTarget.Plan {
				supersedes = old.ID
			}
		}
	}
	if err := e.active(); err != nil {
		return q, err
	}
	_, err := e.append(&ledger.Question{Question: q, Supersedes: supersedes})
	return q, err
}
func (e *Engine) issueQuestion(state *ledger.State, q form.Question) (Result, error) {
	if run, ok := state.LatestStageRun(); ok && run.StageID == q.StoppedStage && run.State != ledger.StageCompleted && run.State != ledger.StageInvalidated && state.WorkStatus() == ledger.WorkActive {
		waiting := ledger.StageState("")
		switch q.StopCategory {
		case "human_stage":
			waiting = ledger.StageAnswerWait
		case "external_operation", "external_unknown":
			waiting = ledger.StageExternalWait
		}
		if waiting != "" && (waiting != run.State || run.NextActor != "human" || run.StopReason != q.StopReason) {
			update := *run
			update.State, update.NextActor, update.StopReason = waiting, "human", q.StopReason
			if _, err := e.append(&update); err != nil {
				return Result{}, err
			}
		}
	}
	meta, fresh, err := e.issuance(form.InstructionQuestion, q.ID, nil, 0, q.ID)
	if err != nil {
		return Result{}, err
	}
	text, err := form.RenderMarkdown(q)
	if err != nil {
		return Result{}, err
	}
	return e.publishInstruction(meta, fresh, instructionData{Instruction: form.Instruction{Question: &q, Text: q.Text}}, text)
}
