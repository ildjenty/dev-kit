package plan

import (
	"fmt"
	"strings"
	"testing"
)

func TestFixedReferences(t *testing.T) {
	pattern, hash := loadTemplate(t, validator(t), "docs")
	p := newTestPlan(pattern, hash, "Edit documents", []CompletionCondition{{"C1", "Correct wording"}})
	body := "pattern body"
	p.Pattern.ContentHash = ContentHash([]byte(body))
	checks := map[string]CheckDefinition{
		"wording": {Runner: "judge", Criteria: []Selection{{"principles", "review/wording.md"}}},
		"harness": {Runner: "judge"},
	}
	resolver := func(selection Selection) ([]string, error) {
		if selection.Source == "plans" {
			return []string{body}, nil
		}
		return []string{"fixed document"}, nil
	}
	if err := ValidateReferences(p, checks, resolver); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		resolve Resolver
	}{
		{"missing", func(Selection) ([]string, error) { return nil, nil }},
		{"read error", func(Selection) ([]string, error) { return nil, fmt.Errorf("cannot read reference") }},
		{"empty selection", func(s Selection) ([]string, error) {
			if s.Source == "stages" {
				return nil, nil
			}
			return resolver(s)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReferences(p, checks, tc.resolve); err == nil {
				t.Fatal("accepted unresolved or mismatched reference")
			}
		})
	}
	for _, selection := range []Selection{{"templates", "missing-report"}, {"context", "missing-input"}, {"principles", "missing-criteria"}} {
		missing := func(s Selection) ([]string, error) {
			if s == selection {
				return nil, nil
			}
			return resolver(s)
		}
		candidate := clonePlan(p)
		candidate.Stages[0].Report = &selection
		if err := ValidateReferences(candidate, checks, missing); err == nil {
			t.Fatal("accepted missing report")
		}
		candidate.Stages[0].Report = nil
		candidate.Stages[0].Inputs = []Selection{selection}
		if err := ValidateReferences(candidate, checks, missing); err == nil {
			t.Fatal("accepted missing input")
		}
	}
	delete(checks, "wording")
	if err := ValidateReferences(p, checks, resolver); err == nil {
		t.Fatal("accepted unconfigured check")
	}
	p.Pattern.ContentHash = strings.Repeat("0", 64)
	if err := ValidatePatternReference(p.Pattern, resolver); err == nil {
		t.Fatal("accepted wrong pattern hash")
	}
}

func TestReferenceKinds(t *testing.T) {
	stage := Stage{ID: "edit-1", Definition: "edit", Report: &Selection{"templates", "report"}, UnitReport: &Selection{"templates", "unit"}, HumanPrompt: &Selection{"templates", "question"}, Delegations: []Selection{{"rules", "delegation"}}, Inputs: []Selection{{"context", "input"}}, Checks: []Check{{Aspect: "wording", Required: true}}}
	definitions := map[string]CheckDefinition{"wording": {Runner: "judge", Criteria: []Selection{{"principles", "review"}}}}
	for _, missing := range []string{"edit.md", "report", "unit", "question", "delegation", "input", "review"} {
		resolve := func(s Selection) ([]string, error) {
			if s.Select == missing {
				return nil, nil
			}
			return []string{"document"}, nil
		}
		if err := validateStageReferences(stage, definitions, resolve); err == nil {
			t.Fatalf("accepted missing selection %s", missing)
		}
	}
}
