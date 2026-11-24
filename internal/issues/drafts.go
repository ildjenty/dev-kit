package issues

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
)

type UnitDraft struct {
	Key                  string   `json:"key"`
	Title                string   `json:"title"`
	Goal                 string   `json:"goal"`
	CompletionConditions []string `json:"completion_conditions"`
	BlockedBy            []string `json:"blocked_by"`
}

type TopicDraft struct {
	Key       string   `json:"key"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	BlockedBy []string `json:"blocked_by"`
}

type Drafts struct {
	Units  []UnitDraft  `json:"units"`
	Topics []TopicDraft `json:"topics"`
}

func ReadDrafts(value any) (Drafts, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Drafts{}, err
	}
	var drafts Drafts
	err = json.Unmarshal(data, &drafts)
	return drafts, err
}

func (d Drafts) topicsAndUnits() []TopicDraft {
	all := append([]TopicDraft{}, d.Topics...)
	for _, unit := range d.Units {
		body := unit.Goal + "\n\n"
		for _, condition := range unit.CompletionConditions {
			body += "- " + condition + "\n"
		}
		all = append(all, TopicDraft{unit.Key, unit.Title, body, unit.BlockedBy})
	}
	return all
}

func (d Drafts) Errors() []string {
	errors := []string{}
	all := d.topicsAndUnits()
	known := map[string]TopicDraft{}
	for _, issue := range all {
		if _, exists := known[issue.Key]; exists {
			errors = append(errors, "duplicate issue key: "+issue.Key)
		}
		known[issue.Key] = issue
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string)
	visit = func(key string) {
		if visiting[key] {
			errors = append(errors, "blocked_by cycle at "+key)
			return
		}
		if visited[key] {
			return
		}
		visiting[key] = true
		for _, blocker := range known[key].BlockedBy {
			if _, exists := known[blocker]; !exists {
				errors = append(errors, key+": missing blocker "+blocker)
			} else {
				visit(blocker)
			}
		}
		delete(visiting, key)
		visited[key] = true
	}
	for _, issue := range all {
		visit(issue.Key)
	}
	return errors
}

type Creation struct {
	RunID     string             `json:"run_id"`
	Commit    string             `json:"commit"`
	Parent    *Created           `json:"parent"`
	Created   map[string]Created `json:"created"`
	SubIssues map[string]bool    `json:"sub_issues"`
	BlockedBy map[string]bool    `json:"blocked_by"`
	Commented bool               `json:"commented"`
}

func ReadCreation(path string) (Creation, error) {
	progress := Creation{Created: map[string]Created{}, SubIssues: map[string]bool{}, BlockedBy: map[string]bool{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return progress, nil
	}
	if err != nil {
		return Creation{}, err
	}
	err = json.Unmarshal(data, &progress)
	return progress, err
}

func SaveCreation(path string, progress Creation) error {
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".issues-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(data, '\n'))
	closed := f.Close()
	if err != nil {
		return err
	}
	if closed != nil {
		return closed
	}
	return os.Rename(f.Name(), path)
}

func (c Client) CreateDrafts(path string, drafts Drafts, labels config.IssueLabels, parent *TopicDraft, number int, comment string) (Creation, error) {
	progress, err := ReadCreation(path)
	if err != nil {
		return progress, err
	}
	if errors := drafts.Errors(); len(errors) > 0 {
		return progress, fmt.Errorf("invalid issue drafts: %s", strings.Join(errors, "; "))
	}
	save := func() error { return SaveCreation(path, progress) }
	if parent != nil && progress.Parent == nil {
		if number > 0 {
			progress.Parent = &Created{Number: number}
		} else {
			created, err := c.Create(parent.Title, parent.Body, []string{labels.Topic})
			if err != nil {
				return progress, err
			}
			progress.Parent = &created
		}
		if err = save(); err != nil {
			return progress, err
		}
	}
	for _, issue := range drafts.topicsAndUnits() {
		if _, exists := progress.Created[issue.Key]; exists {
			continue
		}
		label := labels.Unit
		if slices.ContainsFunc(drafts.Topics, func(t TopicDraft) bool { return t.Key == issue.Key }) {
			label = labels.Topic
		}
		created, err := c.Create(issue.Title, issue.Body, []string{label})
		if err != nil {
			return progress, err
		}
		progress.Created[issue.Key] = created
		if err = save(); err != nil {
			return progress, err
		}
	}
	if progress.Parent != nil {
		for _, unit := range drafts.Units {
			if progress.SubIssues[unit.Key] {
				continue
			}
			if err = c.AddSubIssue(progress.Parent.Number, progress.Created[unit.Key].Number); err != nil {
				return progress, err
			}
			progress.SubIssues[unit.Key] = true
			if err = save(); err != nil {
				return progress, err
			}
		}
	}
	for _, issue := range drafts.topicsAndUnits() {
		for _, blocker := range issue.BlockedBy {
			key := issue.Key + ":" + blocker
			if progress.BlockedBy[key] {
				continue
			}
			if err = c.AddBlockedBy(progress.Created[issue.Key].Number, progress.Created[blocker].Number); err != nil {
				return progress, err
			}
			progress.BlockedBy[key] = true
			if err = save(); err != nil {
				return progress, err
			}
		}
	}
	if progress.Parent != nil && !progress.Commented {
		for _, issue := range drafts.topicsAndUnits() {
			comment += fmt.Sprintf("\n- #%d %s", progress.Created[issue.Key].Number, issue.Title)
		}
		if len(drafts.Topics) == 0 {
			err = c.Close(progress.Parent.Number, comment)
		} else {
			err = c.Comment(progress.Parent.Number, comment)
		}
		if err != nil {
			return progress, err
		}
		progress.Commented = true
		if err = save(); err != nil {
			return progress, err
		}
	}
	return progress, nil
}

func (p Creation) Numbers() []int {
	numbers := []int{}
	if p.Parent != nil {
		numbers = append(numbers, p.Parent.Number)
	}
	for _, issue := range p.Created {
		numbers = append(numbers, issue.Number)
	}
	slices.Sort(numbers)
	return numbers
}

func (c Client) Comment(number int, comment string) error {
	if _, err := endpoint(number); err != nil {
		return err
	}
	_, err := c.run(strings.NewReader(comment), "issue", "comment", fmt.Sprint(number), "--body-file", "-")
	return err
}
