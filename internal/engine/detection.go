package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/hook"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
	"github.com/ildjenty/dev-kit/internal/source"
)

type detectionInput = ledger.InstructionDetection

// detectionInput reads the specification and examples selected by the check, resolves its
// criteria, and derives the pass identity from them, the runner, and the request template.
func (e *Engine) detectionInput(ctx context.Context, ref string, item plan.Check) (detectionInput, []check.Criterion, error) {
	if item.Examples == nil {
		return detectionInput{}, nil, fmt.Errorf("judge-run check %q selects no detection examples; select examples or make the check not required", item.Aspect)
	}
	spec, specDocument, examples, err := e.detectionExamples(ctx, ref, *item.Examples)
	if err != nil {
		return detectionInput{}, nil, err
	}
	if spec.Aspect != item.Aspect {
		return detectionInput{}, nil, errors.New("detection specification aspect does not match")
	}
	criteria, err := e.criteria(ctx, ref, item.Criteria)
	if err != nil {
		return detectionInput{}, nil, err
	}
	runner, err := e.runnerDefinition(ctx, ref, item.Runner)
	if err != nil {
		return detectionInput{}, nil, err
	}
	template, err := e.asset(ctx, ref, "templates/brief/judgment.md.tmpl")
	if err != nil {
		return detectionInput{}, nil, err
	}
	texts := map[string]string{}
	for file, body := range examples {
		texts[file] = string(body)
	}
	identity, err := check.NewDetectionIdentity(runner, template, criteria, []byte(specDocument.Body), examples)
	return detectionInput{Check: item, Spec: spec, Identity: identity, Examples: texts}, criteria, err
}

// detectionExamples reads the selected specification and the example documents it names,
// relative to the specification, from the same source.
func (e *Engine) detectionExamples(ctx context.Context, ref string, selection plan.Selection) (check.DetectionSpec, source.Document, map[string][]byte, error) {
	specs, err := e.selectDocuments(ctx, ref, selection.Source, selection.Select)
	if err != nil {
		return check.DetectionSpec{}, source.Document{}, nil, err
	}
	if len(specs) != 1 {
		return check.DetectionSpec{}, source.Document{}, nil, fmt.Errorf("examples selection %q from source %q must select one specification; it selected %d", selection.Select, selection.Source, len(specs))
	}
	spec, err := check.ParseDetectionSpec([]byte(specs[0].Body))
	if err != nil {
		return check.DetectionSpec{}, source.Document{}, nil, err
	}
	examples := map[string][]byte{}
	for _, example := range spec.Cases {
		if !filepath.IsLocal(example.File) || strings.Contains(example.File, "\\") {
			return check.DetectionSpec{}, source.Document{}, nil, errors.New("detection example path must be relative")
		}
		document, err := e.document(ctx, ref, selection.Source, path.Join(path.Dir(specs[0].ID), example.File))
		if err != nil {
			return check.DetectionSpec{}, source.Document{}, nil, err
		}
		examples[example.File] = []byte(document.Body)
	}
	return spec, specs[0], examples, nil
}

func (e *Engine) runnerDefinition(ctx context.Context, ref string, runner plan.Runner) ([]byte, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	if command, ok := settings.Judges[string(runner)]; ok {
		return json.Marshal(command)
	}
	obs, err := hook.ReadSession(e.runRoot(), e.sessionID)
	if err != nil {
		return nil, err
	}
	if obs == nil {
		return nil, errors.New("session observation is required for detection tests")
	}
	name := ".claude/agents/" + string(runner) + ".md"
	if obs.Host == "codex" {
		name = ".codex/agents/" + string(runner) + ".toml"
	}
	return e.workspace.ReadFile(ctx, gitReference(ref), name)
}

func (e *Engine) detectionRecords(ctx context.Context, ref string) ([]check.DetectionTest, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(e.Root(), filepath.FromSlash(settings.DetectionPath())))
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
		input, _, err := e.detectionInput(ctx, ref, item)
		if err != nil {
			return nil, nil, err
		}
		requirements = append(requirements, check.DetectionRequirement{Aspect: item.Aspect, Runner: string(item.Runner), Identity: input.Identity})
	}
	records, err := e.detectionRecords(ctx, ref)
	return requirements, records, err
}
func (e *Engine) ensureDetection(ctx context.Context, state *ledger.State, items []plan.Check, ref string, stageRunID ...string) (Result, bool, error) {
	settings, err := e.settings(ctx, ref)
	if err != nil || !settings.DetectionRequired() {
		return Result{}, false, err
	}
	records, err := e.detectionRecords(ctx, ref)
	if err != nil {
		return Result{}, false, err
	}
	for _, item := range items {
		if item.Runner.IsCommand() {
			continue
		}
		input, criteria, err := e.detectionInput(ctx, ref, item)
		if err != nil {
			return Result{}, false, err
		}
		requirement := []check.DetectionRequirement{{Aspect: item.Aspect, Runner: string(item.Runner), Identity: input.Identity}}
		if len(check.MissingDetection(requirement, records)) == 0 {
			continue
		}
		return e.issueDetection(ctx, state, input, criteria, ref, stageRunID...)
	}
	return Result{}, false, nil
}

// issueDetection requests a detection test. The request holds only the criteria, the anonymous
// example bodies, and the output contract; it carries no norms, work history, or specification.
func (e *Engine) issueDetection(ctx context.Context, state *ledger.State, input detectionInput, criteria []check.Criterion, ref string, stageRunID ...string) (Result, bool, error) {
	item := input.Check
	identity, _ := json.Marshal(input.Identity)
	key := item.Aspect + ":" + string(item.Runner) + ":" + hash(identity)
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
	path := filepath.Join(e.workRoot(), "brief", meta.ID+".md")
	tree, err := e.workspace.CommitTree(ctx, gitReference(ref))
	if err != nil {
		tree = gitReference(ref)
	}
	output, err := e.bundledSchema(e.schemas["detection-result"])
	if err != nil {
		return Result{}, false, err
	}
	request, err := check.PrepareRequest(check.RequestInput{Kind: check.DetectionRequest, InstructionID: meta.ID, Runner: string(item.Runner), Aspect: check.Aspect{ID: item.Aspect}, Criteria: criteria, TargetVersion: form.TargetVersion{Kind: form.TargetTree, Tree: tree}, TargetPath: path, Norms: []check.Norm{}, OutputSchema: output, CompletionConditionIDs: []string{}})
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

// detectionBrief gives the judge only the criteria documents, the anonymous example bodies,
// and the output contract.
func (e *Engine) detectionBrief(ctx context.Context, data instructionData) (brief.Brief, error) {
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
	criteria, err := e.criteriaDocuments(ctx, data.Reference, request.Criteria)
	if err != nil {
		return brief.Brief{}, err
	}
	supporting, err := e.judgmentSchemas(ctx, data.Reference)
	if err != nil {
		return brief.Brief{}, err
	}
	return brief.BuildJudgment(brief.JudgmentInput{RequestTemplate: string(template), InstructionID: data.Instruction.ID, Content: text, SupportingSchemas: supporting, Criteria: criteria, Detection: true, OutputSchema: string(schema), Repository: brief.RepositoryFacts{LayoutGuides: []brief.Document{}}, Norms: []brief.Document{}, NormPaths: []brief.NormPath{}})
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
		expected, err := e.workspace.SnapshotTree(ctx)
		if err != nil {
			return Result{}, false, err
		}
		request, err := detectionRequest(data)
		if err != nil {
			return Result{}, false, err
		}
		encoded, err := e.judgmentCommandInput(ctx, data, request)
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
	test, err := e.validateDetection(meta, data, source, provenance)
	if err != nil {
		return err
	}
	return e.recordDetection(ctx, state, meta, data, test)
}

func (e *Engine) validateDetection(meta ledger.Instruction, data instructionData, source []byte, provenance check.Provenance) (check.DetectionTest, error) {
	schema := e.forms["detection-result"]
	var err error
	var result form.DetectionResult
	if err = schema.Decode(source, &result); err != nil {
		return check.DetectionTest{}, err
	}
	if result.InstructionID != meta.ID {
		return check.DetectionTest{}, errors.New("detection instruction ID does not match")
	}
	observations := []check.DetectionObservation{}
	for _, observation := range result.Observations {
		raw, err := json.Marshal(observation.Judgment)
		if err != nil {
			return check.DetectionTest{}, err
		}
		judgment, _, err := check.AcceptResult(*data.Request, raw, e.forms["judgment"], provenance, nil)
		if err != nil {
			return check.DetectionTest{}, err
		}
		observations = append(observations, check.DetectionObservation{File: observation.File, Judgment: judgment, Reason: observation.Reason})
	}
	test, err := check.AssessDetection(data.Detection.Spec, data.Instruction.Runner, data.Detection.Identity, observations)
	if err != nil {
		return check.DetectionTest{}, err
	}
	return test, nil
}

func (e *Engine) recordDetection(ctx context.Context, state *ledger.State, meta ledger.Instruction, data instructionData, test check.DetectionTest) error {
	if _, err := e.append(&ledger.DetectionTest{DetectionTest: test, InstructionID: meta.ID}); err != nil {
		return err
	}
	if !test.Passed {
		q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: data.Request.TargetVersion.Tree}, "detection_failed", detectionFailureText(meta), "")
		_, err := e.saveQuestion(state, q, "")
		return err
	}
	return e.publishDetection(ctx, data.Reference, test)
}

func (e *Engine) publishDetection(ctx context.Context, ref string, test check.DetectionTest) error {
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
	return fsstore.New(e.Root()).Publish(filepath.FromSlash(settings.DetectionPath()), append(encoded, '\n'))
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
		return e.restoreDetection(ctx, state, meta, data)
	}
	r, err := e.publishInstruction(meta, false, data, "")
	if err != nil {
		return Result{}, false, err
	}
	return e.executeDetection(ctx, state, meta, data, r)
}

func (e *Engine) restoreDetection(ctx context.Context, state *ledger.State, meta ledger.Instruction, data instructionData) (Result, bool, error) {
	recorded, err := e.records()
	if err != nil {
		return Result{}, false, err
	}
	var test *check.DetectionTest
	for _, record := range recorded {
		if result, ok := record.Payload.(*ledger.DetectionTest); ok && result.InstructionID == meta.ID {
			copy := result.DetectionTest
			test = &copy
		}
	}
	if test == nil {
		return Result{}, false, errors.New("accepted detection result is missing")
	}
	if test.Passed {
		return Result{}, true, e.publishDetection(ctx, data.Reference, *test)
	}
	q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: data.Request.TargetVersion.Tree}, "detection_failed", detectionFailureText(meta), "")
	q, err = e.saveQuestion(state, q, "")
	if err != nil {
		return Result{}, false, err
	}
	r, err := e.issueQuestion(state, q)
	return r, true, err
}

func detectionFailureText(meta ledger.Instruction) string {
	text := "The detection test failed. Revise the check's criteria or examples."
	if meta.StageRunID == "" {
		text += " The human must commit the fix before continuing because out-of-round judgments read committed norms."
	}
	return text + " Alternatively, make this check not required in the plan template or set [detection] required = false in devkit.toml."
}
