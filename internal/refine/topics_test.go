package refine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func (r *fixture) restart(out doc) {
	r.t.Helper()
	r.ctx.RunID += "-next"
	answers := []doc{{"question_id": items(out["questions"])[0]["id"], "answer": "Revise the proposal"}}
	started := testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_ANSWERS": string(testutil.Must(json.Marshal(answers)))})))
	testutil.Equal(r.t, started.Stopped, false)
	r.artifacts = filepath.Join(r.root, r.ctx.RunID)
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
}

func TestApprovalRequiresSuccessfulStartAndItsRecordedAnswerBeforeAnyWrite(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	out := r.finish()
	head := r.git("rev-parse", "HEAD")
	before := readText(t, filepath.Join(r.cwd, ".devkit/decisions/"+stringsOf(out["decision_ids"])[0]+".md"))
	r.ctx.RunID = "unstarted"
	_, err := approve(r.ctx, "1")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Fails(t, err, "successful refine start")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	testutil.Equal(t, readText(t, filepath.Join(r.cwd, ".devkit/decisions/"+stringsOf(out["decision_ids"])[0]+".md")), before)
	testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "result"))), 1)
	qid := str(items(out["questions"])[0]["id"])
	testutil.OK(t, record.RecordAnswers(r.ctx.Dir, "refine", []record.Answer{{QuestionID: qid, Answer: "approve=1"}}, r.ctx.RunID, "2026-01-01T00:00:00Z"))
	_, err = approve(r.ctx, "1")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Fails(t, err, "successful refine start")
}

func TestScopeGateSeesIndexOnlyChangesAndStopsWithoutCommittingThem(t *testing.T) {
	r := setup(t, nil)
	r.put("outside.txt", "original\n")
	r.commit()
	r.begin()
	r.writeDraft()
	r.put("outside.txt", "staged outside\n")
	r.git("add", "outside.txt")
	r.put("outside.txt", "original\n")
	head := r.git("rev-parse", "HEAD")
	testutil.Contains(t, r.errors(draft()), "outside the docs folder: outside.txt")
	testutil.Equal(t, r.round(judgment())["valid"], false)
	testutil.Equal(t, r.finish()["outcome"], "stop")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), head)
	testutil.Equal(t, r.git("show", ":outside.txt"), "staged outside")
}

func TestCommitContainsOnlyExplicitAllowedPaths(t *testing.T) {
	r := setup(t, nil)
	r.put("outside.txt", "original\n")
	r.commit()
	r.put("outside.txt", "staged outside\n")
	r.git("add", "outside.txt")
	r.put("docs/topic.md", "Updated\n")
	before := r.git("rev-parse", "HEAD")
	_, err := commitPaths(r.cwd, []string{"docs/topic.md"}, "docs: update topic")
	testutil.Fails(t, err, "unapproved path: outside.txt")
	testutil.Equal(t, r.git("rev-parse", "HEAD"), before)
	r.git("restore", "--staged", "--worktree", "outside.txt")
	testutil.Must(commitPaths(r.cwd, []string{"docs/topic.md"}, "docs: update topic"))
	testutil.Equal(t, r.git("diff", "HEAD^", "HEAD", "--name-only"), "docs/topic.md")
	testutil.Equal(t, r.git("show", "HEAD:outside.txt"), "original")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}

func TestReproposalRemovesWithdrawnDecisionUnitAndTopicAndKeepsRetainedIDs(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.commit()
	r.begin()
	d := draft()
	second := clone(items(d["decisions"])[0])
	second["key"] = "N2"
	d["decisions"] = append(items(d["decisions"]), second)
	secondUnit := clone(items(d["units"])[0])
	secondUnit["key"] = "UN2"
	secondUnit["decisions"] = []string{"N2"}
	d["units"] = append(items(d["units"]), secondUnit)
	d["topics"] = []doc{{"key": "TN1", "title": "Independent question", "body": "Which delivery channel? A settled answer names the channel.", "depends_on": []string{"D-000001"}}}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 N2 UN1 UN2 TN1 approve=1 or answers for corrections")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	first := r.finish()
	old := obj(obj(r.latest("proposal")["content"])["mapping"])
	for key, folder := range map[string]string{"N2": "decisions", "UN2": "units", "TN1": "topics"} {
		_, err := os.Stat(filepath.Join(r.cwd, ".devkit", folder, str(old[key])+".md"))
		testutil.OK(t, err)
	}
	r.restart(first)
	r.writeDraft()
	r.artifact("approval.txt", "N1 UN1 approve=2 or answers for corrections")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	next := r.finish()
	testutil.Equal(t, next["decision_ids"], []string{str(old["N1"])})
	testutil.Equal(t, next["unit_ids"], []string{str(old["UN1"])})
	for key, folder := range map[string]string{"N2": "decisions", "UN2": "units", "TN1": "topics"} {
		_, err := os.Stat(filepath.Join(r.cwd, ".devkit", folder, str(old[key])+".md"))
		if !os.IsNotExist(err) {
			t.Fatalf("withdrawn %s remains: %v", key, err)
		}
	}
	testutil.Equal(t, find(r.data().Decisions, "D-000001")["status"], "agreed")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
	r.ctx.RunID = "approval"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "2"}))).Stopped, false)
	testutil.Equal(t, testutil.Must(approve(r.ctx, "2"))["outcome"], "pass")
	testutil.Equal(t, find(r.data().Decisions, str(old["N1"]))["status"], "agreed")
}

func TestTopicDependencyGateAndApprovalSettlesOnlySelectedTopic(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.put(".devkit/topics/T-abcdef.md", topicText(doc{"id": "T-abcdef", "title": "Access channel", "status": "open", "depends_on": []string{"D-000001"}, "body": "Which access channel? The answer must name the channel."}))
	r.put(".devkit/topics/T-bcdefa.md", topicText(doc{"id": "T-bcdefa", "title": "Another area", "status": "open", "depends_on": []string{"D-000001"}, "body": "Which reporting channel? Name it."}))
	r.commit()
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_TOPIC": "T-abcdef"}))).Stopped, false)
	prepared := testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", "", "T-abcdef"))
	input := testutil.Must(readJSON(str(prepared["input"])))
	testutil.Equal(t, obj(input["topic"])["id"], "T-abcdef")
	d := draft()
	d["units"] = []doc{}
	d["topics"] = []doc{{"key": "TN1", "title": "Next choice", "body": "Which duration? A settled answer contains a duration.", "depends_on": []string{"N1"}}}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 TN1 approve=1 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	r.finish()
	testutil.Equal(t, find(r.data().Topics, "T-abcdef")["status"], "open")
	r.ctx.RunID = "approval"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Must(approve(r.ctx, "1"))
	testutil.Equal(t, find(r.data().Topics, "T-abcdef")["status"], "settled")
	testutil.Equal(t, find(r.data().Topics, "T-bcdefa")["status"], "open")
	nextID := testutil.Must(decisions.NextID("topic", r.ctx.Branch, "TN1", decisions.Data{}))
	testutil.Equal(t, find(decisions.PickableTopics(r.data()), nextID)["status"], "open")
}

func TestUnpickableTopicStopsBeforeRecords(t *testing.T) {
	for _, status := range []string{"proposed", "superseded", "missing", "settled"} {
		t.Run(status, func(t *testing.T) {
			r := setup(t, nil)
			if status != "missing" {
				r.decision("D-000001", doc{"status": status})
			}
			topicStatus := "open"
			if status == "settled" {
				topicStatus = "settled"
				r.decision("D-000001", nil)
			}
			r.put(".devkit/topics/T-abcdef.md", topicText(doc{"id": "T-abcdef", "title": "Question", "status": topicStatus, "depends_on": []string{"D-000001"}, "body": "Question and required answer"}))
			r.commit()
			testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_TOPIC": "T-abcdef"}))).Stopped, true)
			testutil.Equal(t, len(testutil.Must(record.List(r.ctx.Dir, "run"))), 0)
		})
	}
}

func TestDecideAndDocumentHaveIndependentGatesAndFreezeDecisions(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["units"] = []doc{}
	r.artifact("draft.json", d)
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, judgmentFor("decide"), "decide"))["round"], 1)
	testutil.Equal(t, testutil.Must(decided(r.ctx, r.artifacts))["ready"], true)
	r.put("docs/refined.md", "N1\n")
	r.artifact("approval.txt", "N1 approve=1 or answers")
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, judgment(), "document"))["round"], 1)
	d2 := clone(d)
	d2["decisions"] = []doc{clone(items(d["decisions"])[0])}
	items(d2["decisions"])[0]["body"] = "Different meaning"
	testutil.Contains(t, r.errors(d2), "document changed settled decisions")
	r.artifact("draft.json", d2)
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, judgment(), "document"))["valid"], false)
	testutil.Equal(t, r.finish()["outcome"], "stop")
}

func TestUnitAreaWaitsForTopicsThroughTransitiveDecisionDependencies(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.decision("D-000002", doc{"depends_on": []string{"D-000001"}})
	r.put(".devkit/topics/T-abcdef.md", topicText(doc{"id": "T-abcdef", "title": "Open choice", "status": "open", "depends_on": []string{"D-000001"}, "body": "A question and what is needed"}))
	r.commit()
	r.begin()
	d := draft()
	items(d["decisions"])[0]["depends_on"] = []string{"D-000002"}
	r.writeDraft(d)
	testutil.Contains(t, r.errors(d), "area has an open topic T-abcdef")
	d["units"] = []doc{}
	r.writeDraft(d)
	testutil.Equal(t, r.errors(d), "")
}

func TestNextTopicBlocksUnitsButAllowsDocumentProposal(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["topics"] = []doc{{"key": "TN1", "title": "Provider", "body": "Which provider? A settled answer selects one.", "depends_on": []string{"N1"}}}
	r.writeDraft(d)
	testutil.Contains(t, r.errors(d), "area has an open topic TN1")
	d["units"] = []doc{}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 TN1 approve=1 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	testutil.Equal(t, r.finish()["unit_ids"], []string{})
}

func TestIndirectAffectedDocumentsAndUnitsRequireDisposition(t *testing.T) {
	r := setup(t, nil)
	r.decision("D-000001", nil)
	r.decision("D-000002", doc{"depends_on": []string{"D-000001"}})
	r.put("docs/dependent.md", "D-000002\n")
	r.put(".devkit/units/U-000001.md", recordText(doc{"id": "U-000001", "title": "Existing work", "decisions": []string{"D-000002"}, "depends_on": []string{}}, []string{"id", "title", "decisions", "depends_on"}, "An existing goal"))
	r.commit()
	r.begin()
	d := draft()
	items(d["decisions"])[0]["supersedes"] = []string{"D-000001"}
	d["rebased"] = []doc{{"id": "D-000002", "from": "D-000001", "to": "N1", "reason": "Substantive meaning is preserved"}}
	r.writeDraft(d)
	testutil.Contains(t, r.errors(d), "docs/dependent.md: affected document")
	testutil.Contains(t, r.errors(d), "U-000001: affected unit")
	d["kept_docs"] = []doc{{"path": "docs/dependent.md", "reason": "The rebased decision preserves meaning"}}
	d["kept_units"] = []doc{{"id": "U-000001", "reason": "The same work implements the rebased meaning"}}
	r.writeDraft(d)
	testutil.Equal(t, r.errors(d), "")
}

func TestBothPhaseJudgmentsAndLoopCountsAreRecorded(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	r.round(judgment())
	r.finish()
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, judgments[0]["phase"], "decide")
	testutil.Equal(t, judgments[1]["phase"], "document")
	loops := items(r.latest("result")["loops"])
	testutil.Equal(t, len(loops), 2)
	testutil.Equal(t, loops[0]["name"], "decide")
	testutil.Equal(t, loops[1]["name"], "document")
	testutil.Equal(t, num(loops[0]["rounds"]), 1)
	testutil.Equal(t, num(loops[1]["rounds"]), 1)
}

func TestDecideCannotEditDocumentsAndDocumentRequiresDecide(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["units"] = []doc{}
	r.artifact("draft.json", d)
	testutil.Contains(t, strings.Join(testutil.Must(draftErrors(r.ctx, r.artifacts, d, "document")), "\n"), "requires a cleared decide phase")
	r.put("docs/topic.md", "Premature prose N1\n")
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, judgmentFor("decide"), "decide"))["valid"], false)
	testutil.Equal(t, testutil.Must(decided(r.ctx, r.artifacts))["ready"], false)
	testutil.Equal(t, r.finish()["outcome"], "stop")
	testutil.Equal(t, r.git("status", "--porcelain"), "")
}

func TestWithdrawingNextTopicAllowsAreaToBeCutOnReproposal(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["units"] = []doc{}
	d["topics"] = []doc{{"key": "TN1", "title": "Scope detail", "body": "Which detail? Name the required detail.", "depends_on": []string{"N1"}}}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 TN1 approve=1 or answers")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	out := r.finish()
	r.restart(out)
	r.writeDraft()
	r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
	testutil.Equal(t, r.errors(draft()), "")
	testutil.Equal(t, r.round(judgment())["valid"], true)
	testutil.Equal(t, len(stringsOf(r.finish()["unit_ids"])), 1)
}

func TestApprovedOpenTopicsRemainAreaConstraintsOnTheNextRun(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["units"] = []doc{}
	d["topics"] = []doc{{"key": "TN1", "title": "Scope detail", "body": "Which detail? Name it.", "depends_on": []string{"N1"}}, {"key": "TN2", "title": "Delivery detail", "body": "Which channel? Name it.", "depends_on": []string{"N1"}}}
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 TN1 TN2 approve=1 or answers")
	r.round(judgment())
	r.finish()
	mapping := obj(obj(r.latest("proposal")["content"])["mapping"])
	r.ctx.RunID = "approval"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Must(approve(r.ctx, "1"))
	r.ctx.RunID = "next-topic"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(nil))).Stopped, true)
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_TOPIC": str(mapping["TN1"])}))).Stopped, false)
	r.artifacts = filepath.Join(r.root, "next-topic")
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", "", str(mapping["TN1"])))
	next := draft()
	items(next["decisions"])[0]["depends_on"] = []string{str(mapping["N1"])}
	r.writeDraft(next)
	testutil.Contains(t, r.errors(next), "area has an open topic "+str(mapping["TN2"]))
}

func TestPreparedSchemasAreRealAndMatchShippedSchemas(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	for _, name := range []string{"draft", "judgment"} {
		embedded := testutil.Must(schemas.ReadFile("schemas/" + name + ".schema.json"))
		generated := testutil.Must(os.ReadFile(file(r.artifacts, name+".schema.json")))
		shipped := testutil.Must(os.ReadFile(filepath.Join("../../kit/.archon/workflows/dev/refine/scripts", name+".schema.json")))
		testutil.Equal(t, string(generated), string(embedded))
		testutil.Equal(t, string(shipped), string(embedded))
	}
}

func TestStoppedReproposalKeepsOwnershipOfUnapprovedFiles(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	second := clone(items(d["decisions"])[0])
	second["key"] = "N2"
	d["decisions"] = append(items(d["decisions"]), second)
	r.writeDraft(d)
	r.artifact("approval.txt", "N1 N2 UN1 approve=1 or answers")
	r.round(judgment())
	first := r.finish()
	mapping := obj(obj(r.latest("proposal")["content"])["mapping"])
	r.restart(first)
	r.writeDraft()
	r.artifact("approval.txt", "N1 UN1 approve=2 or answers")
	failed := judgment()
	items(failed["criteria"])[0]["verdict"] = "fail"
	r.round(failed)
	testutil.Equal(t, r.finish()["outcome"], "stop")
	r.ctx.RunID = "retry-stopped"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(nil))).Stopped, false)
	r.artifacts = filepath.Join(r.root, "retry-stopped")
	testutil.Must(prepare(r.ctx, envVars(nil), r.artifacts, "", "", ""))
	r.writeDraft()
	r.artifact("approval.txt", "N1 UN1 approve=3 or answers")
	r.round(judgment())
	final := r.finish()
	testutil.Equal(t, final["decision_ids"], []string{str(mapping["N1"])})
	_, err := os.Stat(filepath.Join(r.cwd, ".devkit/decisions", str(mapping["N2"])+".md"))
	if !os.IsNotExist(err) {
		t.Fatalf("withdrawn decision remains after stopped proposal: %v", err)
	}
}

func TestNextTopicReferencesAndDocumentReflectionAreMachineChecked(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	d := draft()
	d["units"] = []doc{}
	d["topics"] = []doc{{"key": "TN1", "title": "Next", "body": "How does N99 affect the answer?", "depends_on": []string{"D-ffffff"}}}
	errors := strings.Join(testutil.Must(draftErrors(r.ctx, r.artifacts, d, "decide")), "\n")
	testutil.Contains(t, errors, "TN1: missing draft key N99")
	testutil.Contains(t, errors, "TN1: missing decision D-ffffff")
	r.writeDraft()
	r.put("docs/refined.md", "Unrelated prose\n")
	testutil.Contains(t, r.errors(draft()), "N1: new decision is not reflected in a document")
}

func TestConfiguredTopicFolderIsUsedForProposalAndApproval(t *testing.T) {
	r := setup(t, doc{"topics": "questions"})
	r.begin()
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.Equal(t, obj(input["folders"])["topics"], "questions")
	r.writeDraft()
	r.round(judgment())
	testutil.Equal(t, r.finish()["outcome"], "question")
	r.ctx.RunID = "r2"
	testutil.Equal(t, testutil.Must(start(r.ctx, envVars(map[string]string{"INPUTS_APPROVE": "1"}))).Stopped, false)
	testutil.Equal(t, testutil.Must(approve(r.ctx, "1"))["outcome"], "pass")
	topics := testutil.Must(decisions.List(r.cwd)).Topics
	testutil.Equal(t, len(topics), 1)
	testutil.Equal(t, topics[0]["status"], "settled")
	testutil.Equal(t, topics[0]["path"], "questions/"+str(topics[0]["id"])+".md")
}

func TestF12RefineKeepsEveryRoundAndResolution(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	j := judgment()
	j["findings"] = []doc{{"aspect": "wording", "item": "N1", "blocking": true, "needs_decision": false, "mechanical": true, "failure_scenario": "The promise is ambiguous", "evidence": "docs/topic.md", "fix": "Name the promise"}}
	r.round(j)
	r.round(judgment())
	r.finish()
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 3)
	testutil.Equal(t, items(judgments[1]["findings"])[0]["resolved"], true)
	testutil.Equal(t, items(judgments[1]["findings"])[0]["mechanical"], true)
}

func TestF12UnassessedRoundDoesNotResolveEarlierFinding(t *testing.T) {
	r := setup(t, nil)
	r.begin()
	r.writeDraft()
	j := judgment()
	j["findings"] = []doc{{"aspect": "wording", "item": "N1", "blocking": true, "needs_decision": false, "mechanical": true, "failure_scenario": "The promise is ambiguous", "evidence": "docs/topic.md", "fix": "Name the promise"}}
	r.round(j)
	r.round(doc{"criteria": []doc{}, "findings": []doc{}})
	r.finish()
	judgments := testutil.Must(record.List(r.ctx.Dir, "judgment"))
	testutil.Equal(t, items(judgments[1]["findings"])[0]["resolved"], false)
	testutil.Equal(t, judgments[2]["verdict"], "not_assessed")
}
