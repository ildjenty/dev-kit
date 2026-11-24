package issues

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
)

type Reference struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

type Issue struct {
	Reference
	Body      string      `json:"body"`
	Labels    []string    `json:"labels"`
	Parent    *Reference  `json:"parent"`
	SubIssues []Reference `json:"sub_issues"`
	BlockedBy []Reference `json:"blocked_by"`
	Blocking  []Reference `json:"blocking"`
}

type Created struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type ReadyIssue struct {
	Number int        `json:"number"`
	Title  string     `json:"title"`
	URL    string     `json:"url"`
	Parent *Reference `json:"parent"`
}

type Ready struct {
	Units  []ReadyIssue `json:"units"`
	Topics []ReadyIssue `json:"topics"`
}

type Client struct {
	Env cli.Env
}

func (c Client) run(input io.Reader, args ...string) ([]byte, error) {
	executable := c.Env.Getenv("DEVKIT_GH")
	if executable == "" {
		executable = "gh"
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = c.Env.Cwd, c.Env.Environ, input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s failed: %w\n%s\nRun devkit doctor to check GitHub CLI installation and authentication", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(stderr.String()))
	}
	return data, nil
}

func (c Client) Create(title, body string, labels []string) (Created, error) {
	args := []string{"issue", "create", "--title", title, "--body-file", "-"}
	if len(labels) > 0 {
		var values bytes.Buffer
		writer := csv.NewWriter(&values)
		if err := writer.Write(labels); err != nil {
			return Created{}, err
		}
		writer.Flush()
		args = append(args, "--label", strings.TrimSuffix(values.String(), "\n"))
	}
	data, err := c.run(strings.NewReader(body), args...)
	if err != nil {
		return Created{}, err
	}
	address := strings.TrimSpace(string(data))
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		return Created{}, fmt.Errorf("gh issue create returned an invalid issue URL: %q", address)
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) < 3 || parts[len(parts)-2] != "issues" {
		return Created{}, fmt.Errorf("gh issue create returned an invalid issue URL: %q", address)
	}
	number, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || number <= 0 {
		return Created{}, fmt.Errorf("gh issue create returned an invalid issue URL: %q", address)
	}
	return Created{number, address}, nil
}

const fields = "number,title,body,state,labels,url"

type issueJSON struct {
	Reference
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Parent *Reference `json:"parent"`
}

func (i issueJSON) issue() Issue {
	result := Issue{Reference: i.Reference, Body: i.Body, Labels: []string{}, Parent: i.Parent, SubIssues: []Reference{}, BlockedBy: []Reference{}, Blocking: []Reference{}}
	result.State = strings.ToLower(result.State)
	if result.Parent != nil {
		result.Parent.State = strings.ToLower(result.Parent.State)
	}
	for _, label := range i.Labels {
		result.Labels = append(result.Labels, label.Name)
	}
	return result
}

func endpoint(number int) (string, error) {
	if number <= 0 {
		return "", fmt.Errorf("issue number must be a positive integer")
	}
	return "repos/{owner}/{repo}/issues/" + strconv.Itoa(number), nil
}

func apiArgs(path string) []string {
	return []string{"api", path, "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2026-03-10"}
}

func (c Client) references(path string) ([]Reference, error) {
	data, err := c.run(nil, append(apiArgs(path+"?per_page=100"), "--paginate", "--slurp")...)
	if err != nil {
		return nil, err
	}
	var pages [][]struct {
		Reference
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, fmt.Errorf("parse gh api %s: %w", path, err)
	}
	result := []Reference{}
	for _, page := range pages {
		for _, item := range page {
			item.URL, item.State = item.HTMLURL, strings.ToLower(item.State)
			result = append(result, item.Reference)
		}
	}
	return result, nil
}

func (c Client) Read(number int) (Issue, error) {
	path, err := endpoint(number)
	if err != nil {
		return Issue{}, err
	}
	data, err := c.run(nil, "issue", "view", strconv.Itoa(number), "--json", fields+",parent")
	if err != nil {
		return Issue{}, err
	}
	var raw issueJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return Issue{}, fmt.Errorf("parse gh issue view: %w", err)
	}
	issue := raw.issue()
	for _, relation := range []struct {
		path string
		into *[]Reference
	}{{"sub_issues", &issue.SubIssues}, {"dependencies/blocked_by", &issue.BlockedBy}, {"dependencies/blocking", &issue.Blocking}} {
		*relation.into, err = c.references(path + "/" + relation.path)
		if err != nil {
			return Issue{}, err
		}
	}
	return issue, nil
}

func (c Client) ReadyUnit(number int, label string) (Issue, error) {
	issue, err := c.Read(number)
	if err != nil {
		return Issue{}, fmt.Errorf("issue #%d could not be read: %w", number, err)
	}
	if issue.State != "open" {
		return Issue{}, fmt.Errorf("issue #%d is not open (%s)", number, issue.State)
	}
	if !slices.Contains(issue.Labels, label) {
		return Issue{}, fmt.Errorf("issue #%d does not carry unit label %q", number, label)
	}
	for _, blocker := range issue.BlockedBy {
		if blocker.State != "closed" {
			return Issue{}, fmt.Errorf("issue #%d is blocked by issue #%d (%s)", number, blocker.Number, blocker.State)
		}
	}
	return issue, nil
}

func (c Client) addRelation(number, related int, relation, field string) error {
	path, err := endpoint(number)
	if err != nil {
		return err
	}
	other, err := endpoint(related)
	if err != nil {
		return err
	}
	exists := func() (bool, error) {
		references, err := c.references(path + "/" + relation)
		return slices.ContainsFunc(references, func(r Reference) bool { return r.Number == related }), err
	}
	if found, err := exists(); err != nil || found {
		return err
	}
	data, err := c.run(nil, apiArgs(other)...)
	if err != nil {
		return err
	}
	var issue struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &issue); err != nil {
		return fmt.Errorf("parse gh api issue ID: %w", err)
	}
	if issue.ID <= 0 {
		return fmt.Errorf("gh api returned no issue ID for #%d", related)
	}
	_, err = c.run(nil, append(apiArgs(path+"/"+relation), "--method", "POST", "--field", field+"="+strconv.FormatInt(issue.ID, 10))...)
	if err != nil {
		if found, readErr := exists(); readErr == nil && found {
			return nil
		}
	}
	return err
}

func (c Client) AddSubIssue(parent, child int) error {
	return c.addRelation(parent, child, "sub_issues", "sub_issue_id")
}

func (c Client) AddBlockedBy(number, blocker int) error {
	return c.addRelation(number, blocker, "dependencies/blocked_by", "issue_id")
}

func (c Client) Close(number int, comment string) error {
	if _, err := endpoint(number); err != nil {
		return err
	}
	_, err := c.run(nil, "issue", "close", strconv.Itoa(number), "--comment", comment)
	return err
}

func (c Client) ListOpen(label string) ([]Issue, error) {
	data, err := c.run(nil, "issue", "list", "--state", "open", "--limit", strconv.Itoa(int(^uint(0)>>1)), "--json", fields)
	if err != nil {
		return nil, err
	}
	var raw []issueJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse gh issue list: %w", err)
	}
	result := []Issue{}
	for _, item := range raw {
		issue := item.issue()
		if issue.State == "open" && slices.Contains(issue.Labels, label) {
			result = append(result, issue)
		}
	}
	return result, nil
}

func (c Client) Ready(labels config.IssueLabels) (Ready, error) {
	result := Ready{Units: []ReadyIssue{}, Topics: []ReadyIssue{}}
	for _, group := range []struct {
		label string
		into  *[]ReadyIssue
	}{{labels.Unit, &result.Units}, {labels.Topic, &result.Topics}} {
		listed, err := c.ListOpen(group.label)
		if err != nil {
			return Ready{}, err
		}
		for _, item := range listed {
			issue, err := c.Read(item.Number)
			if err != nil {
				return Ready{}, err
			}
			if issue.State != "open" || slices.ContainsFunc(issue.BlockedBy, func(blocker Reference) bool { return blocker.State != "closed" }) {
				continue
			}
			*group.into = append(*group.into, ReadyIssue{issue.Number, issue.Title, issue.URL, issue.Parent})
		}
	}
	return result, nil
}
