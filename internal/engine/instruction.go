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
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/kit"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/router"
	"github.com/ildjenty/dev-kit/internal/source"
)

// instructionData is a regenerable request artifact, never a ledger record.
type instructionData struct {
	Instruction  form.Instruction           `json:"instruction"`
	Reference    string                     `json:"reference"`
	Request      *check.Request             `json:"request,omitempty"`
	Origin       plan.ProposalOrigin        `json:"origin,omitempty"`
	TemplateName string                     `json:"template_name,omitempty"`
	Detection    *detectionInput            `json:"detection,omitempty"`
	BriefText    string                     `json:"brief_text,omitempty"`
	Context      *ledger.InstructionContext `json:"context,omitempty"`
	Inputs       []plan.Selection           `json:"inputs,omitempty"`
}

func (e *Engine) asset(ctx context.Context, ref, name string) ([]byte, error) {
	if strings.HasPrefix(name, "schemas/") {
		source, err := kit.Schema(strings.TrimSuffix(strings.TrimPrefix(name, "schemas/"), ".json"))
		if err != nil {
			return nil, err
		}
		return e.bundledSchema(source)
	}
	return e.logicalAsset(ctx, ref, name)
}
func (e *Engine) expanded(ctx context.Context, ref string) (string, error) {
	tree, err := e.workspace.CommitTree(ctx, gitReference(ref))
	if err != nil {
		tree = gitReference(ref)
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
	norms := []check.Norm{}
	for _, norm := range refs {
		item := check.Norm{Reference: norm}
		if norm.Path == "devkit.toml" {
			body, err := e.workspace.ReadFile(ctx, gitReference(ref), norm.Path)
			if err != nil {
				return nil, nil, err
			}
			item.Content = string(body)
		}
		norms = append(norms, item)
	}
	return norms, nil, nil
}

// normPaths lists every fixed norm that is not supplied in full: each norm source with the
// location of its fixed copy, and the norms outside the sources in the expanded snapshot.
func (e *Engine) normPaths(ctx context.Context, ref string) ([]brief.NormPath, error) {
	refs, err := e.norms(ctx, ref)
	if err != nil {
		return nil, err
	}
	expanded, err := e.expanded(ctx, ref)
	if err != nil {
		return nil, err
	}
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{}
	for name, item := range sources {
		if location, inside := e.repositoryLocation(item); inside {
			labels[location] = "source " + name + " (" + location + ")"
		}
	}
	paths := []brief.NormPath{}
	for _, norm := range refs {
		if name, ok := strings.CutPrefix(norm.Path, "source:"); ok {
			pin, err := e.pin(ctx, ref, false)
			if err != nil {
				return nil, err
			}
			directory, err := e.fixedDirectory(name, pin[name])
			if err != nil {
				return nil, err
			}
			paths = append(paths, brief.NormPath{Path: "source " + name + " (" + sources[name].Path + ")", SnapshotPath: directory})
			continue
		}
		label := labels[norm.Path]
		if label == "" {
			label = norm.Path
		}
		paths = append(paths, brief.NormPath{Path: label, SnapshotPath: filepath.Join(expanded, filepath.FromSlash(norm.Path))})
	}
	return paths, nil
}

// selectionDocuments resolves selections into documents for a brief. Norm sources are read
// from the snapshot fixed for ref and products from productRef, from their fixed outside copy.
func (e *Engine) selectionDocuments(ctx context.Context, ref, productRef string, selections []plan.Selection) ([]brief.Document, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return nil, err
	}
	docs := []brief.Document{}
	seen := map[string]bool{}
	for _, selection := range selections {
		documents, err := e.productDocuments(ctx, ref, productRef, selection, sources[selection.Source])
		if err != nil {
			return nil, err
		}
		for _, document := range documents {
			key := selection.Source + "/" + document.ID
			if !seen[key] {
				seen[key] = true
				docs = append(docs, brief.Document{Path: key, Content: document.Body, Exists: true})
			}
		}
	}
	return docs, nil
}

func (e *Engine) productDocuments(ctx context.Context, ref, productRef string, selection plan.Selection, item config.Source) ([]source.Document, error) {
	if item.Norm {
		return e.selectDocuments(ctx, ref, selection.Source, selection.Select)
	}
	if _, inside := e.repositoryLocation(item); !inside {
		productRef = ref
	}
	return e.selectFrom(ctx, ref, productRef, selection.Source, selection.Select)
}

// additionalInputs resolves selections not rendered by the repository fields.
func (e *Engine) additionalInputs(ctx context.Context, ref, productRef string, inputs []plan.Selection) ([]brief.Document, error) {
	selections := slices.DeleteFunc(slices.Clone(inputs), func(selection plan.Selection) bool {
		return slices.Contains(plan.DefaultInputs(), selection)
	})
	return e.selectionDocuments(ctx, ref, productRef, selections)
}

func (e *Engine) repository(ctx context.Context, ref string, inputs []plan.Selection, productRef ...string) (brief.RepositoryFacts, error) {
	sources, err := e.knowledgeSources(ctx, ref)
	if err != nil {
		return brief.RepositoryFacts{}, err
	}
	facts := brief.RepositoryFacts{LayoutGuides: []brief.Document{}, AvailableChecks: []plan.Check{}}
	products := ref
	if len(productRef) > 0 {
		products = productRef[0]
	}
	for _, item := range []struct {
		name   string
		target *brief.Document
	}{
		{"rules", &facts.Rules}, {"requirements", &facts.Requirements}, {"structure", &facts.Structure},
	} {
		if !slices.Contains(inputs, plan.Selection{Source: item.name}) {
			continue
		}
		if *item.target, err = e.productFact(ctx, ref, products, item.name, sources[item.name]); err != nil {
			return facts, err
		}
	}
	if facts.Structure.Exists {
		facts.LayoutGuides = append(facts.LayoutGuides, facts.Structure)
	}
	return facts, nil
}

func (e *Engine) productFact(ctx context.Context, ref, productRef, name string, item config.Source) (brief.Document, error) {
	location := item.Path
	documents, err := e.productDocuments(ctx, ref, productRef, plan.Selection{Source: name}, item)
	if err != nil {
		return brief.Document{}, err
	}
	if len(documents) == 0 {
		if !item.CanWrite() {
			return brief.Document{Path: location, Content: "Read-only source; no documents were found. Record improvements as lessons for its maintainers."}, nil
		}
		return brief.Document{Path: location, Content: "not created yet; create it at " + location}, nil
	}
	notice := ""
	if !item.CanWrite() {
		notice = "Read-only source; do not edit it or write it back. Record improvements as lessons for its maintainers.\n\n"
	}
	if len(documents) == 1 {
		return brief.Document{Path: location, Content: notice + documents[0].Body, Exists: true}, nil
	}
	var text strings.Builder
	text.WriteString(notice)
	for _, document := range documents {
		fmt.Fprintf(&text, "## %s/%s\n\n%s\n\n", name, document.ID, document.Body)
	}
	return brief.Document{Path: location, Content: text.String(), Exists: true}, nil
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

	outputs = append(outputs, completedStageOutputs(state, records)...)
	return outputs, nil
}
func originalRequest(work *ledger.Work) string {
	if work.OriginalRequest == nil {
		return ""
	}
	return *work.OriginalRequest
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
	if data.Context != nil {
		meta.Context = data.Context
	}
	// The detection specification and mapping stay in the ledger, not next to the brief.
	published := data
	published.Detection = nil
	body, err := json.Marshal(published)
	if err != nil {
		return Result{}, err
	}
	if err = e.store().Publish("brief/"+i.ID+".json", body); err != nil {
		return Result{}, err
	}
	if fresh {
		if err := e.recordInstruction(meta, data); err != nil {
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
		case form.InstructionUnitWork:
			i.Text = "Launch the worker with: devkit-instruction " + id
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
		return instructionData{Instruction: i, Reference: meta.Reference, Origin: meta.Origin, TemplateName: meta.TemplateName, Request: meta.Request, Detection: meta.Detection, Context: meta.Context, Inputs: meta.Inputs}, nil
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
	case form.InstructionWork, form.InstructionUnitWork:
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
	if ref, err = e.refreshSources(ctx, ref); err != nil {
		return Result{}, err
	}
	name := t.Name
	if t, source, err = e.template(ctx, ref, name); err != nil {
		return Result{}, err
	}
	if t.Name != name {
		return Result{}, errors.New("template name does not match its file name")
	}
	b, err := e.plannerBrief(ctx, state, t, source, version, origin, ref, &meta)
	if err != nil {
		return Result{}, err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	runner, err := e.runnerAgent(ctx, ref, settings.PlanningRole())
	if err != nil {
		return Result{}, err
	}
	return e.publishInstruction(meta, fresh, instructionData{Instruction: form.Instruction{Runner: runner, Text: "Launch the planner with: devkit-instruction " + meta.ID}, Reference: ref, Origin: origin, TemplateName: t.Name}, b.Text)
}

func (e *Engine) plannerBrief(ctx context.Context, state *ledger.State, t plan.PlanTemplate, source []byte, version int, origin plan.ProposalOrigin, ref string, meta *ledger.Instruction) (brief.Brief, error) {
	template, err := e.asset(ctx, ref, "templates/brief/planner.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	paths, err := e.normPaths(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, ref, t.PlannerSelections())
	if err != nil {
		return brief.Brief{}, err
	}
	definitions := []brief.Document{}
	for _, stage := range append(slices.Clone(t.Stages), t.OptionalStages...) {
		data, err := e.asset(ctx, ref, "stages/"+stage.ID+".md")
		if err != nil {
			return brief.Brief{}, err
		}
		definitions = append(definitions, brief.Document{Path: "stages/" + stage.ID + ".md", Content: string(data)})
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
	docs, err := e.additionalInputs(ctx, ref, ref, t.PlannerSelections())
	if err != nil {
		return brief.Brief{}, err
	}
	input := brief.PlannerInput{RequestTemplate: string(template), InstructionID: meta.ID, Version: version, Template: t, TemplateHash: plan.ContentHash(source), Task: string(task), OriginalRequest: originalRequest(work), StageDefinitions: definitions, Repository: facts, Norms: docs, NormPaths: paths, PreviousOutputs: outputs, OutputSchema: string(outputSchema)}
	if p, ok := latestPlan(state); ok {
		input.CurrentPlan = &p.Plan
		if origin == plan.BackProposal {
			remaining, err := e.remainingPlan(state)
			if err != nil {
				return brief.Brief{}, err
			}
			input.RemainingPlan = remaining
		}
	}
	if err := e.plannerHistory(state, &input); err != nil {
		return brief.Brief{}, err
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
	return e.publishInstruction(meta, fresh, instructionData{Instruction: form.Instruction{Text: "Perform the work described in the brief, then run devkit submit."}, Reference: roundReference(round)}, b.Text)
}

func (e *Engine) workBrief(ctx context.Context, state *ledger.State, p plan.Plan, stage plan.Stage, round *ledger.Round, meta *ledger.Instruction) (brief.Brief, error) {
	stage.Assignee = roundAssignee(stage, round)
	ref := roundReference(round)
	name := "templates/brief/work.md.tmpl"
	if meta.Unit != nil {
		name = "templates/brief/unit.md.tmpl"
	}
	template, err := e.asset(ctx, ref, name)
	if err != nil {
		return brief.Brief{}, err
	}
	definition, err := e.asset(ctx, ref, "stages/"+stage.ID+".md")
	if err != nil {
		return brief.Brief{}, err
	}
	paths, err := e.normPaths(ctx, ref)
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, ref, stage.WorkerInputs(), round.StartTarget.Tree)
	if err != nil {
		return brief.Brief{}, err
	}
	facts.AvailableChecks = stage.Checks
	docs, err := e.additionalInputs(ctx, ref, round.StartTarget.Tree, stage.WorkerInputs())
	if err != nil {
		return brief.Brief{}, err
	}
	outputs, err := e.outputs(state)
	if err != nil {
		return brief.Brief{}, err
	}
	outputSchema, err := e.asset(ctx, ref, "schemas/progress.json")
	if err != nil {
		return brief.Brief{}, err
	}
	input := brief.WorkInput{RequestTemplate: string(template), InstructionID: meta.ID, Stage: stage, StageDefinition: string(definition), Plan: p, Purpose: p.Task, Repository: facts, Norms: docs, NormPaths: paths, PreviousOutputs: outputs, PreviousFindings: state.UnresolvedBlocking(round.StageRunID), OutputSchema: string(outputSchema)}
	input.Unit, input.WorktreePath = meta.Unit, e.unitPath(meta.ID)
	if meta.Unit != nil {
		input.WorktreePath = meta.TargetPath
	}
	for _, condition := range p.CompletionConditions {
		input.Verification = append(input.Verification, condition.ID+": "+condition.Description)
	}
	commands, err := e.workCommandOutputs(state, round.StageRunID, &input)
	if err != nil {
		return brief.Brief{}, err
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
	history, err := e.workHistory(state, meta)
	if err != nil {
		return brief.Brief{}, err
	}
	input.WorkHistoryPath = history
	for _, name := range []string{"decision", "evidence", "lesson", "structure"} {
		source := e.schemas[name]
		if name == "lesson" {
			source = []byte(`{"type":"object","properties":{"content":{"type":"string","minLength":1},"evidence":{"type":"array","items":{"$ref":"urn:devkit:form:reference"}}},"required":["content","evidence"],"additionalProperties":false}`)
		}
		bundled, err := e.bundledSchema(source)
		if err != nil {
			return brief.Brief{}, err
		}
		input.SupportingSchemas = append(input.SupportingSchemas, brief.Document{Path: name, Content: string(bundled)})
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
	data, err := kit.RouterDefaults()
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
	destinations := []router.LessonDestination{}
	for _, destination := range settings.LessonDestinations() {
		destinations = append(destinations, router.LessonDestination(destination))
	}
	table = router.MergeTables(table, router.Table{HumanConfirmation: settings.HumanConfirmation(), Lessons: router.Lessons{Destinations: destinations}})
	mode := router.ConfirmNow
	if settings.MetaLoopEditApproval() == "allow" {
		mode = router.NoConfirm
	}
	table.ConfirmationRules[config.MetaLoopEdit] = mode
	return table, nil
}

// question builds a devkit-generated question. Devkit has no grounds to recommend an answer, so the recommendation is left empty.
func (e *Engine) question(target form.ApprovalTarget, category, text, stage string) form.Question {
	return form.Question{ID: ulidString(), Text: text, ApprovalTarget: target, Options: []string{"approve", "reject", "defer"}, Recommendation: "", StopCategory: category, NextStage: stage, StoppedStage: stage, StopReason: text, Evidence: []form.Reference{}}
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

func completedStageOutputs(state *ledger.State, records []ledger.Record) []brief.StageOutput {
	var outputs []brief.StageOutput
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
	return outputs
}

func (e *Engine) recordInstruction(meta ledger.Instruction, data instructionData) error {
	meta.Reference, meta.Runner, meta.TargetPath = data.Reference, data.Instruction.Runner, data.Instruction.TargetPath
	meta.TemplateName, meta.Origin = data.TemplateName, data.Origin
	meta.Inputs = slices.Clone(data.Inputs)
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
			return err
		}
		if round, ok := state.OpenRound(); ok && round.ID == meta.RoundID && round.SessionID != e.sessionID {
			copy := *round
			copy.SessionID = e.sessionID
			if _, err = e.append(&copy); err != nil {
				return err
			}
		}
	}
	if err := e.active(); err != nil {
		return err
	}
	if _, err := e.append(&meta); err != nil {
		return err
	}
	return nil
}

func (e *Engine) plannerHistory(state *ledger.State, input *brief.PlannerInput) error {
	records, err := e.records()
	if err != nil {
		return err
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
	return nil
}

func (e *Engine) workCommandOutputs(state *ledger.State, stageRunID string, input *brief.WorkInput) ([]check.Judgment, error) {
	commands := []check.Judgment{}
	records, err := e.records()
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		switch value := r.Payload.(type) {
		case *ledger.Judgment:
			if value.Command != nil {
				for _, previous := range state.Rounds(stageRunID) {
					if previous.ID == value.RoundID {
						input.CommandOutputs = append(input.CommandOutputs, brief.CommandOutput{Check: plan.Check{Aspect: value.Aspect, Runner: plan.Runner(value.Runner)}, ExitCode: value.Command.ExitCode, Output: value.Command.Output})
						commands = append(commands, value.Judgment)
					}
				}
			}
		case *ledger.StageRun:
			if value.ID == stageRunID && value.StopReason != "" {
				input.PreCommitOutput = value.StopReason
			}
		}
	}
	return commands, nil
}
