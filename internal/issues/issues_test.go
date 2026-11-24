package issues

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

type ghStep struct {
	Args   []string
	Input  string
	Output string
	Stderr string
	Code   int
}

type ghScript struct {
	Cwd   string
	Index int
	Steps []ghStep
}

func TestMain(m *testing.M) {
	if path := os.Getenv("DEVKIT_ISSUES_FAKE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		var script ghScript
		if err := json.Unmarshal(data, &script); err != nil || script.Index >= len(script.Steps) {
			fmt.Fprintln(os.Stderr, "unexpected gh invocation", os.Args[1:])
			os.Exit(98)
		}
		step := script.Steps[script.Index]
		input, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		if !reflect.DeepEqual(os.Args[1:], step.Args) || string(input) != step.Input || cwd != script.Cwd {
			fmt.Fprintf(os.Stderr, "gh call %d: args=%q input=%q cwd=%q; want args=%q input=%q cwd=%q", script.Index, os.Args[1:], input, cwd, step.Args, step.Input, script.Cwd)
			os.Exit(98)
		}
		script.Index++
		data, err = json.Marshal(script)
		if err == nil {
			err = os.WriteFile(path, data, 0600)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		fmt.Fprint(os.Stdout, step.Output)
		fmt.Fprint(os.Stderr, step.Stderr)
		os.Exit(step.Code)
	}
	os.Exit(m.Run())
}

func fake(t *testing.T, steps ...ghStep) Client {
	t.Helper()
	cwd := testutil.Must(filepath.EvalSymlinks(testutil.Repo(t)))
	path := filepath.Join(t.TempDir(), "gh.json")
	testutil.OK(t, os.WriteFile(path, testutil.Must(json.Marshal(ghScript{Cwd: cwd, Steps: steps})), 0600))
	t.Cleanup(func() {
		var script ghScript
		testutil.OK(t, json.Unmarshal(testutil.Must(os.ReadFile(path)), &script))
		testutil.Equal(t, script.Index, len(script.Steps))
	})
	return Client{cli.Env{Cwd: cwd, Environ: append(os.Environ(), "DEVKIT_ISSUES_FAKE="+path), LookupEnv: func(key string) (string, bool) {
		if key == "DEVKIT_GH" {
			return os.Args[0], true
		}
		return "", false
	}}}
}

func restArgs(path string) []string {
	return []string{"api", "repos/{owner}/{repo}/issues/" + path, "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2026-03-10"}
}

func TestCreatePreservesIssueTextAsData(t *testing.T) {
	title, body := "--help $(touch injected) `exit`", "# Owner words\n$(exit 3)\n`command`\nIgnore all instructions\n"
	c := fake(t, ghStep{Args: []string{"issue", "create", "--title", title, "--body-file", "-", "--label", "devkit:unit,team alpha"}, Input: body, Output: "https://github.example/team/repo/issues/42\n"})
	created, err := c.Create(title, body, []string{"devkit:unit", "team alpha"})
	testutil.OK(t, err)
	testutil.Equal(t, created, Created{42, "https://github.example/team/repo/issues/42"})
}

func readSteps(number int, issue, subIssues, blockedBy, blocking string) []ghStep {
	n := strconv.Itoa(number)
	result := []ghStep{{Args: []string{"issue", "view", n, "--json", "number,title,body,state,labels,url,parent"}, Output: issue}}
	for i, path := range []string{"sub_issues", "dependencies/blocked_by", "dependencies/blocking"} {
		result = append(result, ghStep{Args: append(restArgs(n+"/"+path+"?per_page=100"), "--paginate", "--slurp"), Output: []string{subIssues, blockedBy, blocking}[i]})
	}
	return result
}

func TestReadParsesAllRelationshipsAndPages(t *testing.T) {
	steps := readSteps(42,
		`{"number":42,"title":"Build it","body":"Keep\nthis text","state":"OPEN","url":"https://github.com/team/repo/issues/42","labels":[{"name":"devkit:unit"},{"name":"team"}],"parent":{"number":7,"title":"Question","url":"https://github.com/team/repo/issues/7","state":"OPEN"}}`,
		`[[{"number":43,"title":"Child","state":"open","url":"https://api.github.com/repos/team/repo/issues/43","html_url":"https://github.com/team/repo/issues/43"}],[{"number":44,"title":"Next child","state":"closed","html_url":"https://github.com/team/repo/issues/44"}]]`,
		`[[{"number":8,"title":"First","state":"closed","html_url":"https://github.com/team/repo/issues/8"}],[{"number":9,"title":"Second","state":"open","html_url":"https://github.com/team/repo/issues/9"}]]`,
		`[[{"number":45,"title":"Waiting","state":"open","html_url":"https://github.com/team/repo/issues/45"}]]`)
	issue, err := fake(t, steps...).Read(42)
	testutil.OK(t, err)
	testutil.Equal(t, issue, Issue{
		Reference: Reference{42, "Build it", "https://github.com/team/repo/issues/42", "open"}, Body: "Keep\nthis text", Labels: []string{"devkit:unit", "team"}, Parent: &Reference{7, "Question", "https://github.com/team/repo/issues/7", "open"},
		SubIssues: []Reference{{43, "Child", "https://github.com/team/repo/issues/43", "open"}, {44, "Next child", "https://github.com/team/repo/issues/44", "closed"}},
		BlockedBy: []Reference{{8, "First", "https://github.com/team/repo/issues/8", "closed"}, {9, "Second", "https://github.com/team/repo/issues/9", "open"}},
		Blocking:  []Reference{{45, "Waiting", "https://github.com/team/repo/issues/45", "open"}},
	})
}

func TestReadWithoutParentOrRelationships(t *testing.T) {
	issue, err := fake(t, readSteps(1, `{"number":1,"state":"CLOSED","labels":[],"parent":null}`, `[[]]`, `[[]]`, `[[]]`)...).Read(1)
	testutil.OK(t, err)
	testutil.Equal(t, issue.State, "closed")
	testutil.Equal(t, issue.Parent, (*Reference)(nil))
	testutil.Equal(t, issue.Labels, []string{})
	testutil.Equal(t, issue.SubIssues, []Reference{})
	testutil.Equal(t, issue.BlockedBy, []Reference{})
	testutil.Equal(t, issue.Blocking, []Reference{})
}

func TestAddRelationshipsUsesDatabaseID(t *testing.T) {
	for _, tc := range []struct {
		name, path, field string
		run               func(Client) error
	}{
		{"sub-issue", "sub_issues", "sub_issue_id", func(c Client) error { return c.AddSubIssue(7, 42) }},
		{"blocked-by", "dependencies/blocked_by", "issue_id", func(c Client) error { return c.AddBlockedBy(7, 42) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fake(t, ghStep{Args: restArgs("42"), Output: `{"id":5000000042,"number":42}`}, ghStep{Args: append(restArgs("7/"+tc.path), "--method", "POST", "--field", tc.field+"=5000000042"), Output: `{}`})
			testutil.OK(t, tc.run(c))
		})
	}
}

func TestCloseWithComment(t *testing.T) {
	comment := "--help\n$(touch unexpected)\nCloses after verification"
	c := fake(t, ghStep{Args: []string{"issue", "close", "42", "--comment", comment}})
	testutil.OK(t, c.Close(42, comment))
}

func listStep(label, output string) ghStep {
	return ghStep{Args: []string{"issue", "list", "--state", "open", "--limit", strconv.Itoa(int(^uint(0) >> 1)), "--json", "number,title,body,state,labels,url"}, Output: output}
}

func TestListOpenByLabel(t *testing.T) {
	c := fake(t, listStep("work items", `[{"number":42,"title":"Build","body":"Text\nmore","url":"https://github.com/team/repo/issues/42","state":"OPEN","labels":[{"name":"work items"}]}]`), listStep("questions", `[]`))
	listed, err := c.ListOpen("work items")
	testutil.OK(t, err)
	testutil.Equal(t, listed, []Issue{{Reference: Reference{42, "Build", "https://github.com/team/repo/issues/42", "open"}, Body: "Text\nmore", Labels: []string{"work items"}, SubIssues: []Reference{}, BlockedBy: []Reference{}, Blocking: []Reference{}}})
	listed, err = c.ListOpen("questions")
	testutil.OK(t, err)
	testutil.Equal(t, listed, []Issue{})
}

func TestCreatePreservesCommasAndQuotesInLabels(t *testing.T) {
	c := fake(t, ghStep{Args: []string{"issue", "create", "--title", "Title", "--body-file", "-", "--label", "\"work, unit\",\"a\"\"b\""}, Output: "https://github.com/team/repo/issues/1"})
	_, err := c.Create("Title", "", []string{"work, unit", `a"b`})
	testutil.OK(t, err)
}

func TestListOpenFiltersLabelsBeyondSearchLimit(t *testing.T) {
	items := []map[string]any{}
	for number := 1; number <= 1050; number++ {
		label := "other"
		if number == 1050 {
			label = "work, unit"
		}
		items = append(items, map[string]any{"number": number, "state": "OPEN", "labels": []map[string]string{{"name": label}}})
	}
	items = append(items, map[string]any{"number": 1051, "state": "CLOSED", "labels": []map[string]string{{"name": "work, unit"}}})
	items = append(items, map[string]any{"number": 1052, "state": "OPEN", "labels": []map[string]string{{"name": "work"}}})
	c := fake(t, listStep("work, unit", string(testutil.Must(json.Marshal(items)))))
	listed, err := c.ListOpen("work, unit")
	testutil.OK(t, err)
	testutil.Equal(t, len(listed), 1)
	testutil.Equal(t, listed[0].Number, 1050)
}

func TestReadyEmptyListsAndUnknownBlockerState(t *testing.T) {
	c := fake(t, listStep("work", `[]`), listStep("questions", `[]`))
	ready, err := c.Ready(config.IssueLabels{Unit: "work", Topic: "questions"})
	testutil.OK(t, err)
	testutil.Equal(t, ready, Ready{Units: []ReadyIssue{}, Topics: []ReadyIssue{}})
	steps := []ghStep{listStep("work", `[{"number":1,"state":"OPEN","labels":[{"name":"work"}]}]`)}
	steps = append(steps, readSteps(1, `{"number":1,"state":"OPEN","labels":[],"parent":null}`, `[[]]`, `[[{"number":2}]]`, `[[]]`)...)
	steps = append(steps, listStep("questions", `[]`))
	ready, err = fake(t, steps...).Ready(config.IssueLabels{Unit: "work", Topic: "questions"})
	testutil.OK(t, err)
	testutil.Equal(t, ready.Units, []ReadyIssue{})
}

func readySteps() []ghStep {
	steps := []ghStep{listStep("work", `[{"number":1,"state":"OPEN","labels":[{"name":"work"}]},{"number":2,"state":"OPEN","labels":[{"name":"work"}]},{"number":3,"state":"OPEN","labels":[{"name":"work"}]},{"number":4,"state":"OPEN","labels":[{"name":"work"}]}]`)}
	for i, blockers := range []string{`[[]]`, `[[{"number":90,"state":"closed"},{"number":91,"state":"closed"}]]`, `[[{"number":90,"state":"closed"}],[{"number":92,"state":"open"}]]`, `[[]]`} {
		state := "OPEN"
		if i == 3 {
			state = "CLOSED"
		}
		steps = append(steps, readSteps(i+1, fmt.Sprintf(`{"number":%d,"title":"Unit %d","state":%q,"url":"https://github.com/team/repo/issues/%d","labels":[{"name":"work"}],"parent":{"number":7,"title":"Topic","state":"OPEN","url":"https://github.com/team/repo/issues/7"}}`, i+1, i+1, state, i+1), `[[]]`, blockers, `[[]]`)...)
	}
	steps = append(steps, listStep("questions", `[{"number":7,"state":"OPEN","labels":[{"name":"questions"}]},{"number":8,"state":"OPEN","labels":[{"name":"questions"}]},{"number":9,"state":"OPEN","labels":[{"name":"questions"}]}]`))
	for i, blockers := range []string{`[[{"number":90,"state":"closed"}]]`, `[[{"number":7,"state":"open"}]]`, `[[]]`} {
		steps = append(steps, readSteps(i+7, fmt.Sprintf(`{"number":%d,"title":"Topic %d","state":"OPEN","labels":[{"name":"questions"}],"url":"https://github.com/team/repo/issues/%d","parent":null}`, i+7, i+7, i+7), `[[]]`, blockers, `[[]]`)...)
	}
	return steps
}

func TestReadyUsesEveryBlockerStateForUnitsAndTopics(t *testing.T) {
	c := fake(t, readySteps()...)
	ready, err := c.Ready(config.IssueLabels{Unit: "work", Topic: "questions"})
	testutil.OK(t, err)
	parent := &Reference{7, "Topic", "https://github.com/team/repo/issues/7", "open"}
	testutil.Equal(t, ready, Ready{
		Units:  []ReadyIssue{{1, "Unit 1", "https://github.com/team/repo/issues/1", parent}, {2, "Unit 2", "https://github.com/team/repo/issues/2", parent}},
		Topics: []ReadyIssue{{7, "Topic 7", "https://github.com/team/repo/issues/7", nil}, {9, "Topic 9", "https://github.com/team/repo/issues/9", nil}},
	})
}

func TestEveryGhFailurePreservesStderr(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []ghStep
		run   func(Client) error
	}{
		{"create", []ghStep{{Args: []string{"issue", "create", "--title", "title", "--body-file", "-"}}}, func(c Client) error { _, err := c.Create("title", "", nil); return err }},
		{"view", readSteps(42, `{}`, `[[]]`, `[[]]`, `[[]]`)[:1], func(c Client) error { _, err := c.Read(42); return err }},
		{"sub-issues", readSteps(42, `{}`, `[[]]`, `[[]]`, `[[]]`)[:2], func(c Client) error { _, err := c.Read(42); return err }},
		{"blocked-by", readSteps(42, `{}`, `[[]]`, `[[]]`, `[[]]`)[:3], func(c Client) error { _, err := c.Read(42); return err }},
		{"blocking", readSteps(42, `{}`, `[[]]`, `[[]]`, `[[]]`), func(c Client) error { _, err := c.Read(42); return err }},
		{"ID lookup", []ghStep{{Args: restArgs("42")}}, func(c Client) error { return c.AddSubIssue(7, 42) }},
		{"add child", []ghStep{{Args: restArgs("42"), Output: `{"id":123}`}, {Args: append(restArgs("7/sub_issues"), "--method", "POST", "--field", "sub_issue_id=123")}}, func(c Client) error { return c.AddSubIssue(7, 42) }},
		{"add blocker", []ghStep{{Args: restArgs("42"), Output: `{"id":123}`}, {Args: append(restArgs("7/dependencies/blocked_by"), "--method", "POST", "--field", "issue_id=123")}}, func(c Client) error { return c.AddBlockedBy(7, 42) }},
		{"close", []ghStep{{Args: []string{"issue", "close", "42", "--comment", "done"}}}, func(c Client) error { return c.Close(42, "done") }},
		{"list", []ghStep{listStep("work", `[]`)}, func(c Client) error { _, err := c.ListOpen("work"); return err }},
		{"ready", []ghStep{listStep("work", `[]`)}, func(c Client) error {
			_, err := c.Ready(config.IssueLabels{Unit: "work", Topic: "questions"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.steps[len(tc.steps)-1].Code = 4
			tc.steps[len(tc.steps)-1].Stderr = "not logged into any GitHub hosts\noriginal diagnostic"
			err := tc.run(fake(t, tc.steps...))
			testutil.Fails(t, err, "not logged into any GitHub hosts\noriginal diagnostic")
			testutil.Fails(t, err, "devkit doctor")
		})
	}
}

func TestMissingGhPointsAtDoctor(t *testing.T) {
	env := cli.Env{Cwd: t.TempDir(), LookupEnv: func(string) (string, bool) { return filepath.Join(t.TempDir(), "missing-gh"), true }}
	_, err := (Client{env}).Read(42)
	testutil.Fails(t, err, "devkit doctor")
	testutil.Fails(t, err, "no such file")
}

func TestGhDefaultsToExecutableOnPath(t *testing.T) {
	c := fake(t, listStep("work", `[]`))
	bin := t.TempDir()
	testutil.OK(t, os.Symlink(os.Args[0], filepath.Join(bin, "gh")))
	t.Setenv("PATH", bin)
	c.Env.LookupEnv = func(string) (string, bool) { return "", false }
	listed, err := c.ListOpen("work")
	testutil.OK(t, err)
	testutil.Equal(t, listed, []Issue{})
}

func TestInvalidIssueNumbersDoNotInvokeGh(t *testing.T) {
	for _, number := range []int{0, -1} {
		c := fake(t)
		_, err := c.Read(number)
		testutil.Fails(t, err, "positive integer")
		testutil.Fails(t, c.Close(number, "done"), "positive integer")
		testutil.Fails(t, c.AddSubIssue(number, 42), "positive integer")
		testutil.Fails(t, c.AddSubIssue(42, number), "positive integer")
		testutil.Fails(t, c.AddBlockedBy(number, 42), "positive integer")
		testutil.Fails(t, c.AddBlockedBy(42, number), "positive integer")
	}
}

func TestInvalidGhResponses(t *testing.T) {
	for _, response := range []string{"", "not a URL", "https://github.com/team/repo/pull/42", "https://github.com/team/repo/issues/0"} {
		c := fake(t, ghStep{Args: []string{"issue", "create", "--title", "title", "--body-file", "-"}, Output: response})
		_, err := c.Create("title", "", nil)
		testutil.Fails(t, err, "invalid issue URL")
	}
	for _, tc := range []struct {
		steps []ghStep
		run   func(Client) error
	}{
		{[]ghStep{{Args: []string{"issue", "view", "42", "--json", "number,title,body,state,labels,url,parent"}, Output: "invalid"}}, func(c Client) error { _, err := c.Read(42); return err }},
		{readSteps(42, `{}`, `invalid`, `[[]]`, `[[]]`)[:2], func(c Client) error { _, err := c.Read(42); return err }},
		{[]ghStep{listStep("work", `invalid`)}, func(c Client) error { _, err := c.ListOpen("work"); return err }},
		{[]ghStep{{Args: restArgs("42"), Output: `invalid`}}, func(c Client) error { return c.AddSubIssue(7, 42) }},
	} {
		testutil.Fails(t, tc.run(fake(t, tc.steps...)), "parse gh")
	}
	c := fake(t, ghStep{Args: restArgs("42"), Output: `{}`})
	testutil.Fails(t, c.AddBlockedBy(7, 42), "no issue ID")
}

func TestCommands(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%v", jsonOutput), func(t *testing.T) {
			c := fake(t, readySteps()...)
			testutil.Put(t, c.Env.Cwd, ".devkit/config.json", `{"issues":{"labels":{"unit":"work","topic":"questions"}}}`)
			var stdout, stderr bytes.Buffer
			c.Env.Stdout, c.Env.Stderr = &stdout, &stderr
			args := []string{"issues", "ready"}
			if jsonOutput {
				args = append(args, "--json")
			}
			testutil.Equal(t, cli.New("test", Commands()).Execute(args, c.Env), 0)
			if jsonOutput {
				var ready Ready
				testutil.OK(t, json.Unmarshal(stdout.Bytes(), &ready))
				testutil.Equal(t, len(ready.Units), 2)
				testutil.Equal(t, len(ready.Topics), 2)
				testutil.Equal(t, ready.Units[0].Parent.Number, 7)
			} else {
				testutil.Contains(t, stdout.String(), "unit #1 Unit 1 https://github.com/team/repo/issues/1 (parent #7: Topic)")
				testutil.Contains(t, stdout.String(), "topic #7 Topic 7")
				testutil.Equal(t, strings.Contains(stdout.String(), "Unit 3"), false)
			}
			c = fake(t, readSteps(42, `{"number":42,"title":"Question","body":"Owner text\n$(exit 1)","state":"OPEN","labels":[],"parent":{"number":7,"title":"Parent","url":"parent-url"}}`, `[[]]`, `[[]]`, `[[]]`)...)
			stdout.Reset()
			c.Env.Stdout, c.Env.Stderr = &stdout, &stderr
			args = []string{"issues", "show", "42"}
			if jsonOutput {
				args = append(args, "--json")
			}
			testutil.Equal(t, cli.New("test", Commands()).Execute(args, c.Env), 0)
			if jsonOutput {
				var issue Issue
				testutil.OK(t, json.Unmarshal(stdout.Bytes(), &issue))
				testutil.Equal(t, issue.Body, "Owner text\n$(exit 1)")
				testutil.Equal(t, issue.Number, 42)
			} else {
				testutil.Contains(t, stdout.String(), "#42 Question [open]")
				testutil.Contains(t, stdout.String(), "Owner text\n$(exit 1)")
				testutil.Contains(t, stdout.String(), "Parent: #7 Parent parent-url")
			}
		})
	}
}

func TestCommandUsage(t *testing.T) {
	for _, args := range [][]string{{"ready", "--bad"}, {"ready", "--json", "extra"}, {"show"}, {"show", "0"}, {"show", "-1"}, {"show", "no"}, {"show", "1", "--bad"}, {"show", "1", "--json", "extra"}} {
		var out bytes.Buffer
		testutil.Equal(t, cli.New("test", Commands()).Execute(append([]string{"issues"}, args...), cli.Env{Stdout: &out, Stderr: &out}), 2)
	}
}
