package plan

import (
	"bytes"
	"encoding/json"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/decisions"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func must[T any](v T, e error) T { return testutil.Must(v, e) }
func sha(c string) string        { return strings.Repeat(c, 40) }
func condition() document {
	return document{"completion_conditions": []document{{"id": "C1", "text": "A result is observable"}}}
}
func decision(quality string) document {
	return document{"id": "D1", "what": "Place the entry point", "chosen": "Keep its current module", "candidates": []string{"Keep", "Move"}, "principles": []string{"Keep the existing boundary"}, "evidence": []document{{"source": "author", "text": "src/entry.ext"}}, "decided_by": "principle", "when": "now", "decide_when": "", "nature": document{"kind": "unknown", "reasons_for_change": "unknown", "volatility": "unknown", "callers": "unknown"}, "quality": quality, "touches": []string{"src/entry.ext"}}
}
func draft() document {
	return document{"decisions": []document{decision("structure")}, "increments": []document{{"id": "U1", "summary": "Reach an observable result", "criteria": []string{"C1"}}}, "tests": []document{{"criterion": "C1", "method": "test", "file": "tests/entry.ext", "name": "reaches the result"}}, "kept_behaviors": []string{}}
}
func settled() document {
	return document{"requirements_version": 1, "round": 3, "valid": true, "errors": []string{}, "skeleton_commit": sha("a"), "draft": draft(), "changes": []document{}, "prototype": "", "structure_question": "May work proceed with this structure?", "judgments": document{}, "needs_decision": []document{}, "not_assessed": []document{}, "fixable": []document{}, "back": nil}
}
func finding(extra document) document {
	d := document{"row": "D1", "blocking": true, "needs_decision": false, "deferred": false, "mechanical": false, "failure_scenario": "The entry point does not reach the result", "evidence": "src/entry.ext", "fix": "Wire the entry point"}
	for k, v := range extra {
		d[k] = v
	}
	return d
}
func judgment(extra document) document {
	d := document{"verdict": "pass", "checked": "Decision and skeleton", "not_checked": "", "findings": []document{}}
	for k, v := range extra {
		d[k] = v
	}
	return d
}
func repository(t *testing.T, c document) (record.ContextData, string) {
	t.Helper()
	repo := testutil.Repo(t)
	testutil.Git(t, repo, "config", "user.name", "test")
	testutil.Git(t, repo, "config", "user.email", "test@example.invalid")
	testutil.Git(t, repo, "config", "commit.gpgsign", "false")
	testutil.Git(t, repo, "config", "core.hooksPath", "/dev/null")
	testutil.Put(t, repo, "src/entry.ext", "existing entry\n")
	testutil.Put(t, repo, "src/peer.ext", "existing peer\n")
	testutil.OK(t, write(filepath.Join(repo, ".devkit/config.json"), c))
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "base")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	return record.ContextData{Cwd: repo, BaseBranch: "main", RunID: "p1", Branch: "feature/x"}, t.TempDir()
}
func inputFiles(t *testing.T, artifacts string) {
	t.Helper()
	r := condition()
	r["goal"] = "Change src/entry.ext"
	r["version"] = 1
	r["constraints"] = []string{}
	r["open_points"] = []document{}
	testutil.Standards(t, artifacts, "criteria/rules")
	mapping := must(concerns.Load(filepath.Join(artifacts, "criteria/rules")))
	selected := document{}
	for _, q := range mapping.Judged("plan") {
		data := must(json.Marshal(mapping.Materials(filepath.Join(artifacts, "criteria/rules"), q, "plan")))
		var material document
		testutil.OK(t, json.Unmarshal(data, &material))
		selected[q] = material
	}
	testutil.OK(t, write(filepath.Join(artifacts, "plan/input.json"), document{"language": "fr", "standards": "rules", "qualities": mapping.Judged("plan"), "quality_materials": selected, "criteria_folder": filepath.Join(artifacts, "criteria/rules"), "requirements": r, "answers": []document{}}))
	testutil.OK(t, write(filepath.Join(artifacts, "plan/draft.json"), draft()))
}
func request(dir string) {
	must(record.Append(dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": sha("c"), "run_id": "r0", "recorded_at": "t"}))
}
func requirements(dir, id string) {
	r := condition()
	r["request"] = 1
	r["run_id"] = id
	r["recorded_at"] = "t"
	r["goal"] = "Goal"
	r["constraints"] = []string{}
	r["open_points"] = []document{}
	must(record.Append(dir, "requirements", r))
}
func run(dir, id string) {
	must(record.Append(dir, "run", document{"run_id": id, "workflow": "plan", "started_at": "t", "start": "other", "counted": false, "answers": []string{}}))
}
func errorsText(d document) string { return strings.Join(must(draftErrors(d, condition(), "")), "\n") }

func TestDraftSchemaAndCoverage(t *testing.T) {
	testutil.Equal(t, must(draftErrors(draft(), condition(), "")), []string{})
	testutil.Contains(t, errorsText(nil), "missing")
	noTest := draft()
	noTest["tests"] = []document{}
	testutil.Contains(t, errorsText(noTest), "no test-plan entry")
	entries(noTest["increments"])[0]["criteria"] = []string{"C2"}
	testutil.Contains(t, errorsText(noTest), "no release unit")
	testutil.Contains(t, errorsText(noTest), "C2 is not")
	for _, field := range []string{"decisions", "increments"} {
		duplicate := draft()
		duplicate[field] = append(entries(duplicate[field]), strip(entries(duplicate[field])[0]))
		testutil.Contains(t, errorsText(duplicate), "used twice")
	}
	invalid := draft()
	entries(invalid["decisions"])[0]["touches"] = []string{}
	if errorsText(invalid) == "" {
		t.Fatal("empty touches accepted")
	}
	invalid["decisions"] = []document{decision("unknown")}
	if errorsText(invalid) == "" {
		t.Fatal("unknown quality accepted")
	}
	later := draft()
	entries(later["decisions"])[0]["when"] = "later"
	if errorsText(later) == "" {
		t.Fatal("later without decide_when accepted")
	}
}
func TestPathFacts(t *testing.T) {
	tracked := []string{"README.md", "src/main.ext", "src/nested/child.ext", "src/nested/deep/hidden.ext", "other/peer.ext", "space folder/file.ext"}
	paths := pathsIn([]string{"Update `src`, missing/new.ext and README.md. Also `space folder/file.ext`."}, tracked)
	names := []string{}
	for _, p := range paths {
		names = append(names, text(p["path"]))
	}
	testutil.Equal(t, names, []string{"README.md", "missing/new.ext", "space folder/file.ext", "src"})
	facts := object(factsOf(paths, tracked)["paths"])
	testutil.Equal(t, object(facts["missing/new.ext"])["exists"], false)
	testutil.Equal(t, object(object(facts["missing/new.ext"])["parent"])["exists"], false)
	testutil.Equal(t, object(object(facts["README.md"])["parent"])["path"], ".")
	testutil.Contains(t, strings.Join(stringsOf(object(facts["src"])["siblings"]), ","), "other")
	for _, p := range entries(object(facts["src"])["tree"]) {
		if p["path"] == "src/nested" {
			testutil.Equal(t, p["entries"], []string{"src/nested/child.ext", "src/nested/deep"})
		}
	}
	testutil.Equal(t, strings.Contains(string(must(json.Marshal(object(facts["src"])["tree"]))), "hidden.ext"), false)
	testutil.Equal(t, len(filter(pathsIn([]string{"prefixsrc/main.extsuffix"}, tracked), func(d document) bool { return d["path"] == "src/main.ext" })), 0)
}
func TestFactsReadHEAD(t *testing.T) {
	ctx, artifacts := repository(t, document{})
	inputFiles(t, artifacts)
	testutil.Put(t, ctx.Cwd, "src/new.ext", "new\n")
	testutil.Git(t, ctx.Cwd, "add", "src/new.ext")
	testutil.Put(t, ctx.Cwd, "src/untracked.ext", "new\n")
	out := must(factsCommand(ctx, artifacts, ""))
	facts := must(read(text(out["path"])))
	testutil.Equal(t, facts["commit"], testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"))
	testutil.Equal(t, object(object(facts["paths"])["src/entry.ext"])["siblings"], []string{"src/peer.ext"})
}
func TestJudgeMaterials(t *testing.T) {
	input := document{"language": "fr", "standards": "rules", "criteria_folder": "/fixed/rules", "requirements": condition(), "answers": []document{}, "previous": document{"findings": []document{{"judge": "data", "id": "old-data"}, {"judge": "security", "id": "old-security"}}}}
	p := draft()
	d := decision("data")
	d["id"] = "D2"
	p["decisions"] = append(entries(p["decisions"]), d)
	testutil.Equal(t, qualitiesOf(p), []string{"structure", "data"})
	args := document{"items": []document{{"quality": "data"}, {"quality": "security"}}}
	data := materials("data", input, p, document{}, document{"before": "before", "after": "after"}, args)
	testutil.Equal(t, entries(data["decisions"])[0]["id"], "D2")
	testutil.Equal(t, data["changes"], []document{{"quality": "data"}})
	testutil.Equal(t, data["facts"], nil)
	testutil.Equal(t, data["language"], "fr")
	for _, p := range stringsOf(data["criteria"]) {
		if !strings.HasPrefix(p, "/fixed/rules/") {
			t.Fatal(p)
		}
	}
	testutil.Equal(t, entries(data["previous_findings"])[0]["id"], "old-data")
	testutil.Equal(t, len(entries(materials("delivery", input, p, document{}, document{}, args)["decisions"])), 2)
}
func TestSkeletonCheckSelection(t *testing.T) {
	ctx, a := repository(t, document{"checks": []document{{"name": "unselected", "command": "exit 1", "timeout_seconds": 5}}, "skeleton_check": []string{}})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	skipped := must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, skipped["valid"], true)
	_, err := os.Stat(filepath.Join(a, "checks/round"))
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.Equal(t, skipped["structure"], true)
	testutil.Equal(t, skipped["data"], false)
	ctx, a = repository(t, document{"checks": []document{{"name": "skeleton", "command": "exit 0", "timeout_seconds": 5}, {"name": "other", "command": "exit 1", "timeout_seconds": 5}}, "skeleton_check": []string{"skeleton"}})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	passed := must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, passed["valid"], true)
	lines := strings.Split(strings.TrimSpace(string(must(os.ReadFile(filepath.Join(a, "checks/1.jsonl"))))), "\n")
	names := []string{}
	for _, line := range lines {
		var d document
		testutil.OK(t, json.Unmarshal([]byte(line), &d))
		names = append(names, text(d["name"]))
	}
	testutil.Equal(t, names, []string{"skeleton"})
	ctx, a = repository(t, document{"checks": []document{{"name": "skeleton", "command": "exit 1", "timeout_seconds": 5}}, "skeleton_check": []string{"skeleton"}})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	marker := filepath.Join(t.TempDir(), "prepared")
	testutil.Put(t, ctx.Cwd, ".archon/config.yaml", "worktree:\n  postCreate:\n    - touch '"+marker+"'\n")
	blocked := must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, blocked["valid"], false)
	testutil.Equal(t, blocked["structure"], false)
	testutil.Contains(t, strings.Join(stringsOf(blocked["errors"]), ","), "Skeleton checks did not pass")
	_, err = os.Stat(marker)
	testutil.Equal(t, os.IsNotExist(err), true)
	testutil.Equal(t, must(read(filepath.Join(text(blocked["materials"]), "state.json")))["judges"], []string{})
}
func TestChangeDetection(t *testing.T) {
	ctx, _ := repository(t, document{})
	base := testutil.Git(t, ctx.Cwd, "rev-parse", "main")
	testutil.Git(t, ctx.Cwd, "mv", "src/entry.ext", "src/renamed.ext")
	testutil.Put(t, ctx.Cwd, "new-folder/new.ext", "new file\n")
	testutil.Git(t, ctx.Cwd, "add", "-A")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "skeleton")
	c := config.Defaults("")
	c.DecisionPaths = []config.DecisionRule{{Paths: []string{"src/**"}, Quality: "contract", Ask: true}}
	changes := must(collectChanges(ctx, base, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), c))
	items := entries(changes["items"])
	for _, predicate := range []func(document) bool{func(i document) bool {
		return i["quality"] == "contract" && i["ask"] == true && i["oldPath"] == "src/entry.ext"
	}, func(i document) bool {
		return i["quality"] == "structure" && i["file"] == "new-folder" && i["kind"] == "directory"
	}, func(i document) bool { return i["quality"] == "structure" && i["oldPath"] == "src/entry.ext" }} {
		if len(filter(items, predicate)) == 0 {
			t.Fatal(items)
		}
	}
}
func TestSettle(t *testing.T) {
	a := t.TempDir()
	state := settled()
	state["judges"] = []string{"structure", "delivery"}
	check := func(outputs map[string]string, prototype string) document {
		testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
		return must(settle(a, outputs, prototype, ""))
	}
	encode := func(d document) string { return string(must(json.Marshal(d))) }
	outputs := map[string]string{"structure": encode(judgment(nil)), "delivery": encode(judgment(nil))}
	testutil.Equal(t, check(outputs, "")["done"], true)
	outputs["structure"] = "not JSON"
	testutil.Equal(t, check(outputs, "")["not_assessed_count"], 1)
	outputs["structure"] = ""
	testutil.Equal(t, check(outputs, "")["done"], false)
	outputs["structure"] = encode(judgment(document{"verdict": "fail", "findings": []document{finding(nil)}}))
	testutil.Equal(t, check(outputs, "")["done"], false)
	outputs["structure"] = encode(judgment(document{"findings": []document{finding(document{"needs_decision": true})}}))
	testutil.Equal(t, check(outputs, "")["done"], true)
	outputs["delivery"] = encode(judgment(document{"back": document{"needed": true, "premise": "Missing feature", "observation": "No feature found"}}))
	testutil.Equal(t, check(outputs, "")["back"], true)
	outputs["delivery"] = encode(judgment(nil))
	testutil.Equal(t, check(outputs, "Measure compatibility")["done"], true)
}
func TestOwnerQuestions(t *testing.T) {
	state := settled()
	entries(object(state["draft"])["decisions"])[0]["decided_by"] = "owner"
	f := finding(document{"needs_decision": true, "judge": "security"})
	state["needs_decision"] = []document{f}
	state["changes"] = []document{{"quality": "structure", "file": "src/new.ext", "change": "A file"}}
	qs := ownerQuestions(state, nil)
	kinds := []string{}
	answers := []document{}
	for _, q := range qs {
		kinds = append(kinds, text(object(q["target"])["kind"]))
		answers = append(answers, document{"target": q["target"], "versions": document{"requirements_version": state["requirements_version"]}})
	}
	testutil.Equal(t, kinds, []string{"decision", "finding", "change"})
	testutil.Equal(t, ownerQuestions(state, answers), []document{})
	entries(state["changes"])[0]["file"] = "src/other.ext"
	testutil.Equal(t, len(ownerQuestions(state, answers)), 1)
	entries(object(state["draft"])["decisions"])[0]["when"] = "later"
	testutil.Equal(t, object(ownerQuestions(state, nil)[0]["target"])["kind"], "finding")
}
func TestOutcomePrecedence(t *testing.T) {
	state := settled()
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "pass")
	state["fixable"] = []document{finding(document{"judge": "structure"})}
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "carry")
	state["not_assessed"] = []document{{"judge": "data", "not_checked": "Missing source"}}
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "stop")
	entries(object(state["draft"])["decisions"])[0]["decided_by"] = "owner"
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "question")
	state["back"] = document{"premise": "Invalid premise", "observation": "Observed fact"}
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "back")
	state["valid"] = false
	state["errors"] = []string{"Invalid schema"}
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "stop")
	state["prototype"] = "Measure compatibility"
	testutil.Equal(t, outcomeOf(state, nil)["outcome"], "question")
}
func TestFinishPersistsValidPlanOnly(t *testing.T) {
	a := t.TempDir()
	dir := filepath.Join(a, "record")
	request(dir)
	requirements(dir, "r1")
	run(dir, "p1")
	inputFiles(t, a)
	state := settled()
	object(state["judgments"])["structure"] = judgment(document{"verdict": "fail", "findings": []document{finding(nil)}})
	state["fixable"] = []document{finding(document{"judge": "structure"})}
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
	ctx := record.ContextData{Dir: dir, RunID: "p1"}
	o := record.ResultOptions{Base: sha("b"), Commit: sha("c"), Now: "t"}
	result := must(finishTestPlan(t, ctx, a, o))
	testutil.Equal(t, result["outcome"], "carry")
	testutil.Equal(t, must(record.Latest(dir, "plan"))["skeleton_commit"], state["skeleton_commit"])
	testutil.Equal(t, must(record.Latest(dir, "judgment"))["kind"], "plan")
	testutil.Equal(t, must(record.Latest(dir, "result"))["outcome"], "carry")
	ctx.RunID = "p2"
	run(dir, "p2")
	state["valid"] = false
	state["errors"] = []string{"Invalid schema"}
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
	testutil.Equal(t, must(finishTestPlan(t, ctx, a, o))["outcome"], "stop")
	testutil.Equal(t, len(must(record.List(dir, "plan"))), 1)
}
func TestCheckCommitsSkeletonAndAsksStructure(t *testing.T) {
	ctx, a := repository(t, document{})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	testutil.Put(t, ctx.Cwd, "src/new.ext", "fail loudly\n")
	old := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	checked := must(checkCommand(ctx, a, nil, "", "structure"))
	testutil.Equal(t, checked["valid"], true)
	if checked["skeleton_commit"] == old {
		t.Fatal("skeleton was not committed")
	}
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain"), "")
	state := must(read(filepath.Join(text(checked["materials"]), "state.json")))
	if len(filter(entries(state["changes"]), func(i document) bool { return i["file"] == "src/new.ext" })) == 0 {
		t.Fatal(state)
	}
	j := string(must(json.Marshal(judgment(nil))))
	must(settle(a, map[string]string{"structure": j, "delivery": j}, "", "May work proceed with the proposed structure?"))
	out := outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil)
	testutil.Equal(t, out["outcome"], "question")
	q := text(entries(out["questions"])[0]["text"])
	testutil.Equal(t, strings.HasPrefix(q, "May work proceed with the proposed structure?"), true)
	testutil.Contains(t, q, "src/new.ext")
}
func TestPrepareCustomStandardsAndCurrentRequirements(t *testing.T) {
	ctx, a := repository(t, document{"language": "fr", "standards": "rules"})
	testutil.Standards(t, ctx.Cwd, "rules")
	testutil.Git(t, ctx.Cwd, "add", "-A")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "standards")
	ctx.Dir = t.TempDir()
	ctx.RunID = "p1"
	_, err := prepare(ctx, a, "")
	testutil.Fails(t, err, "no requirements")
	must(record.Append(ctx.Dir, "request", document{"owner_words": "", "supplement": "Issue text supplied by the session", "criteria_commit": testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), "run_id": "r1", "recorded_at": "t"}))
	requirements(ctx.Dir, "r1")
	p := draft()
	p["requirements_version"] = 1
	p["run_id"] = "p1"
	p["recorded_at"] = "t"
	p["skeleton_commit"] = sha("e")
	must(record.Append(ctx.Dir, "plan", p))
	run(ctx.Dir, ctx.RunID)
	input := must(read(text(must(prepare(ctx, a, ""))["input"])))
	testutil.Equal(t, input["language"], "fr")
	testutil.Equal(t, input["standards"], "rules")
	testutil.Equal(t, entries(input["requests"])[0]["supplement"], "Issue text supplied by the session")
	testutil.Equal(t, input["qualities"], must(concerns.Load(filepath.Join(ctx.Cwd, "rules"))).Judged("plan"))
	testutil.Equal(t, object(object(input["previous"])["plan"])["skeleton_commit"], sha("e"))
	requirements(ctx.Dir, "r2")
	testutil.Equal(t, must(read(text(must(prepare(ctx, a, ""))["input"])))["previous"], nil)
}
func TestCommands(t *testing.T) {
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { return "", false }}
	registry := cli.New("test", Commands())
	testutil.Equal(t, registry.Execute([]string{"plan", "facts"}, env), 1)
	testutil.Contains(t, stderr.String(), "ARTIFACTS_DIR must be set")
	testutil.Equal(t, registry.Execute([]string{"plan", "not-started", `{"reason":"blocked","remaining":["answer"]}`}, env), 0)
	testutil.Contains(t, out.String(), `"reasons":["blocked","answer"]`)
}

func TestStructureReferenceMatchesOriginalJSON(t *testing.T) {
	changes := []document{{"quality": "structure", "file": "src/z.ext", "change": "A file"}, {"quality": "structure", "file": "src/a&<.ext", "change": "R100 file from src/old.ext"}}
	testutil.Equal(t, structureRef(changes), "structure:deb6d56036f2")
	testutil.Equal(t, pathsIn([]string{"`\ue000.ext` and `😀.ext`"}, nil), []document{{"path": "😀.ext", "kind": "missing"}, {"path": "\ue000.ext", "kind": "missing"}})
}
func TestCommandsKeepRelativePathsAndCheckEnvironment(t *testing.T) {
	ctx, a := repository(t, document{"checks": []document{{"name": "skeleton", "command": `test "$ARTIFACTS_DIR" = "$EXPECTED_ARTIFACTS" && test "$BASE_BRANCH" = main && test "$PRESERVED" = value`, "timeout_seconds": 5}}, "skeleton_check": []string{"skeleton"}})
	inputFiles(t, a)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), structureDraft()))
	relative := must(filepath.Rel(ctx.Cwd, a))
	vars := map[string]string{"PLAN_PHASE": "structure", "STATE_DIR": t.TempDir(), "WORKFLOW_ID": "p1", "ARTIFACTS_DIR": relative}
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: ctx.Cwd, Stdout: &out, Stderr: &stderr, Environ: append(os.Environ(), "EXPECTED_ARTIFACTS="+relative, "ARTIFACTS_DIR=wrong", "BASE_BRANCH=wrong", "PRESERVED=value"), LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	registry := cli.New("test", Commands())
	for _, mode := range []string{"facts", "check"} {
		out.Reset()
		if code := registry.Execute([]string{"plan", mode}, env); code != 0 {
			t.Fatalf("%s: %d: %s", mode, code, stderr.String())
		}
		var result document
		testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
		if mode == "facts" {
			testutil.Equal(t, result["path"], filepath.Join(relative, "plan/facts-before.json"))
		} else {
			testutil.Equal(t, result["valid"], true)
			testutil.Equal(t, result["materials"], filepath.Join(rounds(relative), "1"))
			materials := must(read(filepath.Join(rounds(a), "1/structure.json")))
			testutil.Equal(t, object(materials["skeleton"])["diff"], filepath.Join(rounds(relative), "1/skeleton.patch"))
			testutil.Equal(t, materials["research"], nil)
		}
	}
}

func TestPrepareRejectsInvalidFixedConcernMap(t *testing.T) {
	ctx, a := repository(t, document{})
	testutil.Standards(t, ctx.Cwd, ".devkit/standards")
	testutil.Put(t, ctx.Cwd, ".devkit/standards/concerns.json", `{}`)
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "standards")
	ctx.Dir = t.TempDir()
	must(record.Append(ctx.Dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"), "run_id": "r1", "recorded_at": "t"}))
	requirements(ctx.Dir, "r1")
	_, err := prepare(ctx, a, "")
	testutil.Fails(t, err, "missing quality: scope")
}

func TestSkeletonChangeLaunchesQualityWithMissingRow(t *testing.T) {
	TestF4DetailLaunchesDetectedQualityWithoutRows(t)
}

func TestOwnerAnswerIsBoundToRequirementsAndFullRowContent(t *testing.T) {
	state := settled()
	row := entries(object(state["draft"])["decisions"])[0]
	row["decided_by"] = "owner"
	q := ownerQuestions(state, nil)[0]
	answers := []document{{"target": q["target"], "versions": document{"requirements_version": 1}}}
	testutil.Equal(t, len(ownerQuestions(state, answers)), 0)
	row["chosen"] = "Move to a paid provider"
	testutil.Equal(t, len(ownerQuestions(state, answers)), 1)
	row["chosen"] = "Keep its current module"
	state["requirements_version"] = 2
	testutil.Equal(t, len(ownerQuestions(state, answers)), 1)
}

func TestPlanFailWithoutFindingsAndMissingJudgeNeverPass(t *testing.T) {
	for _, output := range []string{`{"verdict":"fail","checked":"Invalid","not_checked":"","findings":[]}`, `{"verdict":"not_assessed","checked":"","not_checked":"Missing","findings":[]}`, `{}`} {
		a := t.TempDir()
		state := settled()
		state["judges"] = []string{"structure"}
		testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
		r := must(settle(a, map[string]string{"structure": output}, "", ""))
		testutil.Equal(t, r["done"], false)
		if outcomeOf(must(read(filepath.Join(rounds(a), "latest.json"))), nil)["outcome"] == "pass" {
			t.Fatal("unpassed verdict passed")
		}
	}
}

func TestPassingPlanCommitsAgreedRecordsAndReferencesTheirSources(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Branch = "feature/x"
	ctx.Dir = t.TempDir()
	ctx.RunID = "p1"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	source := "---\nid: D-abcdef\ntitle: Existing\nstatus: agreed\ndecided_by: owner\nrequest: refine/a\ndepends_on: []\nsupersedes: []\n---\nKeep existing behavior\n"
	testutil.Put(t, ctx.Cwd, ".devkit/decisions/D-abcdef.md", source)
	testutil.Git(t, ctx.Cwd, "add", ".")
	testutil.Git(t, ctx.Cwd, "commit", "-qm", "source")
	state := settled()
	row := entries(object(state["draft"])["decisions"])[0]
	row["evidence"] = []document{{"source": "author", "text": "D-abcdef"}}
	reused := strip(row)
	reused["id"] = "D2"
	reused["ref"] = "D-abcdef"
	provisional := strip(row)
	provisional["id"] = "D3"
	provisional["decided_by"] = "provisional"
	object(state["draft"])["decisions"] = []document{row, reused, provisional}
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
	result := must(finishTestPlan(t, ctx, a, record.ResultOptions{Base: sha("b"), Commit: sha("c"), Now: "t"}))
	testutil.Equal(t, result["outcome"], "pass")
	data := must(decisions.List(ctx.Cwd))
	testutil.Equal(t, len(data.Decisions), 2)
	for _, d := range data.Decisions {
		if d["id"] != "D-abcdef" {
			testutil.Equal(t, d["status"], "agreed")
			testutil.Equal(t, d["request"], "feature/x")
			testutil.Equal(t, stringsOf(d["depends_on"]), []string{"D-abcdef"})
		}
	}
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "status", "--porcelain"), "")
	testutil.Equal(t, must(record.Latest(ctx.Dir, "plan"))["skeleton_commit"], testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD"))
	testutil.Equal(t, must(os.ReadFile(filepath.Join(ctx.Cwd, ".devkit/decisions/D-abcdef.md"))), []byte(source))
}

func TestOwnerRowBecomesAgreedOnlyAfterItsBoundAnswer(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Branch = "feature/x"
	ctx.Dir = t.TempDir()
	ctx.RunID = "p1"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	state := settled()
	entries(object(state["draft"])["decisions"])[0]["decided_by"] = "owner"
	testutil.OK(t, write(filepath.Join(rounds(a), "latest.json"), state))
	o := record.ResultOptions{Base: sha("b"), Commit: sha("c"), Now: "t"}
	first := must(finishTestPlan(t, ctx, a, o))
	testutil.Equal(t, first["outcome"], "question")
	testutil.Equal(t, len(must(decisions.List(ctx.Cwd)).Decisions), 0)
	ctx.RunID = "p2"
	run(ctx.Dir, ctx.RunID)
	id := text(entries(first["questions"])[0]["id"])
	testutil.OK(t, record.RecordAnswers(ctx.Dir, "plan", []record.Answer{{QuestionID: id, Answer: "Use the proposed choice"}}, ctx.RunID, "t"))
	second := must(finishTestPlan(t, ctx, a, o))
	testutil.Equal(t, second["outcome"], "pass")
	records := must(decisions.List(ctx.Cwd)).Decisions
	testutil.Equal(t, len(records), 1)
	testutil.Equal(t, records[0]["decided_by"], "owner")
}

func TestPlanChecksRequirePendingRowsAndAgreedReferences(t *testing.T) {
	ctx, a := repository(t, document{})
	inputFiles(t, a)
	completeStructure(t, ctx, a)
	input := must(read(filepath.Join(a, "plan/input.json")))
	input["pending_owner_rows"] = []document{{"id": "D9", "what": "Choose audience", "decided_by": "owner"}}
	testutil.OK(t, write(filepath.Join(a, "plan/input.json"), input))
	testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], false)
	d := draft()
	row := decision("data")
	row["id"] = "D9"
	row["decided_by"] = "owner"
	d["decisions"] = append(entries(d["decisions"]), row)
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), d))
	testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], true)
	row["ref"] = "D-abcdef"
	testutil.OK(t, write(filepath.Join(a, "plan/draft.json"), d))
	testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], false)
	input["agreed_decisions"] = []document{{"id": "D-abcdef", "status": "agreed"}}
	testutil.OK(t, write(filepath.Join(a, "plan/input.json"), input))
	testutil.Equal(t, must(checkCommand(ctx, a, nil, "", "detail"))["valid"], true)
	state := must(read(filepath.Join(rounds(a), "latest.json")))
	testutil.Equal(t, len(ownerQuestions(state, nil)), 0)
}

func TestAgreedRecordsAreInTheSkeletonCommit(t *testing.T) {
	ctx, a := repository(t, document{})
	ctx.Branch = "feature/x"
	ctx.Dir = t.TempDir()
	ctx.RunID = "p1"
	request(ctx.Dir)
	requirements(ctx.Dir, "r1")
	run(ctx.Dir, ctx.RunID)
	inputFiles(t, a)
	completeStructure(t, ctx, a)
	parent := testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	testutil.Put(t, ctx.Cwd, "src/entry.ext", "skeleton signature\n")
	must(checkCommand(ctx, a, nil, "", "detail"))
	j := string(must(json.Marshal(judgment(nil))))
	must(settle(a, map[string]string{"structure": j, "delivery": j}, "", ""))
	testutil.Equal(t, must(finishTestPlan(t, ctx, a, record.ResultOptions{Base: parent, Commit: parent}))["outcome"], "pass")
	testutil.Equal(t, testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD^"), parent)
	files := testutil.Git(t, ctx.Cwd, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	testutil.Contains(t, files, "src/entry.ext")
	testutil.Contains(t, files, ".devkit/decisions/D-")
}

func TestAgreedStructureReferencesDoNotAskTheSameChoiceAgain(t *testing.T) {
	state := settled()
	row := entries(object(state["draft"])["decisions"])[0]
	row["ref"] = "D-abcdef"
	row["decided_by"] = "owner"
	row["touches"] = []string{"src"}
	state["changes"] = []document{{"quality": "structure", "file": "src/new.ext", "change": "A file"}}
	testutil.Equal(t, len(ownerQuestions(state, nil)), 0)
	state["changes"] = append(entries(state["changes"]), document{"quality": "structure", "file": "other/new.ext", "change": "A file"})
	testutil.Equal(t, len(ownerQuestions(state, nil)), 1)
}

func completeStructure(t *testing.T, ctx record.ContextData, artifacts string) {
	t.Helper()
	if ctx.Dir == "" {
		ctx.Dir = t.TempDir()
	}
	testutil.OK(t, write(filepath.Join(artifacts, "plan/draft.json"), structureDraft()))
	must(checkCommand(ctx, artifacts, nil, "", "structure"))
	must(settle(artifacts, map[string]string{"structure": string(must(json.Marshal(judgment(nil))))}, "", ""))
	testutil.Equal(t, must(advance(ctx, artifacts))["proceed"], true)
	testutil.OK(t, write(filepath.Join(artifacts, "plan/draft.json"), draft()))
}

func finishTestPlan(t *testing.T, ctx record.ContextData, artifacts string, options record.ResultOptions) (document, error) {
	t.Helper()
	state := must(read(filepath.Join(rounds(artifacts), "latest.json")))
	state["phase"], state["run_id"] = "detail", ctx.RunID
	if ctx.Cwd != "" && state["skeleton_commit"] == sha("a") {
		state["skeleton_commit"] = testutil.Git(t, ctx.Cwd, "rev-parse", "HEAD")
	}
	object(state["judgments"])["delivery"] = judgment(nil)
	fixed := settled()
	fixed["phase"], fixed["run_id"] = "structure", ctx.RunID
	fixed["draft"] = structureDraft()
	fixed["judgments"] = document{"structure": judgment(nil)}
	testutil.OK(t, write(filepath.Join(artifacts, "plan/structure.json"), fixed))
	testutil.OK(t, write(filepath.Join(rounds(artifacts), "latest.json"), state))
	return finish(ctx, artifacts, options)
}
