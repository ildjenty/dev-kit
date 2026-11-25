package refine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func agreedWithoutIssues(t *testing.T, old bool) (*fixture, string) {
	t.Helper()
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin(strings.Repeat("Owner request. ", 110))
	d := draft()
	d["topics"] = []doc{{"key": "TN1", "title": "Choose capacity", "body": "How many employees?", "blocked_by": []string{}}}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 UN1 TN1 approve=1 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	out := r.finish()
	text := str(items(out["questions"])[0]["text"])
	testutil.Contains(t, text, "new topic: Serve employees\nWho can use the service? Employees only.")
	testutil.Equal(t, strings.Contains(text, "#0"), false)
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	decision := r.data().Decisions[0]
	decision["status"] = "agreed"
	r.put(str(decision["path"]), decisionText(decision))
	testutil.OK(t, proposal.CommitPaths(r.ctx, []string{str(decision["path"])}, "docs: agree refine proposal 1"))
	progress := testutil.Must(issues.ReadCreation(creationPath(r.ctx, "1")))
	progress.RunID, progress.Commit = r.ctx.RunID, r.git("rev-parse", "HEAD")
	testutil.OK(t, issues.SaveCreation(creationPath(r.ctx, "1"), progress))
	if old {
		p := r.latest("proposal")
		content := obj(p["content"])
		delete(obj(content["draft"]), "parent")
		content["topic"] = doc{"number": 0, "title": strings.Repeat("Owner request. ", 110), "body": "Original request"}
		testutil.OK(t, proposal.Save(r.ctx, p, content))
	}
	for _, name := range []string{"calls", "count", "labels"} {
		_, err := os.Stat(filepath.Join(log, name))
		testutil.Equal(t, os.IsNotExist(err), true)
	}
	testutil.Equal(t, r.git("ls-remote", "origin", "refs/heads/"+r.ctx.Branch), "")
	return r, log
}

func TestApprovalRetryFromAgreedCommitWithoutIssuesOrLabels(t *testing.T) {
	r, log := agreedWithoutIssues(t, false)
	head := r.git("rev-parse", "HEAD")
	r.ctx.RunID = "r3"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	result := testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, result["outcome"], "pass")
	testutil.Equal(t, len(result["issue_numbers"].([]int)), 3)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("log", "--format=%s", "--grep=^docs: agree refine"), "docs: agree refine proposal 1")
	testutil.Equal(t, r.git("ls-remote", "origin", "refs/heads/"+r.ctx.Branch), head+"\trefs/heads/"+r.ctx.Branch)
	calls := readText(t, filepath.Join(log, "calls"))
	testutil.Equal(t, strings.Index(calls, "pr create") < strings.Index(calls, "label create"), true)
	testutil.Equal(t, strings.Index(calls, "label create devkit:topic --force") < strings.Index(calls, "issue create"), true)
	testutil.Contains(t, calls, "label create devkit:unit --force")
	testutil.Contains(t, calls, "issue create --title Serve employees")
	testutil.Contains(t, calls, "issue create --title Choose capacity")
	testutil.Contains(t, calls, "issue create --title Deliver staff access")
	testutil.Contains(t, readText(t, filepath.Join(log, "bodies")), "Who can use the service? Employees only.")
	testutil.Contains(t, readText(t, filepath.Join(log, "pr-bodies")), "#101 Serve employees")
}

func TestOldProposalRetryRefusesBeforeAnyExternalWrite(t *testing.T) {
	r, log := agreedWithoutIssues(t, true)
	head := r.git("rev-parse", "HEAD")
	r.ctx.RunID = "r3"
	started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"})))
	testutil.Equal(t, started.Stopped, true)
	testutil.Contains(t, started.Reason, "make the proposal again so the new parent topic title can be approved")
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "make the proposal again so the new parent topic title can be approved")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("ls-remote", "origin", "refs/heads/"+r.ctx.Branch), "")
	_, err = os.Stat(filepath.Join(log, "calls"))
	testutil.Equal(t, os.IsNotExist(err), true)
}

func TestRefineReproposesParentAfterOldApprovalAgreedDecisions(t *testing.T) {
	r, _ := agreedWithoutIssues(t, true)
	decision := r.data().Decisions[0]
	before := readText(t, filepath.Join(r.cwd, str(decision["path"])))
	r.ctx.RunID = "r3"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(nil))).Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Equal(t, input["topic"], nil)
	testutil.Equal(t, input["next_proposal_version"], float64(2))
	d := clone(obj(obj(r.latest("proposal")["content"])["draft"]))
	d["parent"] = obj(draft()["parent"])
	d["decisions"] = []doc{}
	r.writeDraft(d)
	r.artifact("approval.txt", "UN1 TN1 approve=2 or answers; agreed decision remains in the log.")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	result := r.finish()
	testutil.Equal(t, result["outcome"], "question")
	testutil.Equal(t, result["proposal_version"], 2)
	testutil.Contains(t, str(items(result["questions"])[0]["text"]), "new topic: Serve employees")
	r.ctx.RunID = "r4"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "2"}))).Stopped, false)
	approved := testutil.Must(approve(r.ctx, "2"))
	testutil.Equal(t, approved["outcome"], "pass")
	testutil.Equal(t, len(approved["issue_numbers"].([]int)), 3)
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, str(decision["path"]))), before)
	testutil.Equal(t, len(r.data().Decisions), 1)
	testutil.Equal(t, r.git("log", "--format=%s", "--grep=^docs: agree refine"), "docs: agree refine proposal 1")
}

func TestDraftRequiresNewParentAndChecksEveryIssueTitle(t *testing.T) {
	r := setup(t, nil)
	r.begin(strings.Repeat("Owner request. ", 110))
	r.writeDraft()
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Equal(t, input["topic"], nil)
	for _, group := range []string{"parent", "units", "topics"} {
		for _, length := range []int{256, 257} {
			d := draft()
			d["topics"] = []doc{{"key": "TN1", "title": "Capacity", "body": "How many employees?", "blocked_by": []string{}}}
			item := obj(d["parent"])
			if group != "parent" {
				item = items(d[group])[0]
			}
			item["title"] = strings.Repeat("界", length)
			r.artifact("approval.txt", "N1 UN1 TN1 approve=1 or answers")
			problems := r.errors(d)
			if length == 256 {
				testutil.Equal(t, problems, "")
			} else {
				testutil.Contains(t, problems, "256")
			}
		}
	}
	for _, field := range []string{"title", "body"} {
		d := draft()
		obj(d["parent"])[field] = "   "
		testutil.Contains(t, r.errors(d), "required")
	}
	d := draft()
	delete(d, "parent")
	testutil.Contains(t, r.errors(d), "required property parent")
}
