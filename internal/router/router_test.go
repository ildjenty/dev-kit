package router

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
	devkit "github.com/ildjenty/dev-kit"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func defaultTable(t *testing.T) Table {
	t.Helper()
	data, err := fs.ReadFile(devkit.FS(), "kit/router/default.toml")
	if err != nil {
		t.Fatal(err)
	}
	var table Table
	if _, err := toml.Decode(string(data), &table); err != nil {
		t.Fatal(err)
	}
	return table
}

func TestTableAndConfirmation(t *testing.T) {
	base := defaultTable(t)
	if !base.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}}).Required {
		t.Fatal("dependency change did not require confirmation")
	}
	merged := MergeTables(base, Table{ConfirmationRules: map[string]ConfirmationMode{DependencyChanged: Provisional}})
	confirmation := merged.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}, NeedsDecision: true})
	if confirmation.Required || !confirmation.Provisional {
		t.Fatalf("could not apply per-name override: %+v", confirmation)
	}
	if !base.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}}).Required {
		t.Fatal("modified the default table")
	}
	if !merged.ClassifyConfirmation(ConfirmationInput{Gate: plan.Gate{Required: true}}).Required {
		t.Fatal("skipped an explicit gate")
	}
}

func TestPreferredLesson(t *testing.T) {
	base := defaultTable(t)
	if destination, ok := base.PreferredLesson([]LessonDestination{Kit, Skill, HookLintType}); !ok || destination != HookLintType {
		t.Fatalf("lesson destination: %q, %v", destination, ok)
	}
}

func TestMergeConfirmationCategories(t *testing.T) {
	base := defaultTable(t)
	additions := Table{
		HumanConfirmation: []string{"billing_changed", DependencyChanged, "billing_changed"},
		ConfirmationRules: map[string]ConfirmationMode{DependencyChanged: NoConfirm},
	}
	merged := MergeTables(base, additions)
	want := append(slices.Clone(base.HumanConfirmation), "billing_changed")
	if !slices.Equal(merged.HumanConfirmation, want) {
		t.Fatalf("confirmation categories: %v; want: %v", merged.HumanConfirmation, want)
	}
	for _, category := range []string{"billing_changed", UndeclaredMetaLoopEdit, TargetChangedAgain} {
		if !merged.ClassifyConfirmation(ConfirmationInput{Categories: []string{category}}).Required {
			t.Fatalf("category did not require confirmation: %s", category)
		}
	}
	if merged.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}, NeedsDecision: true}).Required {
		t.Fatal("ignored an explicit per-name override")
	}
	if slices.Contains(base.HumanConfirmation, "billing_changed") || len(additions.HumanConfirmation) != 3 {
		t.Fatal("modified the original category lists")
	}
	if !slices.Equal(MergeTables(base, Table{HumanConfirmation: []string{}}).HumanConfirmation, base.HumanConfirmation) {
		t.Fatal("empty additions removed default categories")
	}
}

func TestConfirmationPerItem(t *testing.T) {
	table := MergeTables(defaultTable(t), Table{ConfirmationRules: map[string]ConfirmationMode{
		DependencyChanged: Provisional, "delegated": NoConfirm,
	}})
	cases := []struct {
		name       string
		categories []string
		required   bool
	}{
		{"provisional only", []string{DependencyChanged}, false},
		{"unclassified only", []string{"unclassified"}, true},
		{"no category", nil, true},
		{"unclassified after provisional", []string{DependencyChanged, "unclassified"}, true},
		{"uncategorized after provisional", []string{DependencyChanged, ""}, true},
		{"unclassified before provisional", []string{"unclassified", DependencyChanged}, true},
		{"unclassified after delegation", []string{"delegated", "unclassified"}, true},
		{"all classified", []string{DependencyChanged, "delegated"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			confirmation := table.ClassifyConfirmation(ConfirmationInput{Categories: tc.categories, NeedsDecision: true})
			if confirmation.Required != tc.required {
				t.Fatalf("confirmation: %+v; want required: %v", confirmation, tc.required)
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	table := defaultTable(t)
	base := Input{
		Stage:           plan.Stage{Assignee: plan.Worker, Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2}},
		CompletedRounds: 1, ProgressMade: true, TerminationKind: form.TerminationCompleted,
		Summary: check.Summary{Judgment: check.Pass}, Completion: Completion{ReadyToCommit: true},
	}
	cases := []struct {
		name   string
		change func(*Input)
		want   plan.Transition
	}{
		{"completion processing", func(i *Input) {}, plan.Done},
		{"correction", func(i *Input) { i.Summary.Judgment = check.Fail }, plan.Fix},
		{"not assessed", func(i *Input) { i.Summary.Judgment = check.NotAssessed }, plan.Ask},
		{"broken premise", func(i *Input) { i.PremiseBroken = true }, plan.Back},
		{"round limit", func(i *Input) { i.Summary.Judgment = check.Fail; i.CompletedRounds = 3 }, plan.Ask},
		{"no-progress limit", func(i *Input) {
			i.Summary.Judgment = check.Fail
			i.ProgressMade = false
			i.PreviousNoProgressRounds = 1
		}, plan.Ask},
		{"limit at completion", func(i *Input) { i.CompletedRounds = 3 }, plan.Done},
		{"reported question", func(i *Input) { i.TerminationKind = form.TerminationQuestion }, plan.Ask},
		{"reported stop", func(i *Input) { i.TerminationKind = form.TerminationStopped }, plan.Ask},
		{"unverified", func(i *Input) { i.Completion.ReadyToCommit = false }, plan.Ask},
		{"gate", func(i *Input) { i.Stage.Gate.Required = true }, plan.Ask},
		{"correction before gate", func(i *Input) { i.Stage.Gate.Required = true; i.Summary.Judgment = check.Fail }, plan.Fix},
		{"human stage", func(i *Input) { i.Stage.Assignee = plan.Human }, plan.Ask},
		{"reference information", func(i *Input) { i.Summary.Findings = []check.Finding{{Kind: check.NeedsDecision}} }, plan.Done},
		{"decision required", func(i *Input) {
			i.Summary.Findings = []check.Finding{{ID: "F1", Blocking: true, Kind: check.NeedsDecision}}
			i.Summary.Judgment = check.Fail
		}, plan.Ask},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.change(&input)
			decision, err := table.Decide(input)
			if err != nil || decision.Action != tc.want {
				t.Fatalf("classification: %+v, %v; want: %s", decision, err, tc.want)
			}
		})
	}
	provisional := MergeTables(table, Table{ConfirmationRules: map[string]ConfirmationMode{DependencyChanged: Provisional}})
	input := base
	input.Summary = check.Summary{Judgment: check.Fail, Findings: []check.Finding{{ID: "F1", Blocking: true, Kind: check.NeedsDecision}}}
	input.Confirmation.Categories = []string{DependencyChanged}
	decision, err := provisional.Decide(input)
	if err != nil || decision.Action != plan.Ask {
		t.Fatalf("skipped a finding decision because of another confirmation category: %+v, %v", decision, err)
	}
	input.FindingCategories = map[string]string{"F1": DependencyChanged}
	decision, err = provisional.Decide(input)
	if err != nil || decision.Action != plan.Fix || !decision.Confirmation.Provisional {
		t.Fatalf("provisional decision: %+v, %v", decision, err)
	}
}

func TestCompletion(t *testing.T) {
	stage := plan.Stage{Checks: []plan.Check{
		{Aspect: "purpose", Runner: "review", Required: true, CompletionConditionIDs: []string{"C1"}},
		{Aspect: "command:test", Runner: "command:test", Required: true},
	}}
	target := form.TargetVersion{Kind: form.TargetTree, Tree: "tree"}
	norms := []form.NormReference{{Path: "AGENTS.md", BlobID: "blob"}}
	identity := check.DetectionIdentity{AspectHash: "aspect", SpecHash: "spec", RunnerHash: "runner"}
	input := CompletionInput{
		Rule: stage.CompletionRule(), TargetVersion: target, Norms: norms,
		Detection:      []check.DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}},
		DetectionTests: []check.DetectionTest{{Aspect: "purpose", Runner: "review", Identity: identity, Passed: true}},
		Judgments: []check.Judgment{
			{JudgmentResult: form.JudgmentResult{Aspect: "purpose", TargetVersion: target, Norms: norms, Judgment: check.Pass, CompletionConditionIDs: []string{"C1"}}, Runner: "review"},
			{JudgmentResult: form.JudgmentResult{Aspect: "command:test", TargetVersion: target, Norms: norms, Judgment: check.Pass}, Runner: "command:test"},
		},
	}
	completion := CheckCompletion(input)
	if !completion.ReadyToCommit || completion.Complete {
		t.Fatalf("pre-commit assessment: %+v", completion)
	}
	input.CommitSucceeded, input.TargetMatches = true, true
	if !CheckCompletion(input).Complete {
		t.Fatal("could not finalize despite satisfied completion conditions")
	}
	input.Detection = nil
	if CheckCompletion(input).ReadyToCommit {
		t.Fatal("passed without checking content identity")
	}
	input.Detection = []check.DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}
	input.Judgments[0].TargetVersion.Tree = "stale"
	if CheckCompletion(input).ReadyToCommit {
		t.Fatal("completed using a judgment for a stale target")
	}
}
