package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

type detectionInput = ledger.InstructionDetection

func (e *Engine) detectionInput(ctx context.Context, ref string, item plan.Check) (detectionInput, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return detectionInput{}, err
	}
	base := settings.RepositoryPaths().Checks + "/" + item.Aspect
	specPath := base + "/spec.json"
	aspect, err := e.asset(ctx, ref, "checks/"+item.Aspect+".md")
	if err != nil {
		return detectionInput{}, err
	}
	specSource, err := e.workspace.ReadFile(ctx, ref, specPath)
	if err != nil {
		return detectionInput{}, err
	}
	spec, err := check.ParseDetectionSpec(specSource)
	if err != nil {
		return detectionInput{}, err
	}
	if spec.Aspect != item.Aspect {
		return detectionInput{}, errors.New("detection specification aspect does not match")
	}
	examples := map[string][]byte{}
	texts := map[string]string{}
	for _, example := range spec.Cases {
		if !filepath.IsLocal(example.File) {
			return detectionInput{}, errors.New("detection example path must be relative")
		}
		data, err := e.workspace.ReadFile(ctx, ref, filepath.ToSlash(filepath.Join(filepath.Dir(specPath), example.File)))
		if err != nil {
			return detectionInput{}, err
		}
		examples[example.File] = data
		texts[example.File] = string(data)
	}
	var runner []byte
	if command, ok := settings.Judges[string(item.Runner)]; ok {
		runner, err = json.Marshal(command)
	} else {
		obs, obsErr := hook.ReadSession(e.runRoot(), e.sessionID)
		if obsErr != nil {
			return detectionInput{}, obsErr
		}
		if obs == nil {
			return detectionInput{}, errors.New("session observation is required for detection tests")
		}
		path := ".claude/agents/" + string(item.Runner) + ".md"
		if obs.Host == "codex" {
			path = ".codex/agents/" + string(item.Runner) + ".toml"
		}
		runner, err = e.workspace.ReadFile(ctx, ref, path)
	}
	if err != nil {
		return detectionInput{}, err
	}
	references := map[string][]byte{}
	for _, name := range check.AspectReferences(string(aspect)) {
		references[name], err = e.asset(ctx, ref, name)
		if err != nil {
			return detectionInput{}, err
		}
	}
	identity, err := check.NewDetectionIdentity(aspect, specSource, runner, examples, references)
	return detectionInput{Spec: spec, Identity: identity, Examples: texts}, err
}
func (e *Engine) detectionRecords(ctx context.Context, ref string) ([]check.DetectionTest, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(e.Root(), settings.RepositoryPaths().Detect))
	if errors.Is(err, os.ErrNotExist) {
		return []check.DetectionTest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var records []check.DetectionTest
	if err = json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("detection index: %w", err)
	}
	return records, nil
}
func (e *Engine) detectionContext(ctx context.Context, items []plan.Check, ref string) ([]check.DetectionRequirement, []check.DetectionTest, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	if !settings.DetectionRequired() {
		return nil, nil, nil
	}
	requirements := []check.DetectionRequirement{}
	for _, item := range items {
		if item.Runner.IsCommand() {
			continue
		}
		input, err := e.detectionInput(ctx, ref, item)
		if err != nil {
			return nil, nil, err
		}
		requirements = append(requirements, check.DetectionRequirement{Aspect: item.Aspect, Runner: string(item.Runner), Identity: input.Identity})
	}
	records, err := e.detectionRecords(ctx, ref)
	return requirements, records, err
}
func (e *Engine) ensureDetection(ctx context.Context, state *ledger.State, items []plan.Check, ref string, stageRunID ...string) (Result, bool, error) {
	requirements, records, err := e.detectionContext(ctx, items, ref)
	if err != nil {
		return Result{}, false, err
	}
	missing := check.MissingDetection(requirements, records)
	if len(missing) == 0 {
		return Result{}, false, nil
	}
	requirement := missing[0]
	item := plan.Check{Aspect: requirement.Aspect, Runner: plan.Runner(requirement.Runner), Required: true}
	input, err := e.detectionInput(ctx, ref, item)
	if err != nil {
		return Result{}, false, err
	}
	identity, _ := json.Marshal(input.Identity)
	key := requirement.Aspect + ":" + requirement.Runner + ":" + hash(identity)
	meta, fresh, err := e.issuance(form.InstructionDetectionTest, key, nil, 0, "")
	if err != nil {
		return Result{}, false, err
	}
	if len(stageRunID) > 0 {
		meta.StageRunID = stageRunID[0]
	}
	if !fresh {
		return e.resumeDetection(ctx, state, meta)
	}
	aspect, err := e.aspect(ctx, ref, item.Aspect)
	if err != nil {
		return Result{}, false, err
	}
	norms, _, err := e.normDocuments(ctx, ref)
	if err != nil {
		return Result{}, false, err
	}
	path := filepath.Join(e.workRoot(), "brief", meta.ID+".md")
	tree, err := e.workspace.CommitTree(ctx, ref)
	if err != nil {
		tree = ref
	}
	if err := e.requestNorms(ctx, ref, plan.Check{Aspect: item.Aspect, Runner: item.Runner, Target: "detection"}, norms); err != nil {
		return Result{}, false, err
	}
	output, err := e.bundledSchema(e.schemas["detection-result"])
	if err != nil {
		return Result{}, false, err
	}
	evidence, err := e.judgmentEvidence(ctx, &meta, state, nil)
	if err != nil {
		return Result{}, false, err
	}
	if len(norms) > 0 {
		norms[0].Content += "\n" + evidence
	}
	request, err := check.PrepareRequest(check.RequestInput{Kind: check.DetectionRequest, InstructionID: meta.ID, Runner: string(item.Runner), Aspect: aspect, TargetVersion: form.TargetVersion{Kind: form.TargetTree, Tree: tree}, TargetPath: path, Norms: norms, OutputSchema: output, CompletionConditionIDs: []string{}})
	if err != nil {
		return Result{}, false, err
	}
	data := instructionData{Reference: ref, Context: meta.Context, Instruction: form.Instruction{ID: meta.ID, Kind: form.InstructionDetectionTest, Runner: string(item.Runner), TargetPath: path, Text: "Run the detection test with the assigned judge: devkit-instruction " + meta.ID}, Request: &request, Detection: &input}
	b, err := e.detectionBrief(ctx, data)
	if err != nil {
		return Result{}, false, err
	}
	r, err := e.publishInstruction(meta, true, data, b.Text)
	if err != nil {
		return Result{}, false, err
	}
	return e.executeDetection(ctx, state, meta, data, r)
}
func (e *Engine) detectionBrief(ctx context.Context, data instructionData) (brief.Brief, error) {
	if err := e.hydrateRequest(ctx, data); err != nil {
		return brief.Brief{}, err
	}
	template, err := e.asset(ctx, data.Reference, "templates/brief/judgment.md.tmpl")
	if err != nil {
		return brief.Brief{}, err
	}
	request, err := detectionRequest(data)
	if err != nil {
		return brief.Brief{}, err
	}
	content, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return brief.Brief{}, err
	}
	text := "Evaluate each example body independently. Return an observation for each opaque example ID in the file field, with the actual judgment and grounds. Each nested judgment must use the instruction ID, aspect, target, and norms from the request.\n\n```json\n" + string(content) + "\n```"
	schema, err := e.asset(ctx, data.Reference, "schemas/detection-result.json")
	if err != nil {
		return brief.Brief{}, err
	}
	facts, err := e.repository(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	docs, err := e.aspectDocuments(ctx, data.Reference, []plan.Check{{Aspect: request.Aspect.ID, Runner: plan.Runner(request.Runner)}})
	if err != nil {
		return brief.Brief{}, err
	}
	paths, err := e.normPaths(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	// Detection inputs contain only the aspect and its referenced principles and rules.
	facts.LayoutGuides = nil
	if facts.Requirements.Exists {
		facts.Requirements.Content = ""
	}
	return brief.BuildJudgment(brief.JudgmentInput{RequestTemplate: string(template), InstructionID: data.Instruction.ID, Content: text, OutputSchema: string(schema), Repository: facts, Norms: docs, NormPaths: paths})
}

func detectionRequest(data instructionData) (check.Request, error) {
	request := *data.Request
	examples, err := check.PrepareDetectionExamples(data.Detection.Spec, data.Detection.Examples)
	request.DetectionExamples = examples
	return request, err
}
func (e *Engine) executeDetection(ctx context.Context, state *ledger.State, meta ledger.Instruction, data instructionData, r Result) (Result, bool, error) {
	settings, err := e.settings(ctx, data.Reference)
	if err != nil {
		return Result{}, false, err
	}
	if command, ok := settings.Judges[data.Instruction.Runner]; ok {
		if err := e.hydrateRequest(ctx, data); err != nil {
			return Result{}, false, err
		}
		expected, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, false, err
		}
		request, err := detectionRequest(data)
		if err != nil {
			return Result{}, false, err
		}
		encoded, err := json.Marshal(request)
		if err != nil {
			return Result{}, false, err
		}
		result, stdout, err := e.runCommand(ctx, command.Command, "", encoded)
		if err != nil {
			return Result{}, false, err
		}
		matches, err := e.matches(ctx, expected)
		if err != nil {
			return Result{}, false, err
		}
		if !matches || result.ExitCode != 0 || result.TimedOut {
			q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: expected}, "detection_command_failed", "The detection command failed or changed its target.\n"+result.Output, "")
			q, err = e.saveQuestion(state, q, "")
			if err != nil {
				return Result{}, false, err
			}
			r, err = e.issueQuestion(state, q)
			return r, true, err
		}
		if err = e.acceptDetection(ctx, state, meta, data, stdout, check.Provenance{Source: check.JudgeCommand, InstructionID: meta.ID, Runner: data.Instruction.Runner}); err != nil {
			return Result{}, false, err
		}
		state, err = e.load()
		if err != nil {
			return Result{}, false, err
		}
		r = response(state)
	}
	return r, true, nil
}
func (e *Engine) acceptDetection(ctx context.Context, state *ledger.State, meta ledger.Instruction, data instructionData, source []byte, provenance check.Provenance) error {
	schema := e.forms["detection-result"]
	var err error
	var result form.DetectionResult
	if err = schema.Decode(source, &result); err != nil {
		return err
	}
	if result.InstructionID != meta.ID {
		return errors.New("detection instruction ID does not match")
	}
	observations := []check.DetectionObservation{}
	for _, observation := range result.Observations {
		raw, err := json.Marshal(observation.Judgment)
		if err != nil {
			return err
		}
		judgment, _, err := check.AcceptResult(*data.Request, raw, e.forms["judgment"], provenance, nil)
		if err != nil {
			return err
		}
		observations = append(observations, check.DetectionObservation{File: observation.File, Judgment: judgment, Reason: observation.Reason})
	}
	test, err := check.AssessDetection(data.Detection.Spec, data.Instruction.Runner, data.Detection.Identity, observations)
	if err != nil {
		return err
	}
	if _, err = e.append(&ledger.DetectionTest{DetectionTest: test, InstructionID: meta.ID}); err != nil {
		return err
	}
	if !test.Passed {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: data.Request.TargetVersion.Tree}, "detection_failed", "The detection test failed. Revise the aspect or propose a plan without this required check.", "")
		_, err = e.saveQuestion(state, q, "")
		return err
	}
	return e.publishDetection(ctx, test)
}

func (e *Engine) publishDetection(ctx context.Context, test check.DetectionTest) error {
	ref, err := e.workspace.SnapshotTree(ctx)
	if err != nil {
		return err
	}
	records, err := e.detectionRecords(ctx, ref)
	if err != nil {
		return err
	}
	records = append(records, test)
	encoded, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return err
	}
	return fsstore.New(e.Root()).Publish(settings.RepositoryPaths().Detect, append(encoded, '\n'))
}

func (e *Engine) resumeDetection(ctx context.Context, state *ledger.State, meta ledger.Instruction) (Result, bool, error) {
	data, err := e.readInstruction(meta.ID)
	if err != nil {
		return Result{}, false, err
	}
	pending := false
	for _, i := range state.UnacceptedInstructions() {
		if i.ID == meta.ID {
			pending = true
		}
	}
	if !pending {
		return e.restoreDetection(ctx, state, meta.ID, data.Request)
	}
	r, err := e.publishInstruction(meta, false, data, "")
	if err != nil {
		return Result{}, false, err
	}
	return e.executeDetection(ctx, state, meta, data, r)
}

func (e *Engine) restoreDetection(ctx context.Context, state *ledger.State, instructionID string, request *check.Request) (Result, bool, error) {
	recorded, err := e.records()
	if err != nil {
		return Result{}, false, err
	}
	var test *check.DetectionTest
	for _, record := range recorded {
		if result, ok := record.Payload.(*ledger.DetectionTest); ok && result.InstructionID == instructionID {
			copy := result.DetectionTest
			test = &copy
		}
	}
	if test == nil {
		return Result{}, false, errors.New("accepted detection result is missing")
	}
	if test.Passed {
		return Result{}, true, e.publishDetection(ctx, *test)
	}
	q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: request.TargetVersion.Tree}, "detection_failed", "The detection test failed. Revise the aspect or propose a plan without this required check.", "")
	q, err = e.saveQuestion(state, q, "")
	if err != nil {
		return Result{}, false, err
	}
	r, err := e.issueQuestion(state, q)
	return r, true, err
}
