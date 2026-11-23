package requirements

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
)

func must[T any](v T, e error) T { return testutil.Must(v, e) }
func requirements() document {
	return document{"goal": "Filter the list", "completion_conditions": []document{{"id": "C1", "text": "Only matching entries are shown"}}, "constraints": []string{}, "open_points": []document{}}
}
func judgment() document {
	cs := []document{}
	for _, a := range []string{"coverage", "additions", "open_points", "decidable"} {
		cs = append(cs, document{"aspect": a, "verdict": "pass", "checked": "Requests and answers", "not_checked": ""})
	}
	return document{"criteria": cs, "findings": []document{}}
}
func request(dir string) {
	must(record.Append(dir, "request", document{"owner_words": "Request", "supplement": "", "criteria_commit": strings.Repeat("c", 40), "run_id": "r0", "recorded_at": "t"}))
}
func run(dir, id string) {
	must(record.Append(dir, "run", document{"run_id": id, "workflow": "requirements", "started_at": "t", "start": "other", "counted": false, "answers": []string{}}))
}

func TestRequirementsOutcomes(t *testing.T) {
	e := document{"round": 3, "schema_errors": []string{}, "requirements": requirements(), "judgment": judgment()}
	testutil.Equal(t, outcomeOf(e)["outcome"], "pass")
	object(e["requirements"])["open_points"] = []document{{"id": "O1", "text": "Which filter should be used?"}}
	q := outcomeOf(e)
	testutil.Equal(t, q["outcome"], "question")
	testutil.Equal(t, entries(q["questions"])[0]["target"], document{"kind": "open_point", "ref": "O1"})
	object(e["judgment"])["findings"] = []document{{"aspect": "coverage", "item": "goal", "blocking": true, "needs_decision": false, "failure_scenario": "The requested filter is omitted", "evidence": "Request", "fix": "Keep the filter"}}
	testutil.Equal(t, verdictOf(object(e["judgment"])), "fail")
	testutil.Equal(t, outcomeOf(e)["outcome"], "stop")
	e["judgment"] = judgment()
	c := entries(object(e["judgment"])["criteria"])[0]
	c["verdict"] = "not_assessed"
	c["not_checked"] = "Issue text missing"
	testutil.Equal(t, verdictOf(object(e["judgment"])), "not_assessed")
	testutil.Contains(t, strings.Join(stringsOf(object(outcomeOf(e)["stop"])["unresolved"]), "\n"), "Issue text missing")
	e["schema_errors"] = []string{"duplicate ID"}
	testutil.Equal(t, object(outcomeOf(e)["stop"])["unresolved"], []string{"duplicate ID"})
	invalid := requirements()
	invalid["completion_conditions"] = append(entries(invalid["completion_conditions"]), entries(invalid["completion_conditions"])[0])
	testutil.Contains(t, strings.Join(must(schemaErrors(invalid)), "\n"), "used twice")
}
func TestInvalidRoundsAreNotPersisted(t *testing.T) {
	artifacts := t.TempDir()
	dir := filepath.Join(artifacts, "record")
	request(dir)
	run(dir, "r1")
	invalid := requirements()
	invalid["goal"] = ""
	testutil.Equal(t, must(round(artifacts, invalid, judgment()))["valid"], false)
	testutil.Equal(t, must(round(artifacts, invalid, judgment()))["round"], 2)
	result := must(finish(record.ContextData{Dir: dir, RunID: "r1"}, artifacts, record.ResultOptions{Base: strings.Repeat("a", 40), Commit: strings.Repeat("b", 40), Now: "t"}))
	testutil.Equal(t, result["outcome"], "stop")
	testutil.Equal(t, result["requirements_version"], 0)
	testutil.Equal(t, must(record.Latest(dir, "requirements")), nil)
}
func TestPrepareSeparatesSupplementsAndPinsStandards(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, "rules")
	testutil.Put(t, repo, "rules/review/purpose.md", "Original criterion\n")
	testutil.Put(t, repo, ".devkit/config.json", `{"language":"fr","standards":"rules"}`)
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "standards")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	ctx := record.ContextData{Cwd: repo, BaseBranch: "main", Branch: "feature/x", Dir: record.RecordDir(t.TempDir(), "feature/x"), RunID: "r1"}
	run(ctx.Dir, ctx.RunID)
	_, err := prepare(ctx, t.TempDir(), "", "Issue text", "")
	testutil.Fails(t, err, "supplement needs")
	_, err = prepare(ctx, t.TempDir(), "", "", "")
	testutil.Fails(t, err, "no request")
	artifacts := t.TempDir()
	prepared := must(prepare(ctx, artifacts, "Filter the list", "Issue body and comments", ""))
	input := must(read(text(prepared["input"])))
	testutil.Equal(t, input["language"], "fr")
	testutil.Equal(t, input["standards"], "rules")
	testutil.Equal(t, entries(input["requests"])[0]["supplement"], "Issue body and comments")
	testutil.Put(t, repo, "rules/review/purpose.md", "Changed criterion\n")
	must(prepare(ctx, artifacts, "", "Additional repository fact", ""))
	testutil.Equal(t, string(must(os.ReadFile(filepath.Join(text(input["criteria_folder"]), "review/purpose.md")))), "Original criterion\n")
	must(round(artifacts, requirements(), judgment()))
	result := must(finish(ctx, artifacts, record.ResultOptions{Base: testutil.Git(t, repo, "rev-parse", "main"), Commit: testutil.Git(t, repo, "rev-parse", "HEAD"), Now: "t"}))
	testutil.Equal(t, result["outcome"], "pass")
	testutil.Equal(t, must(record.Latest(ctx.Dir, "requirements"))["goal"], "Filter the list")
}
func TestCommands(t *testing.T) {
	artifacts := t.TempDir()
	vars := map[string]string{"ARTIFACTS_DIR": artifacts}
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: t.TempDir(), Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	registry := cli.New("test", Commands())
	d := must(json.Marshal(requirements()))
	j := must(json.Marshal(judgment()))
	vars["INPUTS_REQUIREMENTS"] = string(d)
	vars["INPUTS_JUDGMENT"] = string(j)
	testutil.Equal(t, registry.Execute([]string{"requirements", "round"}, env), 0)
	testutil.Contains(t, out.String(), `"valid":true`)
	out.Reset()
	testutil.Equal(t, registry.Execute([]string{"requirements", "not-started", `{"reason":"blocked","remaining":["answer"]}`}, env), 0)
	testutil.Contains(t, out.String(), `"reasons":["blocked","answer"]`)
	delete(vars, "ARTIFACTS_DIR")
	testutil.Equal(t, registry.Execute([]string{"requirements", "round"}, env), 1)
	testutil.Contains(t, stderr.String(), "ARTIFACTS_DIR must be set")
}

func TestCommandsKeepRelativePathsAndOneFinishTime(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, ".devkit/standards/review/purpose.md", "Criterion\n")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-qm", "standards")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	artifacts, state := t.TempDir(), t.TempDir()
	relative := must(filepath.Rel(repo, artifacts))
	relativeState := must(filepath.Rel(repo, state))
	dir := record.RecordDir(state, "feature/x")
	run(dir, "r1")
	vars := map[string]string{"STATE_DIR": relativeState, "WORKFLOW_ID": "r1", "ARTIFACTS_DIR": relative, "INPUTS_OWNER_WORDS": "Filter the list", "INPUTS_REQUIREMENTS": string(must(json.Marshal(requirements()))), "INPUTS_JUDGMENT": string(must(json.Marshal(judgment())))}
	var out, stderr bytes.Buffer
	env := cli.Env{Cwd: repo, Stdout: &out, Stderr: &stderr, LookupEnv: func(k string) (string, bool) { v, ok := vars[k]; return v, ok }}
	registry := cli.New("test", Commands())
	for _, mode := range []string{"prepare", "round", "finish"} {
		out.Reset()
		if code := registry.Execute([]string{"requirements", mode}, env); code != 0 {
			t.Fatalf("%s: %d: %s", mode, code, stderr.String())
		}
		var result document
		testutil.OK(t, json.Unmarshal(out.Bytes(), &result))
		if mode == "prepare" {
			testutil.Equal(t, result["input"], filepath.Join(relative, "requirements/input.json"))
			input := must(read(filepath.Join(artifacts, "requirements/input.json")))
			testutil.Equal(t, input["criteria_folder"], filepath.Join(relative, "criteria/.devkit/standards"))
		}
		if mode == "finish" {
			testutil.Equal(t, result["outcome"], "pass")
		}
	}
	r := must(record.Latest(dir, "requirements"))
	j := must(record.Latest(dir, "judgment"))
	result := must(record.Latest(dir, "result"))
	testutil.Equal(t, r["recorded_at"], j["recorded_at"])
	testutil.Equal(t, r["recorded_at"], result["recorded_at"])
}

func TestRequiredAspectsCannotPassWithoutFindings(t *testing.T) {
	for _, verdict := range []string{"fail", "not_assessed", "missing"} {
		t.Run(verdict, func(t *testing.T) {
			j := judgment()
			if verdict == "missing" {
				j["criteria"] = entries(j["criteria"])[1:]
			} else {
				entries(j["criteria"])[0]["verdict"] = verdict
			}
			a := t.TempDir()
			r := must(round(a, requirements(), j))
			if r["blocking_count"] == 0 && r["not_assessed_count"] == 0 {
				t.Fatal("loop would terminate successfully")
			}
			e := must(read(filepath.Join(rounds(a), "latest.json")))
			testutil.Equal(t, outcomeOf(e)["outcome"], "stop")
		})
	}
}

func TestUnitInputLoadsAgreedDecisionsAsConstraintsAndSurvivesAnswers(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "---\nid: D-abcdef\ntitle: Audience\nstatus: agreed\ndecided_by: owner\nrequest: refine/a\ndepends_on: []\nsupersedes: []\n---\nEmployees only.\n")
	testutil.Put(t, repo, ".devkit/units/U-abcdef.md", "---\nid: U-abcdef\ntitle: Sign in\ndecisions: [D-abcdef]\ndepends_on: []\n---\nAllow employee sign-in.\n")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "refined unit")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	ctx := record.ContextData{Cwd: repo, BaseBranch: "main", Dir: t.TempDir(), RunID: "r1"}
	run(ctx.Dir, ctx.RunID)
	a := t.TempDir()
	must(prepare(ctx, a, "", "", "", "U-abcdef"))
	input := must(read(filepath.Join(a, "requirements/input.json")))
	testutil.Equal(t, object(input["unit"])["id"], "U-abcdef")
	testutil.Equal(t, entries(input["agreed_decisions"])[0]["body"], "Employees only.")
	must(prepare(ctx, a, "", "", ""))
	testutil.Equal(t, object(must(read(filepath.Join(a, "requirements/input.json")))["unit"])["id"], "U-abcdef")
	_, err := prepare(ctx, a, "", "", "", "U-123456")
	testutil.Fails(t, err, "does not exist")
	for _, status := range []string{"proposed", "superseded"} {
		testutil.Put(t, repo, ".devkit/decisions/D-abcdef.md", "---\nid: D-abcdef\nstatus: "+status+"\n---\nNot agreed\n")
		testutil.Put(t, repo, ".devkit/decisions/D-fedcba.md", "---\nid: D-fedcba\nstatus: agreed\nsupersedes: [D-abcdef]\n---\nSuccessor\n")
		_, err = prepare(ctx, a, "", "", "", "U-abcdef")
		testutil.Fails(t, err, "unit U-abcdef cites decision D-abcdef which is not agreed")
	}
}

func TestF7PrepareIncludesReturnedPremiseAndObservation(t *testing.T) {
	repo := testutil.Repo(t)
	testutil.Standards(t, repo, ".devkit/standards")
	testutil.Git(t, repo, "add", ".")
	testutil.Git(t, repo, "commit", "-qm", "standards")
	testutil.Git(t, repo, "branch", "-f", "main", "HEAD")
	ctx := record.ContextData{Cwd: repo, BaseBranch: "main", Branch: "feature/x", Dir: t.TempDir(), RunID: "r1"}
	run(ctx.Dir, "r1")
	a := t.TempDir()
	must(prepare(ctx, a, "Request", "", ""))
	must(record.Append(ctx.Dir, "run", document{"run_id": "p1", "workflow": "plan", "started_at": "t", "start": "other", "counted": false, "answers": []string{}}))
	must(record.WriteResult(ctx.Dir, "plan", document{"outcome": "back", "back": document{"to": "requirements", "premise": "Ordering was assumed", "observation": "src/entry.ext cannot preserve order"}}, record.ResultOptions{RunID: "p1", Base: strings.Repeat("a", 40), Commit: strings.Repeat("b", 40)}))
	ctx.RunID = "r2"
	run(ctx.Dir, ctx.RunID)
	must(prepare(ctx, a, "", "", ""))
	input := must(read(filepath.Join(a, "requirements/input.json")))
	returned := entries(input["returned"])
	testutil.Equal(t, len(returned), 1)
	testutil.Equal(t, returned[0]["premise"], "Ordering was assumed")
	testutil.Equal(t, returned[0]["observation"], "src/entry.ext cannot preserve order")
}
