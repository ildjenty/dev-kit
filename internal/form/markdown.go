package form

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderMarkdown renders a fixed-format value for people.
// It accepts Progress, JudgmentResult, PlannerResult, Finding, Question, Instruction, Decision, and Evidence values.
func RenderMarkdown(value any) (string, error) {
	var m markdown
	switch v := value.(type) {
	case Progress:
		if err := m.progress(v); err != nil {
			return "", err
		}
	case JudgmentResult:
		m.title("Judgment result")
		m.field("Judgment", string(v.Judgment))
		m.field("Aspect", v.Aspect)
		m.field("Instruction ID", v.InstructionID)
		if err := m.json("Target identification", v.TargetVersion); err != nil {
			return "", err
		}
		if err := m.json("Norm identification", v.Norms); err != nil {
			return "", err
		}
		m.list("Checked completion condition IDs", v.CompletionConditionIDs)
		m.list("Checked scope", v.CheckedScope)
		m.list("Unchecked scope", v.UncheckedScope)
		m.field("Method", v.Method)
		m.field("Environment", v.Environment)
		for _, finding := range v.Findings {
			m.finding(finding)
		}
	case PlannerResult:
		m.title("Planner result")
		m.field("Instruction ID", v.InstructionID)
		if err := m.json("Plan", v.Plan); err != nil {
			return "", err
		}
	case Finding:
		m.finding(v)
	case Question:
		if err := m.question(v); err != nil {
			return "", err
		}
	case Instruction:
		m.title("Next instruction")
		m.field("Instruction ID", v.ID)
		m.field("Kind", string(v.Kind))
		m.field("Target key", v.TargetKey)
		m.field("Reissue count", fmt.Sprint(v.Reissue))
		m.field("Instruction", v.Text)
		m.field("brief", v.Brief)
		m.field("Target extraction path", v.TargetPath)
		m.field("Runner", v.Runner)
		if v.Question != nil {
			if err := m.question(*v.Question); err != nil {
				return "", err
			}
		}
		if v.External != nil {
			if err := m.json("External operation approval target", v.External); err != nil {
				return "", err
			}
		}
	case Decision:
		m.title("Decision")
		m.field("Content", v.Content)
		m.field("Decision ID", v.ID)
		m.field("Validity", v.Validity)
		if v.Provisional {
			m.field("Finality", "Provisional")
		} else {
			m.field("Finality", "Final")
		}
		m.field("Decider", v.Decider)
		m.list("Conditions", v.Conditions)
		m.references("Supersedes", v.Supersedes)
		m.references("Depends on decisions", v.DependsOn)
		m.references("Evidence", v.Evidence)
		for _, alternative := range v.RejectedAlternatives {
			m.heading("Rejected alternative")
			m.field("Proposal", alternative.Proposal)
			m.field("Reason", alternative.Reason)
			m.field("Example", alternative.Example)
		}
	case Evidence:
		m.title("Evidence")
		m.field("Investigation results", v.Content)
		m.field("Evidence ID", v.ID)
		m.references("Sources", v.Sources)
		m.field("Verification method", v.VerificationMethod)
	default:
		return "", fmt.Errorf("Unsupported type for Markdown rendering: %T", value)
	}
	return m.String(), nil
}

type markdown struct {
	strings.Builder
}

func (m *markdown) title(title string) {
	m.WriteString("# " + title + "\n\n")
}

func (m *markdown) heading(title string) {
	m.WriteString("## " + title + "\n\n")
}

func (m *markdown) field(label, value string) {
	if value != "" {
		m.WriteString(label + ": " + value + "\n\n")
	}
}

func (m *markdown) list(title string, items []string) {
	if len(items) == 0 {
		return
	}
	m.heading(title)
	for _, item := range items {
		m.WriteString("- " + strings.ReplaceAll(item, "\n", "\n  ") + "\n")
	}
	m.WriteString("\n")
}

func (m *markdown) references(title string, refs []Reference) {
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.RecordID != "" {
			items = append(items, "Record: "+ref.RecordID)
		} else {
			items = append(items, "External reference: "+ref.External)
		}
	}
	m.list(title, items)
}

func (m *markdown) json(title string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	m.heading(title)
	m.WriteString("```json\n" + string(data) + "\n```\n\n")
	return nil
}

func (m *markdown) finding(v Finding) {
	m.heading("Finding " + v.ID)
	m.field("Kind", string(v.Kind))
	m.field("blocking", fmt.Sprint(v.Blocking))
	m.field("Failure scenario", v.Scenario)
	if v.Exemption != nil {
		m.field("Exemption claim", v.Exemption.Claim)
		m.references("Exemption evidence", v.Exemption.Evidence)
	}
	m.list("Resolved finding IDs", v.ResolvedFindingIDs)
}

func (m *markdown) question(v Question) error {
	m.title("Confirmation")
	m.WriteString(v.Text + "\n\n")
	m.field("Question ID", v.ID)
	m.field("Screen ID", v.ScreenID)
	m.field("Stopped stage", v.StoppedStage)
	m.field("Stop category", v.StopCategory)
	m.field("Stop reason", v.StopReason)
	m.field("Next stage after answering", v.NextStage)
	if err := m.json("Approval target", v.ApprovalTarget); err != nil {
		return err
	}
	m.list("Options", v.Options)
	m.field("Recommendation", v.Recommendation)
	m.field("Recommendation reason", v.RecommendationReason)
	m.references("Evidence", v.Evidence)
	if v.StopObservation != nil {
		m.references("Stop observation", []Reference{*v.StopObservation})
	}
	m.WriteString("Answer in your own words. You may give an answer outside the options, a conditional answer, or ask a question in return. You may revise your answer later.\n\n")
	return nil
}

func (m *markdown) progress(v Progress) error {
	m.title("Work result")
	m.field("Termination kind", string(v.TerminationKind))
	m.field("Instruction ID", v.InstructionID)
	if len(v.Report) > 0 {
		if err := m.json("Stage report", v.Report); err != nil {
			return err
		}
	}
	if v.Confirmation != nil {
		m.heading("Confirmation request")
		m.WriteString(v.Confirmation.Text + "\n\n")
		m.list("Options", v.Confirmation.Options)
		m.field("Recommendation", v.Confirmation.Recommendation)
		m.field("Recommendation reason", v.Confirmation.RecommendationReason)
		m.references("Materials", v.Confirmation.Evidence)
	}
	m.list("Work performed", v.WorkPerformed)
	m.list("Confirmed", v.Confirmed)
	m.list("Unconfirmed", v.Unconfirmed)
	m.list("Obstacles", v.Obstacles)
	for _, state := range v.TemporaryState {
		m.heading("Temporary state")
		m.field("Content", state.Content)
		m.field("Verification method", state.VerificationMethod)
		m.field("Sources", state.Source)
		m.field("Invalid when", state.InvalidWhen)
	}
	m.list("Resume conditions", v.ResumeConditions)
	m.list("Out-of-scope work", v.OutOfScope)
	m.field("Change reason", v.ChangeReason)
	return nil
}
