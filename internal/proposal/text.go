package proposal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/issues"
)

type prText struct {
	Title   string `json:"pr_title"`
	Summary string `json:"pr_summary"`
}

func PRTextErrors(value any) []string {
	var text prText
	data, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(data, &text)
	}
	if err != nil || strings.TrimSpace(text.Title) == "" || strings.TrimSpace(text.Summary) == "" {
		return []string{"make the proposal again so its PR title and summary can be approved"}
	}
	if utf8.RuneCountInString(text.Title) > 100 || strings.ContainsAny(text.Title, "\r\n") {
		return []string{"make the proposal again with a single-line PR title of at most 100 characters"}
	}
	return nil
}

func japanese(language string) bool {
	language = strings.ToLower(strings.ReplaceAll(language, "_", "-"))
	return language == "ja" || strings.HasPrefix(language, "ja-")
}

func ApprovalPRText(draft map[string]any, language string) string {
	title, summary := "PR title", "PR summary"
	if japanese(language) {
		title, summary = "PR タイトル", "PR の概要"
	}
	return fmt.Sprintf("\n\n%s: %s\n\n%s:\n%s", title, draft["pr_title"], summary, draft["pr_summary"])
}

func branchLink(repository, kind, branch, path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return repository + "/" + kind + "/" + url.PathEscape(branch) + "/" + strings.Join(parts, "/")
}

func proposalText(content map[string]any, progress issues.Creation, branch, repository string, cfg config.Config) (string, string, error) {
	var approved struct {
		Topic struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
		} `json:"topic"`
		Mapping map[string]string `json:"mapping"`
		Draft   struct {
			prText
			issues.Drafts
			Changes   []struct{ Path, Reason, Evidence string } `json:"changes"`
			Triage    []issues.TriageItem                       `json:"triage"`
			Decisions []struct{ Key, Title string }             `json:"decisions"`
			Bindings  []struct{ Path, Decision, Reason string } `json:"bindings"`
			Standards []struct {
				Path, Reason, Evidence string
				Entries                []string
			} `json:"standards"`
			Checks []struct {
				Name, Reason string
				Entries      []string
			} `json:"checks"`
			Escapes []struct{ Entry, Stage, Quality, Reason, Evidence string } `json:"escapes"`
			Cases   []struct{ Entry, Name string }                             `json:"cases"`
		} `json:"draft"`
	}
	data, err := json.Marshal(content)
	if err != nil {
		return "", "", err
	}
	if err = json.Unmarshal(data, &approved); err != nil {
		return "", "", err
	}
	if problems := PRTextErrors(content["draft"]); len(problems) > 0 {
		return "", "", fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	draft := approved.Draft
	review, decisions, escapes, cases, created := "What to review", "Decision records", "Escapes", "Eval cases", "Issues created"
	log, ready, blocked := "These records are a log for people and do not bind delivery.", "Units become ready when this PR merges, subject to blocked-by dependencies.", "blocked by"
	if japanese(cfg.Language) {
		review, decisions, escapes, cases, created = "レビュー対象", "判断ログ", "流出した欠陥", "評価ケース", "作成した Issue"
		log, ready, blocked = "判断ログは人のための記録であり、開発作業を拘束しません。", "作業単位は、この PR のマージ後、blocked-by の依存条件を満たすと開始可能になります。", "依存先"
	}
	body := draft.Summary
	section := func(heading string, rows []string) {
		if len(rows) > 0 {
			body += "\n\n## " + heading + "\n\n" + strings.Join(rows, "\n")
		}
	}
	decisionLabel := func(key string) string {
		id := key
		if mapped := approved.Mapping[key]; mapped != "" {
			id = mapped
		}
		for _, d := range draft.Decisions {
			if d.Key == key || approved.Mapping[d.Key] == key {
				return id + " " + d.Title
			}
		}
		return id
	}
	rows := []string{}
	for _, change := range draft.Changes {
		rows = append(rows, fmt.Sprintf("- `%s` — %s; %s", change.Path, change.Reason, change.Evidence))
	}
	for _, binding := range draft.Bindings {
		rows = append(rows, fmt.Sprintf("- `%s` — %s: %s", binding.Path, decisionLabel(binding.Decision), binding.Reason))
	}
	for _, standard := range draft.Standards {
		rows = append(rows, fmt.Sprintf("- `%s` — %s; %s (%s)", standard.Path, standard.Reason, standard.Evidence, strings.Join(standard.Entries, ", ")))
	}
	for _, check := range draft.Checks {
		rows = append(rows, fmt.Sprintf("- `.devkit/config.json` — `%s`: %s (%s)", check.Name, check.Reason, strings.Join(check.Entries, ", ")))
	}
	section(review, rows)
	rows = nil
	for _, d := range draft.Decisions {
		id := approved.Mapping[d.Key]
		path := cfg.Decisions + "/" + id + ".md"
		rows = append(rows, fmt.Sprintf("- [%s](%s) %s", id, branchLink(repository, "blob", branch, path), d.Title))
	}
	if len(rows) > 0 {
		rows = append([]string{log, ""}, rows...)
	}
	section(decisions, rows)
	rows = nil
	for _, escape := range draft.Escapes {
		stage := escape.Stage
		if escape.Quality != "" {
			stage += " / " + escape.Quality
		}
		rows = append(rows, fmt.Sprintf("- `%s` — %s: %s; %s", escape.Entry, stage, escape.Reason, escape.Evidence))
	}
	section(escapes, rows)
	rows = nil
	for _, c := range draft.Cases {
		rows = append(rows, fmt.Sprintf("- [%s](%s) — `%s`", c.Name, branchLink(repository, "tree", branch, cfg.Eval+"/judge/"+c.Name), c.Entry))
	}
	section(cases, rows)
	rows = nil
	issueLink := func(issue issues.Created, title string) string {
		label := fmt.Sprintf("#%d", issue.Number)
		if title != "" {
			label += " " + strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]").Replace(title)
		}
		address := issue.URL
		if address == "" {
			address = fmt.Sprintf("%s/issues/%d", repository, issue.Number)
		}
		return fmt.Sprintf("[%s](%s)", label, address)
	}
	if progress.Parent != nil {
		title := approved.Topic.Title
		if approved.Topic.Number == 0 && draft.Parent != nil {
			title = draft.Parent.Title
		}
		rows = append(rows, "- "+issueLink(*progress.Parent, title))
	}
	appendIssue := func(key, title string, blockers []string) {
		if issue, ok := progress.Created[key]; ok {
			prefix := "- "
			if progress.Parent != nil {
				prefix = "  - "
			}
			row := prefix + issueLink(issue, title)
			links := []string{}
			for _, key := range blockers {
				if other, ok := progress.Created[key]; ok {
					links = append(links, issueLink(other, ""))
				}
			}
			if len(links) > 0 {
				row += " — " + blocked + ": " + strings.Join(links, ", ")
			}
			rows = append(rows, row)
		}
	}
	for _, unit := range draft.Units {
		appendIssue(unit.Key, unit.Title, unit.BlockedBy)
	}
	for _, topic := range draft.Topics {
		appendIssue(topic.Key, topic.Title, topic.BlockedBy)
	}
	if len(draft.Units) > 0 && len(rows) > 0 {
		rows = append([]string{ready, ""}, rows...)
	}
	section(created, rows)
	rows = nil
	heading, parentLabel := "Issue triage", "parent"
	kinds := map[string]string{"unit": "Unit", "topic": "Topic", "leave": "Left out"}
	if japanese(cfg.Language) {
		heading, parentLabel = "Issue の整理", "親"
		kinds = map[string]string{"unit": "作業単位", "topic": "トピック", "leave": "対象外"}
	}
	for _, item := range draft.Triage {
		if item.Kind == "leave" {
			rows = append(rows, fmt.Sprintf("- #%d — %s: %s", item.Number, kinds[item.Kind], item.Reason))
			continue
		}
		if issue, ok := progress.Created[item.Key]; ok {
			row := "- " + issueLink(issue, item.Title) + " — " + kinds[item.Kind] + ": " + item.Reason
			if parent, ok := progress.Created[item.Parent]; ok {
				row += "; " + parentLabel + ": " + issueLink(parent, "")
			}
			for _, key := range item.BlockedBy {
				if blocker, ok := progress.Created[key]; ok {
					row += "; " + blocked + ": " + issueLink(blocker, "")
				}
			}
			rows = append(rows, row)
		} else {
			rows = append(rows, fmt.Sprintf("- %s (#%d) — %s: %s", item.Title, item.Number, kinds[item.Kind], item.Reason))
		}
	}
	section(heading, rows)

	return draft.Title, body, nil
}
