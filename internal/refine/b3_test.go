package refine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPRDraftTextIsRequiredAndSavedInDetails(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	for _, field := range []string{"pr_title", "pr_summary"} {
		d := draft()
		delete(d, field)
		testutil.Contains(t, r.errors(d), "required property "+field)
		d = draft()
		d[field] = "   "
		testutil.Equal(t, r.errors(d) == "", false)
		d[field] = "　"
		testutil.Equal(t, r.errors(d) == "", false)
	}
	for _, title := range []string{strings.Repeat("界", 101), "First\nSecond"} {
		d := draft()
		d["pr_title"] = title
		testutil.Equal(t, r.errors(d) == "", false)
	}
	d := draft()
	d["pr_title"] = strings.Repeat("界", 100)
	testutil.Equal(t, r.errors(d), "")
	r.writeDraft(d)
	testutil.Equal(t, r.round(judgment())["valid"], true)
	result := r.finish()
	text := str(items(result["questions"])[0]["text"])
	testutil.Equal(t, text, readText(t, file(r.artifacts, "approval.txt")))
	for _, field := range []string{"PR title:", "PR summary:", "Proposal version:", "Commit:", "Parent:", "mapping", "blocked_by", "{", "}"} {
		testutil.Equal(t, strings.Contains(text, field), false)
	}
	testutil.Equal(t, strings.Count(text, "approve=1"), 1)
	noApprovalErrors(t, r.ctx, "1")
	rejectsApproval(t, r.ctx, "2")
	details := testutil.Must(readJSON(filepath.Join(r.ctx.Dir, "proposals/1.details.json")))
	testutil.Equal(t, details, r.latest("proposal"))
	testutil.Equal(t, obj(details["content"])["draft"], testutil.Must(readJSON(file(r.artifacts, "draft.json"))))
}

func TestOldProposalWithoutPRTextRefusesBeforeExternalWrites(t *testing.T) {
	for _, field := range []string{"pr_title", "pr_summary"} {
		t.Run(field, func(t *testing.T) {
			r := setup(t, nil)
			log, _ := testutil.IssueCreationCLI(t)
			r.begin()
			r.writeDraft()
			r.round(judgment())
			r.finish()
			p := r.latest("proposal")
			content := obj(p["content"])
			delete(obj(content["draft"]), field)
			testutil.OK(t, proposal.Save(r.ctx, p, content))
			head := r.git("rev-parse", "HEAD")
			r.ctx.RunID = "r2"
			started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"})))
			testutil.Equal(t, started.Stopped, true)
			testutil.Contains(t, started.Reason, "make the proposal again so its PR title and summary can be approved")
			_, err := approve(r.ctx, "1")
			testutil.Fails(t, err, "make the proposal again so its PR title and summary can be approved")
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
			testutil.Equal(t, r.git("ls-remote", "origin", "refs/heads/"+r.ctx.Branch), "")
			_, err = os.Stat(filepath.Join(log, "calls"))
			testutil.Equal(t, os.IsNotExist(err), true)
			testutil.Equal(t, r.data().Decisions[0]["status"], "proposed")
		})
	}
}
