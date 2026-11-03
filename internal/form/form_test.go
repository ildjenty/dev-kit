package form

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func schemaFor(t *testing.T, name string) *Schema {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "kit", "schemas", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := CompileSchema(source)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestProgressExample(t *testing.T) {
	data := []byte(`{
		"instruction_id":"work-instruction",
		"termination_kind":"完了を主張",
		"work_performed":["保存ボタンの文言を『登録』から『保存』に変更した"],
		"confirmed":["保存ボタンに『保存』と表示され、既存のテストが通る"],
		"unconfirmed":[],"obstacles":[],"temporary_state":[],"resume_conditions":[]
	}`)
	var progress Progress
	if err := schemaFor(t, "progress").Decode(data, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.InstructionID != "work-instruction" || progress.TerminationKind != TerminationCompleted {
		t.Fatalf("終了ブロックの解析結果: %+v", progress)
	}
	for _, kind := range []TerminationKind{TerminationCompleted, TerminationQuestion, TerminationStopped} {
		data, err := json.Marshal(Progress{InstructionID: "work-instruction", TerminationKind: kind})
		if err != nil {
			t.Fatal(err)
		}
		if err := schemaFor(t, "progress").Validate(data); err != nil {
			t.Fatalf("正本が明示しない種別別の必須項目を要求した: %v", err)
		}
	}
}

func TestFixedFormats(t *testing.T) {
	refs := []Reference{{RecordID: "record"}, {External: "docs/design.md"}}
	finding := Finding{
		ID: "finding", Blocking: true, Kind: NeedsDecision, Scenario: "必要な判断基準を確認できない",
		Exemption: &Exemption{Claim: "対象外の条件", Evidence: refs}, ResolvedFindingIDs: []string{"previous"},
	}
	question := Question{
		ID: "question", Text: "画面の設計を進めますか。", ScreenID: "settings",
		ApprovalTarget: ApprovalTarget{Kind: ApprovalTree, Tree: "tree"},
		Options:        []string{"進める", "見直す"}, Recommendation: "進める", RecommendationReason: "表示内容を確認したため",
		StopCategory: "design", NextStage: "design", StoppedStage: "requirements", StopReason: "表示内容の確認待ち", Evidence: refs,
		StopObservation: &Reference{External: "stops/observation.json"},
	}
	cases := []struct {
		name  string
		value any
	}{
		{"progress", Progress{
			InstructionID: "instruction", TerminationKind: TerminationStopped,
			TemporaryState: []TemporaryState{{Content: "実行環境", VerificationMethod: "起動を確認", Source: "実行ログ", InvalidWhen: "プロセス終了時"}},
		}},
		{"finding", finding},
		{"judgment", JudgmentResult{
			InstructionID: "instruction", Aspect: "wording", TargetVersion: TargetVersion{Kind: TargetTree, Tree: "tree"},
			Norms: []NormReference{{Path: "AGENTS.md", BlobID: "blob"}}, CompletionConditionIDs: []string{"save-label"},
			Judgment: NotAssessed, Findings: []Finding{finding}, CheckedScope: []string{}, UncheckedScope: []string{"画面"},
			Method: "画面の起動", Environment: "ローカル",
		}},
		{"planner-result", PlannerResult{InstructionID: "instruction", Plan: json.RawMessage(`{"task":"計画の内容は plan が検証する"}`)}},
		{"question", question},
		{"instruction", Instruction{ID: "instruction", Kind: InstructionQuestion, TargetKey: "question", Question: &question}},
		{"decision", Decision{
			ID: "decision", Content: "既存の保存処理を使う", Validity: "保存先を維持する間", Supersedes: []Reference{}, DependsOn: refs,
			Conditions: []string{"保存先は既存のまま"}, Decider: "人", Provisional: true,
			RejectedAlternatives: []RejectedAlternative{{Proposal: "保存処理の追加", Reason: "重複する", Example: "同じ入力の保存"}}, Evidence: refs,
		}},
		{"evidence", Evidence{ID: "evidence", Content: "既存処理で保存できる", Sources: refs, VerificationMethod: "既存テストを実行"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if err := schemaFor(t, tc.name).Validate(data); err != nil {
				t.Fatal(err)
			}
			if text, err := RenderMarkdown(tc.value); err != nil || text == "" {
				t.Fatalf("Markdown の描画: %q, %v", text, err)
			}
		})
	}
	text, err := RenderMarkdown(question)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"画面 ID: settings", "requirements", "表示内容の確認待ち", "自由な言葉", "選択肢にない答え", "条件付きの答え", "質問返し", "言い直す", "docs/design.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("問いの表示に %q がない", want)
		}
	}
}

func TestInvalidFormats(t *testing.T) {
	cases := []struct{ schema, data string }{
		{"progress", `{"termination_kind":"完了を主張"}`},
		{"progress", `{"instruction_id":"instruction"}`},
		{"progress", `{"instruction_id":"instruction","termination_kind":"PASS"}`},
		{"progress", `{"instruction_id":"instruction","termination_kind":"停止して確認","temporary_state":[{"content":"状態"}]}`},
		{"planner-result", `{"plan":{}}`},
		{"planner-result", `{"instruction_id":"instruction","plan":[]}`},
		{"finding", `{"id":"finding","blocking":true,"kind":"unknown","scenario":"失敗"}`},
		{"instruction", `{"id":"instruction","kind":"judgment","target_key":"round/aspect","reissue":0,"brief":"依頼"}`},
		{"instruction", `{"id":"instruction","kind":"question","target_key":"question","reissue":0}`},
		{"evidence", `{"id":"evidence","content":"結果","verification_method":"確認"}`},
	}
	for _, tc := range cases {
		t.Run(tc.schema+"/"+tc.data, func(t *testing.T) {
			if err := schemaFor(t, tc.schema).Validate([]byte(tc.data)); err == nil {
				t.Fatal("不正な形式を受け付けた")
			}
		})
	}
	schema := schemaFor(t, "progress")
	if err := schema.Validate([]byte(`{"instruction_id":"i","termination_kind":"完了を主張"} {}`)); err == nil {
		t.Fatal("複数の JSON を受け付けた")
	}
}

func TestSchemaDoesNotLoadFiles(t *testing.T) {
	if _, err := CompileSchema([]byte(`{"$ref":"file:///not-supplied.json"}`)); err == nil {
		t.Fatal("供給していない参照先を受け付けた")
	}
}

func TestJudgmentBoundary(t *testing.T) {
	schema := schemaFor(t, "judgment")
	base := `{
		"instruction_id":"instruction","aspect":"wording",
		"target_version":{"kind":"plan","plan":1},
		"norms":[],"completion_condition_ids":[],"judgment":"PASS",
		"findings":[],"checked_scope":[],"unchecked_scope":[],
		"method":"計画と照合","environment":"ローカル"
	}`
	for _, field := range []string{"instruction_id", "judgment", "target_version"} {
		t.Run(field, func(t *testing.T) {
			var value map[string]json.RawMessage
			if err := json.Unmarshal([]byte(base), &value); err != nil {
				t.Fatal(err)
			}
			switch field {
			case "instruction_id":
				delete(value, field)
			case "judgment":
				value[field] = json.RawMessage(`"UNKNOWN"`)
			case "target_version":
				value[field] = json.RawMessage(`{"kind":"plan","plan":1,"tree":"tree"}`)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(data); err == nil {
				t.Fatal("不正な判定結果を受け付けた")
			}
		})
	}
	if err := schema.Validate([]byte(base)); err != nil {
		t.Fatal(err)
	}
}

func TestTemplates(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  []string
	}{
		{"skill", SkillTemplate{Name: "devkit", Description: "開発を進める", Purpose: "目的", Steps: "手順"}, []string{"name: \"devkit\"", "description: \"開発を進める\"", "## 手順"}},
		{"aspect", AspectTemplate{ID: "wording", Checks: "保存ボタンの表示", Blocking: "文言が異なる"}, []string{"# wording", "- 確認事項:", "- blocking:", "- 報告しない事項:", "- 出力形式:", "instruction_id"}},
		{"pr", PRTemplate{PlanSummary: "文言変更の計画", Decisions: "既存処理を使う", Verification: "保存ボタンを起動して確認"}, []string{"文言変更の計画", "既存処理を使う", "保存ボタンを起動して確認"}},
		{"design", DesignTemplate{Title: "保存画面", Irreversible: "計画の一覧への参照"}, []string{"# 保存画面", "計画の一覧への参照"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "..", "kit", "templates", tc.name+".md.tmpl"))
			if err != nil {
				t.Fatal(err)
			}
			text, err := ApplyTemplate(string(source), tc.value)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("テンプレートの結果に %q がない", want)
				}
			}
		})
	}
	custom := "## リポジトリの検証見出し\n\n{{.Verification}}\n"
	text, err := ApplyTemplate(custom, PRTemplate{Verification: "確認結果"})
	if err != nil || text != "## リポジトリの検証見出し\n\n確認結果\n" {
		t.Fatalf("既存テンプレートの適用: %q, %v", text, err)
	}
	if _, err := ApplyTemplate("{{.missing}}", map[string]string{}); err == nil {
		t.Fatal("不足したテンプレート入力を受け付けた")
	}
}
