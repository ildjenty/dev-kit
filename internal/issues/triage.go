package issues

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
)

type TriageItem struct {
	Key                  string   `json:"key"`
	Number               int      `json:"number"`
	Kind                 string   `json:"kind"`
	Title                string   `json:"title"`
	Body                 string   `json:"body"`
	CompletionConditions []string `json:"completion_conditions"`
	HeldUnits            []string `json:"held_units"`
	Parent               string   `json:"parent"`
	BlockedBy            []string `json:"blocked_by"`
	Reason               string   `json:"reason"`
}

func TriageErrors(items []TriageItem, existing []Issue, labels config.IssueLabels) []string {
	problems := []string{}
	known := map[string]TriageItem{}
	numbers := map[int]bool{}
	drafts := Drafts{}
	for _, item := range items {
		if item.Key == "" || known[item.Key].Key != "" {
			problems = append(problems, "triage keys must be nonempty and unique: "+item.Key)
		}
		known[item.Key] = item
		if item.Kind != "unit" && item.Kind != "topic" && item.Kind != "leave" {
			problems = append(problems, item.Key+": invalid triage kind")
		}
		if strings.TrimSpace(item.Reason) == "" {
			problems = append(problems, item.Key+": triage reason is required")
		}
		if item.Number == 0 {
			if item.Kind != "topic" || item.Parent == "" {
				problems = append(problems, item.Key+": new issues must be topics under an existing parent")
			}
		} else {
			if numbers[item.Number] {
				problems = append(problems, fmt.Sprintf("issue #%d is triaged twice", item.Number))
			}
			numbers[item.Number] = true
			if !slices.ContainsFunc(existing, func(i Issue) bool {
				return i.Number == item.Number && i.State == "open" && !slices.Contains(i.Labels, labels.Unit) && !slices.Contains(i.Labels, labels.Topic)
			}) {
				problems = append(problems, item.Key+": triage must reference an existing open issue without kit labels")
			}
		}
		if item.Kind == "leave" {
			if item.Parent != "" || len(item.BlockedBy) > 0 || len(item.HeldUnits) > 0 {
				problems = append(problems, item.Key+": left-out issue cannot have proposed relations")
			}
			continue
		}
		drafts.Topics = append(drafts.Topics, TopicDraft{Key: item.Key, Title: item.Title, Body: item.Body, BlockedBy: item.BlockedBy})
		if item.Kind == "unit" {
			if len(item.CompletionConditions) == 0 || slices.ContainsFunc(item.CompletionConditions, func(s string) bool { return strings.TrimSpace(s) == "" }) {
				problems = append(problems, item.Key+": units require completion conditions")
			}
			if len(item.HeldUnits) > 0 {
				problems = append(problems, item.Key+": only topics hold units")
			}
		} else if len(item.CompletionConditions) > 0 {
			problems = append(problems, item.Key+": topics cannot have completion conditions")
		}
	}
	problems = append(problems, drafts.Errors()...)
	for _, issue := range existing {
		if issue.State == "open" && !slices.Contains(issue.Labels, labels.Unit) && !slices.Contains(issue.Labels, labels.Topic) && !numbers[issue.Number] {
			problems = append(problems, fmt.Sprintf("open issue #%d has no triage or reasoned exclusion", issue.Number))
		}
	}
	for _, item := range items {
		if item.Parent != "" {
			parent, ok := known[item.Parent]
			if !ok || parent.Kind != "topic" || parent.Key == item.Key || item.Kind == "leave" {
				problems = append(problems, item.Key+": parent must be another topic")
			}
			if item.Number == 0 && parent.Number == 0 {
				problems = append(problems, item.Key+": new topic requires an existing parent issue")
			}
			seen := map[string]bool{item.Key: true}
			for p := item.Parent; p != ""; p = known[p].Parent {
				if seen[p] {
					problems = append(problems, item.Key+": parent cycle")
					break
				}
				seen[p] = true
			}
		}
		for _, blocker := range item.BlockedBy {
			if known[blocker].Kind == "leave" {
				problems = append(problems, item.Key+": left-out issue cannot block triaged work")
			}
		}
		if item.Kind != "topic" {
			continue
		}
		held := map[string]bool{}
		for _, key := range item.HeldUnits {
			if held[key] || known[key].Kind != "unit" || !slices.Contains(known[key].BlockedBy, item.Key) {
				problems = append(problems, item.Key+": held_units must match units blocked by this topic")
			}
			held[key] = true
			if !strings.Contains(item.Body, key) {
				problems = append(problems, item.Key+": body must state held unit key "+key)
			}
		}
		for _, unit := range items {
			if unit.Kind == "unit" && slices.Contains(unit.BlockedBy, item.Key) && !held[unit.Key] {
				problems = append(problems, item.Key+": blocked_by does not match held_units")
			}
		}
	}
	return problems
}

func (c Client) Edit(number int, title, body, label string) error {
	if _, err := endpoint(number); err != nil {
		return err
	}
	_, err := c.run(strings.NewReader(body), "issue", "edit", fmt.Sprint(number), "--title", title, "--body-file", "-", "--add-label", label)
	return err
}

func (c Client) ApplyTriage(path string, items []TriageItem, labels config.IssueLabels) (Creation, error) {
	progress, err := ReadCreation(path)
	if err != nil {
		return progress, err
	}
	if progress.RunID == "" || progress.Commit == "" || progress.PRNumber == 0 {
		return progress, fmt.Errorf("issue triage requires a recorded approval and published PR")
	}
	save := func() error { return SaveCreation(path, progress) }
	if !progress.LabelsEnsured {
		if err = c.EnsureLabels(labels); err != nil {
			return progress, err
		}
		progress.LabelsEnsured = true
		if err = save(); err != nil {
			return progress, err
		}
	}
	for _, item := range items {
		if item.Kind == "leave" {
			continue
		}
		if _, ok := progress.Created[item.Key]; ok {
			continue
		}
		label := labels.Topic
		if item.Kind == "unit" {
			label = labels.Unit
		}
		body := item.Body
		for _, condition := range item.CompletionConditions {
			body += "\n\n- " + condition
		}
		if item.Number > 0 {
			if err = c.Edit(item.Number, item.Title, body, label); err != nil {
				return progress, err
			}
			progress.Created[item.Key] = Created{Number: item.Number}
		} else {
			created, e := c.createDraft(progress.Commit, filepath.Base(path), "triage", TopicDraft{Key: item.Key, Title: item.Title, Body: body}, label, 0)
			if e != nil {
				return progress, e
			}
			progress.Created[item.Key] = created
		}
		if err = save(); err != nil {
			return progress, err
		}
	}
	for _, item := range items {
		if item.Kind == "leave" {
			continue
		}
		if item.Parent != "" && !progress.SubIssues[item.Key] {
			if err = c.AddSubIssue(progress.Created[item.Parent].Number, progress.Created[item.Key].Number); err != nil {
				return progress, err
			}
			progress.SubIssues[item.Key] = true
			if err = save(); err != nil {
				return progress, err
			}
		}
		for _, blocker := range item.BlockedBy {
			key := item.Key + ":" + blocker
			if progress.BlockedBy[key] {
				continue
			}
			if err = c.AddBlockedBy(progress.Created[item.Key].Number, progress.Created[blocker].Number); err != nil {
				return progress, err
			}
			progress.BlockedBy[key] = true
			if err = save(); err != nil {
				return progress, err
			}
		}
	}
	return progress, nil
}
