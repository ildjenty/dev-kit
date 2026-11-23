package judge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func must[T any](v T, err error) T { return testutil.Must(v, err) }
func testConfig() config.Config {
	c := config.Defaults("")
	c.DecisionPaths = []config.DecisionRule{{Paths: []string{"infra/**"}, Quality: "cost", Ask: true}}
	return c
}
func change(p, status string) gitx.Changes {
	return gitx.Changes{Files: []gitx.File{{Path: p, Status: status}}, AddedDirectories: []string{}, RemovedDirectories: []string{}}
}
func row(q string, touches ...string) doc { return doc{"quality": q, "touches": touches} }
func criterion(verdict string) doc {
	return doc{"aspect": "security", "verdict": verdict, "checked": "source", "not_checked": ""}
}
func finding(extra doc) doc {
	f := doc{"id": "security-1", "blocking": true, "needs_decision": false, "deferred": false, "mechanical": false, "failure_scenario": "A caller reads another user's data", "evidence": "security.md", "file": "a.js", "line": "1", "fix": "Check access"}
	for k, v := range extra {
		f[k] = v
	}
	return f
}
func inputs(extra doc) doc {
	d := doc{"prepare": doc{"stopped": false}, "changes": doc{"unlisted": []doc{}}, "gate": doc{"passed": true, "failing": []string{}, "results": "checks/1.jsonl"}, "completion": doc{"missing": []doc{}}, "findings": doc{"judges": []doc{{"judge": "scope", "criteria": []doc{criterion("pass")}, "findings": []doc{}}}}}
	for k, v := range extra {
		d[k] = v
	}
	return d
}
func TestDecisionDetectionNeedsPlanOnlyForSkippedPlanAndReturnsUnlistedToPlan(t *testing.T) {
	changes := change("infra/service.json", "M")
	skipped := summary(must(concerns.Load("../../kit/.devkit/standards")), testConfig(), changes, nil)
	testutil.Equal(t, skipped["proceed"], true)
	testutil.Equal(t, docs(skipped["decisions"])[0]["ask"], true)
	testutil.Equal(t, summary(must(concerns.Load("../../kit/.devkit/standards")), testConfig(), changes, nil)["proceed"], true)
	missing := summary(must(concerns.Load("../../kit/.devkit/standards")), testConfig(), changes, doc{"decisions": []doc{}})
	testutil.Equal(t, missing["proceed"], false)
	testutil.Equal(t, missing["unlisted_count"], 1)
	testutil.Equal(t, summary(must(concerns.Load("../../kit/.devkit/standards")), testConfig(), changes, doc{"decisions": []doc{row("cost", "infra")}})["proceed"], true)
}
func TestPathsCoverFilesOrFoldersWithoutPrefixCollisionOrGlobs(t *testing.T) {
	for _, c := range []struct {
		touch, path string
		want        bool
	}{{"src/a/", "src/a/file.js", true}, {"src/a", "src/another.js", false}, {"src/*.js", "src/a.js", false}, {"./src/a.js", "src/a.js", true}, {".", "src/a.js", true}} {
		testutil.Equal(t, covers(c.touch, c.path), c.want)
	}
}
func TestFolderChangesAreStructuralDecisionsAndMovesLaunchStructure(t *testing.T) {
	changes := gitx.Changes{Files: []gitx.File{{Status: "R100", OldPath: "old/a.js", Path: "new/a.js"}}, AddedDirectories: []string{"new"}, RemovedDirectories: []string{"old"}}
	r := summary(must(concerns.Load("../../kit/.devkit/standards")), testConfig(), changes, doc{"decisions": []doc{row("structure", "new", "old")}})
	testutil.Equal(t, r["proceed"], true)
	testutil.Equal(t, len(docs(r["decisions"])), 2)
	for _, d := range docs(r["decisions"]) {
		testutil.Equal(t, d["kind"], "directory")
		testutil.Equal(t, d["ask"], false)
	}
	testutil.Equal(t, r["launch_structure"], true)
}
func TestQualitySelectionCombinesGlobsRowsAndDiff(t *testing.T) {
	c := testConfig()
	c.Qualities["security"] = config.Quality{Paths: []string{"src/auth.js"}}
	r := summary(must(concerns.Load("../../kit/.devkit/standards")), c, change("src/auth.js", "M"), doc{"decisions": []doc{row("data", "src/model.js")}})
	for _, q := range []string{"security", "data", "scope"} {
		testutil.Equal(t, r["launch_"+q], true)
	}
	testutil.Equal(t, r["launch_ui"], false)
}
func TestCompletionChecksCommittedFilesAndNamesRatherThanCheckoutOrDirectories(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, "tests/a.test.js", `test("stores an order", () => {})`)
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "test")
	g := must(gitx.New(repo, ""))
	commit := must(g.Head())
	testutil.Put(t, repo, "tests/a.test.js", "test removed from checkout")
	entry := doc{"criterion": "C1", "method": "test", "file": "tests/a.test.js", "name": "stores an order"}
	testutil.Equal(t, must(completion(doc{"tests": []doc{entry}}, commit, g))["proceed"], true)
	for _, c := range []struct{ key, value, status string }{{"name", "missing", "missing_name"}, {"file", "missing.js", "missing_file"}, {"file", "tests", "missing_file"}} {
		e := clone(entry)
		e[c.key] = c.value
		testutil.Equal(t, docs(must(completion(doc{"tests": []doc{e}}, commit, g))["missing"])[0]["status"], c.status)
	}
	testutil.Equal(t, must(completion(doc{"tests": []doc{{"criterion": "C2", "method": "owner_check"}}}, commit, g))["missing"], []doc{})
	testutil.Equal(t, must(completion(nil, commit, g))["checked_count"], 0)
}
func TestCollectionRejectsMissingJudgesAndPreservesFindings(t *testing.T) {
	_, err := gather(doc{}, doc{"launch_scope": true})
	testutil.Fails(t, err, "scope did not finish")
	r := must(gather(doc{"scope": doc{"criteria": []doc{criterion("pass")}, "findings": []doc{finding(nil)}}}, doc{"launch_scope": true}))
	f := docs(r[0]["findings"])[0]
	testutil.Equal(t, f["id"], "scope-1")
	testutil.Equal(t, f["blocking"], true)
}
func TestFinishRequiresMachineNodesOnlyAfterChangesProceed(t *testing.T) {
	testutil.Equal(t, missingStep(doc{"prepare": doc{"stopped": true}}), "")
	testutil.Equal(t, missingStep(doc{"prepare": doc{}, "changes": doc{"proceed": false}}), "")
	complete := doc{"prepare": doc{}, "changes": doc{"proceed": true}, "gate": doc{}, "completion": doc{}, "collect": doc{}}
	for key := range complete {
		input := clone(complete)
		input[key] = nil
		want := key
		if key == "gate" {
			want = "checks"
		}
		testutil.Equal(t, missingStep(input), want)
	}
	testutil.Equal(t, missingStep(complete), "")
}
func TestFinishPreservesPlanNeededStopAndPlanBackBeforeJudging(t *testing.T) {
	_, out := decide(inputs(doc{"prepare": doc{"stopped": true, "reason": "dirty", "details": []string{}}}))
	testutil.Equal(t, out["outcome"], "stop")
	c := doc{"path": "new", "status": "A", "quality": "structure"}
	_, out = decide(inputs(doc{"changes": doc{"unlisted": []doc{c}}}))
	testutil.Equal(t, object(out["back"])["to"], "plan")
}
func TestFailedChecksAndMissingTestsBlockWithoutDiscardingJudges(t *testing.T) {
	judgments, out := decide(inputs(doc{"gate": doc{"passed": false, "failing": []string{"compile"}, "results": "checks/1.jsonl"}, "completion": doc{"missing": []doc{{"criterion": "C1", "file": "a.test.js", "name": "stores an order", "status": "missing_file"}}}, "findings": doc{"judges": []doc{{"judge": "security", "criteria": []doc{criterion("fail")}, "findings": []doc{finding(nil)}}}}}))
	testutil.Equal(t, out["outcome"], "stop")
	for _, j := range judgments {
		if j["judge"] == "security" {
			testutil.Equal(t, len(docs(j["findings"])), 1)
		}
		if j["judge"] == "test" {
			testutil.Equal(t, j["verdict"], "fail")
			testutil.Equal(t, len(docs(j["findings"])), 2)
			for _, f := range docs(j["findings"]) {
				testutil.Equal(t, f["blocking"], true)
			}
		}
	}
}
func TestOwnerFindingsAskAndBlockingOrUnassessedCriteriaPreventPass(t *testing.T) {
	for _, q := range qualities() {
		t.Run(q, func(t *testing.T) {
			for _, c := range []struct {
				findings      []doc
				verdict, want string
			}{{[]doc{finding(nil)}, "fail", "stop"}, {[]doc{finding(doc{"needs_decision": true})}, "fail", "back"}, {[]doc{}, "not_assessed", "stop"}, {[]doc{}, "fail", "stop"}, {[]doc{finding(doc{"blocking": false})}, "pass", "pass"}} {
				_, out := decide(inputs(doc{"findings": doc{"judges": []doc{{"judge": q, "criteria": []doc{criterion(c.verdict)}, "findings": c.findings}}}}))
				testutil.Equal(t, out["outcome"], c.want)
			}
		})
	}
}
func TestSkippedChecksStayVisibleWithoutFailingJudgment(t *testing.T) {
	judgments, out := decide(inputs(doc{"gate": doc{"passed": true, "failing": []string{}, "skipped": []string{"browser"}, "results": "checks/1.jsonl"}}))
	testutil.Equal(t, out["outcome"], "pass")
	for _, j := range judgments {
		if j["judge"] == "test" {
			testutil.Equal(t, docs(j["findings"])[0]["id"], "skipped-browser")
		}
	}
}
func TestPrepareRejectsStalePlansAndAcceptsObjectionsOnlyFromValidFix(t *testing.T) {
	testutil.Equal(t, planOf(doc{"version": 2}, doc{"requirements_version": 1}), nil)
	judgments := []doc{{"kind": "code", "judge": "dispute", "run_id": "fix-1", "target": doc{"commit": "c"}, "findings": []doc{{"id": "F1", "blocking": false, "text": "accepted", "evidence": "proof"}}}}
	testutil.Equal(t, len(acceptedObjections([]doc{{"workflow": "fix", "run_id": "fix-1"}}, judgments)), 1)
	testutil.Equal(t, len(acceptedObjections(nil, judgments)), 0)
}
func runCLI(t *testing.T, repo string, vars map[string]string, args ...string) doc {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, Environ: os.Environ()}
	if code := cli.New("test", Commands()).Execute(append([]string{"judge"}, args...), env); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var out doc
	testutil.OK(t, json.Unmarshal(stdout.Bytes(), &out))
	return out
}
func jsonText(v any) string { return string(must(json.Marshal(v))) }
func TestPrepareAndChangesPinCustomCriteriaChecksLanguageAndGitFacts(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Put(t, repo, ".devkit/config.json", `{"language":"fr","standards":"standards","checks":[{"name":"verify","command":"true","timeout_seconds":10}]}`)
	testutil.Standards(t, repo, "standards")
	testutil.Put(t, repo, "standards/authority.md", "fixed authority")
	testutil.Put(t, repo, "standards/structure/extra.md", "fixed structure")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "criteria")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Put(t, repo, "new/a.js", "export const a = 1")
	testutil.Put(t, repo, "standards/authority.md", "changed authority")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "change")
	testutil.Git(t, repo, "branch", "trunk", "main")
	testutil.Git(t, repo, "branch", "-D", "main")
	state, artifacts := t.TempDir(), t.TempDir()
	g := must(gitx.New(repo, "trunk"))
	ctx := record.ContextData{Dir: record.RecordDir(state, "feature/x"), RunID: "judge-1", Branch: "feature/x", Cwd: repo, BaseBranch: "trunk"}
	must(record.Start(ctx.Dir, "judge", record.StartOptions{RunID: ctx.RunID, Commit: must(g.Head()), BaseCommit: base}))
	p := must(prepare(ctx, artifacts, artifacts))
	testutil.Equal(t, p["stopped"], false)
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(artifacts, "criteria/standards/authority.md")))), "fixed authority")
	out := must(writeChanges(repo, artifacts))
	testutil.Equal(t, out["proceed"], true)
	material := must(read(filepath.Join(artifacts, "judge/materials/structure.json")))
	testutil.Equal(t, material["language"], "fr")
	testutil.Equal(t, material["standards"], "standards")
	testutil.Equal(t, docs(material["checks"])[0]["name"], "verify")
	testutil.Contains(t, jsonText(material["criteria"]), "review/structure.md")
	found := false
	for _, c := range docs(material["without_plan"]) {
		if c["path"] == "new" {
			found = true
		}
	}
	testutil.Equal(t, found, true)
	facts := object(material["facts"])
	testutil.Contains(t, jsonText(facts["addedDirectories"]), "new")
	found = false
	for _, s := range docs(facts["siblings"]) {
		if s["folder"] == "new" {
			testutil.Contains(t, jsonText(s["commit"]), "a.js")
			found = true
		}
	}
	testutil.Equal(t, found, true)
	_, err := os.Stat(filepath.Join(artifacts, "judge/source/commit/new/a.js"))
	testutil.OK(t, err)
	testutil.Equal(t, must(record.List(ctx.Dir, "request"))[0]["criteria_commit"], base)
	testutil.OK(t, save(filepath.Join(artifacts, "judge/completion.json"), doc{"missing": []doc{}}))
	testutil.OK(t, save(filepath.Join(artifacts, "judge/findings.json"), doc{"judges": []doc{{"judge": "scope", "criteria": []doc{criterion("pass")}, "findings": []doc{}}}}))
	vars := map[string]string{"STATE_DIR": state, "WORKFLOW_ID": "judge-1", "ARTIFACTS_DIR": artifacts, "BASE_BRANCH": "trunk", "PREPARE": jsonText(must(prepare(ctx, artifacts, artifacts))), "CHANGES": jsonText(out), "GATE": `{"passed":true,"failing":[],"results":"checks/1.jsonl"}`, "COMPLETION": "{}", "COLLECT": "{}"}
	testutil.Equal(t, runCLI(t, repo, vars, "completion")["missing_count"], 0)
	result := runCLI(t, repo, vars, "finish")
	testutil.Equal(t, result["outcome"], "pass")
	judgments := must(record.List(ctx.Dir, "judgment"))
	testutil.Equal(t, len(judgments), 2)
	testutil.Equal(t, object(judgments[0]["target"])["change_id"], *must(g.ChangeID(must(g.Head()))))
	testutil.Put(t, repo, "dirty", "uncommitted")
	dirty := t.TempDir()
	testutil.Equal(t, must(prepare(ctx, dirty, dirty))["stopped"], true)
}

func TestCodeSelectionExcludesPlanOnlyQuality(t *testing.T) {
	mapping := must(concerns.Load("../../kit/.devkit/standards"))
	c := testConfig()
	c.Qualities["delivery"] = config.Quality{Paths: []string{"src/**"}}
	out := summary(mapping, c, change("src/test.ext", "M"), doc{"decisions": []doc{row("delivery", "src")}})
	_, present := out["launch_delivery"]
	testutil.Equal(t, present, false)
	testutil.Equal(t, out["launch_scope"], true)
}
func TestPrepareRejectsInvalidFixedConcernMap(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, ".devkit/standards/concerns.json", `{}`)
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "standards")
	base := testutil.Git(t, repo, "rev-parse", "HEAD")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	ctx := record.ContextData{Cwd: repo, Dir: t.TempDir(), RunID: "judge-invalid", BaseBranch: "main"}
	must(record.Start(ctx.Dir, "judge", record.StartOptions{RunID: ctx.RunID, Commit: base, BaseCommit: base}))
	_, err := prepare(ctx, t.TempDir(), "artifacts")
	testutil.Fails(t, err, "missing quality: scope")
}

func TestPreexistingCheckIsReportedAndStopsInsteadOfFix(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		failing := []string{"unit"}
		if mixed {
			failing = append(failing, "regressed")
		}
		judgments, out := decide(inputs(doc{"gate": doc{"passed": false, "failing": failing, "preexisting": []string{"unit"}, "results": "checks/1.jsonl", "checks": []doc{{"name": "unit", "status": "fail", "base": "failed", "log": "/checks/unit.log", "base_log": "/checks/base/unit.log"}}}}))
		testutil.Equal(t, out["outcome"], "stop")
		stop := object(out["stop"])
		testutil.Equal(t, stop["base_failed"], true)
		testutil.Contains(t, text(stop["state"]), "the base branch fails unit already")
		testutil.Equal(t, stop["unresolved"], []string{"unit: /checks/unit.log"})
		for _, j := range judgments {
			if j["judge"] != "test" {
				continue
			}
			testutil.Equal(t, j["verdict"], "fail")
			testutil.Contains(t, text(docs(j["findings"])[0]["text"]), "fails on the base branch too")
			testutil.Contains(t, strings.Join(stringsOf(j["observations"]), "\n"), "base: failed / /checks/base/unit.log")
		}
	}
}
