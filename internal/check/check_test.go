package check

import (
	"encoding/json"
	"io/fs"
	"slices"
	"testing"

	devkit "github.com/ildjenty/dev-kit"
	"github.com/ildjenty/dev-kit/internal/form"
)

func asset(t *testing.T, path string) []byte {
	t.Helper()
	data, err := fs.ReadFile(devkit.FS(), path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestKitAspects(t *testing.T) {
	paths, err := fs.Glob(devkit.FS(), "kit/checks/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if path == "kit/checks/README.md" {
			continue
		}
		aspect, err := ParseAspect(string(asset(t, path)))
		if err != nil || aspect.Blocking == "" || aspect.OutputFormat == "" {
			t.Fatalf("%s: %+v, %v", path, aspect, err)
		}
		if _, err := ParseDetectionSpec(asset(t, "kit/examples/"+aspect.ID+"/spec.json")); err != nil {
			t.Fatalf("%s の検出試験: %v", aspect.ID, err)
		}
	}
	if _, err := ParseAspect("# incomplete\n- blocking: 条件"); err == nil {
		t.Fatal("不足した観点を受け付けた")
	}
}

func judgmentRequest(t *testing.T) Request {
	t.Helper()
	request, err := PrepareRequest(RequestInput{
		Kind: RoundRequest, InstructionID: "judgment-1", Runner: "review",
		Aspect: Aspect{ID: "purpose"}, TargetVersion: form.TargetVersion{Kind: form.TargetTree, Tree: "tree"}, TargetPath: "snap/tree",
		Norms:        []Norm{{Reference: form.NormReference{Path: "AGENTS.md", BlobID: "blob"}}},
		OutputSchema: asset(t, "kit/schemas/judgment.json"), CompletionConditionIDs: []string{"C1"},
		Progress:         &form.Progress{InstructionID: "work-1", TerminationKind: form.TerminationCompleted},
		PreviousFindings: []Finding{{ID: "old"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestAcceptResult(t *testing.T) {
	request := judgmentRequest(t)
	schema, err := form.CompileSchema(request.OutputSchema)
	if err != nil {
		t.Fatal(err)
	}
	result := form.JudgmentResult{
		InstructionID: request.InstructionID, Aspect: request.Aspect.ID, TargetVersion: request.TargetVersion,
		Norms: request.normReferences(), CompletionConditionIDs: []string{"C1"}, Judgment: Pass,
		Findings:     []Finding{{ID: "new", Kind: AutoFixable, Scenario: "対象外の参考情報", ResolvedFindingIDs: []string{"old"}}},
		CheckedScope: []string{"目的"}, UncheckedScope: []string{}, Method: "完了条件との照合", Environment: "例の対象",
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	provenance := Provenance{Source: SubagentResult, InstructionID: request.InstructionID, Runner: "review"}
	judgment, accepted, err := AcceptResult(request, data, schema, provenance, nil)
	if err != nil || !accepted || judgment.Judgment != Pass || !slices.Equal(judgment.Content, data) {
		t.Fatalf("判定を受理できない: %+v, %v, %v", judgment, accepted, err)
	}
	for _, change := range []func(*form.JudgmentResult){
		func(r *form.JudgmentResult) { r.InstructionID = "other" },
		func(r *form.JudgmentResult) { r.TargetVersion.Tree = "other" },
		func(r *form.JudgmentResult) { r.Norms = nil },
		func(r *form.JudgmentResult) { r.CompletionConditionIDs = []string{"other"} },
	} {
		changed := result
		change(&changed)
		data, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := AcceptResult(request, data, schema, provenance, nil); err == nil {
			t.Fatal("依頼と一致しない結果を受理した")
		}
	}
	if _, _, err := AcceptResult(request, data, schema, Provenance{}, nil); err == nil {
		t.Fatal("来歴のない結果を受理した")
	}
	late, accepted, err := AcceptResult(request, []byte("不正な後着"), nil, Provenance{}, []Judgment{judgment})
	if err != nil || accepted || late.Judgment != Pass {
		t.Fatalf("後着によって最初の判定が変わった: %+v, %v", late, err)
	}
}

func TestAggregateExemptionAndCommand(t *testing.T) {
	request := judgmentRequest(t)
	request.Runner, request.Aspect.ID = "command:test", "command:test"
	var judgments []Judgment
	for _, command := range []CommandResult{{ExitCode: 0}, {ExitCode: 1, Output: "失敗の出力"}, {ExitCode: 1, TimedOut: true}} {
		judgment, err := FromCommand(request, command)
		if err != nil {
			t.Fatal(err)
		}
		judgments = append(judgments, judgment)
	}
	if judgments[0].Judgment != Pass || judgments[1].Judgment != Fail || judgments[2].Judgment != NotAssessed {
		t.Fatalf("コマンドの判定: %+v", judgments)
	}
	if Aggregate(judgments).Judgment != Fail || Aggregate([]Judgment{judgments[0], judgments[2]}).Judgment != NotAssessed || Aggregate(nil).Judgment != Pass {
		t.Fatal("集約の優先順が異なる")
	}
	judgment := judgments[1]
	judgment.Findings[0].Exemption = &form.Exemption{Claim: "合意した範囲の外", Evidence: []form.Reference{{RecordID: "agreement"}}}
	decision := ExemptionDecision{FindingID: judgment.Findings[0].ID, ExcludedRequirements: []string{"範囲外の要求"}, Reason: "合意した範囲との照合"}
	partial, err := ApplyExemptions(judgment, []ExemptionDecision{decision})
	if err != nil || partial.Judgment != Fail {
		t.Fatalf("部分的な適用除外で違反全体を外した: %+v, %v", partial, err)
	}
	decision.CoversFinding = true
	exempted, err := ApplyExemptions(judgment, []ExemptionDecision{decision})
	if err != nil || exempted.Judgment != Pass || exempted.Findings[0].Blocking || !judgment.Findings[0].Blocking {
		t.Fatalf("適用除外の結果または元の値が不正: %+v, %v", exempted, err)
	}
	required := []Requirement{{Aspect: request.Aspect.ID, Runner: request.Runner, TargetVersion: request.TargetVersion, Norms: request.normReferences(), CompletionConditionIDs: []string{"C1", "C2"}}}
	if CheckRequired(required, []Judgment{judgments[0]})[0].Judgment != NotAssessed {
		t.Fatal("未確認の完了条件を PASS にした")
	}
	if CheckRequired(required, nil)[0].Judgment != NotAssessed {
		t.Fatal("結果のない必須 check を PASS にした")
	}
}

func TestDetection(t *testing.T) {
	source := asset(t, "kit/examples/purpose/spec.json")
	spec, err := ParseDetectionSpec(source)
	if err != nil {
		t.Fatal(err)
	}
	examples := make(map[string][]byte)
	var observations []DetectionObservation
	for _, example := range spec.Cases {
		examples[example.File] = asset(t, "kit/examples/purpose/"+example.File)
		findings := []Finding{}
		if example.Kind == Violation {
			findings = append(findings, Finding{ID: "violation", Blocking: true, Kind: AutoFixable, Scenario: "完了を判定できない"})
		}
		observations = append(observations, DetectionObservation{
			File: example.File, Judgment: Judgment{JudgmentResult: form.JudgmentResult{Aspect: "purpose", Judgment: example.Expected, Findings: findings}, Runner: "review"},
			Reason: "狙った条件と結果を照合した", MatchesBlocking: true, RespectsDifference: true,
		})
	}
	identity, err := NewDetectionIdentity(asset(t, "kit/checks/purpose.md"), source, []byte("担当定義"), examples)
	if err != nil {
		t.Fatal(err)
	}
	record, err := AssessDetection(spec, "review", identity, observations)
	if err != nil || !record.Passed {
		t.Fatalf("検出試験: %+v, %v", record, err)
	}
	observations[0].MatchesBlocking = false
	failed, err := AssessDetection(spec, "review", identity, observations)
	if err != nil || failed.Passed {
		t.Fatalf("違う理由の FAIL が合格した: %+v, %v", failed, err)
	}
	required := []DetectionRequirement{{Aspect: "purpose", Runner: "review", Identity: identity}}
	if len(MissingDetection(required, []DetectionTest{record})) != 0 {
		t.Fatal("同じ内容の合格を再利用できない")
	}
	examples["allowed.md"] = append(examples["allowed.md"], '\n')
	changed, err := NewDetectionIdentity(asset(t, "kit/checks/purpose.md"), source, []byte("担当定義"), examples)
	if err != nil {
		t.Fatal(err)
	}
	required[0].Identity = changed
	if identity == changed || len(MissingDetection(required, []DetectionTest{record})) != 1 {
		t.Fatal("例の変更で合格記録が失効しない")
	}
}
