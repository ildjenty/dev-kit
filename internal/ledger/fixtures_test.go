package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/plan"
)

var fixtureValidator = sync.OnceValues(func() (*Validator, error) {
	sources := make(map[string][]byte)
	for _, name := range []string{"decision", "evidence", "progress", "question", "judgment", "finding", "reference", "instruction", "plan-template", "plan"} {
		source, err := os.ReadFile(filepath.Join("..", "..", "assets", "schemas", name+".json"))
		if err != nil {
			return nil, err
		}
		sources[name] = source
	}
	return NewValidator(sources)
})

func testValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := fixtureValidator()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// completeFixture supplies unrelated required fields for query test inputs.
// Invalid boundary inputs are constructed separately without this helper.
func completeFixture(payload Payload) {
	switch p := payload.(type) {
	case *Decision:
		if p.Content == "" {
			p.Content = "Choose an implementation"
		}
		p.Validity, p.Decider = "Until requirements change", "human"
		if p.Supersedes == nil {
			p.Supersedes = []form.Reference{}
		}
		if p.DependsOn == nil {
			p.DependsOn = []form.Reference{}
		}
		p.Conditions = []string{}
		p.RejectedAlternatives = []form.RejectedAlternative{}
		if p.Evidence == nil {
			p.Evidence = []form.Reference{}
		}
	case *Evidence:
		if p.Sources == nil {
			p.Sources = []form.Reference{}
		}
	case *Question:
		q := &p.Question
		if q.Text == "" {
			q.Text = "Approve this target?"
		}
		q.Options = []string{"approve", "reject", "defer"}
		q.StopCategory, q.StopReason = "approval", "Human approval is required"
		if q.Evidence == nil {
			q.Evidence = []form.Reference{}
		}
	case *Answer:
		completeAnswerFixture(p)
	case *Approval:
		completeAnswerFixture(&p.Answer)
	case *Round:
		if p.StartTarget.Kind == "" {
			p.StartTarget = form.TargetVersion{Kind: form.TargetTree, Tree: "before"}
		}
		if p.Norms == nil {
			p.Norms = []form.NormReference{}
		}
	case *Progress:
		for i := range p.TemporaryState {
			p.TemporaryState[i].VerificationMethod = "Inspect cache"
			p.TemporaryState[i].Source = "Cache directory"
		}
	case *Judgment:
		if p.InstructionID == "" {
			p.InstructionID = "judge"
		}
		if p.Aspect == "" {
			p.Aspect = "test"
		}
		if p.TargetVersion.Kind == "" {
			p.TargetVersion = form.TargetVersion{Kind: form.TargetTree, Tree: "result"}
		}
		if p.Norms == nil {
			p.Norms = []form.NormReference{}
		}
		p.CompletionConditionIDs = []string{}
		if p.Findings == nil {
			p.Findings = []form.Finding{}
		}
		for i := range p.Findings {
			if p.Findings[i].Kind == "" {
				p.Findings[i].Kind = form.AutoFixable
			}
			p.Findings[i].Scenario = "A required test is missing"
		}
		p.CheckedScope, p.UncheckedScope = []string{}, []string{}
		p.Method, p.Environment = "Inspection", "Local test"
		if p.Runner == "" {
			p.Runner = "judge"
		}
		p.Provenance = check.Provenance{Source: check.SubagentResult, InstructionID: p.InstructionID, Runner: p.Runner}
	case *PlanVersion:
		p.Plan = completePlanFixture(p.Plan)
	}
}

func completeAnswerFixture(a *Answer) {
	if a.Original == "" {
		a.Original = "Approved"
	}
	if a.SessionID == "" {
		a.SessionID = "session"
	}
	if a.ObservedAt.IsZero() {
		a.ObservedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
}

func completePlanFixture(p plan.Plan) plan.Plan {
	if p.Template.Name == "" {
		p.Template.Name = "simple"
	}
	p.Template.ContentHash = strings.Repeat("a", 64)
	if p.Task == "" {
		p.Task = "Implement the task"
	}
	if len(p.CompletionConditions) == 0 {
		p.CompletionConditions = []plan.CompletionCondition{{ID: "done", Description: "Task implemented"}}
	}
	if len(p.Stages) == 0 {
		p.Stages = []plan.Stage{{ID: "implement", Transitions: plan.Transitions{Fix: "implement", Ask: "implement", Back: "implement"}}}
	}
	if p.StartStage == "" {
		p.StartStage = p.Stages[0].ID
	}
	for i := range p.Stages {
		p.Stages[i].Assignee = plan.Worker
		p.Stages[i].Gate.Reason = "No approval gate required"
		if p.Stages[i].Checks == nil {
			p.Stages[i].Checks = []plan.Check{}
		}
		p.Stages[i].Limits = plan.Limits{MaxRounds: 3, MaxNoProgress: 3}
	}
	if p.ExternalOperations == nil {
		p.ExternalOperations = []plan.ExternalOperation{}
	}
	return p
}
