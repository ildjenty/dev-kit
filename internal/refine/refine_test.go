package refine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func draft() doc {
	return doc{"pr_title": "Record employee access", "pr_summary": "Record the employee access decision on the base. This preserves the agreed audience for follow-up delivery.", "parent": doc{"title": "Serve employees", "body": "Who can use the service? Employees only."}, "topics": []doc{}, "summary": "A service for employees", "decisions": []doc{{"key": "N1", "title": "Serve employees", "body": "Compare all users with employees only, the simplest scope. Choose employees as requested.", "decided_by": "provisional", "depends_on": []string{}, "supersedes": []string{}, "source": "derived"}}, "units": []doc{{"key": "UN1", "title": "Deliver staff access", "goal": "Employees can sign in", "completion_conditions": []string{"An employee can sign in"}, "blocked_by": []string{}}}, "bindings": []doc{}}
}
func judgment() doc {
	cs := []doc{}
	for _, aspect := range aspects {
		cs = append(cs, doc{"aspect": aspect, "verdict": "pass", "checked": "Compared all materials", "not_checked": ""})
	}
	return doc{"criteria": cs, "findings": []doc{}}
}

type fixture struct {
	t                    *testing.T
	root, cwd, artifacts string
	ctx                  record.ContextData
}

func setup(t *testing.T, settings doc) *fixture {
	t.Helper()
	testutil.IssueCreationCLI(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "repo")
	testutil.OK(t, os.Mkdir(cwd, 0755))
	r := &fixture{t: t, root: root, cwd: cwd, artifacts: filepath.Join(root, "artifacts")}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.name", "Test")
	r.git("config", "user.email", "test@example.invalid")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "core.hooksPath", "/dev/null")
	c := doc{"language": "en"}
	for k, v := range settings {
		c[k] = v
	}
	r.put(".devkit/config.json", string(testutil.Must(json.Marshal(c))))
	standards := str(settings["standards"])
	if standards == "" {
		standards = ".devkit/standards"
	}
	testutil.Standards(t, cwd, standards)
	r.put("docs/topic.md", "# Topic\nExisting prose\n")
	r.commit()
	r.git("checkout", "-qb", "refine/topic")
	testutil.PublicationRemote(t, cwd)
	testutil.OK(t, os.MkdirAll(file(r.artifacts, "."), 0755))
	r.ctx = record.ContextData{Dir: record.RecordDir(filepath.Join(root, "state"), "refine/topic"), Cwd: cwd, RunID: "r1", Branch: "refine/topic", BaseBranch: "main"}
	return r
}
func (r *fixture) git(args ...string) string { r.t.Helper(); return testutil.Git(r.t, r.cwd, args...) }
func (r *fixture) put(path, text string)     { r.t.Helper(); testutil.Put(r.t, r.cwd, path, text) }
func (r *fixture) commit() {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-qm", "Set up test materials")
}
func (r *fixture) artifact(name string, v any) {
	r.t.Helper()
	if s, ok := v.(string); ok {
		testutil.OK(r.t, put(file(r.artifacts, name), s))
	} else {
		testutil.OK(r.t, writeJSON(file(r.artifacts, name), v))
	}
}
func envVars(vars map[string]string) cli.Env {
	return cli.Env{LookupEnv: func(k string) (string, bool) {
		v, ok := vars[k]
		if !ok && k == "DEVKIT_GH" {
			return os.LookupEnv(k)
		}
		return v, ok
	}}
}
func (r *fixture) begin(input ...string) doc {
	r.t.Helper()
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(r.t, e)
	testutil.Equal(r.t, s.Stopped, false)
	owner, supplement, memo := "A service for employees", "", ""
	if len(input) > 0 {
		owner = input[0]
	}
	if len(input) > 1 {
		supplement = input[1]
	}
	if len(input) > 2 {
		memo = input[2]
	}
	return testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, owner, supplement, memo))
}
func (r *fixture) writeDraft(ds ...doc) {
	r.t.Helper()
	d := draft()
	if len(ds) > 0 {
		d = ds[0]
	}
	r.artifact("draft.json", d)
	r.artifact("approval.txt", "Everything else follows the confirmed memo and owner's words. Details: devkit proposal show refine/topic.\nReply approve=1, or answers naming the key to change.")
}
func (r *fixture) decision(id string, fields doc) {
	r.t.Helper()
	d := doc{"id": id, "title": "An existing decision", "status": "agreed", "decided_by": "owner", "depends_on": []string{}, "supersedes": []string{}, "request": "refine/previous", "body": "Existing decision body"}
	for k, v := range fields {
		d[k] = v
	}
	r.put(".devkit/decisions/"+id+".md", decisionText(d))
}
func (r *fixture) errors(d doc) string {
	r.t.Helper()
	return strings.Join(testutil.Must(draftErrors(r.ctx, r.artifacts, d)), "\n")
}
func (r *fixture) round(j doc) doc {
	r.t.Helper()
	testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))
	v, e := round(r.ctx, r.artifacts, j)

	testutil.OK(r.t, e)
	return v
}
func (r *fixture) finish() doc {
	r.t.Helper()
	v, e := finish(r.ctx, r.artifacts)
	testutil.OK(r.t, e)
	return v
}
func (r *fixture) data() decisions.Data { r.t.Helper(); return testutil.Must(decisions.List(r.cwd)) }
func (r *fixture) latest(kind string) doc {
	r.t.Helper()
	return testutil.Must(record.Latest(r.ctx.Dir, kind))
}
func readText(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(path)
	testutil.OK(t, e)
	return string(b)
}
func noApprovalErrors(t *testing.T, ctx record.ContextData, v string) {
	t.Helper()
	errors, e := approvalErrors(ctx, v)
	testutil.OK(t, e)
	testutil.Equal(t, errors, []string{})
}
func rejectsApproval(t *testing.T, ctx record.ContextData, v string) {
	t.Helper()
	errors, e := approvalErrors(ctx, v)
	testutil.OK(t, e)
	if len(errors) == 0 {
		t.Fatal("expected approval errors")
	}
}

func TestPrepareIncludesFullLogAndOwnerSourcesWithoutRelevance(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.commit()
	out := r.begin("Owner words", "Session evidence", "Confirmed memo")
	input := testutil.Must(readJSON(str(out["input"])))
	testutil.Equal(t, items(input["requests"])[0]["owner_words"], "Owner words")
	testutil.Equal(t, items(input["requests"])[0]["supplement"], "Session evidence")
	testutil.Equal(t, input["memo"], "Confirmed memo")
	testutil.Equal(t, items(input["decisions"])[0]["body"], "Existing decision body")
	for _, key := range []string{"relevant_decisions", "documents", "units", "topics", "affected"} {
		if _, exists := input[key]; exists {
			t.Fatalf("obsolete input %s", key)
		}
	}
}

func TestProposalCommitContainsProposedDecisionsAndBindingOnly(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	before := r.git("rev-parse", "HEAD")
	d := draft()
	d["bindings"] = []doc{{"path": "CLAUDE.md", "decision": "N1", "reason": "Enforce the staff audience"}}
	r.put("CLAUDE.md", "Serve employees only.\n")
	r.writeDraft(d)
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))["valid"], true)
	head := r.git("rev-parse", "HEAD")
	testutil.Equal(t, r.git("rev-parse", "HEAD^"), before)
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts, cli.Env{Environ: os.Environ()}))["valid"], true)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.data().Decisions[0]["status"], "proposed")
	changed := strings.Split(r.git("show", "--format=", "--name-only", "HEAD"), "\n")
	testutil.Equal(t, len(changed), 2)
	testutil.Contains(t, strings.Join(changed, "\n"), "CLAUDE.md")
	testutil.Contains(t, strings.Join(changed, "\n"), ".devkit/decisions/D-")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	out := r.finish()
	testutil.Equal(t, out["outcome"], "question")
	testutil.Equal(t, str(items(out["questions"])[0]["text"]), readText(t, file(r.artifacts, "approval.txt")))
	details := testutil.Must(readJSON(filepath.Join(r.ctx.Dir, "proposals/1.details.json")))
	testutil.Equal(t, obj(details["content"]), obj(r.latest("proposal")["content"]))
	testutil.Equal(t, len(items(obj(obj(r.latest("proposal")["content"])["draft"])["units"])), 1)
	_, err := os.Stat(filepath.Join(log, "calls"))
	testutil.Equal(t, os.IsNotExist(err), true)
}

func TestApprovalCreatesParentSubIssuesDependenciesAndClosesOrComments(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, unsettled := range []bool{false, true} {
			t.Run(strconv.FormatBool(existing)+strconv.FormatBool(unsettled), func(t *testing.T) {
				r := setup(t, nil)
				log, _ := testutil.IssueCreationCLI(t)
				r.decision("D-000001", nil)
				r.commit()
				topic := ""
				if existing {
					topic = "7"
				}
				started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_TOPIC": topic})))
				testutil.Equal(t, started.Stopped, false)
				testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "Serve employees", "", "", topic))
				d := draft()
				if existing {
					d["parent"] = doc{"title": "", "body": ""}
				}
				items(d["decisions"])[0]["supersedes"] = []string{"D-000001"}
				second := clone(items(d["units"])[0])
				second["key"], second["blocked_by"] = "UN2", []string{"UN1"}
				d["units"] = append(items(d["units"]), second)
				if unsettled {
					d["topics"] = []doc{{"key": "TN1", "title": "Choose capacity", "body": "How many employees? A settled answer gives a number.", "blocked_by": []string{}}, {"key": "TN2", "title": "Choose hosting", "body": "Which hosting supports that capacity?", "blocked_by": []string{"TN1"}}}
				}
				r.writeDraft(d)
				r.artifact("approval.txt", "N1 UN1 UN2 TN1 TN2 approve=1 or answers; parent and blockers in the draft.")
				testutil.Equal(t, r.round(judgment())["valid"], true)
				r.finish()
				proposed := r.git("rev-parse", "HEAD")
				_, err := os.Stat(filepath.Join(log, "count"))
				testutil.Equal(t, os.IsNotExist(err), true)
				r.ctx.RunID = "r2"
				testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
				approved := testutil.Must(approve(r.ctx, "1"))
				testutil.Equal(t, approved["outcome"], "pass")
				testutil.Equal(t, r.git("rev-parse", "HEAD^"), proposed)
				testutil.Equal(t, find(r.data().Decisions, "D-000001")["status"], "superseded")
				testutil.Equal(t, r.data().Decisions[1]["status"], "agreed")
				calls := readText(t, filepath.Join(log, "calls"))
				testutil.Equal(t, strings.Count(calls, "--field sub_issue_id="), 2)
				testutil.Contains(t, calls, "dependencies/blocked_by --header")
				if existing {
					testutil.Equal(t, strings.Count(calls, "issue create --title Serve employees"), 0)
				} else {
					testutil.Contains(t, calls, "issue create --title Serve employees")
				}
				if unsettled {
					testutil.Contains(t, calls, "issue comment")
					testutil.Equal(t, strings.Count(calls, "issue close"), 0)
				} else {
					testutil.Contains(t, calls, "issue close")
				}
				testutil.Contains(t, calls, "devkit:unit")
				testutil.Equal(t, len(items(r.latest("result")["questions"])), 0)
				testutil.Equal(t, r.git("status", "--porcelain"), "")
			})
		}
	}
}

func TestApprovalRetryCreatesOnlyMissingIssuesAfterPartialFailure(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	d := draft()
	second := clone(items(d["units"])[0])
	second["key"], second["title"], second["blocked_by"] = "UN2", "Second unit", []string{"UN1"}
	d["units"] = append(items(d["units"]), second)
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 UN1 UN2 approve=1 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	r.finish()
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Put(t, log, "fail", "3")
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "gh issue create failed")
	testutil.Put(t, log, "pr-title", "Stale PR title")
	testutil.Put(t, log, "pr-body", "Stale PR body")
	head := r.git("rev-parse", "HEAD")
	testutil.Equal(t, r.data().Decisions[0]["status"], "agreed")
	progress := testutil.Must(issues.ReadCreation(creationPath(r.ctx, "1")))
	testutil.Equal(t, progress.RunID, "r2")
	testutil.Equal(t, progress.Parent.Number, 101)
	testutil.Equal(t, progress.Created["UN1"].Number, 102)
	testutil.Equal(t, len(progress.Created), 1)
	testutil.OK(t, os.Remove(filepath.Join(log, "fail")))
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	calls := readText(t, filepath.Join(log, "calls"))
	testutil.Equal(t, strings.Count(calls, "issue create --title Serve employees"), 1)
	testutil.Equal(t, strings.Count(calls, "issue create --title Deliver staff access"), 1)
	testutil.Equal(t, readText(t, filepath.Join(log, "count")), "3")
	testutil.Equal(t, r.latest("result")["issue_numbers"], []any{float64(101), float64(102), float64(103)})
	testutil.Equal(t, strings.Count(calls, "pr create"), 1)
	testutil.Equal(t, strings.Index(calls, "pr create") < strings.Index(calls, "issue create"), true)
	testutil.Contains(t, readText(t, filepath.Join(log, "bodies")), "Source: #80")
	testutil.Contains(t, readText(t, filepath.Join(log, "bodies")), "\nsource_pr: 80 -->")
	testutil.Contains(t, readText(t, filepath.Join(log, "pr-bodies")), "#102 Deliver staff access")
	testutil.Equal(t, r.latest("result")["pr_number"], float64(80))
	testutil.Equal(t, readText(t, filepath.Join(log, "pr-title")), str(d["pr_title"]))
	body := readText(t, filepath.Join(log, "pr-body"))
	testutil.Equal(t, strings.HasPrefix(body, str(d["pr_summary"])+"\n\n"), true)
	testutil.Equal(t, strings.Contains(body, "Stale PR body"), false)
	testutil.Equal(t, strings.Contains(body, "approve="), false)
	testutil.Equal(t, strings.Count(calls, "pr edit"), 2)
}

func TestApprovalPushFailureStopsBeforeIssuesAndCanResume(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	remote := r.git("remote", "get-url", "origin")
	testutil.Put(t, remote, "hooks/pre-receive", "#!/bin/sh\nexit 1\n")
	testutil.OK(t, os.Chmod(filepath.Join(remote, "hooks/pre-receive"), 0755))
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "git push")
	testutil.Equal(t, readText(t, filepath.Join(log, "calls")), "repo view --json url --jq .url\n")
	testutil.Equal(t, r.data().Decisions[0]["status"], "agreed")
	testutil.OK(t, os.Remove(filepath.Join(remote, "hooks/pre-receive")))
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["pr_number"], 80)
}

func TestApprovalReusesPRAfterLostCreateResponse(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Put(t, log, "pr-lost", "1")
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "gh pr create")
	_, err = os.Stat(filepath.Join(log, "count"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.OK(t, os.Remove(filepath.Join(log, "pr-lost")))
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["pr_number"], 80)
	testutil.Equal(t, strings.Count(readText(t, filepath.Join(log, "calls")), "pr create"), 1)
}

func TestApprovalWithNothingToPublishStillCreatesIssues(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	d := draft()
	d["decisions"] = []doc{}
	r.writeDraft(d)
	r.round(judgment())
	r.finish()
	r.git("branch", "-f", "main", "HEAD")
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	result := testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, result["publication"], "nothing_to_publish")
	testutil.Equal(t, result["pr_number"], 0)
	testutil.Equal(t, len(result["issue_numbers"].([]int)), 2)
	testutil.Equal(t, strings.Contains(readText(t, filepath.Join(log, "calls")), "pr "), false)
}

func TestApprovalRejectsUnrecordedAnswerChangedHeadOrDirtyCheckout(t *testing.T) {
	for _, change := range []string{"no-start", "dirty", "commit"} {
		t.Run(change, func(t *testing.T) {
			r := setup(t, nil)
			log, _ := testutil.IssueCreationCLI(t)
			r.begin()
			r.writeDraft()
			r.round(judgment())
			r.finish()
			r.ctx.RunID = "r2"
			if change != "no-start" {
				testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
				r.put("owner.txt", "Owner work")
				if change == "commit" {
					r.commit()
				}
			}
			_, err := approve(r.ctx, "1")
			if err == nil {
				t.Fatal("unapproved checkout accepted")
			}
			testutil.Equal(t, r.data().Decisions[0]["status"], "proposed")
			_, err = os.Stat(filepath.Join(log, "calls"))
			testutil.Equal(t, os.IsNotExist(err), true)
		})
	}
}

func TestDraftGatesRejectCyclesMissingReferencesUntracedEditsAndOwnerChoice(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	items(d["decisions"])[0]["depends_on"] = []string{"N1", "D-ffffff"}
	items(d["units"])[0]["blocked_by"] = []string{"UN1", "missing"}
	r.writeDraft(d)
	r.put("docs/topic.md", "Not a binding")
	problems := r.errors(d)
	for _, part := range []string{"cycle", "missing dependency", "missing blocker", "outside the traced"} {
		testutil.Contains(t, problems, part)
	}
	items(d["decisions"])[0]["decided_by"] = "owner"
	testutil.Contains(t, r.errors(d), "decided_by must be one of the allowed values")
}

func TestFreshJudgeUnassessedAndPostReviewChangesCannotPass(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	j := judgment()
	items(j["criteria"])[0]["verdict"] = "not_assessed"
	testutil.Equal(t, r.round(j)["not_assessed_count"], 1)
	testutil.Equal(t, r.finish()["outcome"], "stop")
	r = setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.put("AGENTS.md", "Changed after judgment")
	testutil.Equal(t, r.finish()["outcome"], "stop")
}

func TestRevisionKeepsDecisionIdsAndWithdrawsOmittedProposedFiles(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	first := r.data().Decisions[0]["id"]
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Revise"}]`}))).Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	d := draft()
	items(d["decisions"])[0]["title"] = "Revised audience"
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	testutil.Equal(t, r.finish()["proposal_version"], 2)
	testutil.Equal(t, r.data().Decisions[0]["id"], first)
	testutil.Equal(t, r.data().Decisions[0]["title"], "Revised audience")
}

func TestPartialApprovalCanResumeInAnotherRun(t *testing.T) {
	r := setup(t, nil)
	log, _ := testutil.IssueCreationCLI(t)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Put(t, log, "fail", "2")
	_, err := approve(r.ctx, "1")
	testutil.Fails(t, err, "gh issue create failed")
	r.ctx.RunID = "r3"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.OK(t, os.Remove(filepath.Join(log, "fail")))
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
	testutil.Equal(t, readText(t, filepath.Join(log, "count")), "2")
}

func TestInvalidTopicStopsBeforeStartAndDirectDecisionEditsFailGate(t *testing.T) {
	r := setup(t, nil)
	started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_TOPIC": "T-abcdef"})))
	testutil.Equal(t, started.Stopped, true)
	testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "run")), []doc{})
	r.decision("D-abcdef", nil)
	r.commit()
	r.begin()
	r.writeDraft()
	r.decision("D-abcdef", doc{"body": "Changed the log directly"})
	testutil.Contains(t, r.errors(draft()), "writer edited decision records directly")
}

func TestAnotherJudgmentRoundReusesIdsAndWithdrawsProposedDecisions(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	j := judgment()
	items(j["criteria"])[0]["verdict"] = "fail"
	r.round(j)
	first := str(r.data().Decisions[0]["id"])
	d := draft()
	items(d["decisions"])[0]["title"] = "Revised within the same run"
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	testutil.Equal(t, len(r.data().Decisions), 1)
	testutil.Equal(t, r.data().Decisions[0]["id"], first)
	testutil.Equal(t, r.finish()["proposal_version"], 2)
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": `[{"question_id":"Q1","answer":"Withdraw the decision log entry"}]`}))).Stopped, false)
	r.artifacts = t.TempDir()
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	d = draft()
	d["decisions"] = []doc{}
	r.writeDraft(d)
	r.artifact("approval.txt", "UN1 approve=3 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	testutil.Equal(t, len(r.data().Decisions), 0)
	testutil.Equal(t, r.finish()["proposal_version"], 3)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}
