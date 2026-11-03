package form

import (
	"bytes"
	"text/template"
)

// ApplyTemplate applies supplied text/template content.
// PR headings also follow the supplied template.
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

// SkillTemplate is the input to a skill composition template.
type SkillTemplate struct {
	Name        string
	Description string
	Purpose     string
	Inputs      string
	Steps       string
	Outputs     string
	References  string
}

// AspectTemplate is the input to an aspect file template.
type AspectTemplate struct {
	ID           string
	Checks       string
	Blocking     string
	DoNotReport  string
	OutputFormat string
}

// PRTemplate is the input to a PR body template.
type PRTemplate struct {
	Summary      string
	Scope        string
	PlanSummary  string
	Decisions    string
	Verification string
	Risks        string
	References   string
}

// DesignTemplate is the input to a design proposal template.
// Irreversible supplies a reference to the plan's list and any required updates.
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
