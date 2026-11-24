package retro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestUnitDraftsWaitForApprovalAndRetryOnlyMissingIssues(t *testing.T) {
	r := setup(t, backlog())
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	d := r.draft()
	second := doc{}
	for key, value := range items(d["units"])[0] {
		second[key] = value
	}
	second["key"], second["title"], second["blocked_by"] = "second", "Second repair", []string{"repair"}
	d["units"] = append(items(d["units"]), second)
	r.write(d)
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "question")
	_, err := os.Stat(filepath.Join(log, "calls"))
	testutil.Equal(t, os.IsNotExist(err), true)
	_, err = os.Stat(filepath.Join(r.cwd, ".devkit/units"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.Equal(t, len(items(obj(obj(testutil.Must(record.Latest(r.ctx.Dir, "proposal"))["content"])["draft"])["units"])), 2)
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Put(t, log, "fail", "2")
	_, err = approve(r.ctx, "1")
	testutil.Fails(t, err, "gh issue create failed")
	path := filepath.Join(r.ctx.Dir, "issue-creation", "1.json")
	progress := testutil.Must(issues.ReadCreation(path))
	testutil.Equal(t, progress.RunID, "r2")
	testutil.Equal(t, progress.Created["repair"].Number, 101)
	r.ctx.RunID = "r3"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.OK(t, os.Remove(filepath.Join(log, "fail")))
	result := testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, result["outcome"], "pass")
	testutil.Equal(t, readFile(t, filepath.Join(log, "count")), "2")
	calls := readFile(t, filepath.Join(log, "calls"))
	testutil.Equal(t, strings.Count(calls, "issue create --title Repair defect"), 1)
	testutil.Contains(t, calls, "/issues/102/dependencies/blocked_by")
	testutil.Equal(t, strings.Contains(calls, "sub_issues"), false)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	return string(testutil.Must(os.ReadFile(path)))
}
