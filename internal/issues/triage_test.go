package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func triageItems() []TriageItem {
	return []TriageItem{
		{Key: "parent", Number: 7, Kind: "topic", Title: "Resolve delivery", Body: "Holds unit. Which audience?", HeldUnits: []string{"unit"}, Reason: "Audience is unsettled"},
		{Key: "unit", Number: 42, Kind: "unit", Title: "Deliver result", Body: "Deliver the requested result.", CompletionConditions: []string{"The requested result is visible"}, Parent: "parent", BlockedBy: []string{"parent", "questions"}, Reason: "Settled work and observable completion"},
		{Key: "questions", Kind: "topic", Title: "Resolve access", Body: "Holds unit. Which access rules?", HeldUnits: []string{"unit"}, Parent: "parent", Reason: "Separate access question"},
		{Key: "excluded", Number: 8, Kind: "leave", Reason: "Historical request outside this fit"},
	}
}
func triageExisting() []Issue {
	return []Issue{{Reference: Reference{Number: 7, State: "open"}}, {Reference: Reference{Number: 42, State: "open"}}, {Reference: Reference{Number: 8, State: "open"}}}
}
func TestTriageStructuralChecks(t *testing.T) {
	labels := config.IssueLabels{Unit: "devkit:unit", Topic: "devkit:topic"}
	testutil.Equal(t, TriageErrors(triageItems(), triageExisting(), labels), []string{})
	for _, tc := range []struct {
		name, want string
		edit       func([]TriageItem) []TriageItem
	}{
		{"missing disposition", "no triage", func(d []TriageItem) []TriageItem { return d[:3] }},
		{"unknown issue", "existing open issue", func(d []TriageItem) []TriageItem { d[0].Number = 99; return d }},
		{"no completion", "completion conditions", func(d []TriageItem) []TriageItem { d[1].CompletionConditions = nil; return d }},
		{"mismatched held units", "does not match", func(d []TriageItem) []TriageItem { d[0].HeldUnits = nil; return d }},
		{"unmentioned held unit", "body must state", func(d []TriageItem) []TriageItem { d[0].Body = "Question"; return d }},
		{"new unit", "new issues must be topics", func(d []TriageItem) []TriageItem { d[1].Number = 0; return d }},
		{"missing parent", "another topic", func(d []TriageItem) []TriageItem { d[1].Parent = "absent"; return d }},
		{"cycle", "blocked_by cycle", func(d []TriageItem) []TriageItem { d[0].BlockedBy = []string{"unit"}; return d }},
		{"title limit", "256 characters", func(d []TriageItem) []TriageItem { d[0].Title = strings.Repeat("x", 257); return d }},
		{"exclude blocker", "left-out issue cannot block", func(d []TriageItem) []TriageItem { d[1].BlockedBy = append(d[1].BlockedBy, "excluded"); return d }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Contains(t, strings.Join(TriageErrors(tc.edit(triageItems()), triageExisting(), labels), "\n"), tc.want)
		})
	}
	for _, tc := range []struct {
		name string
		edit func([]Issue)
	}{
		{"closed", func(d []Issue) { d[0].State = "closed" }},
		{"already unit", func(d []Issue) { d[0].Labels = []string{labels.Unit} }},
		{"already topic", func(d []Issue) { d[0].Labels = []string{labels.Topic} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := triageExisting()
			tc.edit(existing)
			testutil.Contains(t, strings.Join(TriageErrors(triageItems(), existing, labels), "\n"), "existing open issue")
		})
	}
}
func TestTriageApplicationApprovalAndRetry(t *testing.T) {
	log, gh := testutil.IssueCreationCLI(t)
	env := cli.Env{Cwd: t.TempDir(), Environ: os.Environ()}
	client := Client{Env: env}
	path := filepath.Join(t.TempDir(), "1.json")
	labels := config.IssueLabels{Unit: "work", Topic: "question"}
	_, err := client.ApplyTriage(path, triageItems(), labels)
	testutil.Fails(t, err, "recorded approval")
	_, err = os.Stat(filepath.Join(log, "calls"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.OK(t, SaveCreation(path, Creation{RunID: "approval-1", Commit: strings.Repeat("a", 40), PRNumber: 80, Created: map[string]Created{}, SubIssues: map[string]bool{}, BlockedBy: map[string]bool{}}))
	wrapper := filepath.Join(t.TempDir(), "gh")
	testutil.Put(t, filepath.Dir(wrapper), "gh", `#!/bin/sh
case "$*" in
 *dependencies/blocked_by*--method*)
  if test ! -f '`+log+`/failed'; then touch '`+log+`/failed'; exit 1; fi ;;
esac
exec '`+gh+`' "$@"
`)
	testutil.OK(t, os.Chmod(wrapper, 0755))
	t.Setenv("DEVKIT_GH", wrapper)
	client.Env.Environ = os.Environ()
	_, err = client.ApplyTriage(path, triageItems(), labels)
	testutil.Fails(t, err, "failed")
	progress := testutil.Must(client.ApplyTriage(path, triageItems(), labels))
	testutil.Equal(t, len(progress.Created), 3)
	testutil.Equal(t, progress.RunID, "approval-1")
	testutil.Equal(t, progress.Numbers(), []int{7, 42, 101})
	body := string(testutil.Must(os.ReadFile(filepath.Join(log, "edited-42"))))
	testutil.Contains(t, body, "The requested result is visible")
	created := string(testutil.Must(os.ReadFile(filepath.Join(log, "bodies"))))
	testutil.Contains(t, created, "<!-- devkit:draft ")
	calls := string(testutil.Must(os.ReadFile(filepath.Join(log, "calls"))))
	testutil.Contains(t, calls, "--add-label work")
	testutil.Contains(t, calls, "--add-label question")
	testutil.Contains(t, calls, "sub_issues")
	testutil.Contains(t, calls, "dependencies/blocked_by")
	testutil.Equal(t, strings.Count(calls, "issue create"), 1)
	testutil.Equal(t, strings.Count(calls, "issue edit 42"), 1)
	testutil.Must(client.ApplyTriage(path, triageItems(), labels))
	again := string(testutil.Must(os.ReadFile(filepath.Join(log, "calls"))))
	testutil.Equal(t, strings.Count(again, "issue create"), 1)
	testutil.Equal(t, strings.Count(again, "issue edit"), 2)
	testutil.Equal(t, strings.Count(again, "--method POST"), strings.Count(calls, "--method POST"))
}
