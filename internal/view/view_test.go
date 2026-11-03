package view

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	decisions, err := state.ResolveDecisions(ledger.DecisionContext{})
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

func answeredInput(t *testing.T) Input {
	t.Helper()
	old := question("review-1", "Approve the previous result?")
	stopQuestion := question("continue", "May the work continue?")
	stopQuestion.ApprovalTarget = form.ApprovalTarget{Kind: form.ApprovalWork, WorkID: "work"}
	input := loadInput(t, []ledger.StopObservation{{ID: "stop", WorkID: "work", LimitReached: true, Question: &stopQuestion}},
		&ledger.Question{Question: old},
		&ledger.Approval{Answer: ledger.Answer{QuestionID: old.ID, Target: old.ApprovalTarget, Interpretation: ledger.AnswerApproved,
			Original: "Approved", SessionID: "session", ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		&ledger.Lesson{Content: "Keep the existing contract", Evidence: []form.Reference{}},
		&ledger.Pause{Position: form.Reference{RecordID: recordID(1)}, Reason: "Requested by the person"})
	return input
}

func TestGenerateNewRecords(t *testing.T) {
	html := render(t, answeredInput(t))
	recent := sectionHTML(t, html, "recent")
	if strings.Contains(recent, recordID(3)) || !strings.Contains(recent, recordID(4)) || !strings.Contains(recent, recordID(5)) {
		t.Fatalf("unexpected new-record boundary: %s", recent)
	}
}

func TestGeneratePausedGuidance(t *testing.T) {
	html := render(t, answeredInput(t))
	if !strings.Contains(html, "Run devkit resume") {
		t.Fatal("missing explicit resume guidance")
	}
}

func TestGenerateQuestions(t *testing.T) {
	html := render(t, answeredInput(t))
	for _, want := range []string{"May the work continue?", "Answer in your own words.", "review-1: approved"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, text := range []string{"Approve the previous result?", "May the work continue?"} {
		if strings.Count(html, text) != 1 {
			t.Errorf("question %q does not have one body", text)
		}
	}
}

func TestGenerateRecordCoverage(t *testing.T) {
	html := render(t, answeredInput(t))
	if !strings.Contains(html, "Limited to artifacts recorded by submit.") {
		t.Error("missing record coverage")
	}
}

func TestGenerateReadOnly(t *testing.T) {
	input := answeredInput(t)
	render(t, input)
	if len(input.Ledger.UnansweredQuestions()) != 1 || input.Ledger.LastRecordID() != recordID(5) {
		t.Error("rendering changed ledger state")
	}
}

func TestGeneratePreservesConditions(t *testing.T) {
	decision := form.Decision{
		ID: "conditional", Content: "Keep the current service", Validity: "While the owner accepts the cost",
		Conditions: []string{"The requirements still apply"}, Decider: "human", Provisional: true,
		Supersedes: []form.Reference{}, DependsOn: []form.Reference{{External: "https://example.invalid/service"}},
		RejectedAlternatives: []form.RejectedAlternative{}, Evidence: []form.Reference{},
	}
	temporary := form.TemporaryState{Content: "Existing cache", VerificationMethod: "Inspect the cache", Source: "Cache directory", InvalidWhen: "The cache is replaced"}
	input := loadInput(t, nil, &ledger.Decision{Decision: decision},
		&ledger.StageRun{ID: recordID(3), StageID: "implement", PlanVersion: 1, State: ledger.StageWorking},
		&ledger.Round{ID: recordID(4), StageRunID: recordID(3), SessionID: "session", StartTarget: form.TargetVersion{Kind: form.TargetTree, Tree: "before"}, Norms: []form.NormReference{}},
		&ledger.Progress{RoundID: recordID(4), Progress: form.Progress{InstructionID: "work", TerminationKind: form.TerminationCompleted, TemporaryState: []form.TemporaryState{temporary}}})
	html := render(t, input)
	for _, want := range []string{decision.Validity, decision.Conditions[0], decision.DependsOn[0].External, temporary.Content, temporary.VerificationMethod, temporary.Source, temporary.InvalidWhen, "For agents and people to assess"} {
		if !strings.Contains(html, want) {
			t.Errorf("view did not preserve %q", want)
		}
	}
	body := articleContaining(t, sectionHTML(t, html, "current"), "Decision conditional")
	if strings.Contains(body, "<dt>Validity</dt><dd>true</dd>") {
		t.Fatal("graph state was presented as condition validity")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	input := answeredInput(t)
	html := render(t, input)
	if second := render(t, input); second != html {
		t.Error("identical input did not produce identical HTML")
	}
}

func coverageInput(t *testing.T) Input {
	t.Helper()
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
		UncheckedScope: []string{"Guide examples"}, Method: "Compare the output", Environment: "Local checkout"}, Runner: "review",
		Provenance: check.Provenance{Source: check.JudgeCommand, InstructionID: "check-1", Runner: "review"}}
	newer := judgment
	newer.InstructionID = "check-2"
	newer.Provenance.InstructionID = newer.InstructionID
	newer.Norms = []form.NormReference{{Path: "AGENTS.md", BlobID: "norm-old"}}
	newer.JudgmentResult.Judgment = form.Fail
	newer.Findings = []form.Finding{{ID: "F1", Blocking: true, Kind: form.NeedsDecision, Scenario: "The output contract needs a decision"}}
	current := judgment
	current.InstructionID, current.Runner = "check-current", "current-review"
	current.Provenance.InstructionID, current.Provenance.Runner = current.InstructionID, current.Runner
	p.Stages[0].Checks = append(p.Stages[0].Checks, plan.Check{Aspect: "contract", Runner: "current-review", Required: true, CompletionConditionIDs: []string{"C1"}, Target: "returned tree"})
	input := loadInput(t, nil, &ledger.PlanVersion{Plan: p, Origin: plan.InitialProposal},
		&ledger.StageRun{ID: recordID(3), StageID: "implement", PlanVersion: 1, State: ledger.StageJudgmentWait, StopReason: "Checks are pending", NextActor: "judge"},
		&ledger.Round{ID: recordID(4), StageRunID: recordID(3), SessionID: "session", StartTarget: target, ReturnedTarget: &target, Norms: norms},
		&ledger.Decision{Decision: decision},
		&ledger.Artifact{Before: form.TargetVersion{Kind: form.TargetTree, Tree: "before"}, After: target,
			CompletionConditionIDs: []string{"C1"}, ChangeKind: "Preserve public output", Decisions: []form.Reference{{RecordID: recordID(5)}}},
		&ledger.Judgment{Judgment: judgment, RoundID: recordID(4)}, &ledger.Judgment{Judgment: newer, RoundID: recordID(4)},
		&ledger.Judgment{Judgment: current, RoundID: recordID(4)},
		&ledger.Question{Question: question("review", "Approve the current result?")})
	input.Targets, input.Norms = []form.TargetVersion{target}, norms
	return input
}

func TestGenerateCoverage(t *testing.T) {
	html := render(t, coverageInput(t))
	coverage := sectionHTML(t, html, "coverage")
	rows := regexp.MustCompile(`(?s)<tr><td>.*?</tr>`).FindAllString(coverage, -1)
	want := [][]string{
		{"C1: Existing output is preserved", "Tree current", "Compare the output<p><a href=\"#" + anchor("record", recordID(9)) + "\">contract</a></p>", "Public output", "Guide examples", "PASS<br>Valid", "Unreviewed"},
		{"C2: The guide is updated", "", "", "", "No current check recorded for this condition.", "NOT ASSESSED<br>", "Unreviewed"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d coverage rows, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		cells := regexp.MustCompile(`(?s)<td>(.*?)</td>`).FindAllStringSubmatch(row, -1)
		if len(cells) != len(want[i]) {
			t.Fatalf("coverage row %d has %d cells, want %d", i, len(cells), len(want[i]))
		}
		for j, cell := range cells {
			if cell[1] != want[i][j] {
				t.Errorf("coverage row %d cell %d = %q, want %q", i, j, cell[1], want[i][j])
			}
		}
	}
	if strings.Contains(coverage, "Invalid for current comparison") {
		t.Error("historical checks appeared in current coverage")
	}
}

func TestGenerateJudgmentValidity(t *testing.T) {
	html := render(t, coverageInput(t))
	for _, test := range []struct{ instruction, validity, reason, section string }{
		{"check-1", "Invalid for current comparison", "A newer result exists", "history"},
		{"check-2", "Invalid for current comparison", "Norm mismatch", "history"},
		{"check-current", "Valid", "", "current"},
	} {
		t.Run(test.instruction, func(t *testing.T) {
			body := articleContaining(t, sectionHTML(t, html, test.section), "Instruction ID: "+test.instruction+"\n")
			if !strings.Contains(body, "<dt>Validity</dt><dd>"+test.validity+"</dd>") {
				t.Errorf("unexpected judgment validity: %s", body)
			}
			if !strings.Contains(body, test.reason) {
				t.Errorf("missing reason %q", test.reason)
			}
		})
	}
}

func TestGenerateBlockingFinding(t *testing.T) {
	html := render(t, coverageInput(t))
	if strings.Index(html, "Unresolved blocking finding F1") > strings.Index(html, "Review provisional decision") || !strings.Contains(html, "Unresolved blocking finding F1") {
		t.Error("a blocking item did not appear before provisional decisions")
	}
	if strings.Count(html, "The output contract needs a decision") != 1 {
		t.Error("finding does not have one body")
	}
}

func TestGenerateDecisionDependencies(t *testing.T) {
	html := render(t, coverageInput(t))
	body := articleContaining(t, sectionHTML(t, html, "current"), "<h3>Decision D1</h3>")
	for _, want := range []string{"Affected change: Preserve public output", "Check to reassess on withdrawal", "While the contract applies"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, "Withdrawal requirements") || strings.Contains(body, "not recorded") {
		t.Error("unrecorded withdrawal requirements were displayed")
	}
}

func TestGenerateQuestionDecisionConditions(t *testing.T) {
	decision := form.Decision{ID: "D1", Content: "Keep the current format", Validity: "While the contract applies", Provisional: true,
		Supersedes: []form.Reference{}, DependsOn: []form.Reference{}, Conditions: []string{"The owner accepts the cost", "The service remains available"}, Decider: "worker",
		RejectedAlternatives: []form.RejectedAlternative{}, Evidence: []form.Reference{}}
	unrelated := decision
	unrelated.ID, unrelated.Content, unrelated.Validity = "D2", "Keep the unrelated service", "While the unrelated contract applies"
	unrelated.Conditions = []string{"The unrelated owner accepts the cost"}
	for _, target := range []form.ApprovalTarget{
		{Kind: form.ApprovalTree, Tree: "current"},
		{Kind: form.ApprovalDecision, DecisionID: "D1"},
	} {
		t.Run(string(target.Kind), func(t *testing.T) {
			q := question("review", "Approve the current result?")
			q.ApprovalTarget = target
			input := loadInput(t, nil, &ledger.Decision{Decision: decision}, &ledger.Decision{Decision: unrelated},
				&ledger.Artifact{Before: form.TargetVersion{Kind: form.TargetTree, Tree: "before"}, After: form.TargetVersion{Kind: form.TargetTree, Tree: "current"},
					ChangeKind: "Preserve public output", CompletionConditionIDs: []string{}, Decisions: []form.Reference{{RecordID: recordID(2)}}},
				&ledger.Question{Question: q})
			html := render(t, input)
			body := articleContaining(t, sectionHTML(t, html, "attention"), q.Text)
			for _, want := range []string{
				"<dt>Decision D1 validity</dt><dd>" + decision.Validity + "</dd>",
				"<dt>Decision D1 conditions</dt><dd>" + strings.Join(decision.Conditions, "\n") + "</dd>",
				`href="#` + anchor("record", recordID(2)) + `"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("question is missing %q", want)
				}
			}
			if strings.Contains(body, unrelated.Validity) || strings.Contains(body, unrelated.Conditions[0]) {
				t.Error("question includes conditions from an unrelated decision")
			}
			if strings.Count(html, decision.Content) != 1 {
				t.Error("related decision does not have one body")
			}
		})
	}
}

func structureInput(t *testing.T) Input {
	t.Helper()
	input := loadInput(t, nil)
	before := Structure{Version: "old", Modules: []Module{{ID: "core", Name: "Core", Responsibility: "Compute", Path: "internal/core"}}}
	input.Structure = &Structure{Version: "new", Modules: []Module{before.Modules[0], {ID: "ui", Name: "UI", Path: "internal/ui"}},
		Dependencies: []Dependency{{From: "ui", To: "core"}}}
	input.StructureHistory = []StructureRevision{{Commit: "current", Structure: input.Structure}, {Commit: "previous", Structure: &before}, {Commit: "deleted"}}
	return input
}

func TestGenerateEscaping(t *testing.T) {
	input := loadInput(t, nil, &ledger.Question{Question: question("unsafe", `<script>alert("test")</script>`)})
	html := render(t, input)
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("record text was not escaped")
	}
}

func TestGenerateStructureHistory(t *testing.T) {
	html := render(t, structureInput(t))
	current := sectionHTML(t, html, "structure")
	history := sectionHTML(t, html, "history")
	if !strings.Contains(current, "Added module: ui") || strings.Contains(current, "Structure at commit") || strings.Contains(current, "Structure deleted") {
		t.Error("current structure contains historical bodies or omits the current change")
	}
	for _, want := range []string{"Structure at commit previous", "Structure deleted", "internal/core"} {
		if !strings.Contains(history, want) {
			t.Errorf("missing %q in structure history", want)
		}
	}
	bodies := regexp.MustCompile(`(?s)<details id="([^"]+)">(.*?)</details>`).FindAllStringSubmatch(html, -1)
	for _, version := range []string{"new", "old"} {
		count := 0
		for _, body := range bodies {
			if strings.Contains(body[2], " — "+version+"</summary>") {
				count++
			}
		}
		if count != 1 {
			t.Errorf("structure version %q has %d bodies", version, count)
		}
	}
}

func TestStructureMermaidDependencyDirection(t *testing.T) {
	mermaid := StructureMermaid(*structureInput(t).Structure)
	nodes := make(map[string]string)
	for _, node := range regexp.MustCompile(`(?m)^\s*(\w+)\["([^"]+)"\]`).FindAllStringSubmatch(mermaid, -1) {
		label := regexp.MustCompile(`#([0-9]+);`).ReplaceAllStringFunc(node[2], func(entity string) string {
			value, err := strconv.Atoi(entity[1 : len(entity)-1])
			if err != nil {
				t.Fatal(err)
			}
			return string(rune(value))
		})
		nodes[label] = node[1]
	}
	if nodes["Core"] == "" || nodes["UI"] == "" || !strings.Contains(mermaid, nodes["UI"]+" -->|depends on| "+nodes["Core"]) {
		t.Fatalf("diagram does not show UI depending on Core: %s", mermaid)
	}
}

func TestStructureMermaidEscaping(t *testing.T) {
	structure := Structure{Modules: []Module{{ID: "unsafe", Name: `UI"] --> injected["`}}}
	mermaid := StructureMermaid(structure)
	if strings.Contains(mermaid, `injected[`) || !strings.HasPrefix(mermaid, "flowchart LR") {
		t.Fatalf("unsafe module name was not encoded: %s", mermaid)
	}
	node := regexp.MustCompile(`(?m)^  m0\["([^"]+)"\]$`).FindStringSubmatch(mermaid)
	if len(node) != 2 {
		t.Fatalf("diagram is missing the encoded module: %s", mermaid)
	}
	label := regexp.MustCompile(`#([0-9]+);`).ReplaceAllStringFunc(node[1], func(entity string) string {
		value, err := strconv.Atoi(entity[1 : len(entity)-1])
		if err != nil {
			t.Fatal(err)
		}
		return string(rune(value))
	})
	if label != structure.Modules[0].Name {
		t.Fatalf("module label = %q, want %q", label, structure.Modules[0].Name)
	}
}

func sectionHTML(t *testing.T, html, id string) string {
	t.Helper()
	_, section, ok := strings.Cut(html, `<section id="`+id+`">`)
	if !ok {
		t.Fatalf("missing section %q", id)
	}
	section, _, _ = strings.Cut(section, "</section>")
	return section
}

func articleContaining(t *testing.T, html, text string) string {
	t.Helper()
	var found []string
	for _, body := range regexp.MustCompile(`(?s)<article\b[^>]*>.*?</article>`).FindAllString(html, -1) {
		if strings.Contains(body, text) {
			found = append(found, body)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d articles containing %q", len(found), text)
	}
	return found[0]
}

func TestGenerateRequiresWork(t *testing.T) {
	if _, err := Generate(Input{}); err == nil {
		t.Fatal("accepted a missing ledger")
	}
	if _, err := Generate(Input{Ledger: &ledger.State{}}); err == nil {
		t.Fatal("accepted an empty ledger")
	}
}

func TestGenerateQuestionAnswersSurviveReplacement(t *testing.T) {
	old := question("old-review", "Approve the old result?")
	replacement := question("new-review", "Approve the replacement result?")
	input := loadInput(t, nil,
		&ledger.Question{Question: old},
		&ledger.Approval{Answer: ledger.Answer{QuestionID: old.ID, Target: old.ApprovalTarget, Interpretation: ledger.AnswerApproved,
			Original: "Approved", SessionID: "session", ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		&ledger.Question{Question: replacement, Supersedes: old.ID})
	html := render(t, input)
	historical := articleContaining(t, sectionHTML(t, html, "history"), old.Text)
	if !strings.Contains(historical, "<dt>Approval state</dt><dd>approved</dd>") || !strings.Contains(historical, "<dt>Effective approval for target</dt><dd>No</dd>") {
		t.Errorf("the original answer was lost or treated as effective: %s", historical)
	}
	pending := articleContaining(t, sectionHTML(t, html, "attention"), replacement.Text)
	if !strings.Contains(pending, "<dt>Approval state</dt><dd>Unreviewed</dd>") {
		t.Errorf("the replacement inherited an old answer: %s", pending)
	}
	if !strings.Contains(sectionHTML(t, html, "answered"), "old-review: approved") {
		t.Error("the answered list lost the original answer")
	}
}

func TestGenerateEffectiveQuestionApproval(t *testing.T) {
	body := articleContaining(t, render(t, answeredInput(t)), "Approve the previous result?")
	if !strings.Contains(body, "<dt>Effective approval for target</dt><dd>Yes</dd>") {
		t.Error("an effective approval was not identified")
	}
}

func TestGenerateCurrentChanges(t *testing.T) {
	before := form.TargetVersion{Kind: form.TargetTree, Tree: "before"}
	after := form.TargetVersion{Kind: form.TargetTree, Tree: "after"}
	input := loadInput(t, nil,
		&ledger.Artifact{Before: before, After: after, ChangeKind: "Add feature", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}},
		&ledger.Artifact{Before: after, After: before, ChangeKind: "Remove feature", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}})
	input.Targets = []form.TargetVersion{before}
	html := render(t, input)
	current, history := sectionHTML(t, html, "current"), sectionHTML(t, html, "history")
	if strings.Contains(current, "Change Add feature") || !strings.Contains(current, "Change Remove feature") {
		t.Error("current changes include an undone change or omit its replacement")
	}
	if !strings.Contains(history, "Change Add feature") || strings.Contains(history, "Change Remove feature") {
		t.Error("change history does not preserve the undone change separately")
	}
	header := regexp.MustCompile(`(?s)<header>.*?</header>`).FindString(html)
	if strings.Contains(header, "Add feature") || !strings.Contains(header, "Remove feature") {
		t.Error("current achievement includes an undone change or omits the current change")
	}
}

func TestGenerateReintroducedChanges(t *testing.T) {
	before := form.TargetVersion{Kind: form.TargetTree, Tree: "before"}
	after := form.TargetVersion{Kind: form.TargetTree, Tree: "after"}
	input := loadInput(t, nil,
		&ledger.Artifact{Before: before, After: after, ChangeKind: "Add feature", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}},
		&ledger.Artifact{Before: after, After: before, ChangeKind: "Remove feature", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}},
		&ledger.Artifact{Before: before, After: after, ChangeKind: "Restore feature", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}})
	input.Targets = []form.TargetVersion{after}
	html := render(t, input)
	current, history := sectionHTML(t, html, "current"), sectionHTML(t, html, "history")
	if strings.Contains(current, "Change Add feature") || strings.Contains(current, "Change Remove feature") || !strings.Contains(current, "Change Restore feature") {
		t.Error("an earlier occurrence of the current target was treated as a current change")
	}
	if !strings.Contains(history, "Change Add feature") || !strings.Contains(history, "Change Remove feature") {
		t.Error("replaced changes were not retained in history")
	}
}

func TestGenerateJudgmentTargetMismatch(t *testing.T) {
	input := coverageInput(t)
	input.Targets = []form.TargetVersion{{Kind: form.TargetTree, Tree: "different"}}
	html := render(t, input)
	body := articleContaining(t, sectionHTML(t, html, "history"), "Instruction ID: check-current\n")
	if !strings.Contains(body, "<dt>Validity</dt><dd>Invalid for current comparison</dd>") || !strings.Contains(body, "Target mismatch") {
		t.Error("a check for a different target was not identified as invalid")
	}
	if strings.Contains(sectionHTML(t, html, "coverage"), "Public output") {
		t.Error("coverage includes a check for a different target")
	}
}

func TestGenerateRejectsInvalidStructure(t *testing.T) {
	for _, test := range []struct {
		name      string
		structure Structure
	}{
		{"duplicate module ID", Structure{Modules: []Module{{ID: "core"}, {ID: "core"}}}},
		{"missing dependent", Structure{Modules: []Module{{ID: "core"}}, Dependencies: []Dependency{{From: "missing", To: "core"}}}},
		{"missing dependency", Structure{Modules: []Module{{ID: "core"}}, Dependencies: []Dependency{{From: "core", To: "missing"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, historical := range []bool{false, true} {
				t.Run(fmt.Sprintf("historical=%t", historical), func(t *testing.T) {
					input := loadInput(t, nil)
					if historical {
						input.StructureHistory = []StructureRevision{{Commit: "invalid", Structure: &test.structure}}
					} else {
						input.Structure = &test.structure
					}
					if html, err := Generate(input); err == nil || html != "" {
						t.Error("invalid structure produced a successful view")
					}
				})
			}
			if text := StructureMermaid(test.structure); text != "" {
				t.Errorf("invalid structure produced a diagram: %s", text)
			}
		})
	}
}
