package requirements

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func TestPrepareCarriesIssueAndParentRequestTextAndSurvivesAnswers(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "malformed decision record")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "criteria and log")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	testutil.IssueCLI(t, "OPEN", "devkit:unit", "closed")
	ctx := record.ContextData{Cwd: repo, BaseBranch: "main", Dir: t.TempDir(), RunID: "r1"}
	run(ctx.Dir, ctx.RunID)
	a := t.TempDir()
	env := cli.Env{Cwd: repo, Environ: os.Environ(), LookupEnv: func(k string) (string, bool) {
		if k == "INPUTS_ISSUE" {
			return "42", true
		}
		return os.LookupEnv(k)
	}}
	must(prepare(ctx, a, "Owner request", "Investigated context", "", env))
	input := must(read(filepath.Join(a, "requirements/input.json")))
	testutil.Equal(t, object(input["issue"])["title"], "Allow sign-in")
	testutil.Equal(t, object(input["issue"])["body"], "Allow employee sign-in.\nKeep the submitted request text.")
	testutil.Equal(t, object(input["parent_topic"])["title"], "Audience")
	testutil.Equal(t, object(input["parent_topic"])["body"], "Who may sign in?\nEmployees only.")
	testutil.Equal(t, entries(input["requests"])[0]["owner_words"], "Owner request")
	testutil.Equal(t, entries(input["requests"])[0]["supplement"], "Investigated context")
	_, present := input["agreed_decisions"]
	testutil.Equal(t, present, false)
	testutil.Equal(t, input["returned"], []document{})
	testutil.Equal(t, must(record.Latest(ctx.Dir, "request"))["issue"], 42)
	must(prepare(ctx, a, "", "", ""))
	testutil.Equal(t, object(must(read(filepath.Join(a, "requirements/input.json")))["issue"])["number"], 42)
	testutil.Equal(t, len(must(record.List(ctx.Dir, "request"))), 1)
}

func TestPrepareRejectsUnreadyIssueBeforeWriting(t *testing.T) {
	for _, tc := range []struct{ state, label, blocker, reason string }{
		{"missing", "devkit:unit", "", "issue #42 could not be read"},
		{"CLOSED", "devkit:unit", "", "issue #42 is not open"},
		{"OPEN", "other", "", "issue #42 does not carry unit label"},
		{"OPEN", "devkit:unit", "open", "issue #42 is blocked by issue #8"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			repo, a := testutil.Repo(t), t.TempDir()
			testutil.IssueCLI(t, tc.state, tc.label, tc.blocker)
			ctx := record.ContextData{Cwd: repo, BaseBranch: "main", Dir: t.TempDir(), RunID: "r1"}
			run(ctx.Dir, ctx.RunID)
			testutil.Put(t, a, "requirements/input.json", "previous input\n")
			env := cli.Env{Cwd: repo, LookupEnv: func(k string) (string, bool) {
				if k == "INPUTS_ISSUE" {
					return "42", true
				}
				return os.LookupEnv(k)
			}}
			_, err := prepare(ctx, a, "New request", "", "", env)
			testutil.Fails(t, err, tc.reason)
			testutil.Equal(t, len(must(record.List(ctx.Dir, "request"))), 0)
			testutil.Equal(t, string(must(os.ReadFile(filepath.Join(a, "requirements/input.json")))), "previous input\n")
		})
	}
}
