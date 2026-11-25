package refine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func ownerEnv(t *testing.T, mode string, ds ...doc) cli.Env {
	t.Helper()
	return envVars(map[string]string{"INPUTS_AI": mode, "INPUTS_OWNER_DECISIONS": string(testutil.Must(json.Marshal(ds)))})
}

func ownerDecision() doc {
	return doc{"title": "Keep the owner's exact choice", "body": "  Keep this wording.\nNo rewrite, even here.  \n", "owner_words": "Keep this wording.\nNo rewrite, even here."}
}

func TestOwnerDecisionsAreCommittedVerbatimAsAgreedBeforeWriter(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-abcdef", nil)
	r.commit()
	oldPath := filepath.Join(r.cwd, ".devkit/decisions/D-abcdef.md")
	before := readText(t, oldPath)
	given := ownerDecision()
	given["title"] = "  Exact title: \"決定\"  "
	given["supersedes"], given["depends_on"] = []string{"D-abcdef"}, []string{"D-abcdef"}
	env := ownerEnv(t, "additions", given)
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	base := r.git("rev-parse", "HEAD")
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Equal(t, input["owner_decisions_fixed"], true)
	fixed := items(input["fixed_owner_decisions"])
	testutil.Equal(t, len(fixed), 1)
	d := fixed[0]
	testutil.Equal(t, d["status"], "agreed")
	testutil.Equal(t, d["decided_by"], "owner")
	testutil.Equal(t, d["provenance"], "owner")
	testutil.Equal(t, d["title"], given["title"])
	text := readText(t, filepath.Join(r.cwd, str(d["path"])))
	testutil.Contains(t, text, "---\n"+str(given["body"])+"\n\nOwner words: "+string(testutil.Must(json.Marshal(given["owner_words"]))))
	testutil.Equal(t, readText(t, oldPath), strings.Replace(before, `status: "agreed"`, `status: "superseded"`, 1))
	testutil.Equal(t, r.git("show", "HEAD:"+str(d["path"])), strings.TrimRight(text, "\n"))
	testutil.Equal(t, base == r.git("rev-parse", "HEAD"), false)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	head := r.git("rev-parse", "HEAD")
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, len(r.data().Decisions), 2)
	r.writeDraft()
	testutil.Equal(t, r.round(judgment())["valid"], true)
	review := testutil.Must(readJSON(file(r.artifacts, "review.json")))
	testutil.Equal(t, slices.Contains(stringsOf(review["decision_ids"]), str(d["id"])), false)
	testutil.Equal(t, len(items(obj(review["draft"])["decisions"])), 1)
	testutil.Equal(t, strings.Contains(string(testutil.Must(json.Marshal(review))), str(given["title"])), false)
	result := r.finish()
	testutil.Equal(t, result["outcome"], "question")
	testutil.Equal(t, strings.Contains(str(items(result["questions"])[0]["text"]), str(given["title"])), false)
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, str(d["path"]))), text)
	r.ctx.RunID, r.artifacts = "r2", t.TempDir()
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Revise the AI additions"}]`}))).Stopped, false)
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	testutil.Equal(t, len(items(testutil.Must(readJSON(file(r.artifacts, "input.json")))["fixed_owner_decisions"])), 1)
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, str(d["path"]))), text)
}

func TestOwnerRecordsStayFixedThroughApprovalRetry(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	env := ownerEnv(t, "additions", ownerDecision())
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	owner := r.data().Decisions[0]
	before := readText(t, filepath.Join(r.cwd, str(owner["path"])))
	r.writeDraft()
	testutil.Equal(t, r.round(judgment())["valid"], true)
	r.finish()
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Put(t, log, "fail", "2")
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "gh issue create failed")
	testutil.OK(t, os.Remove(filepath.Join(log, "fail")))
	r.ctx.RunID = "r3"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	result := testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, result["outcome"], "pass")
	testutil.Equal(t, len(stringsOf(result["decision_ids"])), 2)
	testutil.Equal(t, len(r.data().Decisions), 2)
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, str(owner["path"]))), before)
	testutil.Contains(t, readText(t, filepath.Join(log, "pr-bodies")), str(owner["title"]))
}

func TestOwnerSupersessionPreservesOriginalBodyAndProvenance(t *testing.T) {
	r := setup(t, nil)
	first := ownerDecision()
	env := ownerEnv(t, "additions", first)
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	old := r.data().Decisions[0]
	path := filepath.Join(r.cwd, str(old["path"]))
	before := readText(t, path)
	second := doc{"title": "The owner changed the choice", "body": "  The new confirmed wording.\n", "owner_words": "The new confirmed wording.", "supersedes": []string{str(old["id"])}}
	env = ownerEnv(t, "additions", second)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	testutil.Equal(t, readText(t, path), strings.Replace(before, `status: "agreed"`, `status: "superseded"`, 1))
	head := r.git("rev-parse", "HEAD")
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, len(r.data().Decisions), 2)
}

func TestNoAIRecordsAndPublishesWithoutDraftJudgmentOrApproval(t *testing.T) {
	for _, language := range []string{"en", "ja-JP"} {
		t.Run(language, func(t *testing.T) {
			r := setup(t, doc{"language": language})
			log, _ := testutil.IssueCreationCLI(t)
			given := ownerDecision()
			given["title"] = strings.Repeat("界", 120)
			env := ownerEnv(t, "none", given)
			testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
			testutil.Equal(t, testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))["ai"], "none")
			result := testutil.Must(publishOwner(r.ctx, r.artifacts, cli.Env{Cwd: r.cwd}))
			testutil.Equal(t, result["outcome"], "pass")
			testutil.Equal(t, result["pr_number"], 80)
			testutil.Equal(t, result["questions"], []doc{})
			testutil.Equal(t, len(stringsOf(result["decision_ids"])), 1)
			testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "judgment")), []doc{})
			for _, name := range []string{"draft.json", "review.json", "approval.txt", "rounds/latest.json"} {
				_, err := os.Stat(file(r.artifacts, name))
				testutil.Equal(t, os.IsNotExist(err), true)
			}
			calls := readText(t, filepath.Join(log, "calls"))
			testutil.Equal(t, strings.Contains(calls, "issue create"), false)
			testutil.Contains(t, readText(t, filepath.Join(log, "pr-bodies")), str(given["title"]))
			head := r.git("rev-parse", "HEAD")
			r.ctx.RunID, r.artifacts = "r2", t.TempDir()
			testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
			testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
			testutil.Equal(t, testutil.Must(publishOwner(r.ctx, r.artifacts, cli.Env{Cwd: r.cwd}))["outcome"], "pass")
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
			testutil.Equal(t, len(r.data().Decisions), 1)
			testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "proposal"))), 1)
		})
	}
}

func TestInvalidOwnerInputIsRejectedWithoutDecisionWrites(t *testing.T) {
	for _, raw := range []string{
		`null`, `{}`, `[null]`, `[{}]`, `[{"title":"x","body":"y","owner_words":"z","extra":true}]`,
		`[{"title":1,"body":"y","owner_words":"z"}]`, `[{"title":"x","body":"　","owner_words":"z"}]`,
		`[{"title":"x","body":"y","owner_words":""}]`, `[{"title":"x","body":"y","owner_words":"z","depends_on":null}]`,
		`[{"title":"x","body":"y","owner_words":"z","supersedes":["N1"]}]`,
		`[{"title":"x","body":"y","owner_words":"z","depends_on":["D-abcdef","D-abcdef"]}]`,
		`[{"title":"x","body":"y","owner_words":"z"},{"title":"x","body":"y","owner_words":"z"}]`, `[] trailing`,
	} {
		t.Run(raw, func(t *testing.T) {
			r := setup(t, nil)
			head := r.git("rev-parse", "HEAD")
			env := envVars(map[string]string{"INPUTS_OWNER_DECISIONS": raw})
			testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, true)
			_, err := prepare(r.ctx, env, r.artifacts, "", "", "")
			if err == nil {
				t.Fatal("invalid owner input accepted")
			}
			testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
			testutil.Equal(t, len(r.data().Decisions), 0)
		})
	}
	for _, field := range []string{"depends_on", "supersedes"} {
		r := setup(t, nil)
		given := ownerDecision()
		given[field] = []string{"D-ffffff"}
		env := ownerEnv(t, "additions", given)
		testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
		_, err := prepare(r.ctx, env, r.artifacts, "", "", "")
		testutil.Fails(t, err, "D-ffffff")
		testutil.Equal(t, len(r.data().Decisions), 0)
		testutil.Equal(t, r.git("status", "--porcelain"), "")
	}
}

func TestNoAIRetryReusesRecordsProposalAndLostPRResponse(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	second := ownerDecision()
	second["title"], second["body"] = "A second choice", "Keep this second wording."
	env := ownerEnv(t, "none", ownerDecision(), second)
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	testutil.Put(t, log, "pr-lost", "1")
	_, err := publishOwner(r.ctx, r.artifacts, cli.Env{Cwd: r.cwd})
	testutil.Fails(t, err, "gh pr create")
	head := r.git("rev-parse", "HEAD")
	testutil.OK(t, os.Remove(filepath.Join(log, "pr-lost")))
	r.ctx.RunID, r.artifacts = "r2", t.TempDir()
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	testutil.Equal(t, testutil.Must(publishOwner(r.ctx, r.artifacts, cli.Env{Cwd: r.cwd}))["outcome"], "pass")
	testutil.Equal(t, len(r.data().Decisions), 2)
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "proposal"))), 1)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, strings.Count(readText(t, filepath.Join(log, "calls")), "pr create"), 1)
}

func TestOwnerDecisionCannotBeEditedOrRedecidedByAI(t *testing.T) {
	r := setup(t, nil)
	env := ownerEnv(t, "additions", ownerDecision())
	testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, false)
	testutil.Must(prepare(r.ctx, env, r.artifacts, "", "", ""))
	owner := r.data().Decisions[0]
	d := draft()
	items(d["decisions"])[0]["supersedes"] = []string{str(owner["id"])}
	r.writeDraft(d)
	testutil.Contains(t, r.errors(d), "cannot re-decide fixed owner decision")
	r.put(str(owner["path"]), decisionText(owner)+"Edited by writer.\n")
	testutil.Contains(t, r.errors(d), "writer edited decision records directly")
}

func TestNoAIRequiresExplicitValidSignalAndCannotPublishPendingAIChanges(t *testing.T) {
	for _, env := range []cli.Env{envVars(map[string]string{"INPUTS_AI": "none"}), ownerEnv(t, "auto", ownerDecision())} {
		r := setup(t, nil)
		testutil.Equal(t, testutil.Must(start(r.ctx, env)).Stopped, true)
	}
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	_, err := prepare(r.ctx, ownerEnv(t, "none", ownerDecision()), r.artifacts, "", "", "")
	testutil.Fails(t, err, "pending AI proposal")
	testutil.Equal(t, r.data().Decisions[0]["status"], "proposed")
	p := r.latest("proposal")
	testutil.Equal(t, len(items(obj(p["content"])["owner_decisions"])), 0)
}
