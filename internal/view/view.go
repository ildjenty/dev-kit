package view

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
	"github.com/ildjenty/dev-kit/internal/plan"
)

// Input contains one already-loaded work snapshot. Records must be the ordered
// records used to load Ledger; Decisions must come from its ResolveDecisions query.
// Targets and Norms identify the current comparison context supplied by the caller.
// No files, git commands, or validity callbacks are accessed during generation.
// StructureText is the structure record body shown when it is not a drawable structure JSON.
type Input struct {
	Ledger           *ledger.State
	Records          []ledger.Record
	Decisions        []ledger.ResolvedDecision
	Targets          []form.TargetVersion
	Norms            []form.NormReference
	Structure        *Structure
	StructureText    *StructureText
	StructureHistory []StructureRevision
}

// StructureText is a structure record body and its content hash.
type StructureText struct {
	Version string
	Body    string
}

type link struct {
	Label string
	Href  string
}

type field struct {
	Label string
	Value string
}

type item struct {
	ID       string
	Title    string
	Tags     []string
	Fields   []field
	Links    []link
	Text     string
	Expanded bool
}

type coverage struct {
	Condition string
	Target    string
	Method    string
	Checked   string
	Unchecked string
	Judgment  string
	Validity  string
	Approval  string
	Check     link
}

type diagram struct {
	ID        string
	Title     string
	Version   string
	Changes   []string
	Modules   []Module
	Mermaid   string
	Text      string
	Reference link
}

type page struct {
	WorkID           string
	Summary          []field
	Attention        []item
	Current          []item
	History          []item
	Answered         []link
	Recent           []link
	Coverage         []coverage
	Structure        []diagram
	StructureHistory []diagram
}

//go:embed view.html
var pageHTML string

var pageTemplate = template.Must(template.New("view").Parse(pageHTML))

// Generate returns self-contained static HTML for one work item.
// It does not write files, change state, or fetch resources.
func Generate(input Input) (string, error) {
	if input.Ledger == nil {
		return "", fmt.Errorf("view requires a loaded ledger")
	}
	position := input.Ledger.CurrentPosition()
	if position.Work == nil {
		return "", fmt.Errorf("view requires a work record")
	}
	if input.Structure != nil {
		if err := validateStructure(*input.Structure); err != nil {
			return "", fmt.Errorf("current structure: %w", err)
		}
	}
	for _, revision := range input.StructureHistory {
		if revision.Structure != nil {
			if err := validateStructure(*revision.Structure); err != nil {
				return "", fmt.Errorf("structure at commit %s: %w", revision.Commit, err)
			}
		}
	}
	p := page{WorkID: position.Work.ID}
	lastAnswer := ""
	for _, record := range input.Records {
		switch record.Payload.(type) {
		case *ledger.Answer, *ledger.Approval:
			lastAnswer = record.ID
		}
	}
	decisions := make(map[string]ledger.ResolvedDecision, len(input.Decisions))
	decisionRecords := make(map[string]string, len(input.Decisions))
	for _, decision := range input.Decisions {
		decisions[decision.RecordID] = decision
		decisionRecords[decision.Decision.ID] = decision.RecordID
	}
	pending := input.Ledger.UnansweredQuestions()
	if err := p.addQuestions(input, pending, decisionRecords, lastAnswer); err != nil {
		return "", err
	}
	p.addBlockingFindings(input, decisionRecords)
	provisional, deferred, err := p.addRecords(input, decisions, decisionRecords, lastAnswer)
	if err != nil {
		return "", err
	}
	p.Attention = append(p.Attention, provisional...)
	p.Attention = append(p.Attention, deferred...)
	p.Summary = summary(input, position, pending, provisional, deferred)
	p.addCoverage(input, position)
	p.Structure, p.StructureHistory = structureDiagrams(input)
	var output bytes.Buffer
	if err := pageTemplate.Execute(&output, p); err != nil {
		return "", fmt.Errorf("render view: %w", err)
	}
	return output.String(), nil
}

func summary(input Input, position ledger.Position, pending []form.Question, provisional, deferred []item) []field {
	task := position.Work.TaskRef
	if position.Plan != nil {
		task = position.Plan.Plan.Task
	}
	completed := []string{}
	outcomes := []string{}
	for _, record := range input.Records {
		if artifact, ok := record.Payload.(*ledger.Artifact); ok && currentArtifact(input, artifact) {
			outcomes = append(outcomes, artifact.ChangeKind)
		}
	}
	for _, run := range input.Ledger.StageRuns() {
		if run.State == ledger.StageCompleted {
			completed = append(completed, stageLabel(input, run))
		}
	}
	state := string(position.Status)
	if position.Paused {
		state += "; paused"
	}
	stage, reason, actor := "Not started", "None recorded", "None recorded"
	if run := position.StageRun; run != nil {
		stage = stageLabel(input, run) + ": " + string(run.State)
		reason, actor = run.StopReason, run.NextActor
	}
	action := "No action required"
	switch {
	case position.Paused && position.Status == ledger.WorkActive:
		action = "Run devkit resume when you explicitly want to continue."
	case len(pending) != 0:
		action = "Answer question " + pending[0].ID + " in chat."
	case len(provisional) != 0:
		action = "Review provisional decisions at the next gate or PR review."
	case len(deferred) != 0:
		action = "Review the recorded conditions for deferred decisions."
	}
	return append([]field{{"Purpose", task}, {"Target", position.Work.TaskRef + "; branch at work start " + position.Work.Branch},
		{"Current achievement", "Completed stages: " + strings.Join(completed, ", ") + ". Recorded changes: " + strings.Join(outcomes, ", ") + "."},
		{"Execution state", state}, {"Stage run", stage}, {"Stop reason", reason}, {"Next actor", actor},
		{"Your next action", action}, {"Record coverage", "Limited to artifacts recorded by submit."}}, metricFields(input.Ledger.Metrics())...)
}

// metricFields shows per-work counts derived from the records.
func metricFields(m ledger.Metrics) []field {
	stages := []string{}
	for _, stage := range m.Stages {
		stages = append(stages, fmt.Sprintf("%s %d rounds in %d runs", stage.StageID, stage.Rounds, stage.Runs))
	}
	reasons := []string{}
	for _, reason := range m.StopReasons {
		reasons = append(reasons, fmt.Sprintf("%s %d", reason.Category, reason.Count))
	}
	return []field{
		{"Rounds", fmt.Sprintf("%d in total. %s", m.TotalRounds, strings.Join(stages, "; "))},
		{"Human stops", fmt.Sprintf("%d. %s", m.HumanStops, strings.Join(reasons, "; "))},
		{"Assignee changes", fmt.Sprint(m.AssigneeChanges)},
		{"Detection tests", fmt.Sprint(m.DetectionTests)},
		{"Request sizes", fmt.Sprintf("%d briefs, %d bytes in total, largest %d bytes, %d with overflow files", m.Briefs, m.BriefBytes, m.LargestBrief, m.BriefOverflows)},
	}
}

func stageLabel(input Input, run *ledger.StageRun) string {
	for _, record := range input.Records {
		if proposal, ok := record.Payload.(*ledger.PlanVersion); ok && proposal.Plan.Version == run.PlanVersion {
			if stage, exists := proposal.Plan.Stage(run.StageID); exists {
				return stage.ID + " (" + string(stage.Assignee) + ")"
			}
		}
	}
	return run.StageID
}

func conditionLabels(input Input, ids []string) string {
	labels := slices.Clone(ids)
	position := input.Ledger.CurrentPosition()
	if position.Plan != nil {
		for i, id := range ids {
			for _, condition := range position.Plan.Plan.CompletionConditions {
				if condition.ID == id {
					labels[i] = id + ": " + condition.Description
				}
			}
		}
	}
	return strings.Join(labels, "\n")
}

func roundInRun(state *ledger.State, roundID, runID string) bool {
	return slices.ContainsFunc(state.Rounds(runID), func(round *ledger.Round) bool { return round.ID == roundID })
}

func currentArtifact(input Input, artifact *ledger.Artifact) bool {
	if artifact.Before == artifact.After || !slices.Contains(input.Targets, artifact.After) {
		return false
	}
	for i := len(input.Records) - 1; i >= 0; i-- {
		if latest, ok := input.Records[i].Payload.(*ledger.Artifact); ok && latest.After.Kind == artifact.After.Kind && latest.Before != latest.After {
			return latest == artifact
		}
	}
	return false
}

func currentJudgment(input Input, judgment *ledger.Judgment) bool {
	if !slices.Contains(input.Targets, judgment.TargetVersion) {
		return false
	}
	for _, latest := range input.Ledger.LatestJudgments(judgment.TargetVersion, input.Norms) {
		if latest.InstructionID == judgment.InstructionID && latest.Aspect == judgment.Aspect && latest.Runner == judgment.Runner {
			return true
		}
	}
	return false
}

func judgmentFields(input Input, judgment *ledger.Judgment) []field {
	fields := []field{{"Judgment", string(judgment.JudgmentResult.Judgment)}}
	targetMatch := slices.Contains(input.Targets, judgment.TargetVersion)
	normMatch := judgment.ValidFor(judgment.TargetVersion, input.Norms)
	current := currentJudgment(input, judgment)
	validity := "Valid"
	if !current {
		validity = "Invalid for current comparison"
	}
	fields = append(fields, field{"Validity", validity})
	if !targetMatch {
		fields = append(fields, field{"Target mismatch", "The recorded target is not in the supplied current targets."})
	}
	if !normMatch {
		fields = append(fields, field{"Norm mismatch", "The recorded norms differ from the supplied current norms."})
	}
	if targetMatch && normMatch && !current {
		fields = append(fields, field{"History", "A newer result exists for this aspect, runner, and target."})
	}
	for _, run := range input.Ledger.StageRuns() {
		if !roundInRun(input.Ledger, judgment.RoundID, run.ID) {
			continue
		}
		for _, record := range input.Records {
			proposal, ok := record.Payload.(*ledger.PlanVersion)
			if !ok || proposal.Plan.Version != run.PlanVersion {
				continue
			}
			stage, _ := proposal.Plan.Stage(run.StageID)
			if !slices.ContainsFunc(stage.Checks, func(check plan.Check) bool {
				return check.Aspect == judgment.Aspect && string(check.Runner) == judgment.Runner
			}) {
				fields = append(fields, field{"Runner mismatch", "The recorded aspect and runner do not match the bound stage checks."})
			}
		}
	}
	return fields
}

func questionChecks(input Input, target form.ApprovalTarget) []link {
	var links []link
	var targets []form.TargetVersion
	if target.Kind == form.ApprovalDecision {
		for _, decision := range input.Decisions {
			if decision.Decision.ID == target.DecisionID {
				for _, record := range input.Records {
					if artifact, ok := record.Payload.(*ledger.Artifact); ok && slices.Contains(artifact.Decisions, form.Reference{RecordID: decision.RecordID}) {
						targets = append(targets, artifact.After)
					}
				}
			}
		}
	}
	for _, record := range input.Records {
		if judgment, ok := record.Payload.(*ledger.Judgment); ok && (approvalTarget(judgment.TargetVersion) == target || slices.Contains(targets, judgment.TargetVersion)) {
			fields := judgmentFields(input, judgment)
			links = append(links, recordLink(record.ID, judgment.Aspect+": "+string(judgment.JudgmentResult.Judgment)+"; "+fields[1].Value))
		}
	}
	return links
}

func relatedChanges(records []ledger.Record, target form.ApprovalTarget, decisions map[string]string) []link {
	var links []link
	decisionID := decisions[target.DecisionID]
	if decisionID != "" {
		links = append(links, recordLink(decisionID, "Related decision "+target.DecisionID))
	}
	for _, record := range records {
		switch value := record.Payload.(type) {
		case *ledger.Artifact:
			if approvalTarget(value.After) == target || decisionID != "" && slices.Contains(value.Decisions, form.Reference{RecordID: decisionID}) {
				links = append(links, recordLink(record.ID, "Affected change: "+value.ChangeKind))
				links = append(links, references(value.Decisions)...)
			}
		case *ledger.Decision:
			if decisionID != "" && slices.Contains(value.DependsOn, form.Reference{RecordID: decisionID}) {
				links = append(links, recordLink(record.ID, "Dependent decision: "+value.ID))
			}
		}
	}
	return links
}

func dependentChecks(input Input, decisionID string) []link {
	var links []link
	for _, record := range input.Records {
		artifact, ok := record.Payload.(*ledger.Artifact)
		if !ok || !slices.Contains(artifact.Decisions, form.Reference{RecordID: decisionID}) {
			continue
		}
		for _, result := range input.Records {
			if judgment, ok := result.Payload.(*ledger.Judgment); ok && judgment.TargetVersion == artifact.After {
				links = append(links, recordLink(result.ID, "Check to reassess on withdrawal: "+judgment.Aspect))
			}
		}
	}
	return links
}

func references(refs []form.Reference) []link {
	links := make([]link, 0, len(refs))
	for _, reference := range refs {
		if reference.RecordID != "" {
			links = append(links, recordLink(reference.RecordID, "Record "+reference.RecordID))
		} else {
			links = append(links, link{reference.External, reference.External})
		}
	}
	return links
}

func recordLink(id, label string) link {
	return link{label, "#" + anchor("record", id)}
}

func anchor(kind, id string) string { return kind + "-" + hex.EncodeToString([]byte(id)) }

func targetLabel(target form.TargetVersion) string {
	switch target.Kind {
	case form.TargetTree:
		label := "Tree " + target.Tree
		if target.Sources != "" {
			label += "; sources " + target.Sources
		}
		return label
	case form.TargetPlan:
		return fmt.Sprintf("Plan %d", target.Plan)
	case form.TargetStructure:
		return "Structure " + target.Structure
	default:
		return "Not recorded"
	}
}

func approvalTarget(target form.TargetVersion) form.ApprovalTarget {
	switch target.Kind {
	case form.TargetTree:
		return form.ApprovalTarget{Kind: form.ApprovalTree, Tree: target.Tree, Sources: target.Sources}
	case form.TargetPlan:
		return form.ApprovalTarget{Kind: form.ApprovalPlan, Plan: target.Plan}
	default:
		return form.ApprovalTarget{}
	}
}

func approvalLabel(target form.ApprovalTarget) string {
	if target.SourcePin != "" {
		return "Tree " + target.Tree + "; source pin " + target.SourcePin
	}
	switch target.Kind {
	case form.ApprovalTree:
		label := "Tree " + target.Tree
		if target.Sources != "" {
			label += "; sources " + target.Sources
		}
		return label
	case form.ApprovalPlan:
		return fmt.Sprintf("Plan %d", target.Plan)
	case form.ApprovalDecision:
		return "Decision " + target.DecisionID
	case form.ApprovalWork:
		return "Work " + target.WorkID
	case form.ApprovalExternal:
		if target.External != nil {
			return target.External.Kind + " " + target.External.Target + " (" + target.External.OperationID + ")"
		}
	}
	return "Not recorded"
}

func structureDiagrams(input Input) ([]diagram, []diagram) {
	var diagrams []diagram
	var history []diagram
	seen := make(map[string]string)
	if input.Structure != nil {
		current := diagram{ID: anchor("structure", input.Structure.Version), Title: "Current dependency structure", Version: input.Structure.Version,
			Modules: input.Structure.Modules, Mermaid: structureMermaid(*input.Structure)}
		seen[input.Structure.Version] = current.ID
		for _, revision := range input.StructureHistory {
			if revision.Structure == nil || revision.Structure.Version != input.Structure.Version {
				current.Changes = structureChanges(revision.Structure, input.Structure)
				break
			}
		}
		diagrams = append(diagrams, current)
	} else if input.StructureText != nil {
		current := diagram{ID: anchor("structure", input.StructureText.Version), Title: "Current structure record", Version: input.StructureText.Version, Text: input.StructureText.Body}
		seen[input.StructureText.Version] = current.ID
		diagrams = append(diagrams, current)
	}
	for _, revision := range input.StructureHistory {
		entry := diagram{Title: "Structure at commit " + revision.Commit}
		if revision.Text != nil {
			entry.Version = revision.Text.Version
			if id, exists := seen[entry.Version]; exists {
				entry.Reference = link{Label: "Structure record for this version", Href: "#" + id}
			} else {
				entry.ID, entry.Text = anchor("structure", entry.Version), revision.Text.Body
				seen[entry.Version] = entry.ID
			}
		} else if revision.Structure == nil {
			entry.Changes = []string{"Structure deleted"}
		} else {
			entry.Version = revision.Structure.Version
			if id, exists := seen[entry.Version]; exists {
				entry.Reference = link{Label: "Dependency structure for this version", Href: "#" + id}
			} else {
				entry.ID = anchor("structure", entry.Version)
				seen[entry.Version] = entry.ID
				entry.Modules = revision.Structure.Modules
				entry.Mermaid = structureMermaid(*revision.Structure)
			}
		}
		history = append(history, entry)
	}
	return diagrams, history
}

func structureChanges(before, after *Structure) []string {
	var changes []string
	old := make(map[string]Module)
	if before != nil {
		for _, module := range before.Modules {
			old[module.ID] = module
		}
	}
	for _, module := range after.Modules {
		previous, exists := old[module.ID]
		if !exists {
			changes = append(changes, "Added module: "+module.ID)
		} else if previous != module {
			changes = append(changes, "Changed module: "+module.ID)
		}
		delete(old, module.ID)
	}
	if before != nil {
		for _, module := range before.Modules {
			if _, exists := old[module.ID]; exists {
				changes = append(changes, "Removed module: "+module.ID)
			}
		}
		for _, dependency := range before.Dependencies {
			if !slices.Contains(after.Dependencies, dependency) {
				changes = append(changes, "Removed dependency: "+dependency.From+" -> "+dependency.To)
			}
		}
	}
	for _, dependency := range after.Dependencies {
		if before == nil || !slices.Contains(before.Dependencies, dependency) {
			changes = append(changes, "Added dependency: "+dependency.From+" -> "+dependency.To)
		}
	}
	return changes
}

func (p *page) addQuestions(input Input, pending []form.Question, decisionRecords map[string]string, lastAnswer string) error {
	questionRecords := make(map[string]string)
	for _, record := range input.Records {
		if question, ok := record.Payload.(*ledger.Question); ok {
			questionRecords[question.ID] = record.ID
		}
	}
	pendingIDs := make(map[string]bool, len(pending))
	for _, question := range pending {
		pendingIDs[question.ID] = true
	}
	answers := input.Ledger.AnsweredQuestions()
	for _, question := range input.Ledger.Questions() {
		recordID := questionRecords[question.ID]
		id := anchor("question", question.ID)
		if recordID != "" {
			id = anchor("record", recordID)
		}
		text, err := form.RenderMarkdown(question)
		if err != nil {
			return err
		}
		entry := item{ID: id, Title: "Question " + question.ID, Text: text, Expanded: true}
		if recordID != "" && recordID > lastAnswer {
			entry.Tags = append(entry.Tags, "New")
		}
		entry.Links = references(question.Evidence)
		if question.StopObservation != nil {
			entry.Links = append(entry.Links, references([]form.Reference{*question.StopObservation})...)
		}
		entry.Links = append(entry.Links, relatedChanges(input.Records, question.ApprovalTarget, decisionRecords)...)
		entry.Fields = append(entry.Fields, field{"Target version", approvalLabel(question.ApprovalTarget)})
		entry.Links = append(entry.Links, questionChecks(input, question.ApprovalTarget)...)
		entry.Fields = append(entry.Fields, linkedDecisionFields(entry.Links, input.Decisions)...)
		if answer, ok := answers[question.ID]; ok {
			entry.Fields = append(entry.Fields, field{"Approval state", string(answer.Answer.Interpretation)})
			if answer.Answer.Interpretation == ledger.AnswerApproved {
				entry.Tags = append(entry.Tags, "Approved")
			}
			effective := "No"
			if approval, ok := input.Ledger.ResolveApproval(question.ApprovalTarget); ok && approval.RecordID == answer.RecordID {
				effective = "Yes"
			}
			entry.Fields = append(entry.Fields, field{"Effective approval for target", effective})
		} else {
			entry.Fields = append(entry.Fields, field{"Approval state", "Unreviewed"})
		}
		if pendingIDs[question.ID] {
			entry.Tags = append(entry.Tags, "Unreviewed")
			p.Attention = append(p.Attention, entry)
		} else {
			p.History = append(p.History, entry)
		}
		if answer, ok := answers[question.ID]; ok {
			p.Answered = append(p.Answered, link{question.ID + ": " + string(answer.Answer.Interpretation), "#" + anchor("record", answer.RecordID)})
		}
	}
	return nil
}

func linkedDecisionFields(links []link, decisions []ledger.ResolvedDecision) []field {
	var fields []field
	for _, decision := range decisions {
		if slices.ContainsFunc(links, func(ref link) bool {
			return ref.Href == "#"+anchor("record", decision.RecordID)
		}) {
			fields = append(fields, field{"Decision " + decision.Decision.ID + " validity", decision.Decision.Validity})
			if len(decision.Decision.Conditions) != 0 {
				fields = append(fields, field{"Decision " + decision.Decision.ID + " conditions", strings.Join(decision.Decision.Conditions, "\n")})
			}
		}
	}
	return fields
}

func (p *page) addBlockingFindings(input Input, decisionRecords map[string]string) {
	// Findings retain their original judgment as their single detailed body.
	for _, run := range input.Ledger.IncompleteStageRuns() {
		for _, finding := range input.Ledger.UnresolvedBlocking(run.ID) {
			entry := blockingFindingItem(input, run, finding.ID, decisionRecords)
			p.Attention = append(p.Attention, entry)
		}
	}
}

func blockingFindingItem(input Input, run *ledger.StageRun, findingID string, decisionRecords map[string]string) item {
	entry := item{Title: "Unresolved blocking finding " + findingID,
		Fields: []field{{"Stage", stageLabel(input, run)}, {"Stop reason", run.StopReason}, {"Next actor", run.NextActor}}}
	for i := len(input.Records) - 1; i >= 0; i-- {
		record := input.Records[i]
		judgment, ok := record.Payload.(*ledger.Judgment)
		if !ok || !roundInRun(input.Ledger, judgment.RoundID, run.ID) {
			continue
		}
		if slices.ContainsFunc(judgment.Findings, func(f form.Finding) bool { return f.ID == findingID }) {
			entry.Links = []link{recordLink(record.ID, "Finding and evidence")}
			entry.Fields = append(entry.Fields, field{"Target version", targetLabel(judgment.TargetVersion)})
			entry.Fields = append(entry.Fields, judgmentFields(input, judgment)...)
			entry.Links = append(entry.Links, relatedChanges(input.Records, approvalTarget(judgment.TargetVersion), decisionRecords)...)
			if answer, ok := input.Ledger.ResolveAnswer(approvalTarget(judgment.TargetVersion)); ok {
				entry.Fields = append(entry.Fields, field{"Approval state", string(answer.Answer.Interpretation)})
			} else {
				entry.Fields = append(entry.Fields, field{"Approval state", "Unreviewed"})
			}
			break
		}
	}
	return entry
}

func (p *page) addRecords(input Input, decisions map[string]ledger.ResolvedDecision, decisionRecords map[string]string, lastAnswer string) ([]item, []item, error) {
	var provisional, deferred []item
	for _, record := range input.Records {
		ref := recordLink(record.ID, string(record.Kind())+" "+record.ID)
		if record.ID > lastAnswer {
			p.Recent = append(p.Recent, ref)
		}
		if _, ok := record.Payload.(*ledger.Question); ok {
			continue
		}
		entry := item{ID: anchor("record", record.ID), Title: ref.Label}
		if record.ID > lastAnswer {
			entry.Tags = append(entry.Tags, "New")
		}
		current := false
		switch value := record.Payload.(type) {
		case *ledger.Decision:
			resolved, ok := decisions[record.ID]
			if !ok {
				return nil, nil, fmt.Errorf("view requires resolved decision %s", record.ID)
			}
			var err error
			entry, err = decisionRecordItem(input, entry, record.ID, value, resolved, decisionRecords)
			if err != nil {
				return nil, nil, err
			}
			current = resolved.Active
			review, reconsider := decisionAttention(record.ID, value, resolved)
			provisional = append(provisional, review...)
			deferred = append(deferred, reconsider...)
		case *ledger.Artifact:
			entry, current = artifactRecordItem(input, entry, value, decisions)
		case *ledger.Judgment:
			var err error
			entry, current, err = judgmentRecordItem(input, entry, value)
			if err != nil {
				return nil, nil, err
			}
		default:
			data, err := json.MarshalIndent(record.Payload, "", "  ")
			if err != nil {
				return nil, nil, fmt.Errorf("render record %s: %w", record.ID, err)
			}
			entry.Text = string(data)
		}
		if current {
			p.Current = append(p.Current, entry)
		} else {
			p.History = append(p.History, entry)
		}
	}
	return provisional, deferred, nil
}

func decisionRecordItem(input Input, entry item, recordID string, value *ledger.Decision, resolved ledger.ResolvedDecision, decisionRecords map[string]string) (item, error) {
	entry.Title = "Decision " + value.ID
	text, err := form.RenderMarkdown(value.Decision)
	if err != nil {
		return item{}, err
	}
	entry.Text = text
	entry.Fields = []field{{"Approval state", string(resolved.Status)}, {"Active in records", fmt.Sprint(resolved.Active)}, {"Condition assessment", "For agents and people to assess"}}
	entry.Links = references(value.DependsOn)
	entry.Links = append(entry.Links, references(value.Evidence)...)
	entry.Links = append(entry.Links, references(resolved.SupersededBy)...)
	entry.Links = append(entry.Links, references(resolved.InactiveDependencies)...)
	entry.Links = append(entry.Links, relatedChanges(input.Records, form.ApprovalTarget{Kind: form.ApprovalDecision, DecisionID: value.ID}, decisionRecords)...)
	if resolved.Status == ledger.DecisionApproved {
		entry.Tags = append(entry.Tags, "Approved")
	} else if resolved.Status == ledger.DecisionPending {
		entry.Tags = append(entry.Tags, "Unreviewed")
	}
	if value.Provisional && resolved.Active {
		entry.Links = append(entry.Links, dependentChecks(input, recordID)...)
	}
	return entry, nil
}

func decisionAttention(recordID string, value *ledger.Decision, resolved ledger.ResolvedDecision) ([]item, []item) {
	var provisional, deferred []item
	if value.Provisional && resolved.Active {
		if resolved.Status != ledger.DecisionApproved {
			provisional = append(provisional, item{Title: "Review provisional decision at the next gate or PR review", Links: []link{recordLink(recordID, value.ID)}})
		}
	}
	if resolved.Status == ledger.DecisionDeferred && len(value.Conditions) != 0 && len(resolved.SupersededBy) == 0 {
		deferred = append(deferred, item{Title: "Reconsider deferred decision", Links: []link{recordLink(recordID, value.ID)}, Fields: []field{{"Conditions", strings.Join(value.Conditions, "\n")}}})
	}
	return provisional, deferred
}

func artifactRecordItem(input Input, entry item, value *ledger.Artifact, decisions map[string]ledger.ResolvedDecision) (item, bool) {
	entry.Title = "Change " + value.ChangeKind
	entry.Fields = []field{{"Before", targetLabel(value.Before)}, {"After", targetLabel(value.After)},
		{"Completion conditions", conditionLabels(input, value.CompletionConditionIDs)}, {"Commit", value.Commit}}
	entry.Links = references(value.Decisions)
	current := currentArtifact(input, value)
	for _, reference := range value.Decisions {
		if decision, ok := decisions[reference.RecordID]; ok && !decision.Active {
			entry.Fields = append(entry.Fields, field{"Decision dependency", "Referenced decision is no longer active in the records: " + decision.Decision.ID + ". This does not undo the recorded change."})
		}
	}
	if answer, ok := input.Ledger.ResolveApproval(approvalTarget(value.After)); ok {
		entry.Tags = append(entry.Tags, "Approved")
		entry.Links = append(entry.Links, recordLink(answer.RecordID, "Approval"))
	} else {
		entry.Tags = append(entry.Tags, "Unreviewed")
	}
	return entry, current
}

func judgmentRecordItem(input Input, entry item, value *ledger.Judgment) (item, bool, error) {
	entry.Title = "Check " + value.Aspect
	entry.Fields = judgmentFields(input, value)
	current := currentJudgment(input, value)
	// The result format contains checked scopes and the full findings.
	text, err := form.RenderMarkdown(value.JudgmentResult)
	if err != nil {
		return item{}, false, err
	}
	entry.Text = text
	if answer, ok := input.Ledger.ResolveAnswer(approvalTarget(value.TargetVersion)); ok {
		entry.Fields = append(entry.Fields, field{"Approval state", string(answer.Answer.Interpretation)})
	} else {
		entry.Fields = append(entry.Fields, field{"Approval state", "Unreviewed"})
	}
	return entry, current, nil
}

func (p *page) addCoverage(input Input, position ledger.Position) {
	if position.Plan != nil {
		for _, condition := range position.Plan.Plan.CompletionConditions {
			found := false
			for _, record := range input.Records {
				if judgment, ok := record.Payload.(*ledger.Judgment); ok && currentJudgment(input, judgment) && slices.Contains(judgment.CompletionConditionIDs, condition.ID) {
					approval := "Unreviewed"
					if answer, ok := input.Ledger.ResolveAnswer(approvalTarget(judgment.TargetVersion)); ok {
						approval = string(answer.Answer.Interpretation)
					}
					p.Coverage = append(p.Coverage, coverage{Condition: condition.ID + ": " + condition.Description,
						Target: targetLabel(judgment.TargetVersion), Method: judgment.Method,
						Checked: strings.Join(judgment.CheckedScope, "\n"), Unchecked: strings.Join(judgment.UncheckedScope, "\n"),
						Judgment: string(judgment.JudgmentResult.Judgment), Validity: "Valid", Approval: approval,
						Check: recordLink(record.ID, judgment.Aspect)})
					found = true
				}
			}
			if !found {
				p.Coverage = append(p.Coverage, coverage{Condition: condition.ID + ": " + condition.Description,
					Judgment: string(form.NotAssessed), Unchecked: "No current check recorded for this condition.", Approval: "Unreviewed"})
			}
		}
	}
}
