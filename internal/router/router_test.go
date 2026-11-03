package router

import (
	"io/fs"
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
		t.Fatal("依存の変更を確認しない")
	}
	merged := MergeTables(base, Table{ConfirmationRules: map[string]ConfirmationMode{DependencyChanged: Provisional}})
	confirmation := merged.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}, NeedsDecision: true})
	if confirmation.Required || !confirmation.Provisional {
		t.Fatalf("名前ごとの上書きを適用できない: %+v", confirmation)
	}
	if !base.ClassifyConfirmation(ConfirmationInput{Categories: []string{DependencyChanged}}).Required {
		t.Fatal("既定の表を変更した")
	}
	if !merged.ClassifyConfirmation(ConfirmationInput{Gate: plan.Gate{Required: true}}).Required {
		t.Fatal("明示したゲートを省略した")
	}
	if destination, ok := base.PreferredLesson([]LessonDestination{Kit, Skill, HookLintType}); !ok || destination != HookLintType {
		t.Fatalf("教訓の反映先: %q, %v", destination, ok)
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
		{"完了処理", func(i *Input) {}, plan.Done},
		{"修正", func(i *Input) { i.Summary.Judgment = check.Fail }, plan.Fix},
		{"未評価", func(i *Input) { i.Summary.Judgment = check.NotAssessed }, plan.Ask},
		{"前提の崩れ", func(i *Input) { i.PremiseBroken = true }, plan.Back},
		{"ラウンド上限", func(i *Input) { i.Summary.Judgment = check.Fail; i.CompletedRounds = 3 }, plan.Ask},
		{"進展なし上限", func(i *Input) {
			i.Summary.Judgment = check.Fail
			i.ProgressMade = false
			i.PreviousNoProgressRounds = 1
		}, plan.Ask},
		{"完了時の上限", func(i *Input) { i.CompletedRounds = 3 }, plan.Done},
		{"確認の申告", func(i *Input) { i.TerminationKind = form.TerminationQuestion }, plan.Ask},
		{"停止の申告", func(i *Input) { i.TerminationKind = form.TerminationStopped }, plan.Ask},
		{"未検証", func(i *Input) { i.Completion.ReadyToCommit = false }, plan.Ask},
		{"ゲート", func(i *Input) { i.Stage.Gate.Required = true }, plan.Ask},
		{"ゲート前の修正", func(i *Input) { i.Stage.Gate.Required = true; i.Summary.Judgment = check.Fail }, plan.Fix},
		{"人のステージ", func(i *Input) { i.Stage.Assignee = plan.Human }, plan.Ask},
		{"参考情報", func(i *Input) { i.Summary.Findings = []check.Finding{{Kind: check.NeedsDecision}} }, plan.Done},
		{"判断が必要", func(i *Input) {
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
				t.Fatalf("分類: %+v, %v; 期待: %s", decision, err, tc.want)
			}
		})
	}
	provisional := MergeTables(table, Table{ConfirmationRules: map[string]ConfirmationMode{DependencyChanged: Provisional}})
	input := base
	input.Summary = check.Summary{Judgment: check.Fail, Findings: []check.Finding{{ID: "F1", Blocking: true, Kind: check.NeedsDecision}}}
	input.Confirmation.Categories = []string{DependencyChanged}
	decision, err := provisional.Decide(input)
	if err != nil || decision.Action != plan.Ask {
		t.Fatalf("別の確認対象を理由に指摘の判断を省略した: %+v, %v", decision, err)
	}
	input.FindingCategories = map[string]string{"F1": DependencyChanged}
	decision, err = provisional.Decide(input)
	if err != nil || decision.Action != plan.Fix || !decision.Confirmation.Provisional {
		t.Fatalf("暫定判断: %+v, %v", decision, err)
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
		t.Fatalf("commit 前の判定: %+v", completion)
	}
	input.CommitSucceeded, input.TargetMatches = true, true
	if !CheckCompletion(input).Complete {
		t.Fatal("完了条件を満たしても確定できない")
	}
	input.Detection = nil
	if CheckCompletion(input).ReadyToCommit {
		t.Fatal("内容の同一性を照合せず合格にした")
	}
	input.Detection = []check.DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}
	input.Judgments[0].TargetVersion.Tree = "stale"
	if CheckCompletion(input).ReadyToCommit {
		t.Fatal("古い対象の判定で完了した")
	}
}
