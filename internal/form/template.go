package form

import (
	"bytes"
	"text/template"
)

// ApplyTemplate は供給された text/template の文面を適用する。
// PR の見出しも供給されたテンプレートに従う。
func ApplyTemplate(source string, value any) (string, error) {
	t, err := template.New("form").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, value); err != nil {
		return "", err
	}
	return out.String(), nil
}

// SkillTemplate は skill の構成テンプレートへの入力を表す。
type SkillTemplate struct {
	Name        string
	Description string
	Purpose     string
	Inputs      string
	Steps       string
	Outputs     string
	References  string
}

// AspectTemplate は観点ファイルのテンプレートへの入力を表す。
type AspectTemplate struct {
	ID           string
	Checks       string
	Blocking     string
	DoNotReport  string
	OutputFormat string
}

// PRTemplate は PR 本文のテンプレートへの入力を表す。
type PRTemplate struct {
	Summary      string
	Scope        string
	PlanSummary  string
	Decisions    string
	Verification string
	Risks        string
	References   string
}

// DesignTemplate は設計案のテンプレートへの入力を表す。
// Irreversible は計画の一覧への参照・更新内容を渡す。
type DesignTemplate struct {
	Title        string
	Proposal     string
	Scope        string
	Constraints  string
	Irreversible string
	Alternatives string
	Verification string
	Risks        string
}
