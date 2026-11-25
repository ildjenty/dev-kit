package pr

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func writerText() doc {
	return doc{"title": "Store orders", "opening": "Orders are stored", "how_to_see": "Create an order", "after_merge": "", "headings": doc{"what_changed": "Changes", "how_checked": "Checks", "what_remains": "Remaining", "decisions": "Decisions", "after_merge": "After merge"}}
}
func jsonText(v any) string { return string(must(json.Marshal(v))) }
func TestPermissionReferencesPreserveJSONStringifyEscapes(t *testing.T) {
	change := doc{"category": "cost", "file": "infra/<a>&\u2028\u2029", "item": `literal \u2028`}
	q := question(change)
	testutil.Equal(t, object(q["target"])["ref"], "{\"category\":\"cost\",\"file\":\"infra/<a>&\u2028\u2029\",\"item\":\"literal \\\\u2028\"}")
}
func TestPermissionAnswersStartWithExactAllowOrDenyToken(t *testing.T) {
	for _, c := range []struct {
		answer string
		want   bool
	}{{"allow this change", true}, {"deny: preserve the old behavior", false}, {"allow", true}} {
		testutil.Equal(t, must(parsePermission(c.answer)), c.want)
	}
	for _, answer := range []string{"yes", " allow", "Allow", "allowed", "denying", "please allow"} {
		_, err := parsePermission(answer)
		testutil.Fails(t, err, "begin with allow or deny")
	}
}
func addRun(dir, id, workflow string) {
	must(record.Append(dir, "run", doc{"run_id": id, "workflow": workflow, "started_at": "t", "start": "other", "counted": false, "answers": []any{}}))
}
func TestInvalidAnswersRejectedBeforeAllowanceAndDenialsRetained(t *testing.T) {
	dir := record.RecordDir(t.TempDir(), "feature/x")
	must(record.Append(dir, "request", doc{"run_id": "r0", "recorded_at": "t", "owner_words": "Request", "supplement": "", "criteria_commit": strings.Repeat("c", 40)}))
	addRun(dir, "pr-1", "pr")
	ctx := record.ContextData{Dir: dir, RunID: "pr-1"}
	change := doc{"category": "cost", "file": "infra/service.json", "item": "patch-1"}
	testutil.OK(t, record.WriteKeyed(dir, "answer", "Q1", doc{"question_id": "Q1", "workflow": "pr", "run_id": "pr-1", "answered_at": "t", "versions": doc{"requirements_version": nil, "plan_version": nil}, "commit": strings.Repeat("a", 40), "question": "Allow?", "answer": "allow", "target": doc{"kind": "change", "ref": "change"}}))
	_, _, err := recordAnswers(ctx, []doc{{"question_id": "Q1", "change": change, "answer": "allow"}, {"question_id": "Q2", "change": change, "answer": "yes"}}, "")
	testutil.Fails(t, err, "begin with allow or deny")
	testutil.Equal(t, len(must(record.List(dir, "allowed"))), 0)
	allowed, denied, err := recordAnswers(ctx, []doc{{"question_id": "Q1", "change": change, "answer": "allow this patch"}, {"question_id": "Q2", "change": change, "answer": "deny"}}, "t")
	testutil.OK(t, err)
	testutil.Equal(t, allowed, 1)
	testutil.Equal(t, denied[0]["question_id"], "Q2")
	testutil.Equal(t, must(record.List(dir, "allowed"))[0]["value"], "patch-1")
}
func TestAsksDeduplicateExactChangesAndPermissionCoversIdenticalPatches(t *testing.T) {
	item := doc{"ask": true, "category": "cost", "file": "infra/a", "item": "patch-1"}
	allowed := []doc{{"category": "cost", "file": "infra/a", "item": "patch-1", "value": "patch-1", "question_id": "Q1"}}
	noAsk := clone(item)
	noAsk["ask"] = false
	testutil.Equal(t, len(asks([]doc{item, item, noAsk}, allowed)), 1)
	testutil.Equal(t, asks([]doc{item}, allowed)[0]["allowed_by"], "Q1")
	other := clone(item)
	other["item"] = "patch-2"
	testutil.Equal(t, asks([]doc{other}, allowed)[0]["allowed_by"], nil)
}
func TestPermissionIdentityChangesWithContentAtSamePath(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "infra/service.json", "one")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "first")
	g := must(gitx.New(repo, "main"))
	base := must(g.MergeBase(must(g.Head())))
	asked := func() doc {
		head := must(g.Head())
		for _, i := range must(itemsOf(g, base, head, config.Defaults(""), must(g.Changes(base, head)))) {
			if truth(i["ask"]) {
				return i
			}
		}
		t.Fatal("no permission item")
		return nil
	}
	first := asked()
	testutil.Put(t, repo, "infra/service.json", "two")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "second")
	second := asked()
	if first["item"] == second["item"] {
		t.Fatal("identity did not change")
	}
	a := clone(first)
	a["value"] = first["item"]
	a["question_id"] = "Q1"
	testutil.Equal(t, asks([]doc{second}, []doc{a})[0]["allowed_by"], nil)
}
func TestFactsRequireCurrentPassingRequirementsAndPlan(t *testing.T) {
	requirements := doc{"version": 1}
	valid := []doc{{"workflow": "requirements", "outcome": "pass", "inputs": doc{"requirements_version": 1}}}
	testutil.Equal(t, len(stringsOf(inputsOf(requirements, nil, valid, false)["reasons"])), 1)
	testutil.Equal(t, len(stringsOf(inputsOf(requirements, nil, valid, true)["reasons"])), 1)
	plan := doc{"version": 2, "requirements_version": 1}
	testutil.Equal(t, len(stringsOf(inputsOf(requirements, plan, valid, false)["reasons"])), 1)
	carried := doc{"workflow": "plan", "outcome": "carry", "inputs": doc{"plan_version": 2}}
	testutil.Equal(t, inputsOf(requirements, plan, append(valid, carried), true)["reasons"], []string{})
	testutil.Equal(t, inputsOf(doc{"version": 2}, plan, valid, false)["plan"], nil)
}
func TestBodyListsOwnerPrincipleDecisionsCarriedFindingsAndNotChecked(t *testing.T) {
	decisions := []doc{}
	for i, by := range []string{"owner", "principle", "provisional", "machine_rule"} {
		decisions = append(decisions, doc{"id": string(rune('0' + i)), "decided_by": by, "what": "choice", "chosen": "chosen", "principles": []string{"principle 1"}})
		decisions[i]["id"] = "D" + text(decisions[i]["id"])
	}
	lists := checkpoints(doc{"completion_conditions": []doc{{"id": "C1", "text": "An order is visible"}}}, doc{"decisions": decisions, "tests": []doc{{"criterion": "C1", "method": "owner_check"}}}, []doc{{"question_id": "Q1", "answer": "allow"}}, []doc{{"status": "M", "path": "a", "quality": "contract", "allowed_by": "Q1"}}, []doc{{"judge": "scope", "verdict": "pass", "observations": []string{"observed"}, "findings": []doc{{"id": "F1", "blocking": false, "text": "later"}}}}, []doc{{"id": "P1", "text": "carried", "judge": "delivery"}})
	by := []string{}
	for _, d := range docs(lists["decisions"]) {
		by = append(by, text(d["decided_by"]))
	}
	testutil.Equal(t, by, []string{"owner", "principle", "provisional"})
	result := body(writerText(), lists)
	for _, s := range []string{"D0", "D1", "P1", "F1", "owner_check", "Q1", "Changes", "Checks", "Remaining", "Decisions", "After merge"} {
		testutil.Contains(t, result, s)
	}
	for _, s := range []string{"D3"} {
		if strings.Contains(result, s) {
			t.Fatalf("unexpected %s", s)
		}
	}
}
func TestReportValidationRejectsMissingHeadingsAndUnpassedOrOwnerFinding(t *testing.T) {
	d := writerText()
	testutil.Equal(t, must(parseText(jsonText(d))), d)
	bad := clone(d)
	bad["title"] = "two\nlines"
	_, err := parseText(jsonText(bad))
	testutil.Fails(t, err, "one line")
	bad = clone(d)
	bad["headings"] = doc{}
	_, err = parseText(jsonText(bad))
	testutil.Fails(t, err, "heading")
	testutil.Equal(t, passed(doc{"verdict": "pass", "findings": []doc{}}), true)
	testutil.Equal(t, passed(doc{"verdict": "not_assessed", "findings": []doc{}}), false)
	testutil.Equal(t, passed(doc{"verdict": "pass", "findings": []doc{{"needs_decision": true}}}), false)
}
func TestPushOnlyReplacesRecordedCommitAndRefusesChangedOrDirtyCheckout(t *testing.T) {
	recorded := recordedCommitsOf([]doc{{"commit": "old"}}, []doc{{"target": doc{"commit": "judged"}}})
	ancestor := func(string, string) bool { return false }
	for _, c := range []struct{ remote, want string }{{"", "new"}, {"new", "forward"}, {"old", "replace"}, {"someone-else", ""}} {
		testutil.Equal(t, pushKind(c.remote, "new", ancestor, recorded), c.want)
	}
	facts := doc{"head": "head"}
	report := doc{"target": doc{"commit": "head"}, "verdict": "pass", "findings": []doc{}}
	testutil.Equal(t, refusals("feature", "trunk", "head", facts, true, report), []string{})
	testutil.Equal(t, len(refusals("trunk", "trunk", "moved", facts, false, report)), 4)
}
func runCLI(t *testing.T, repo string, vars map[string]string, args ...string) doc {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, Environ: os.Environ()}
	if code := cli.New("test", Commands()).Execute(append([]string{"pr"}, args...), env); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var out doc
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &out))
	return out
}
func TestFactsCLIUsesCustomLanguageStandardsAndAsksBeforeDraftingWithoutAllowance(t *testing.T) {
	repo := testutil.Repo(t)
	git := func(args ...string) string { return testutil.Git(t, repo, args...) }
	testutil.Put(t, repo, ".devkit/config.json", `{"language":"es","standards":"rules"}`)
	testutil.Standards(t, repo, "rules")
	testutil.Put(t, repo, "rules/authority.md", "criteria")
	git("add", ".")
	git("commit", "-qm", "criteria")
	git("branch", "-f", "main", "HEAD")
	base := git("rev-parse", "HEAD")
	testutil.Put(t, repo, "infra/service.json", "capacity: 1")
	git("add", ".")
	git("commit", "-qm", "work")
	g := must(gitx.New(repo, "main"))
	head := must(g.Head())
	state, artifacts := t.TempDir(), t.TempDir()
	dir := record.RecordDir(state, "feature/x")
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "pr-1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "main"}
	must(record.Append(dir, "request", doc{"run_id": "r1", "recorded_at": "t", "owner_words": "change", "supplement": "", "criteria_commit": base}))
	addRun(dir, "r1", "requirements")
	must(record.Append(dir, "requirements", doc{"request": 1, "run_id": "r1", "recorded_at": "t", "goal": "work", "completion_conditions": []doc{{"id": "C1", "text": "works"}}, "constraints": []any{}, "open_points": []any{}}))
	must(record.Append(dir, "judgment", doc{"run_id": "r1", "recorded_at": "t", "kind": "requirements", "judge": "requirements", "target": doc{"requirements_version": 1}, "criteria_commit": base, "materials": []string{}, "observations": []string{}, "verdict": "pass", "findings": []doc{}}))
	must(record.Append(dir, "result", doc{"run_id": "r1", "workflow": "requirements", "recorded_at": "t", "outcome": "pass", "inputs": doc{"requirements_version": 1, "plan_version": nil}, "base": base, "commit": head, "criteria_commit": base, "loops": []any{}}))
	addRun(dir, "p1", "plan")
	must(record.Append(dir, "plan", doc{"requirements_version": 1, "run_id": "p1", "recorded_at": "t", "skeleton_commit": head, "decisions": []doc{}, "increments": []doc{{"id": "U1", "summary": "work", "criteria": []string{"C1"}}}, "tests": []doc{}, "kept_behaviors": []doc{}}))
	must(record.WriteResult(dir, "plan", doc{"outcome": "pass"}, record.ResultOptions{RunID: "p1", Base: base, Commit: head}))
	addRun(dir, "j1", "judge")
	must(record.Append(dir, "judgment", doc{"run_id": "j1", "recorded_at": "t", "kind": "code", "judge": "scope", "target": doc{"requirements_version": 1, "plan_version": 1, "base": base, "commit": head, "change_id": must(g.ChangeID(head))}, "criteria_commit": base, "materials": []string{}, "observations": []string{}, "verdict": "pass", "findings": []doc{}}))
	addRun(dir, "pr-1", "pr")
	testutil.Equal(t, runCLI(t, repo, vars, "facts")["outcome"], "question")
	facts := must(read(filepath.Join(artifacts, "pr/facts.json")))
	testutil.Equal(t, facts["language"], "es")
	testutil.Equal(t, facts["standards"], "rules")
	testutil.Equal(t, len(docs(facts["questions"])), 1)
	testutil.Contains(t, string(must(os.ReadFile(filepath.Join(artifacts, "pr/diff.patch")))), "capacity")
	q := docs(must(read(filepath.Join(artifacts, "pr/outcome.json")))["questions"])[0]
	var change doc
	testutil.OK(t, json.Unmarshal([]byte(text(object(q["target"])["ref"])), &change))
	change["value"] = change["item"]
	change["question_id"] = "Q1"
	change["run_id"] = "pr-1"
	change["recorded_at"] = "t"
	must(record.Append(dir, "allowed", change))
	testutil.Equal(t, runCLI(t, repo, vars, "facts")["ready"], true)
	vars["INPUTS_TEXT"] = jsonText(writerText())
	runCLI(t, repo, vars, "assemble")
	for i := 1; i <= 3; i++ {
		vars["INPUTS_JUDGMENT"] = jsonText(doc{"verdict": "fail", "findings": []doc{{"id": "R1", "blocking": true, "needs_decision": false, "text": "The body claims an unexecuted check passed", "evidence": "pr.md"}}, "observations": []string{"read body"}})
		r := runCLI(t, repo, vars, "judgment")
		testutil.Equal(t, r["round"], i)
		testutil.Equal(t, r["passed"], false)
	}
	testutil.Equal(t, runCLI(t, repo, vars, "state")["round"], 3)
	testutil.Equal(t, runCLI(t, repo, vars, "finish")["outcome"], "stop")
	count := 0
	for _, j := range must(record.List(dir, "judgment")) {
		if j["kind"] == "report" {
			count++
		}
	}
	testutil.Equal(t, count, 3)
}

func TestPRAsksOnlyChangesUncoveredByOwnerRows(t *testing.T) {
	item := doc{"ask": true, "file": "infra/service.json", "category": "cost", "item": "patch"}
	for _, by := range []string{"principle", "provisional", "owner"} {
		rows := []doc{{"decided_by": by, "touches": []string{"infra"}}}
		count := 1
		if by == "owner" {
			count = 0
		}
		testutil.Equal(t, len(asks([]doc{item}, nil, rows...)), count)
	}
	testutil.Equal(t, len(asks([]doc{item}, nil, doc{"decided_by": "owner", "touches": []string{"infra/other"}})), 1)
}
