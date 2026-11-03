package brief

import (
	"encoding/json"

	"github.com/ildjenty/dev-kit/internal/form"
)

// BuildWork は作業依頼を組み立てる。前のステージの出力や検証出力を改変しない。
func BuildWork(input WorkInput) (Brief, error) {
	stage, err := jsonText(input.Stage)
	if err != nil {
		return Brief{}, err
	}
	p, err := jsonText(input.Plan)
	if err != nil {
		return Brief{}, err
	}
	checks, err := jsonText(input.Repository.AvailableChecks)
	if err != nil {
		return Brief{}, err
	}
	findings, err := jsonText(input.PreviousFindings)
	if err != nil {
		return Brief{}, err
	}
	progress, err := jsonText(form.Progress{
		InstructionID:   input.InstructionID,
		TerminationKind: form.TerminationCompleted,
	})
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, workTemplate, struct {
		WorkInput
		StageJSON, PlanJSON, ChecksJSON, FindingsJSON, ProgressJSON string
	}{input, stage, p, checks, findings, progress})
}

// BuildJudgment は check が決めた内容を依頼本文にし、指示 ID の返却要件を付ける。
func BuildJudgment(input JudgmentInput) (Brief, error) {
	id, err := jsonText(input.InstructionID)
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, judgmentTemplate, struct {
		JudgmentInput
		InstructionIDJSON string
	}{input, id})
}

// BuildPlanner は雛形と課題、および渡された再提案・再計画の情報から計画依頼を組み立てる。
func BuildPlanner(input PlannerInput) (Brief, error) {
	template, err := jsonText(input.Template)
	if err != nil {
		return Brief{}, err
	}
	current, err := jsonText(input.CurrentPlan)
	if err != nil {
		return Brief{}, err
	}
	remaining, err := jsonText(input.RemainingPlan)
	if err != nil {
		return Brief{}, err
	}
	checks, err := jsonText(input.Repository.AvailableChecks)
	if err != nil {
		return Brief{}, err
	}
	findings, err := jsonText(input.PreviousFindings)
	if err != nil {
		return Brief{}, err
	}
	id, err := jsonText(input.InstructionID)
	if err != nil {
		return Brief{}, err
	}
	return render(input.InstructionID, plannerTemplate, struct {
		PlannerInput
		TemplateJSON, CurrentJSON, RemainingJSON, ChecksJSON, FindingsJSON, InstructionIDJSON string
	}{input, template, current, remaining, checks, findings, id})
}

func jsonText(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return string(data), err
}

func render(id, source string, value any) (Brief, error) {
	text, err := form.ApplyTemplate(source, value)
	if err != nil {
		return Brief{}, err
	}
	return Brief{InstructionID: id, Text: text}, nil
}

const repositoryTemplate = `{{define "repository"}}
## 配置ガイド
{{range .LayoutGuides}}
参照先: {{.Path}}

{{.Content}}
{{end}}
## 規則と強制手段

参照先: {{.Rules.Path}}

{{.Rules.Content}}

## 元に戻せないもの

規則にある項目:
{{range .IrreversibleDecisions}}
- {{.}}
{{end}}
{{end}}`

const normsTemplate = `{{define "norms"}}
## 固定した規範
{{range .}}
参照先: {{.Path}}

{{.Content}}
{{end}}
{{end}}`

const previousOutputsTemplate = `{{define "previousOutputs"}}
## 前のステージの出力
{{range .}}
### {{.StageID}}

{{.Content}}
{{end}}
{{end}}`

const workTemplate = `# 作業の指示

指示 ID: {{.InstructionID}}

## 利用者と目的

{{.Purpose}}

## ステージの定義

{{.StageDefinition}}

## 現在のステージ

` + "```json\n{{.StageJSON}}\n```" + `

## 課題と計画

` + "```json\n{{.PlanJSON}}\n```" + `

完了条件は計画の ID と対応付けて確認する。
現在のステージの上限はラウンド数 {{.Stage.Limits.MaxRounds}}、連続して進展がない回数 {{.Stage.Limits.MaxNoProgress}} とする。

## 変更してよいもの
{{range .AllowedChanges}}
- {{.}}
{{end}}
## 変更してはいけないもの
{{range .ForbiddenChanges}}
- {{.}}
{{end}}
## 完了条件の確認方法
{{range .Verification}}
- {{.}}
{{end}}
{{template "repository" .Repository}}
計画にある項目:
{{range .Plan.IrreversibleDecisions}}
- {{.}}
{{end}}
## 利用可能な check

` + "```json\n{{.ChecksJSON}}\n```" + `
{{template "norms" .Norms}}
{{template "previousOutputs" .PreviousOutputs}}
{{if .PreviousFindings}}
## 前回の指摘と指摘 ID

` + "```json\n{{.FindingsJSON}}\n```" + `
{{end}}
{{range .CommandOutputs}}
## 必須の command: の check の出力

観点: {{.Check.Aspect}}

実行主体: {{.Check.Runner}}

終了コード: {{.ExitCode}}

{{.Output}}
{{end}}
{{if .PreCommitOutput}}
## pre-commit の出力

{{.PreCommitOutput}}
{{end}}
## 終了ブロックの返却

作業結果を JSON にし、` + "`devkit submit <ファイル>`" + ` で提出する。
` + "`instruction_id`" + ` にはこの作業指示の指示 ID を含める。終了の種別は「完了を主張」「停止して確認」「作業停止の申告」のいずれかとする。
実施内容、確認済み事項、未確認事項、障害、再開条件を対応する項目に記録する。一時状態には内容、確認方法、出典、無効になる条件を記録する。
読めなかった入力とその影響は未確認事項に記録する。対象外の作業と変更理由は必要に応じて補足する。
完了の確定と git commit は devkit が行う。

` + "```json\n{{.ProgressJSON}}\n```" + `
{{if .OutputSchema}}
### 終了ブロックの JSON Schema

{{.OutputSchema}}
{{end}}
` + repositoryTemplate + normsTemplate + previousOutputsTemplate

const judgmentTemplate = `# 判定の依頼

指示 ID: {{.InstructionID}}

{{.Content}}

## 結果の返却

指定された出力形式に従い、JSON だけの最終メッセージで結果を返す。
` + "`instruction_id`" + ` の値は {{.InstructionIDJSON}} とする。
{{if .OutputSchema}}
### 結果の JSON Schema

{{.OutputSchema}}
{{end}}
`

const plannerTemplate = `# 計画の依頼

指示 ID: {{.InstructionID}}

## 課題

{{.Task}}

## ループの雛形

内容ハッシュ: {{.TemplateHash}}

` + "```json\n{{.TemplateJSON}}\n```" + `

## ステージの定義
{{range .StageDefinitions}}
参照先: {{.Path}}

{{.Content}}
{{end}}
{{template "repository" .Repository}}
## 利用可能な check

` + "```json\n{{.ChecksJSON}}\n```" + `
{{template "norms" .Norms}}
{{if .CurrentPlan}}
## 現在の計画

` + "```json\n{{.CurrentJSON}}\n```" + `
{{end}}
{{if .RemainingPlan}}
## 残りの実行内容

` + "```json\n{{.RemainingJSON}}\n```" + `
{{end}}
{{if .HumanInstructions}}
## 人の修正指示

{{.HumanInstructions}}
{{end}}
{{if .PreviousFindings}}
## 計画の check の指摘と指摘 ID

` + "```json\n{{.FindingsJSON}}\n```" + `
{{end}}
{{template "previousOutputs" .PreviousOutputs}}
## 計画に記入する内容

指定された課題と雛形から、ID 付きの完了条件と必要なステージを記入する。
関係のないステージを外し、必要な調査・試作は雛形の候補から選ぶ。各 check の対象と完了条件 ID を指定する。
課題固有の元に戻せないものと、必要な外部操作の宣言を記入する。外部操作の種別・対象・入力のハッシュ・登録定義と実行ファイルの内容ハッシュは、入力で確認できる値を使う。
候補にないステージの追加、ゲートの削除、遷移の変更、必須 check の削除は逸脱として示す。逸脱と外部操作の宣言には人の承認が必要になる。
再提案では人の修正指示と check の指摘を反映し、再計画では渡された残りの実行内容を作り直す。

## 結果の返却

計画の番号は {{.Version}} とする。雛形の名前と内容ハッシュを計画の参照に含める。
指定された出力形式に従い、` + "`instruction_id` と `plan`" + ` を持つ JSON だけの最終メッセージで返す。
` + "`instruction_id`" + ` の値は {{.InstructionIDJSON}} とする。
{{if .OutputSchema}}
### 結果の JSON Schema

{{.OutputSchema}}
{{end}}
` + repositoryTemplate + normsTemplate + previousOutputsTemplate
