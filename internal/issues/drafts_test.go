package issues

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func draftListStep(output string) ghStep {
	return ghStep{Args: []string{"api", "repos/{owner}/{repo}/issues?state=all&per_page=100", "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2026-03-10", "--paginate", "--slurp"}, Output: output}
}

func draftMarker(commit, version, kind, key string) string {
	identity := testutil.Must(json.Marshal([]string{commit, version, kind, key}))
	return fmt.Sprintf("<!-- devkit:draft %x -->", sha256.Sum256(identity))
}

func TestEnsureConfiguredLabelsCanBeRepeated(t *testing.T) {
	steps := []ghStep{{Args: []string{"label", "create", "work, unit", "--force"}}, {Args: []string{"label", "create", "open questions", "--force"}}}
	c := fake(t, append(append([]ghStep{}, steps...), steps...)...)
	labels := config.IssueLabels{Unit: "work, unit", Topic: "open questions"}
	testutil.OK(t, c.EnsureLabels(labels))
	testutil.OK(t, c.EnsureLabels(labels))
}

func TestLabelFailureStopsBeforeIssueCreation(t *testing.T) {
	for _, label := range []string{"work", "questions"} {
		path := filepath.Join(t.TempDir(), "1.json")
		progress := testutil.Must(ReadCreation(path))
		progress.Commit = "proposal"
		testutil.OK(t, SaveCreation(path, progress))
		steps := []ghStep{}
		if label == "questions" {
			steps = append(steps, ghStep{Args: []string{"label", "create", "work", "--force"}})
		}
		steps = append(steps, ghStep{Args: []string{"label", "create", label, "--force"}, Code: 1, Stderr: "label denied"})
		c := fake(t, steps...)
		_, err := c.CreateDrafts(path, Drafts{Topics: []TopicDraft{{Key: "first", Title: "Question", Body: "Owner text"}}}, config.IssueLabels{Unit: "work", Topic: "questions"}, nil, 0, "")
		testutil.Fails(t, err, "label denied")
		testutil.Equal(t, testutil.Must(ReadCreation(path)), progress)
	}
}

func TestCreateDraftsAdoptsAfterUnrecordedCreation(t *testing.T) {
	for _, kind := range []string{"unit", "topic", "parent"} {
		for _, failure := range []string{"lost response", "invalid response", "record save"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "1.json")
				initial, err := ReadCreation(path)
				testutil.OK(t, err)
				initial.RunID, initial.Commit = "approval-run", "proposal-commit"
				testutil.OK(t, SaveCreation(path, initial))
				draft := TopicDraft{Key: "first", Title: "Create it", Body: "Owner text", BlockedBy: []string{}}
				drafts := Drafts{}
				var parent *TopicDraft
				label, role, body := "devkit:topic", "draft", draft.Body
				switch kind {
				case "unit":
					label, body = "devkit:unit", "Owner text\n\n- Done\n"
					drafts.Units = []UnitDraft{{Key: draft.Key, Title: draft.Title, Goal: draft.Body, CompletionConditions: []string{"Done"}}}
				case "topic":
					drafts.Topics = []TopicDraft{draft}
				case "parent":
					parent, role = &draft, "parent"
				}
				marker := draftMarker(initial.Commit, "1.json", role, draft.Key)
				body += "\n\n" + marker
				create := ghStep{Args: []string{"issue", "create", "--title", draft.Title, "--body-file", "-", "--label", label}, Input: body, Output: "https://github.example/team/repo/issues/42"}
				if failure == "lost response" {
					create.Output, create.Code, create.Stderr = "", 1, "connection lost after creation"
				} else if failure == "invalid response" {
					create.Output = ""
				} else {
					create.BlockSave = path
				}
				remote := testutil.Must(json.Marshal([][]map[string]any{{{"number": 42, "html_url": "https://github.example/team/repo/issues/42", "body": body, "state": "open", "labels": []map[string]string{{"name": label}}}}}))
				labelsSteps := []ghStep{{Args: []string{"label", "create", "devkit:unit", "--force"}}, {Args: []string{"label", "create", "devkit:topic", "--force"}}}
				steps := append(append([]ghStep{}, labelsSteps...), draftListStep(`[[]]`), create)
				steps = append(steps, labelsSteps...)
				steps = append(steps, draftListStep(string(remote)))
				if kind == "parent" {
					steps = append(steps, ghStep{Args: []string{"issue", "close", "42", "--comment", "Approved"}})
				}
				c := fake(t, steps...)
				labels := config.IssueLabels{Unit: "devkit:unit", Topic: "devkit:topic"}
				_, err = c.CreateDrafts(path, drafts, labels, parent, 0, "Approved")
				if failure == "lost response" {
					testutil.Fails(t, err, "connection lost after creation")
				} else if failure == "invalid response" {
					testutil.Fails(t, err, "invalid issue URL")
				} else {
					testutil.Fails(t, err, "rename")
					testutil.OK(t, os.Remove(path))
					testutil.OK(t, os.Rename(path+".saved", path))
				}
				before := testutil.Must(ReadCreation(path))
				testutil.Equal(t, len(before.Created), 0)
				testutil.Equal(t, before.Parent, (*Created)(nil))
				progress, err := c.CreateDrafts(path, drafts, labels, parent, 0, "Approved")
				testutil.OK(t, err)
				testutil.Equal(t, progress.Numbers(), []int{42})
				testutil.Equal(t, testutil.Must(ReadCreation(path)), progress)
			})
		}
	}
}

func TestFindDraftMatchesLabelAndBodyAcrossPages(t *testing.T) {
	marker := draftMarker("proposal", "1.json", "draft", "first")
	pages := [][]map[string]any{{
		{"number": 1, "body": marker, "labels": []map[string]string{{"name": "other"}}},
		{"number": 2, "title": marker, "body": "Unrelated", "labels": []map[string]string{{"name": "work, unit"}}},
		{"number": 3, "body": marker, "pull_request": map[string]string{"url": "pull-url"}, "labels": []map[string]string{{"name": "work, unit"}}},
	}, {
		{"number": 42, "body": "Owner text\n\n" + marker, "state": "closed", "html_url": "issue-url", "labels": []map[string]string{{"name": "work, unit"}}},
	}}
	c := fake(t, draftListStep(string(testutil.Must(json.Marshal(pages)))))
	created, found, err := c.findDraft(marker, "work, unit")
	testutil.OK(t, err)
	testutil.Equal(t, found, true)
	testutil.Equal(t, created, Created{42, "issue-url"})
}

func TestDraftIdentityDistinguishesProposalsAndParent(t *testing.T) {
	for _, identity := range [][3]string{{"commit-one", "1.json", "draft"}, {"commit-two", "1.json", "draft"}, {"commit-one", "2.json", "draft"}, {"commit-one", "1.json", "parent"}} {
		t.Run(fmt.Sprint(identity), func(t *testing.T) {
			marker := draftMarker(identity[0], identity[1], identity[2], "same-key")
			c := fake(t, draftListStep(`[[]]`), ghStep{Args: []string{"issue", "create", "--title", "Same title", "--body-file", "-", "--label", "work"}, Input: "Same body\n\n" + marker, Output: "https://github.example/team/repo/issues/42"})
			_, err := c.createDraft(identity[0], identity[1], identity[2], TopicDraft{Key: "same-key", Title: "Same title", Body: "Same body"}, "work", 0)
			testutil.OK(t, err)
		})
	}
}

func TestStripDraftMarkerPreservesRequestText(t *testing.T) {
	body := "# Goal\n\nOwner text\n<!-- keep this comment -->\n"
	testutil.Equal(t, StripDraftMarker(body), body)
	testutil.Equal(t, StripDraftMarker(body+"\n\n"+draftMarker("proposal", "1.json", "draft", "first")), body)
}

func TestCreateDraftsAdoptsAndRepairsMissingRelationships(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1.json")
	progress := testutil.Must(ReadCreation(path))
	progress.Commit, progress.Parent = "proposal", &Created{Number: 7}
	progress.Created["first"] = Created{Number: 41}
	testutil.OK(t, SaveCreation(path, progress))
	drafts := Drafts{Units: []UnitDraft{{Key: "first", Title: "First", Goal: "First goal"}, {Key: "second", Title: "Second", Goal: "Second goal", BlockedBy: []string{"first"}}}}
	remote := [][]map[string]any{{{"number": 42, "body": draftMarker("proposal", "1.json", "draft", "second"), "labels": []map[string]string{{"name": "work"}}}}}
	c := fake(t,
		ghStep{Args: []string{"label", "create", "work", "--force"}},
		ghStep{Args: []string{"label", "create", "questions", "--force"}},
		draftListStep(string(testutil.Must(json.Marshal(remote)))),
		relationStep(7, "sub_issues", `[[{"number":41}]]`),
		relationStep(7, "sub_issues", `[[{"number":41}]]`),
		ghStep{Args: restArgs("42"), Output: `{"id":1042}`},
		ghStep{Args: append(restArgs("7/sub_issues"), "--method", "POST", "--field", "sub_issue_id=1042"), Output: `{}`},
		relationStep(42, "dependencies/blocked_by", `[[]]`),
		ghStep{Args: restArgs("41"), Output: `{"id":1041}`},
		ghStep{Args: append(restArgs("42/dependencies/blocked_by"), "--method", "POST", "--field", "issue_id=1041"), Output: `{}`},
		ghStep{Args: []string{"issue", "close", "7", "--comment", "Approved\n- #41 First\n- #42 Second"}},
	)
	progress, err := c.CreateDrafts(path, drafts, config.IssueLabels{Unit: "work", Topic: "questions"}, &TopicDraft{}, 7, "Approved")
	testutil.OK(t, err)
	testutil.Equal(t, progress.Numbers(), []int{7, 41, 42})
	testutil.Equal(t, progress.SubIssues, map[string]bool{"first": true, "second": true})
	testutil.Equal(t, progress.BlockedBy, map[string]bool{"second:first": true})
	testutil.Equal(t, testutil.Must(ReadCreation(path)), progress)
}

func TestAddRelationshipsToleratesExistingAndLostResponses(t *testing.T) {
	for _, relation := range []string{"sub_issues", "dependencies/blocked_by"} {
		for _, lostResponse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lost=%v", relation, lostResponse), func(t *testing.T) {
				steps := []ghStep{}
				if lostResponse {
					field := "issue_id=1042"
					if relation == "sub_issues" {
						field = "sub_issue_id=1042"
					}
					steps = append(steps, relationStep(7, relation, `[[]]`), ghStep{Args: restArgs("42"), Output: `{"id":1042}`}, ghStep{Args: append(restArgs("7/"+relation), "--method", "POST", "--field", field), Code: 1, Stderr: "response lost"})
				}
				steps = append(steps, relationStep(7, relation, `[[{"number":40}],[{"number":42}]]`))
				c := fake(t, steps...)
				if relation == "sub_issues" {
					testutil.OK(t, c.AddSubIssue(7, 42))
				} else {
					testutil.OK(t, c.AddBlockedBy(7, 42))
				}
			})
		}
	}
}

func TestDraftLookupFailureDoesNotCreate(t *testing.T) {
	for _, response := range []string{"invalid JSON", "failure"} {
		step := draftListStep(response)
		if response == "failure" {
			step.Code, step.Stderr = 1, "lookup failed"
		}
		_, err := fake(t, step).createDraft("proposal", "1.json", "draft", TopicDraft{Key: "first"}, "work", 0)
		if err == nil {
			t.Fatal("expected lookup failure")
		}
	}
}
