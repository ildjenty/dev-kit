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

type detectionInput struct {
	Spec     check.DetectionSpec     `json:"spec"`
	Identity check.DetectionIdentity `json:"identity"`
	Examples map[string]string       `json:"examples"`
}

func (e *Engine) detectionInput(ctx context.Context, ref string, item plan.Check) (detectionInput, error) {
	expanded, err := e.expanded(ctx, ref)
	if err != nil {
		return detectionInput{}, err
	}
	base := ".devkit/checks/" + item.Aspect
	aspectPath := base + ".md"
	specPath := base + "/spec.json"
	if _, err = os.Stat(filepath.Join(expanded, aspectPath)); errors.Is(err, os.ErrNotExist) {
		aspectPath = e.kitPath("checks/" + item.Aspect + ".md")
		specPath = e.kitPath("examples/" + item.Aspect + "/spec.json")
	} else if err != nil {
		return detectionInput{}, err
	}
	aspect, err := e.workspace.ReadFile(ctx, ref, aspectPath)
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
	settings, err := e.settings(ctx, ref)
	if err != nil {
		return detectionInput{}, err
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
	identity, err := check.NewDetectionIdentity(aspect, specSource, runner, examples)
	return detectionInput{Spec: spec, Identity: identity, Examples: texts}, err
}
func (e *Engine) detectionRecords(ctx context.Context, ref string) ([]check.DetectionTest, error) {
	path, err := e.expanded(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(path, ".devkit/detect.json"))
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
func (e *Engine) ensureDetection(ctx context.Context, state *ledger.State, items []plan.Check, ref string) (Result, bool, error) {
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
	if !fresh {
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
			q := e.question(form.ApprovalTarget{Kind: form.ApprovalTree, Tree: data.Request.TargetVersion.Tree}, "detection_failed", "The detection test failed. Revise the aspect or propose a plan without this required check.", "")
			q, err = e.saveQuestion(state, q, "")
			if err != nil {
				return Result{}, false, err
			}
			r, err := e.issueQuestion(state, q)
			return r, true, err
		}
		r, err := e.publishInstruction(meta, false, data, "")
		if err != nil {
			return Result{}, false, err
		}
		return e.executeDetection(ctx, state, meta, data, r)
	}
	aspect, err := e.aspect(ctx, ref, item.Aspect)
	if err != nil {
		return Result{}, false, err
	}
	norms, _, err := e.normDocuments(ctx, ref)
	if err != nil {
		return Result{}, false, err
	}
	path, err := e.expanded(ctx, ref)
	if err != nil {
		return Result{}, false, err
	}
	tree, err := e.workspace.CommitTree(ctx, ref)
	if err != nil {
		tree = ref
	}
	output := e.schemas["detection-result"]
	request, err := check.PrepareRequest(check.RequestInput{Kind: check.DetectionRequest, InstructionID: meta.ID, Runner: string(item.Runner), Aspect: aspect, TargetVersion: form.TargetVersion{Kind: form.TargetTree, Tree: tree}, TargetPath: path, Norms: norms, OutputSchema: output, CompletionConditionIDs: []string{}})
	if err != nil {
		return Result{}, false, err
	}
	data := instructionData{Reference: ref, Instruction: form.Instruction{ID: meta.ID, Kind: form.InstructionDetectionTest, Runner: string(item.Runner), TargetPath: path, Text: "Run the detection test with the assigned judge: devkit-instruction " + meta.ID}, Request: &request, Detection: &input}
	template, err := e.asset(ctx, ref, "templates/brief/judgment.md.tmpl")
	if err != nil {
		return Result{}, false, err
	}
	content, err := json.MarshalIndent(struct {
		Request   check.Request  `json:"request"`
		Detection detectionInput `json:"detection"`
	}{request, input}, "", "  ")
	if err != nil {
		return Result{}, false, err
	}
	text := "Evaluate each example independently against the aspect. Return an observation for each file. For violations, explain whether the blocking finding detects the intended violation; for conforming and allowed cases, explain why the relevant difference is respected. Each nested judgment must use the instruction ID, aspect, target, and norms from the request.\n\n```json\n" + string(content) + "\n```"
	b, err := brief.BuildJudgment(brief.JudgmentInput{RequestTemplate: string(template), InstructionID: meta.ID, Content: text, OutputSchema: string(output)})
	if err != nil {
		return Result{}, false, err
	}
	r, err := e.publishInstruction(meta, true, data, b.Text)
	if err != nil {
		return Result{}, false, err
	}
	return e.executeDetection(ctx, state, meta, data, r)
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
		encoded, _ := json.Marshal(data)
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
		observations = append(observations, check.DetectionObservation{File: observation.File, Judgment: judgment, Reason: observation.Reason, MatchesBlocking: observation.MatchesBlocking, RespectsDifference: observation.RespectsDifference})
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
	return fsstore.New(e.Root()).Publish(".devkit/detect.json", append(encoded, '\n'))
}
