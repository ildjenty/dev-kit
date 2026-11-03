package plan

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	devkit "github.com/ildjenty/dev-kit"
)

func validator(t *testing.T) *Validator {
	t.Helper()
	read := func(path string) []byte {
		t.Helper()
		data, err := fs.ReadFile(devkit.FS(), path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	v, err := NewValidator(read(TemplateSchemaPath), read(PlanSchemaPath))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func loadTemplate(t *testing.T, v *Validator, name string) (PlanTemplate, string) {
	t.Helper()
	data, err := fs.ReadFile(devkit.FS(), "kit/plans/"+name+".json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := v.ParseTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	return template, ContentHash(data)
}

func bindChecks(p *Plan) {
	for i := range p.Stages {
		for j := range p.Stages[i].Checks {
			p.Stages[i].Checks[j].CompletionConditionIDs = []string{"C1"}
			p.Stages[i].Checks[j].Target = "返却後のツリー"
		}
	}
}

func TestDefaultsAndExamples(t *testing.T) {
	v := validator(t)
	for _, name := range []string{"default", "feature", "fix", "docs"} {
		t.Run(name, func(t *testing.T) {
			template, hash := loadTemplate(t, v, name)
			p := NewPlan(template, hash, "文言の修正", []CompletionCondition{{"C1", "文言が正しい"}})
			bindChecks(&p)
			if err := v.ValidatePlan(p); err != nil {
				t.Fatal(err)
			}
			if got := Deviations(template, p); len(got) != 0 {
				t.Fatalf("元の計画が逸脱になった: %+v", got)
			}
		})
	}
	data, err := fs.ReadFile(devkit.FS(), "docs/architecture/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := strings.Split(string(data), "```json\n")
	if _, err := v.ParseTemplate([]byte(strings.Split(blocks[1], "```")[0])); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ParsePlan([]byte(strings.Split(blocks[2], "```")[0])); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaAndReferences(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "文言の修正", []CompletionCondition{{"C1", "文言が正しい"}})
	if err := v.ValidatePlan(p); err == nil {
		t.Fatal("check の対象と完了条件の対応がない計画を受理した")
	}
	bindChecks(&p)
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Stages[0].Checks[0].CompletionConditionIDs = []string{"missing"} },
		func(p *Plan) { p.Stages[0].Transitions.Back = "missing" },
		func(p *Plan) { p.Stages[0].Checks[0].Aspect = "different" },
		func(p *Plan) { p.Stages = append(p.Stages, p.Stages[0]) },
		func(p *Plan) { p.Stages[0].Limits.MaxRounds = 0 },
		func(p *Plan) { p.Stages[0].Assignee = "judge" },
	} {
		bad := clonePlan(p)
		change(&bad)
		if err := v.ValidatePlan(bad); err == nil {
			t.Fatal("不正な計画を受理した")
		}
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.ParsePlan(append(data, []byte(" {}")...)); err == nil {
		t.Fatal("JSON の末尾の余分な値を受理した")
	}
	if _, err := v.ParsePlan([]byte(strings.Replace(string(data), `"version":1`, `"version":1,"unknown":true`, 1))); err == nil {
		t.Fatal("スキーマにないキーを受理した")
	}
}

func TestSelectionAndDeviations(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "feature")
	p := NewPlan(template, hash, "内部の修正", []CompletionCondition{{"C1", "期待どおりに動く"}})
	selected, err := p.SelectStages(template, []string{"research", "contract", "implement"})
	if err != nil {
		t.Fatal(err)
	}
	bindChecks(&selected)
	if err := v.ValidatePlan(selected); err != nil {
		t.Fatal(err)
	}
	if got := Deviations(template, selected); len(got) != 0 {
		t.Fatalf("通常の取捨が逸脱になった: %+v", got)
	}
	if selected.StartStage != "research" || *selected.Stages[0].Transitions.Done != "contract" {
		t.Fatal("候補の追加と開始ステージの変更が反映されていない")
	}
	if template.Stages[3].Transitions.Back != "structure" || p.Stages[0].Checks[0].Target != "" {
		t.Fatal("元の雛形または計画を書き換えた")
	}
	leading, err := p.SelectStages(template, []string{"contract", "steps", "implement"})
	if err != nil || len(Deviations(template, leading)) != 0 {
		t.Fatal("先頭の省略による開始ステージの変更が逸脱になった")
	}
	leading.StartStage = "implement"
	if got := Deviations(template, leading); len(got) != 1 || got[0].Kind != ChangedStartStage {
		t.Fatalf("採用済みの開始ステージの変更を見落とした: %+v", got)
	}
	p.Stages[1].Gate.Required = false
	p.Stages[1].Transitions.Back = "implement"
	p.Stages[1].Checks = nil
	got := Deviations(template, p)
	want := []DeviationKind{RemovedGate, ChangedTransition, RemovedRequiredCheck}
	if len(got) != len(want) {
		t.Fatalf("逸脱: %+v", got)
	}
	for i, kind := range want {
		if got[i].Kind != kind {
			t.Fatalf("逸脱: %+v", got)
		}
	}
	unknown := cloneStage(p.Stages[0])
	unknown.ID = "custom"
	p.Stages = append(p.Stages, unknown)
	if got := Deviations(template, p); got[len(got)-1].Kind != AddedStage {
		t.Fatalf("候補にないステージの追加: %+v", got)
	}
}

func TestVersionsAndExternalApproval(t *testing.T) {
	v := validator(t)
	template, hash := loadTemplate(t, v, "default")
	p := NewPlan(template, hash, "修正", []CompletionCondition{{"C1", "正しい"}})
	bindChecks(&p)
	next, diff := Replan(p, p)
	if next.Version != 2 || len(diff.ChangedFields) != 0 || len(diff.ChangedStages) != 0 {
		t.Fatalf("作り直し: %+v, %+v", next, diff)
	}
	next.Stages[0].Checks[0].CompletionConditionIDs[0] = "changed"
	if p.Stages[0].Checks[0].CompletionConditionIDs[0] != "C1" {
		t.Fatal("旧計画を書き換えた")
	}
	old := VersionEvaluation{Plan: p, Judgment: PlanPass, Origin: InitialProposal}
	proposal := VersionEvaluation{Plan: next, Origin: HumanProposal}
	if _, ok := ExecutableVersion(template, []VersionEvaluation{old, proposal}, 0); ok {
		t.Fatal("提案中に旧計画を選んだ")
	}
	proposal.Withdrawn = true
	if got, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 0); !ok || got.Version != 1 {
		t.Fatal("人が自発的に撤回した後に旧計画に戻れない")
	}
	proposal.Origin = BackProposal
	if _, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 1); ok {
		t.Fatal("再計画の撤回後に旧計画を選んだ")
	}
	proposal.Withdrawn = false
	proposal.Judgment = PlanPass
	if proposal.Executable(template) {
		t.Fatal("back による再計画を人の承認なしに実行可能にした")
	}
	proposal.Approved = true
	if got, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 1); !ok || got.Version != 2 {
		t.Fatal("承認済みの再計画を選べない")
	}
	if _, ok := ExecutableVersion(template, []VersionEvaluation{proposal, old}, 2); ok {
		t.Fatal("再度 back したときに前回の再計画を選んだ")
	}
	operation := ExternalOperation{"publish", "repo", hash, hash, hash}
	p.ExternalOperations = []ExternalOperation{operation}
	if !p.RequiresApproval(template) || !p.Declares(operation) {
		t.Fatal("外部操作の宣言を照合できない")
	}
	changed := operation
	changed.ExecutableHash = strings.Repeat("0", 64)
	if p.Declares(changed) {
		t.Fatal("実行ファイルが変わった宣言を受理した")
	}
	approved := VersionEvaluation{Plan: p, Judgment: PlanPass}
	if approved.Executable(template) {
		t.Fatal("承認のない外部操作を実行可能にした")
	}
	approved.Approved = true
	if !approved.Executable(template) {
		t.Fatal("合格と承認の揃った計画を実行可能にできない")
	}
}

func TestLimitsAndTransitions(t *testing.T) {
	limits := Limits{MaxRounds: 3, MaxNoProgress: 2}
	if limits.Reached(2, 1) || !limits.Reached(3, 0) || !limits.Reached(1, 2) {
		t.Fatal("ステージの上限が正しくない")
	}
	template := PlanTemplate{}
	if template.PlanRoundsReached(2) || !template.PlanRoundsReached(3) {
		t.Fatal("計画の既定の上限が正しくない")
	}
	template.PlanLimits = &PlanLimits{MaxRounds: 5}
	if template.PlanRoundsReached(3) || !template.PlanRoundsReached(5) {
		t.Fatal("計画の上限の上書きが反映されていない")
	}
	transitions := Transitions{Fix: "a", Ask: "b", Back: "c"}
	for action, want := range map[Transition]*string{Fix: &transitions.Fix, Ask: &transitions.Ask, Back: &transitions.Back, Done: nil} {
		got, err := transitions.Next(action)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("遷移 %s: %v, %v", action, got, err)
		}
	}
	if _, err := transitions.Next("unknown"); err == nil {
		t.Fatal("不明な分類を受理した")
	}
	if ContentHash([]byte("{}")) == ContentHash([]byte("{}\n")) {
		t.Fatal("元のバイト列をハッシュに使っていない")
	}
}
