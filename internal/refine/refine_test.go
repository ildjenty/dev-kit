package refine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func draft() doc {
	return doc{"topics": []doc{}, "kept_units": []doc{}, "summary": "A service for employees", "decisions": []doc{{"key": "N1", "title": "Serve employees", "body": "Compare all users with employees only, the simplest scope. Choose employees as requested.", "decided_by": "owner", "depends_on": []string{}, "supersedes": []string{}, "source": "owner_words"}}, "rebased": []doc{}, "kept_docs": []doc{}, "units": []doc{{"key": "UN1", "title": "Deliver staff access", "goal": "Employees can sign in", "completion_conditions": []string{"An employee can sign in"}, "decisions": []string{"N1"}, "depends_on": []string{}}}, "open_points": []doc{}}
}
func judgmentFor(phase string) doc {
	cs := []doc{}
	for _, a := range phaseAspects(phase) {
		cs = append(cs, doc{"aspect": a, "verdict": "pass", "checked": "Compared all materials", "not_checked": ""})
	}
	return doc{"criteria": cs, "findings": []doc{}}
}

func judgment() doc { return judgmentFor("document") }

type fixture struct {
	t                    *testing.T
	root, cwd, artifacts string
	ctx                  record.ContextData
}

func setup(t *testing.T, settings doc) *fixture {
	t.Helper()
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
	return cli.Env{LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
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
	if len(array(d["open_points"])) == 0 {
		testutil.OK(r.t, restoreDocs(r.ctx, r.artifacts, "docs"))
		decide := clone(d)
		decide["units"], decide["kept_docs"], decide["kept_units"] = []doc{}, []doc{}, []doc{}
		r.artifact("draft.json", decide)
		testutil.Must(round(r.ctx, r.artifacts, judgmentFor("decide"), "decide"))
		testutil.Must(decided(r.ctx, r.artifacts))
		body := "# Refined decisions\n"
		for _, decision := range items(d["decisions"]) {
			body += str(decision["key"]) + "\n"
		}
		r.put("docs/refined.md", body)
	}
	r.artifact("draft.json", d)
	r.artifact("approval.txt", "Owner decisions: N1 Serve employees. Provisional decisions: none. Unit UN1 Deliver access. Start refine with approve=1, or answers to request changes.")
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
	phase := "document"
	d, _ := readDraft(r.artifacts)
	if len(array(obj(d)["open_points"])) > 0 {
		phase = "decide"
		j = judgmentFor("decide")
	}
	v, e := round(r.ctx, r.artifacts, j, phase)
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

func TestPrepareSeparatesRequestsIncludesMemoAllDecisionsFoldersAndPinnedCriteria(t *testing.T) {
	r := setup(t, doc{"standards": "standards", "docs": "notes"})
	r.decision("D-000001", nil)
	r.commit()
	out := r.begin("Owner words", "Session candidate", "Confirmed discussion")
	input := testutil.Must(readJSON(str(out["input"])))
	request := items(input["requests"])[0]
	testutil.Equal(t, request["owner_words"], "Owner words")
	testutil.Equal(t, request["supplement"], "Session candidate")
	testutil.Equal(t, input["memo"], "Confirmed discussion")
	testutil.Equal(t, items(input["relevant_decisions"])[0]["body"], "Existing decision body")
	testutil.Equal(t, input["language"], "en")
	testutil.Equal(t, obj(input["folders"])["docs"], "notes")
	if !strings.HasSuffix(str(input["criteria"]), "/standards") {
		t.Fatal(input["criteria"])
	}
	testutil.Equal(t, input["previous"], nil)
}
func TestOpenPointsRecordOneQuestionEachPreserveDraftDocumentsAndAllowCleanAnswerRestart(t *testing.T) {
	r := setup(t, nil)
	r.begin("Serve employees", "", "Confirmed memo")
	d := draft()
	d["units"] = []doc{}
	d["open_points"] = []doc{{"id": "O1", "text": "How many employees?"}, {"id": "O2", "text": "Which provider? Candidates: local or hosted. Recommend local for simplicity."}}
	r.writeDraft(d)
	testutil.Equal(t, r.round(judgment())["valid"], true)
	before := r.git("rev-parse", "HEAD")
	out := r.finish()
	qs := items(out["questions"])
	testutil.Equal(t, out["outcome"], "question")
	testutil.Equal(t, len(qs), 2)
	testutil.Equal(t, r.git("rev-parse", "HEAD"), before)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	testutil.Equal(t, obj(obj(r.latest("proposal")["content"])["documents"]), doc{})
	testutil.Equal(t, len(testutil.Must(approvalErrors(r.ctx, "1"))), 1)
	r.ctx.RunID = "r2"
	answers := []doc{}
	for _, q := range qs {
		answers = append(answers, doc{"question_id": q["id"], "answer": "Owner answer"})
	}
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": string(testutil.Must(json.Marshal(answers)))}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	nextArtifacts := filepath.Join(r.root, "next")
	testutil.Must(prepare(r.ctx, envVars(nil), nextArtifacts, "", "", ""))
	next := testutil.Must(readJSON(file(nextArtifacts, "input.json")))
	testutil.Equal(t, items(next["answers"])[0]["question"], "How many employees?")
	testutil.Equal(t, next["memo"], "Confirmed memo")
}
func TestMaterializesProposalIDsThenApprovalAgreesSuccessorsAndReportsAffectedUnits(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.put(".devkit/units/U-000001.md", "---\nid: U-000001\ntitle: Old unit\ndecisions: [D-000001]\ndepends_on: []\n---\nOld goal\n\n- Old completion condition\n")
	r.put(".devkit/units/U-000001.done.json", `{"unit":"U-000001","branch":"feature/previous"}`)
	r.commit()
	r.begin("Serve employees", "", "Confirmed memo")
	d := draft()
	items(d["decisions"])[0]["supersedes"] = []string{"D-000001"}
	d["kept_units"] = []doc{{"id": "U-000001", "reason": "The historical unit will be replaced by UN1"}}
	r.writeDraft(d)
	r.put("docs/topic.md", "# Topic\nN1\n")
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts)), doc{"valid": true, "errors": []string{}})
	r.round(judgment())
	out := r.finish()
	qs := items(out["questions"])
	testutil.Equal(t, out["outcome"], "question")
	testutil.Equal(t, out["decision_ids"], []string{testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{}))})
	testutil.Equal(t, out["unit_ids"], []string{testutil.Must(decisions.NextID("unit", r.ctx.Branch, "UN1", decisions.Data{}))})
	testutil.Contains(t, str(qs[0]["text"]), testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{})))
	testutil.Contains(t, str(qs[0]["text"]), testutil.Must(decisions.NextID("unit", r.ctx.Branch, "UN1", decisions.Data{})))
	testutil.Contains(t, readText(t, filepath.Join(r.cwd, "docs/topic.md")), testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{})))
	testutil.Equal(t, find(r.data().Decisions, testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{})))["status"], "proposed")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	if !strings.HasPrefix(r.git("log", "-1", "--format=%s"), "docs:") {
		t.Fatal("missing docs commit")
	}
	noApprovalErrors(t, r.ctx, "1")
	rejectsApproval(t, r.ctx, "2")
	r.ctx.RunID = "r2"
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	agreed, e := approve(r.ctx, "1")
	testutil.OK(t, e)
	testutil.Equal(t, agreed["outcome"], "pass")
	testutil.Equal(t, agreed["decision_ids"], []string{testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{}))})
	testutil.Equal(t, agreed["affected"].(decisions.Data).Units[0]["id"], "U-000001")
	testutil.Equal(t, find(r.data().Decisions, "D-000001")["status"], "superseded")
	testutil.Equal(t, find(r.data().Decisions, testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{})))["status"], "agreed")
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "proposal"))), 1)
	testutil.Equal(t, r.latest("result")["outcome"], "pass")
	testutil.Equal(t, r.latest("result")["decision_ids"], []string{testutil.Must(decisions.NextID("decision", r.ctx.Branch, "N1", decisions.Data{}))})
	testutil.Equal(t, items(obj(r.latest("result")["affected"])["units"])[0]["id"], "U-000001")
	testutil.Equal(t, r.latest("judgment")["judge"], "refine")
	testutil.Equal(t, r.latest("judgment")["target"], doc{"proposal_version": 1})
	rejectsApproval(t, r.ctx, "1")
}
func TestMachineCheckRejectsCyclesMissingCitationsEmptyUnitDecisionsAndWritesOutsideDocs(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	items(d["decisions"])[0]["depends_on"] = []string{"N1", "D-009999"}
	items(d["units"])[0]["decisions"] = []string{}
	r.writeDraft(d)
	r.put("outside.md", "Changed")
	r.put("docs/topic.md", "D-008888 N99")
	testutil.Contains(t, r.errors(d), "at least 1")
	items(d["units"])[0]["decisions"] = []string{"N1"}
	errors := r.errors(d)
	for _, part := range []string{"cycle", "D-009999", "outside the docs", "D-008888", "N99"} {
		testutil.Contains(t, errors, part)
	}
}
func TestAffectedDecisionsRequireConsistentRebaseAndDocumentsEditOrKeptReason(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", doc{"status": "superseded"})
	r.decision("D-000002", doc{"supersedes": []string{"D-000001"}})
	r.decision("D-000003", doc{"depends_on": []string{"D-000001"}})
	r.put("docs/topic.md", "# Topic\nHistory D-000001\n")
	r.commit()
	r.begin()
	d := draft()
	r.writeDraft(d)
	errors := r.errors(d)
	testutil.Contains(t, errors, "neither superseded nor consistently rebased")
	testutil.Contains(t, errors, "not edited or kept")
	d["rebased"] = []doc{{"id": "D-000003", "from": "D-000001", "to": "D-000002", "reason": "The dependency remains substantively identical"}}
	d["kept_docs"] = []doc{{"path": "docs/topic.md", "reason": "A historical account explicitly cites the past decision"}}
	r.writeDraft(d)
	testutil.Equal(t, testutil.Must(draftErrors(r.ctx, r.artifacts, d)), []string{})
	r.round(judgment())
	r.finish()
	testutil.Equal(t, find(r.data().Decisions, "D-000003")["depends_on"], []string{"D-000002"})
}
func TestProspectiveSupersessionIncludedInAffectedItemChecks(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.decision("D-000002", doc{"depends_on": []string{"D-000001"}})
	r.put("docs/topic.md", "D-000001")
	r.commit()
	r.begin()
	d := draft()
	items(d["decisions"])[0]["supersedes"] = []string{"D-000001"}
	r.writeDraft(d)
	errors := r.errors(d)
	testutil.Contains(t, errors, "D-000002: affected")
	testutil.Contains(t, errors, "docs/topic.md: affected")
}
func TestMissingJudgeAspectsUnassessedCriteriaAndFailedVerdictsCannotClearRound(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	j := judgment()
	j["criteria"] = []doc{}
	testutil.Equal(t, r.round(j)["valid"], false)
	j = judgment()
	items(j["criteria"])[0]["verdict"] = "not_assessed"
	testutil.Equal(t, r.round(j)["not_assessed_count"], 1)
	j = judgment()
	items(j["criteria"])[0]["verdict"] = "fail"
	testutil.Equal(t, r.round(j)["blocking_count"], 1)
	before := r.git("rev-parse", "HEAD")
	r.put("docs/topic.md", "Draft N1")
	out := r.finish()
	testutil.Equal(t, out["outcome"], "stop")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), before)
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}
func TestBranchAndApprovalValidationStopBeforeRecordingOrEditing(t *testing.T) {
	r := setup(t, nil)
	r.ctx.Branch = "feature/topic"
	s, e := start(r.ctx, envVars(nil))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	testutil.Equal(t, testutil.Must(record.List(r.ctx.Dir, "run")), []doc{})
	r.ctx.Branch = "refine/topic"
	s, e = start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, true)
	_, e = os.Stat(r.ctx.Dir)
	if !os.IsNotExist(e) {
		t.Fatalf("record directory exists: %v", e)
	}
}
func TestMissingOrMalformedDraftsFailMachineGateAndFinishWithStop(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts))["valid"], false)
	r.artifact("draft.json", "{invalid JSON")
	testutil.Equal(t, r.round(judgment())["valid"], false)
	out := r.finish()
	testutil.Equal(t, out["outcome"], "stop")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}
func TestWriterCannotDirectlyModifyDecisionRecordsEvenUnderDocs(t *testing.T) {
	r := setup(t, doc{"decisions": "docs/decisions"})
	r.decision("D-000001", nil)
	r.put("docs/decisions/D-000001.md", readText(t, filepath.Join(r.cwd, ".devkit/decisions/D-000001.md")))
	r.commit()
	r.begin()
	r.writeDraft()
	r.put("docs/decisions/D-000001.md", strings.ReplaceAll(readText(t, filepath.Join(r.cwd, "docs/decisions/D-000001.md")), "Existing decision body", "Changed agreed meaning"))
	testutil.Contains(t, r.errors(draft()), "writer edited decision records directly")
}
func TestWriterCannotEditStandardsEvenWhenTheyLieUnderDocs(t *testing.T) {
	for _, folder := range []string{".devkit/standards", "docs/standards"} {
		for _, staged := range []bool{false, true} {
			t.Run(folder+fmt.Sprint(staged), func(t *testing.T) {
				r := setup(t, doc{"standards": folder})
				r.begin()
				r.writeDraft()
				path := folder + "/designing.md"
				original := readText(t, filepath.Join(r.cwd, path))
				r.put(path, original+"\nA weakened rule\n")
				if staged {
					r.git("add", path)
					r.put(path, original)
				}
				want := "edited a standards file"
				if folder == ".devkit/standards" {
					want = "edited outside the docs folder"
				}
				testutil.Contains(t, r.errors(draft()), want)
			})
		}
	}
}
func TestWriterCannotEditReservedRecordFoldersUnderDocs(t *testing.T) {
	for _, kind := range []string{"decisions", "units", "topics"} {
		t.Run(kind, func(t *testing.T) {
			folder := "docs/" + kind
			r := setup(t, doc{kind: folder})
			r.put(folder+"/extra.md", "Existing text\n")
			r.commit()
			r.begin()
			r.writeDraft()
			r.put(folder+"/extra.md", "Writer changed a reserved file\n")
			testutil.Contains(t, r.errors(draft()), "edited a "+kind+" file")
		})
	}
}
func TestApprovalRefusesChangedHEADAndCorrectionsReuseProposedIDsWithoutEditingAgreedRecords(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	out := r.finish()
	head := r.git("rev-parse", "HEAD")
	r.put("docs/extra.md", "Another commit")
	r.commit()
	testutil.Contains(t, strings.Join(testutil.Must(approvalErrors(r.ctx, "1")), "\n"), "HEAD changed")
	r.git("reset", "--hard", head)
	r.ctx.RunID = "r2"
	answers := []doc{{"question_id": items(out["questions"])[0]["id"], "answer": "Change the title"}}
	s, e := start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": string(testutil.Must(json.Marshal(answers)))}))
	testutil.OK(t, e)
	testutil.Equal(t, s.Stopped, false)
	r.artifacts = filepath.Join(r.root, "correction")
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	d := draft()
	items(d["decisions"])[0]["title"] = "Corrected staff scope"
	if len(array(d["open_points"])) == 0 {
		testutil.OK(r.t, restoreDocs(r.ctx, r.artifacts, "docs"))
		decide := clone(d)
		decide["units"], decide["kept_docs"], decide["kept_units"] = []doc{}, []doc{}, []doc{}
		r.artifact("draft.json", decide)
		testutil.Must(round(r.ctx, r.artifacts, judgmentFor("decide"), "decide"))
		testutil.Must(decided(r.ctx, r.artifacts))
		body := "# Refined decisions\n"
		for _, decision := range items(d["decisions"]) {
			body += str(decision["key"]) + "\n"
		}
		r.put("docs/refined.md", body)
	}
	r.artifact("draft.json", d)
	r.artifact("approval.txt", "Owner decision: N1. Provisional: none. Unit UN1. Start approve=2 or answers for changes.")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	corrected := r.finish()
	testutil.Equal(t, corrected["proposal_version"], 2)
	testutil.Equal(t, corrected["decision_ids"], out["decision_ids"])
	testutil.Equal(t, r.data().Decisions[0]["title"], "Corrected staff scope")
	rejectsApproval(t, r.ctx, "1")
	noApprovalErrors(t, r.ctx, "2")
}
func TestWriterCommitCannotHideEditsFromDocsScopeGate(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.put("outside.md", "A change committed by the writer")
	r.commit()
	testutil.Contains(t, r.errors(draft()), "writer changed HEAD")
}

func TestCommands(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute paths"
		if relative {
			name = "relative paths"
		}
		t.Run(name, func(t *testing.T) {
			r := setup(t, nil)
			vars := map[string]string{"INPUTS_PHASE": "document", "STATE_DIR": filepath.Join(r.root, "state"), "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": r.artifacts, "INPUTS_OWNER_WORDS": "A service for employees"}
			if relative {
				vars["STATE_DIR"] = "../state"
				vars["ARTIFACTS_DIR"] = "../artifacts"
			}
			registry := cli.New("test", Commands())
			run := func(mode string) doc {
				t.Helper()
				var out, errOut bytes.Buffer
				env := envVars(vars)
				env.Cwd = r.cwd
				env.Stdin = strings.NewReader("")
				env.Stdout = &out
				env.Stderr = &errOut
				testutil.Equal(t, registry.Execute([]string{"refine", mode}, env), 0)
				if errOut.Len() != 0 {
					t.Fatal(errOut.String())
				}
				var d doc
				testutil.OK(t, json.Unmarshal(out.Bytes(), &d))
				return d
			}
			started := run("start")
			testutil.Equal(t, started["stopped"], false)
			testutil.Equal(t, started["details"], record.RecordDir(vars["STATE_DIR"], "refine/topic"))
			prepared := run("prepare")
			testutil.Equal(t, prepared["input"], file(vars["ARTIFACTS_DIR"], "input.json"))
			testutil.Equal(t, prepared["criteria"], filepath.Join(vars["ARTIFACTS_DIR"], "criteria", ".devkit/standards"))
			testutil.Equal(t, testutil.Must(readJSON(file(r.artifacts, "input.json")))["criteria"], prepared["criteria"])
			r.writeDraft()
			testutil.Equal(t, run("check")["valid"], true)
			vars["INPUTS_PHASE"] = "document"
			vars["INPUTS_REFLECT"] = `{"source":"codex","not_reflected":[]}`
			vars["INPUTS_JUDGMENT"] = string(testutil.Must(json.Marshal(judgment())))
			testutil.Equal(t, run("round")["valid"], true)
			testutil.Equal(t, run("finish")["outcome"], "question")
			vars["WORKFLOW_ID"] = "r2"
			vars["INPUTS_OWNER_WORDS"] = ""
			vars["INPUTS_APPROVE"] = "1"
			testutil.Equal(t, run("start")["stopped"], false)
			testutil.Equal(t, run("approve")["outcome"], "pass")
			vars["INPUTS_START"] = `{"reason":"Not ready","remaining":["A question remains"]}`
			testutil.Equal(t, run("not-started")["reasons"], []string{"Not ready", "A question remains"})
		})
	}
}

func TestCommandErrors(t *testing.T) {
	r := setup(t, nil)
	for _, tc := range []struct {
		name, mode string
		vars       map[string]string
		message    string
	}{
		{"missing record context", "start", nil, "STATE_DIR and WORKFLOW_ID must be set"},
		{"missing artifacts", "prepare", map[string]string{"STATE_DIR": filepath.Join(r.root, "state"), "WORKFLOW_ID": "r1"}, "ARTIFACTS_DIR must be set"},
		{"invalid judgment JSON", "round", map[string]string{"STATE_DIR": filepath.Join(r.root, "state"), "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": r.artifacts, "INPUTS_JUDGMENT": "invalid"}, "invalid character"},
		{"invalid start JSON", "not-started", map[string]string{"STATE_DIR": filepath.Join(r.root, "state"), "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": r.artifacts, "INPUTS_START": "invalid"}, "invalid character"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			env := envVars(tc.vars)
			env.Cwd = r.cwd
			env.Stdout = &stdout
			env.Stderr = &stderr
			testutil.Equal(t, cli.New("test", Commands()).Execute([]string{"refine", tc.mode}, env), 1)
			testutil.Equal(t, stdout.String(), "")
			testutil.Contains(t, stderr.String(), tc.message)
		})
	}
}

func TestPrepareRejectsInvalidFixedConcernMap(t *testing.T) {
	r := setup(t, nil)
	r.put(".devkit/standards/concerns.json", `{}`)
	r.commit()
	r.git("branch", "-f", "main", "HEAD")
	s, err := start(r.ctx, envVars(nil))
	testutil.OK(t, err)
	testutil.Equal(t, s.Stopped, false)
	_, err = prepare(r.ctx, envVars(nil), r.artifacts, "A request", "", "")
	testutil.Fails(t, err, "missing quality: scope")
}

func TestDoneUnitCannotBeEditedOrRemovedByDocumentWriter(t *testing.T) {
	for _, operation := range []string{"edit", "delete", "index"} {
		t.Run(operation, func(t *testing.T) {
			r := setup(t, nil)
			path := ".devkit/units/U-abcdef.md"
			original := "---\nid: U-abcdef\ntitle: Finished\ndecisions: []\ndepends_on: []\n---\nCompleted goal.\n"
			r.put(path, original)
			r.put(".devkit/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"feature/done"}`)
			r.commit()
			r.begin()
			r.writeDraft()
			testutil.Equal(t, testutil.Must(check(r.ctx, r.artifacts))["valid"], true)
			if operation == "delete" {
				testutil.OK(t, os.Remove(filepath.Join(r.cwd, path)))
			} else {
				r.put(path, original+"Changed\n")
				if operation == "index" {
					r.git("add", path)
					r.put(path, original)
				}
			}
			testutil.OK(t, os.Remove(filepath.Join(r.cwd, ".devkit/units/U-abcdef.done.json")))
			errors := testutil.Must(draftErrors(r.ctx, r.artifacts, draft()))
			testutil.Contains(t, strings.Join(errors, "\n"), "done unit may not be edited or removed: U-abcdef")
		})
	}
}

func TestReproposalPreservesDoneUnitAndAllocatesFollowUp(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(fmt.Sprint(retained), func(t *testing.T) {
			r := setup(t, doc{"units": "docs/units"})
			path := "docs/units/U-abcdef.md"
			original := "---\nid: U-abcdef\ntitle: Finished\ndecisions: []\ndepends_on: []\n---\nCompleted N1 goal.\n"
			r.put(path, original)
			r.put("docs/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"feature/done"}`)
			r.commit()
			r.begin()
			input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
			input["previous_unapproved"] = true
			input["pending_proposal"] = doc{"content": doc{"mapping": doc{"UN1": "U-abcdef"}, "unit_ids": []string{"U-abcdef"}}}
			d := draft()
			if !retained {
				d["units"] = []doc{}
			} else {
				items(d["units"])[0]["decisions"] = []string{}
			}
			result := testutil.Must(materialize(r.ctx, r.artifacts, d, input))
			testutil.Equal(t, string(testutil.Must(os.ReadFile(filepath.Join(r.cwd, path)))), original)
			if retained {
				if obj(result["mapping"])["UN1"] == "U-abcdef" {
					t.Fatal("reused a done unit")
				}
			}
		})
	}
}

func TestSupersedingDecisionRequiresOpenUnitSuccessorCitations(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		refs      []string
		done      bool
		pending   bool
		valid     bool
	}{
		{name: "omitted open unit"},
		{name: "retained stale citation", key: "U-abcdef", refs: []string{"D-000001"}},
		{name: "retained unrelated citation", key: "U-abcdef", refs: []string{"D-000002"}},
		{name: "retained successor and stale citation", key: "U-abcdef", refs: []string{"N1", "D-000001"}},
		{name: "retained successor", key: "U-abcdef", refs: []string{"N1"}, valid: true},
		{name: "unchanged done unit", done: true, valid: true},
		{name: "retained done unit", key: "U-abcdef", refs: []string{"N1"}, done: true},
		{name: "retained pending unit", key: "UN1", refs: []string{"N1"}, pending: true, valid: true},
		{name: "stale pending unit", key: "UN1", refs: []string{"D-000001"}, pending: true},
		{name: "withdrawn pending unit", pending: true, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t, doc{"units": "docs/units"})
			r.decision("D-000001", nil)
			r.decision("D-000002", nil)
			path := "docs/units/U-abcdef.md"
			original := recordText(doc{"id": "U-abcdef", "title": "Existing work", "decisions": []string{"D-000001"}, "depends_on": []string{}}, []string{"id", "title", "decisions", "depends_on"}, "Existing goal\n\n- Existing completion condition")
			r.put(path, original)
			if tc.done {
				r.put("docs/units/U-abcdef.done.json", `{"unit":"U-abcdef","branch":"feature/done"}`)
			}
			r.commit()
			r.begin()
			if tc.pending {
				input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
				input["previous_unapproved"] = true
				input["pending_proposal"] = doc{"content": doc{"mapping": doc{"UN1": "U-abcdef"}, "unit_ids": []string{"U-abcdef"}}}
				r.artifact("input.json", input)
			}
			d := draft()
			items(d["decisions"])[0]["supersedes"] = []string{"D-000001"}
			d["kept_units"] = []doc{{"id": "U-abcdef", "reason": "Record the affected unit disposition"}}
			d["kept_docs"] = []doc{{"path": path, "reason": "The unit disposition is applied by refine"}}
			if tc.key == "" {
				d["units"] = []doc{}
			} else {
				u := items(d["units"])[0]
				u["key"], u["decisions"] = tc.key, tc.refs
				u["title"], u["goal"] = "Existing work", "Existing goal"
				u["completion_conditions"] = []string{"Existing completion condition"}
			}
			r.writeDraft(d)
			r.artifact("approval.txt", "N1 U-abcdef UN1 approve=1 answers")
			checked := testutil.Must(check(r.ctx, r.artifacts))
			if checked["valid"] != tc.valid {
				t.Fatalf("got %v; want valid=%v", checked, tc.valid)
			}
			if !tc.valid {
				errors := strings.Join(stringsOf(checked["errors"]), "\n")
				testutil.Contains(t, errors, "U-abcdef")
				if !tc.done {
					testutil.Contains(t, errors, "D-000001")
				}
				return
			}
			r.round(judgment())
			testutil.Equal(t, r.finish()["outcome"], "question")
			r.ctx.RunID = "r2"
			started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"})))
			testutil.Equal(t, started.Stopped, false)
			testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
			if tc.done {
				testutil.Equal(t, readText(t, filepath.Join(r.cwd, path)), original)
			} else if tc.key == "" {
				_, err := os.Stat(filepath.Join(r.cwd, path))
				testutil.Equal(t, os.IsNotExist(err), true)
			} else {
				successor := str(obj(obj(r.latest("proposal")["content"])["mapping"])["N1"])
				u := find(r.data().Units, "U-abcdef")
				testutil.Equal(t, stringsOf(u["decisions"]), []string{successor})
				testutil.Equal(t, u["path"], path)
				testutil.Equal(t, len(r.data().Units), 1)
				testutil.Equal(t, strings.Replace(readText(t, filepath.Join(r.cwd, path)), successor, "D-000001", 1), original)
			}
		})
	}
}
