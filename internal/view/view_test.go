package view

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/ledger/fsstore"
	"github.com/ildjenty/dev-kit/internal/plan"
)

func recordID(n int) string { return fmt.Sprintf("%026d", n) }

func loadInput(t *testing.T, stops []ledger.StopObservation, payloads ...ledger.Payload) Input {
	t.Helper()
	sources := make(map[string][]byte)
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "kit", "schemas", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = data
	}
	validator, err := ledger.NewValidator(sources)
	if err != nil {
		t.Fatal(err)
	}
	store := fsstore.New(t.TempDir())
	work := &ledger.Work{ID: "work", TaskRef: "issue-1", Branch: "devkit/work", BaseCommit: "base", KitVersion: ledger.ReadableKitVersion}
	records := []ledger.Record{}
	for i, payload := range append([]ledger.Payload{work}, payloads...) {
		record := ledger.Record{ID: recordID(i + 1), Payload: payload}
		if err := ledger.Append(store, record, validator); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	state, err := ledger.Load(store, stops, validator)
	if err != nil {
		t.Fatal(err)
	}
	decisions, err := state.ResolveDecisions(ledger.DecisionContext{Valid: func(form.Decision) (bool, error) { return true, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return Input{Ledger: state, Records: records, Decisions: decisions}
}

func question(id, text string) form.Question {
	return form.Question{ID: id, Text: text, ApprovalTarget: form.ApprovalTarget{Kind: form.ApprovalTree, Tree: "current"},
		Options: []string{"approve", "reject", "defer"}, Recommendation: "approve", StopCategory: "approval",
		StoppedStage: "implement", StopReason: "Approval is required", Evidence: []form.Reference{}}
}

func render(t *testing.T, input Input) string {
	t.Helper()
	html, err := Generate(input)
	if err != nil {
		t.Fatal(err)
	}
	// Every local reference resolves to one canonical body.
	for _, match := range regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(html, -1) {
		if count := strings.Count(html, `id="`+match[1]+`"`); count != 1 {
			t.Fatalf("fragment %q resolves to %d bodies", match[1], count)
		}
	}
	return html
}

func TestGenerateAnswerBoundaryAndPause(t *testing.T) {
	old := question("review-1", "Approve the previous result?")
	stopQuestion := question("continue", "May the work continue?")
	stopQuestion.ApprovalTarget = form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: "work"}
	input := loadInput(t, []ledger.StopObservation{{ID: "stop", WorkID: "work", LimitReached: true, Question: &stopQuestion}},
		&ledger.Question{Question: old},
		&ledger.Approval{Answer: ledger.Answer{QuestionID: old.ID, Target: old.ApprovalTarget, Interpretation: ledger.AnswerApproved,
			Original: "Approved", SessionID: "session", ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		&ledger.Lesson{Content: "Keep the existing contract", Evidence: []form.Reference{}},
		&ledger.Pause{Position: form.Reference{RecordID: recordID(1)}, Reason: "Requested by the person"})
	html := render(t, input)
	for _, want := range []string{"Run devkit resume", "May the work continue?", "Answer in your own words.", "Limited to artifacts recorded by submit.", "review-1: approved"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	recent := strings.Split(strings.Split(html, `<section id="recent">`)[1], `</section>`)[0]
	if strings.Contains(recent, recordID(3)) || !strings.Contains(recent, recordID(4)) || !strings.Contains(recent, recordID(5)) {
		t.Fatalf("unexpected new-record boundary: %s", recent)
	}
	if strings.Count(html, old.Text) != 1 || strings.Count(html, stopQuestion.Text) != 1 {
		t.Fatal("question bodies were duplicated")
	}
	if len(input.Ledger.UnansweredQuestions()) != 1 || input.Ledger.LastRecordID() != recordID(5) {
		t.Fatal("rendering changed ledger state")
	}
	if second := render(t, input); second != html {
		t.Fatal("identical input did not produce identical HTML")
	}
}

func TestGenerateCoverageAndDependencies(t *testing.T) {
	target := form.TargetVersion{Kind: form.TargetTree, Tree: "current"}
	norms := []form.NormReference{{Path: "AGENTS.md", BlobID: "norm-current"}}
	p := plan.Plan{Version: 1, Template: plan.TemplateRef{Name: "docs", ContentHash: strings.Repeat("a", 64)}, Task: "Preserve the contract",
		CompletionConditions: []plan.CompletionCondition{{ID: "C1", Description: "Existing output is preserved"}, {ID: "C2", Description: "The guide is updated"}},
		StartStage:           "implement", ExternalOperations: []plan.ExternalOperation{}, Stages: []plan.Stage{{ID: "implement", Assignee: plan.Worker,
			Checks: []plan.Check{{Aspect: "contract", Runner: "review", Required: true, CompletionConditionIDs: []string{"C1"}, Target: "returned tree"}},
			Gate:   plan.Gate{Reason: "Review the result"}, Limits: plan.Limits{MaxRounds: 3, MaxNoProgress: 2},
			Transitions: plan.Transitions{Fix: "implement", Ask: "implement", Back: "implement"}}}}
	decision := form.Decision{ID: "D1", Content: "Keep the current format", Validity: "While the contract applies", Provisional: true,
		Supersedes: []form.Reference{}, DependsOn: []form.Reference{}, Conditions: []string{}, Decider: "worker",
		RejectedAlternatives: []form.RejectedAlternative{}, Evidence: []form.Reference{}}
	judgment := check.Judgment{JudgmentResult: form.JudgmentResult{InstructionID: "check-1", Aspect: "contract", TargetVersion: target, Norms: norms,
		CompletionConditionIDs: []string{"C1"}, Judgment: form.Pass, Findings: []form.Finding{}, CheckedScope: []string{"Public output"},
		UncheckedScope: []string{"Guide examples"}, Method: "Compare the output", Environment: "Local checkout"}, Runner: "review"}
	newer := judgment
	newer.InstructionID = "check-2"
	newer.Norms = []form.NormReference{{Path: "AGENTS.md", BlobID: "norm-old"}}
	newer.JudgmentResult.Judgment = form.Fail
	newer.Findings = []form.Finding{{ID: "F1", Blocking: true, Kind: form.NeedsDecision, Scenario: "The output contract needs a decision"}}
	input := loadInput(t, nil, &ledger.PlanVersion{Plan: p, Origin: plan.InitialProposal},
		&ledger.StageRun{ID: recordID(3), StageID: "implement", PlanVersion: 1, State: ledger.StageJudgmentWait, StopReason: "Checks are pending", NextActor: "judge"},
		&ledger.Round{ID: recordID(4), StageRunID: recordID(3), SessionID: "session", StartTarget: target, ReturnedTarget: &target, Norms: norms},
		&ledger.Decision{Decision: decision},
		&ledger.Artifact{Before: form.TargetVersion{Kind: form.TargetTree, Tree: "before"}, After: target,
			CompletionConditionIDs: []string{"C1"}, ChangeKind: "Preserve public output", Decisions: []form.Reference{{RecordID: recordID(5)}}},
		&ledger.Judgment{Judgment: judgment, RoundID: recordID(4)}, &ledger.Judgment{Judgment: newer, RoundID: recordID(4)},
		&ledger.Question{Question: question("review", "Approve the current result?")})
	input.Targets, input.Norms = []form.TargetVersion{target}, norms
	html := render(t, input)
	for _, want := range []string{"Verification coverage map", "Public output", "Guide examples", "Compare the output", "C2: The guide is updated",
		"No check recorded for this condition.", "Norm mismatch", "A newer result exists", "Unresolved blocking finding F1", "Check to reassess on withdrawal", "Withdrawal requirements"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Index(html, "Unresolved blocking finding F1") > strings.Index(html, "Review provisional decision") {
		t.Fatal("a provisional decision appeared before a blocking item")
	}
	if strings.Count(html, "The output contract needs a decision") != 1 {
		t.Fatal("finding body was duplicated")
	}
	fields := judgmentFields(input, recordID(8), &ledger.Judgment{Judgment: newer, RoundID: recordID(4)})
	if fields[1].Value == "Valid" {
		t.Fatal("a judgment with obsolete norms was marked valid")
	}
}

func TestGenerateEscapingAndStructureHistory(t *testing.T) {
	input := loadInput(t, nil, &ledger.Question{Question: question("unsafe", `<script>alert("test")</script>`)})
	before := Structure{Version: "old", Modules: []Module{{ID: "core", Name: "Core", Responsibility: "Compute", Path: "internal/core"}}}
	input.Structure = &Structure{Version: "new", Modules: []Module{before.Modules[0], {ID: "ui", Name: `UI"] --> injected["`, Path: "internal/ui"}},
		Dependencies: []Dependency{{From: "ui", To: "core"}}}
	input.StructureHistory = []StructureRevision{{Commit: "current", Structure: input.Structure}, {Commit: "previous", Structure: &before}, {Commit: "deleted"}}
	html := render(t, input)
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("record text was not escaped")
	}
	for _, want := range []string{"Added module: ui", "Structure at commit previous", "Structure deleted", "internal/core"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	mermaid := StructureMermaid(*input.Structure)
	if !strings.Contains(mermaid, "m1 -->|depends on| m0") || strings.Contains(mermaid, `injected[`) {
		t.Fatalf("unexpected dependency diagram: %s", mermaid)
	}
	if strings.Count(html, "Responsibility</th>") != 2 {
		t.Fatal("a repeated structure version duplicated the diagram body")
	}
}

func TestGenerateRequiresWork(t *testing.T) {
	if _, err := Generate(Input{}); err == nil {
		t.Fatal("accepted a missing ledger")
	}
	if _, err := Generate(Input{Ledger: &ledger.State{}}); err == nil {
		t.Fatal("accepted an empty ledger")
	}
}
