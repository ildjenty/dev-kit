package view

import (
	"strings"
	"testing"
	"time"

	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

func TestGeneratePreservesOutsideTargetApprovalAndLabels(t *testing.T) {
	target := form.TargetVersion{Kind: form.TargetTree, Tree: "same-git-tree", Sources: strings.Repeat("a", 64)}
	other := target
	other.Sources = strings.Repeat("b", 64)
	q := question("outside-review", "Approve the outside product version?")
	q.ApprovalTarget = form.ApprovalTarget{Kind: form.ApprovalTree, Tree: target.Tree, Sources: target.Sources}
	judgment := check.Judgment{JudgmentResult: form.JudgmentResult{InstructionID: "outside-check", Aspect: "wording", TargetVersion: target, Norms: []form.NormReference{}, CompletionConditionIDs: []string{}, Judgment: form.Pass, Findings: []form.Finding{}, CheckedScope: []string{"Outside products"}, UncheckedScope: []string{}, Method: "Independent review", Environment: "Fixed inputs"}, Runner: "judge", Provenance: check.Provenance{Source: check.JudgeCommand, InstructionID: "outside-check", Runner: "judge"}}
	input := loadInput(t, nil,
		&ledger.Artifact{Before: other, After: target, ChangeKind: "Outside product edit", CompletionConditionIDs: []string{}, Decisions: []form.Reference{}},
		&ledger.Judgment{Judgment: judgment},
		&ledger.Question{Question: q},
		&ledger.Approval{Answer: ledger.Answer{QuestionID: q.ID, Target: q.ApprovalTarget, Interpretation: ledger.AnswerApproved, Original: "Approve this version", SessionID: "session", ObservedAt: time.Now()}},
	)
	input.Targets = []form.TargetVersion{target}
	input.Norms = []form.NormReference{}
	body := render(t, input)
	artifact := articleContaining(t, body, "<h3>Change Outside product edit</h3>")
	if !strings.Contains(artifact, "Approved") || !strings.Contains(artifact, target.Sources) || !strings.Contains(artifact, other.Sources) {
		t.Fatal("outside identity or matching approval was lost from the artifact")
	}
	review := articleContaining(t, body, "Instruction ID: outside-check\n")
	if !strings.Contains(review, "<dt>Approval state</dt><dd>approved</dd>") {
		t.Fatal("outside judgment failed to match its approval")
	}
	questionBody := articleContaining(t, body, q.Text)
	if !strings.Contains(questionBody, "sources "+target.Sources) {
		t.Fatal("approval label omitted outside identity")
	}
}
