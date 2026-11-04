package check

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
)

type JudgmentValue = form.JudgmentValue
type Finding = form.Finding
type FindingKind = form.FindingKind

// FindingID identifies a finding by its aspect and its aspect-local ID.
func FindingID(aspect, id string) string {
	return strconv.Itoa(len(aspect)) + ":" + aspect + id
}

// FindingsForAspect returns aspect-local IDs for a correction request.
// Accepted findings retain their namespace in records and aggregate results.
func FindingsForAspect(aspect string, findings []Finding) []Finding {
	prefix := FindingID(aspect, "")
	result := []Finding{}
	for _, finding := range findings {
		id, ok := strings.CutPrefix(finding.ID, prefix)
		if !ok {
			continue
		}
		finding.ID = id
		finding.ResolvedFindingIDs = slices.Clone(finding.ResolvedFindingIDs)
		for i, resolved := range finding.ResolvedFindingIDs {
			finding.ResolvedFindingIDs[i] = strings.TrimPrefix(resolved, prefix)
		}
		result = append(result, finding)
	}
	return result
}

const (
	Pass          = form.Pass
	Fail          = form.Fail
	NotAssessed   = form.NotAssessed
	AutoFixable   = form.AutoFixable
	NeedsDecision = form.NeedsDecision
)

type ResultSource string

const (
	SubagentResult ResultSource = "subagent"
	JudgeCommand   ResultSource = "judge_command"
	CommandCheck   ResultSource = "command"
)

// Provenance records origin information obtained from hook or devkit execution.
// For subagents, provide InstructionID and Runner values already verified by hook.
// Provide Effort and Model only when observed values are available.
type Provenance struct {
	Source        ResultSource `json:"source"`
	InstructionID string       `json:"instruction_id"`
	Runner        string       `json:"runner"`
	AgentID       string       `json:"agent_id,omitempty"`
	Effort        string       `json:"effort,omitempty"`
	Model         string       `json:"model,omitempty"`
	ResultPath    string       `json:"result_path,omitempty"`
}

// Judgment holds an accepted result, its provenance, and its original content.
type Judgment struct {
	form.JudgmentResult
	Runner     string          `json:"runner"`
	Provenance Provenance      `json:"provenance"`
	Content    json.RawMessage `json:"content,omitempty"`
	Command    *CommandResult  `json:"command,omitempty"`
}

// ValidFor compares target and norm identities, treating norms as a set.
func (j Judgment) ValidFor(target form.TargetVersion, norms []form.NormReference) bool {
	return j.TargetVersion == target && sameSet(j.Norms, norms)
}

func sameSet[T comparable](a, b []T) bool {
	set := make(map[T]bool, len(a))
	for _, value := range a {
		set[value] = true
	}
	other := make(map[T]bool, len(b))
	for _, value := range b {
		other[value] = true
	}
	if len(set) != len(other) {
		return false
	}
	for value := range set {
		if !other[value] {
			return false
		}
	}
	return true
}

// AcceptResult accepts only the first result and returns it without parsing later arrivals.
// For subagents, the caller provides only results written by hook.
func AcceptResult(request Request, data []byte, schema *form.Schema, provenance Provenance, accepted []Judgment) (Judgment, bool, error) {
	for _, judgment := range accepted {
		if judgment.InstructionID == request.InstructionID {
			return judgment, false, nil
		}
	}
	if provenance.Source != SubagentResult && provenance.Source != JudgeCommand {
		return Judgment{}, false, fmt.Errorf("invalid judgment result source: %q", provenance.Source)
	}
	if provenance.InstructionID != request.InstructionID || provenance.Runner != request.Runner {
		return Judgment{}, false, fmt.Errorf("judgment result instruction ID or runner does not match")
	}
	if schema == nil {
		return Judgment{}, false, fmt.Errorf("judgment result schema is required")
	}
	var result form.JudgmentResult
	if err := schema.Decode(data, &result); err != nil {
		return Judgment{}, false, err
	}
	judgment := Judgment{JudgmentResult: result, Runner: request.Runner, Provenance: provenance, Content: slices.Clone(data)}
	if result.InstructionID != request.InstructionID || result.Aspect != request.Aspect.ID || !judgment.ValidFor(request.TargetVersion, request.normReferences()) {
		return Judgment{}, false, fmt.Errorf("judgment result instruction, aspect, target, or norms do not match")
	}
	for _, id := range result.CompletionConditionIDs {
		if !slices.Contains(request.CompletionConditionIDs, id) {
			return Judgment{}, false, fmt.Errorf("completion condition ID not in request: %q", id)
		}
	}
	findingIDs := make(map[string]bool, len(result.Findings))
	for _, finding := range result.Findings {
		if findingIDs[finding.ID] {
			return Judgment{}, false, fmt.Errorf("duplicate finding ID: %q", finding.ID)
		}
		findingIDs[finding.ID] = true
		if result.Judgment == Pass && finding.Blocking {
			return Judgment{}, false, fmt.Errorf("PASS judgment contains a blocking finding: %q", finding.ID)
		}
		for _, id := range finding.ResolvedFindingIDs {
			if !slices.Contains(request.PreviousFindingIDs, id) {
				return Judgment{}, false, fmt.Errorf("ID not in previous findings: %q", id)
			}
		}
	}
	if result.Judgment == Fail && !slices.ContainsFunc(result.Findings, func(f Finding) bool { return f.Blocking }) {
		return Judgment{}, false, fmt.Errorf("FAIL judgment contains no blocking findings")
	}
	for i := range judgment.Findings {
		finding := &judgment.Findings[i]
		finding.ID = FindingID(result.Aspect, finding.ID)
		for j, id := range finding.ResolvedFindingIDs {
			finding.ResolvedFindingIDs[j] = FindingID(result.Aspect, id)
		}
	}
	return judgment, true, nil
}

// CommandResult holds a verification command result validated by engine.
type CommandResult struct {
	ExitCode    int    `json:"exit_code"`
	TimedOut    bool   `json:"timed_out"`
	Output      string `json:"output"`
	Method      string `json:"method"`
	Environment string `json:"environment"`
}

// FromCommand converts an exit code and timeout status into a Judgment.
func FromCommand(request Request, result CommandResult) (Judgment, error) {
	if !strings.HasPrefix(request.Runner, "command:") || request.Runner == "command:" || request.Aspect.ID != request.Runner {
		return Judgment{}, fmt.Errorf("command aspect and runner do not match")
	}
	value := Pass
	findings := []Finding{}
	checked := []string{request.Runner}
	unchecked := []string{}
	conditions := slices.Clone(request.CompletionConditionIDs)
	if result.TimedOut {
		value = NotAssessed
		checked, unchecked = []string{}, checked
		conditions = []string{}
	} else if result.ExitCode != 0 {
		value = Fail
		findings = append(findings, Finding{
			ID: FindingID(request.Aspect.ID, request.InstructionID+":command"), Blocking: true, Kind: AutoFixable,
			Scenario: fmt.Sprintf("%s failed with exit code %d.\n%s", request.Runner, result.ExitCode, result.Output),
		})
	}
	return Judgment{
		JudgmentResult: form.JudgmentResult{
			InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion,
			Norms: request.normReferences(), CompletionConditionIDs: conditions,
			Judgment: value, Findings: findings, CheckedScope: checked, UncheckedScope: unchecked,
			Method: result.Method, Environment: result.Environment,
		},
		Runner: request.Runner, Command: &result,
		Provenance: Provenance{Source: CommandCheck, InstructionID: request.InstructionID, Runner: request.Runner},
	}, nil
}

// UnregisteredCommand returns a fixable failure for a missing verification command.
func UnregisteredCommand(request Request) (Judgment, error) {
	judgment, err := FromCommand(request, CommandResult{ExitCode: 1, Method: "command registration check", Environment: "round-start configuration"})
	if err != nil {
		return Judgment{}, err
	}
	name := strings.TrimPrefix(request.Runner, "command:")
	judgment.Command = nil
	judgment.CompletionConditionIDs = []string{}
	judgment.Findings[0].Scenario = fmt.Sprintf("Verification command `%s` is not registered. Create a verification method and register it in `[commands.%s]` in `devkit.toml`.", name, name)
	return judgment, nil
}
