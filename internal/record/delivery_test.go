package record

import (
	"testing"

	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestDeliveryStartAndReturnedIgnoreDecisionRecords(t *testing.T) {
	repo, dir := testutil.Repo(t), afterPlan(t)
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "malformed log\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "log")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	req := must(Latest(dir, "requirements"))
	delete(req, "version")
	req["constraints"] = []string{"D-abcdef"}
	must(Append(dir, "requirements", req))
	for _, workflow := range []string{"requirements", "plan", "implement", "judge", "fix", "pr", "sync"} {
		t.Run(workflow, func(t *testing.T) {
			without := must(StartConditions(dir, workflow, ConditionOptions{Commit: "head"}))
			with := must(StartConditions(dir, workflow, ConditionOptions{Cwd: repo, BaseBranch: "main", Commit: "head"}))
			testutil.Equal(t, with, without)
		})
	}
	for _, workflow := range []string{"requirements", "plan"} {
		testutil.Equal(t, must(Returned(ContextData{Dir: dir, Cwd: repo, BaseBranch: "main"}, workflow)), []Document{})
	}
}
