package retro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPRDraftTextIsRequiredAndSavedInDetails(t *testing.T) {
	r := setup(t, backlog())
	r.put(".devkit/config.json", `{"language":"ja"}`)
	r.commit()
	r.begin()
	r.write(r.draft())
	for _, field := range []string{"pr_title", "pr_summary"} {
		d := r.draft()
		delete(d, field)
		testutil.Contains(t, r.errors(d), "required property "+field)
		d = r.draft()
		d[field] = "   "
		testutil.Equal(t, r.errors(d) == "", false)
		d[field] = "　"
		testutil.Equal(t, r.errors(d) == "", false)
	}
	for _, title := range []string{strings.Repeat("界", 101), "First\nSecond"} {
		d := r.draft()
		d["pr_title"] = title
		testutil.Equal(t, r.errors(d) == "", false)
	}
	d := r.draft()
	d["pr_title"] = strings.Repeat("界", 100)
	testutil.Equal(t, r.errors(d), "")
	r.write(d)
	testutil.Equal(t, testutil.Must(r.review(judgment()))["valid"], true)
	result := testutil.Must(finish(r.ctx, r.artifacts))
	text := str(items(result["questions"])[0]["text"])
	testutil.Equal(t, text, readFile(t, file(r.artifacts, "approval.txt")))
	for _, field := range []string{"PR タイトル:", "PR の概要:", "Proposal version:", "Commit:", "Parent:", "mapping", "blocked_by", "{", "}"} {
		testutil.Equal(t, strings.Contains(text, field), false)
	}
	testutil.Equal(t, strings.Count(text, "approve=1"), 1)
	details := testutil.Must(readJSON(filepath.Join(r.ctx.Dir, "proposals/1.details.json")))
	testutil.Equal(t, details, testutil.Must(record.Latest(r.ctx.Dir, "proposal")))
	testutil.Equal(t, obj(details["content"])["draft"], testutil.Must(readJSON(file(r.artifacts, "draft.json"))))
	log, _ := testutil.IssueCreationCLI(t)
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, readFile(t, filepath.Join(log, "pr-title")), str(d["pr_title"]))
	body := readFile(t, filepath.Join(log, "pr-body"))
	testutil.Equal(t, strings.HasPrefix(body, str(d["pr_summary"])+"\n\n"), true)
	testutil.Contains(t, body, "## 作成した Issue")
	testutil.Contains(t, body, "作業単位は、この PR のマージ後、blocked-by の依存条件を満たすと開始可能になります。")
}

func TestOldProposalWithoutPRTextRefusesBeforeExternalWrites(t *testing.T) {
	for _, field := range []string{"pr_title", "pr_summary"} {
		t.Run(field, func(t *testing.T) {
			r := setup(t, backlog())
			log, _ := testutil.IssueCreationCLI(t)
			r.begin()
			r.propose()
			p := testutil.Must(record.Latest(r.ctx.Dir, "proposal"))
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
		})
	}
}
