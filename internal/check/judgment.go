package check

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
)

type JudgmentValue = form.JudgmentValue
type Finding = form.Finding
type FindingKind = form.FindingKind

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

// Provenance は hook または devkit の実行から得た来歴である。
// サブエージェントの InstructionID と Runner は hook が照合済みの値を渡す。
// Effort と Model は取得できた実測値だけを渡す。
type Provenance struct {
	Source        ResultSource `json:"source"`
	InstructionID string       `json:"instruction_id"`
	Runner        string       `json:"runner"`
	AgentID       string       `json:"agent_id,omitempty"`
	Effort        string       `json:"effort,omitempty"`
	Model         string       `json:"model,omitempty"`
	ResultPath    string       `json:"result_path,omitempty"`
}

// Judgment は受理した判定と来歴・元の内容を保持する。
type Judgment struct {
	form.JudgmentResult
	Runner     string          `json:"runner"`
	Provenance Provenance      `json:"provenance"`
	Content    json.RawMessage `json:"content,omitempty"`
	Command    *CommandResult  `json:"command,omitempty"`
}

// ValidFor は対象と規範の識別を照合する。規範は集合として扱う。
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

// AcceptResult は最初の結果だけを受理する。後着は解析せず既存の結果を返す。
// サブエージェントの場合、呼び出し側は hook が書いた結果だけを渡す。
func AcceptResult(request Request, data []byte, schema *form.Schema, provenance Provenance, accepted []Judgment) (Judgment, bool, error) {
	for _, judgment := range accepted {
		if judgment.InstructionID == request.InstructionID {
			return judgment, false, nil
		}
	}
	if provenance.Source != SubagentResult && provenance.Source != JudgeCommand {
		return Judgment{}, false, fmt.Errorf("判定結果の来歴が不正です: %q", provenance.Source)
	}
	if provenance.InstructionID != request.InstructionID || provenance.Runner != request.Runner {
		return Judgment{}, false, fmt.Errorf("判定結果の指示 ID または担当が一致しません")
	}
	if schema == nil {
		return Judgment{}, false, fmt.Errorf("判定結果のスキーマが必要です")
	}
	var result form.JudgmentResult
	if err := schema.Decode(data, &result); err != nil {
		return Judgment{}, false, err
	}
	judgment := Judgment{JudgmentResult: result, Runner: request.Runner, Provenance: provenance, Content: slices.Clone(data)}
	if result.InstructionID != request.InstructionID || result.Aspect != request.Aspect.ID || !judgment.ValidFor(request.TargetVersion, request.normReferences()) {
		return Judgment{}, false, fmt.Errorf("判定結果の指示・観点・対象・規範が一致しません")
	}
	for _, id := range result.CompletionConditionIDs {
		if !slices.Contains(request.CompletionConditionIDs, id) {
			return Judgment{}, false, fmt.Errorf("依頼にない完了条件 ID: %q", id)
		}
	}
	for _, finding := range result.Findings {
		for _, id := range finding.ResolvedFindingIDs {
			if !slices.Contains(request.PreviousFindingIDs, id) {
				return Judgment{}, false, fmt.Errorf("前回の指摘にない ID: %q", id)
			}
		}
	}
	return judgment, true, nil
}

// CommandResult は engine が照合を通した検証コマンドの実行結果である。
type CommandResult struct {
	ExitCode    int    `json:"exit_code"`
	TimedOut    bool   `json:"timed_out"`
	Output      string `json:"output"`
	Method      string `json:"method"`
	Environment string `json:"environment"`
}

// FromCommand は終了コードと期限超過を Judgment に変換する。
func FromCommand(request Request, result CommandResult) (Judgment, error) {
	if !strings.HasPrefix(request.Runner, "command:") || request.Runner == "command:" || request.Aspect.ID != request.Runner {
		return Judgment{}, fmt.Errorf("command の観点と実行主体が一致しません")
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
			ID: request.InstructionID + ":command", Blocking: true, Kind: AutoFixable,
			Scenario: fmt.Sprintf("%s が終了コード %d で失敗した。\n%s", request.Runner, result.ExitCode, result.Output),
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
