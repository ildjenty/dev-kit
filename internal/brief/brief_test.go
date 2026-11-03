package brief

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func TestBuildWork(t *testing.T) {
	previous := "  前段の出力\n\n```text\n{{そのまま}}\n```\n末尾の空白  \n"
	command := "検証失敗\n  expected: A\n  actual: B\n"
	input := WorkInput{
		InstructionID:   "work-round-1",
		Stage:           plan.Stage{ID: "implement", Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}},
		StageDefinition: "合意した契約を実装する。",
		Plan: plan.Plan{
			Task:                  "利用者が保存できるようにする",
			CompletionConditions:  []plan.CompletionCondition{{ID: "C1", Description: "保存した内容を読める"}},
			IrreversibleDecisions: []string{"計画の変更困難な決定"},
		},
		Purpose:          "利用者の記録を保存する",
		AllowedChanges:   []string{"internal/save/"},
		ForbiddenChanges: []string{"公開 API"},
		Verification:     []string{"C1: 保存後に読み出し、内容を比較する"},
		PreviousOutputs:  []StageOutput{{StageID: "contract", Content: previous}},
		Repository: RepositoryFacts{
			LayoutGuides:          []Document{{Path: "internal/save/README.md", Content: "保存の配置ガイド"}},
			Rules:                 Document{Path: ".devkit/rules.md", Content: "既存の規則"},
			IrreversibleDecisions: []string{"規則の変更困難な決定"},
			AvailableChecks:       []plan.Check{{Aspect: "command:test", Runner: "command:test"}},
		},
		Norms:            []Document{{Path: "AGENTS.md", Content: "固定した指示"}},
		PreviousFindings: []form.Finding{{ID: "F1", Blocking: true, Kind: form.AutoFixable, Scenario: "保存に失敗する"}},
		CommandOutputs:   []CommandOutput{{Check: plan.Check{Aspect: "command:test", Runner: "command:test", Required: true}, ExitCode: 1, Output: command}},
		PreCommitOutput:  "pre-commit の検証失敗\n詳細\n",
		OutputSchema:     `{"type":"object","required":["instruction_id"]}`,
	}
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildWork(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildWork(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != again || got.InstructionID != input.InstructionID {
		t.Fatal("同じ入力から異なる依頼を組み立てた")
	}
	for _, want := range []string{
		previous, command, input.StageDefinition, input.Purpose, input.OutputSchema,
		"C1", "保存した内容を読める", "internal/save/", "公開 API", input.Verification[0],
		"ラウンド数 3", "進展がない回数 2", "internal/save/README.md", "保存の配置ガイド",
		".devkit/rules.md", "既存の規則", "規則の変更困難な決定", "計画の変更困難な決定",
		"AGENTS.md", "固定した指示", "F1", "保存に失敗する", "command:test", "終了コード: 1",
		input.PreCommitOutput, `"instruction_id": "work-round-1"`, "停止して確認", "作業停止の申告",
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("依頼に %q がない", want)
		}
	}
	after, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("組み立てで入力を変更した")
	}
}

func TestBuildJudgment(t *testing.T) {
	input := JudgmentInput{
		InstructionID: "judge-round-1",
		Content:       "check が選んだ本文\n  対象: snap/tree\n前回の指摘 ID: F1\n{{展開しない}}\n",
		OutputSchema:  `{"required":["instruction_id","judgment"]}`,
	}
	got, err := BuildJudgment(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildJudgment(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != again || got.InstructionID != input.InstructionID {
		t.Fatal("同じ入力から異なる依頼を組み立てた")
	}
	for _, want := range []string{input.Content, input.OutputSchema, `"judge-round-1"`, "JSON だけの最終メッセージ", "instruction_id"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("依頼に %q がない", want)
		}
	}
}

func TestBuildPlanner(t *testing.T) {
	current := plan.Plan{Version: 1, Task: "旧計画の課題"}
	remaining := plan.Plan{Task: "残りの課題", Stages: []plan.Stage{{ID: "implement"}}}
	input := PlannerInput{
		InstructionID:     "plan-2",
		Version:           2,
		Template:          plan.PlanTemplate{Name: "fix", StartStage: "implement", PlanLimits: &plan.PlanLimits{MaxRounds: 4}},
		TemplateHash:      "template-hash",
		Task:              "保存の失敗を直す",
		StageDefinitions:  []Document{{Path: "stages/implement.md", Content: "実装の定義"}},
		Repository:        RepositoryFacts{Rules: Document{Path: ".devkit/rules.md", Content: "リポジトリの規則"}},
		Norms:             []Document{{Path: "principles/harness.md", Content: "ループの原則"}},
		CurrentPlan:       &current,
		RemainingPlan:     &remaining,
		HumanInstructions: "  人の修正指示を保つ\n{{補間しない}}\n",
		PreviousFindings:  []form.Finding{{ID: "PF1", Scenario: "完了条件がない"}},
		PreviousOutputs:   []StageOutput{{StageID: "research", Content: "調査結果\n  詳細\n"}},
		OutputSchema:      `{"required":["instruction_id","plan"]}`,
	}
	got, err := BuildPlanner(input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildPlanner(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, again) || got.InstructionID != input.InstructionID {
		t.Fatal("同じ入力から異なる依頼を組み立てた")
	}
	for _, want := range []string{
		input.Task, input.TemplateHash, `"max_rounds": 4`, "旧計画の課題", "残りの課題", "番号は 2",
		"stages/implement.md", "実装の定義", "リポジトリの規則", "principles/harness.md", "ループの原則",
		input.HumanInstructions, "PF1", "完了条件がない", input.PreviousOutputs[0].Content,
		input.OutputSchema, "instruction_id", `"plan-2"`, "JSON だけの最終メッセージ",
	} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("依頼に %q がない", want)
		}
	}
	initial, err := BuildPlanner(PlannerInput{InstructionID: "plan-1", Version: 1, Template: input.Template, Task: input.Task})
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"## 現在の計画", "## 残りの実行内容", "## 人の修正指示", "## 計画の check の指摘と指摘 ID"} {
		if strings.Contains(initial.Text, unwanted) {
			t.Errorf("初回の依頼に %q を含めた", unwanted)
		}
	}
}
