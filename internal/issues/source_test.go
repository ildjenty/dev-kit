package issues

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func sourceIssueBody() string {
	return "Deliver the result.\n\nSource: #80\n\n" + strings.TrimSuffix(draftMarker("commit", "1.json", "draft", "unit"), " -->") + "\nsource_pr: 80 -->"
}

func sourceIssueSteps(body string) []ghStep {
	issue := map[string]any{"number": 42, "title": "Deliver result", "body": body, "state": "OPEN", "labels": []map[string]string{{"name": "work"}}}
	return readSteps(42, string(testutil.Must(json.Marshal(issue))), `[[]]`, `[[]]`, `[[]]`)
}

func sourcePRStep(state string, merged bool) ghStep {
	var mergedAt any
	if merged {
		mergedAt = "2026-11-25T01:00:00Z"
	}
	return ghStep{Args: []string{"pr", "view", "80", "--json", "number,title,url,state,mergedAt"}, Output: string(testutil.Must(json.Marshal(map[string]any{"number": 80, "title": "Approved proposal", "url": "https://github.example/team/repo/pull/80", "state": state, "mergedAt": mergedAt})))}
}

func TestReadySeparatesUnitsWaitingForSourcePR(t *testing.T) {
	for _, state := range []string{"OPEN", "CLOSED", "MERGED", "manual"} {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(state+"/json="+map[bool]string{true: "true", false: "false"}[jsonOutput], func(t *testing.T) {
				steps := []ghStep{listStep("work", `[{"number":42,"state":"OPEN","labels":[{"name":"work"}]}]`)}
				body := sourceIssueBody()
				if state == "manual" {
					body = "Source: #80\nHand-made unit without machine metadata."
				}
				steps = append(steps, sourceIssueSteps(body)...)
				if state != "manual" {
					steps = append(steps, sourcePRStep(state, state == "MERGED"))
				}
				steps = append(steps, listStep("questions", `[]`))
				c := fake(t, steps...)
				testutil.Put(t, c.Env.Cwd, ".devkit/config.json", `{"issues":{"labels":{"unit":"work","topic":"questions"}}}`)
				var stdout bytes.Buffer
				c.Env.Stdout = &stdout
				args := []string{"issues", "ready"}
				if jsonOutput {
					args = append(args, "--json")
				}
				testutil.OK(t, cli.New("test", Commands()).Run(args, c.Env))
				waiting := state == "OPEN" || state == "CLOSED"
				if jsonOutput {
					var ready Ready
					testutil.OK(t, json.Unmarshal(stdout.Bytes(), &ready))
					testutil.Equal(t, len(ready.Waiting) == 1, waiting)
					testutil.Equal(t, len(ready.Units) == 0, waiting)
					if waiting {
						testutil.Equal(t, ready.Waiting[0].SourcePR.Number, 80)
					}
				} else {
					testutil.Equal(t, strings.Contains(stdout.String(), "waiting unit #42"), waiting)
					if waiting {
						testutil.Contains(t, stdout.String(), "source PR #80")
					}
				}
			})
		}
	}
}

func TestReadyUnitRequiresSourcePRMerge(t *testing.T) {
	for _, state := range []string{"OPEN", "CLOSED", "MERGED", "manual", "lookup failure"} {
		t.Run(state, func(t *testing.T) {
			body := sourceIssueBody()
			if state == "manual" {
				body = "Hand-made unit"
			}
			steps := sourceIssueSteps(body)
			if state != "manual" {
				step := sourcePRStep(state, state == "MERGED")
				if state == "lookup failure" {
					step.Code, step.Stderr = 1, "source unavailable"
				}
				steps = append(steps, step)
			}
			_, err := fake(t, steps...).ReadyUnit(42, "work")
			if state == "MERGED" || state == "manual" {
				testutil.OK(t, err)
			} else {
				testutil.Fails(t, err, "source PR #80")
			}
		})
	}
	testutil.Equal(t, StripDraftMarker(sourceIssueBody()), "Deliver the result.\n\nSource: #80")
}

func TestCreateDraftAdoptsUnitWithSourcePRMetadata(t *testing.T) {
	marker := strings.TrimSuffix(draftMarker("commit", "1.json", "draft", "unit"), " -->") + "\nsource_pr: 80 -->"
	pages := [][]map[string]any{{{"number": 42, "body": "Goal\n\n" + marker, "labels": []map[string]string{{"name": "work"}}}}}
	c := fake(t, draftListStep(string(testutil.Must(json.Marshal(pages)))))
	created, err := c.createDraft("commit", "1.json", "draft", TopicDraft{Key: "unit"}, "work", 80)
	testutil.OK(t, err)
	testutil.Equal(t, created.Number, 42)
}
