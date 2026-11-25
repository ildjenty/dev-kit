package optimize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
	"github.com/ildjenty/dev-kit/internal/testutil"
	"gopkg.in/yaml.v3"
)

type fixture struct {
	t                     *testing.T
	ctx                   record.ContextData
	artifacts, ghlog, bin string
	env                   cli.Env
}

func setup(t *testing.T) *fixture {
	t.Helper()
	root := testutil.Repo(t)
	testutil.Standards(t, root, ".devkit/standards")
	testutil.Put(t, root, ".devkit/config.json", `{"language":"en"}`)
	testutil.Put(t, root, ".archon/config.yaml", "worktree:\n  baseBranch: main\n  postCreate: ['touch prepared']\n")
	testutil.Put(t, root, ".archon/workflows/example/example.yaml", "name: example\ndescription: Example\nnodes:\n  - id: result\n    bash: 'true'\n")
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-qm", "base criteria")
	testutil.Git(t, root, "branch", "-f", "main", "HEAD")
	testutil.Git(t, root, "switch", "-qc", "optimize/fit")
	testutil.PublicationRemote(t, root)
	log, _ := testutil.IssueCreationCLI(t)
	bin := t.TempDir()
	testutil.Put(t, bin, "archon", "#!/bin/sh\nprintf 'validated\\n'\n")
	testutil.OK(t, os.Chmod(filepath.Join(bin, "archon"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := &fixture{t: t, ctx: record.ContextData{Cwd: root, Dir: record.RecordDir(t.TempDir(), "optimize/fit"), Branch: "optimize/fit", BaseBranch: "main", RunID: "r1"}, artifacts: t.TempDir(), ghlog: log, bin: bin}
	r.env = cli.Env{Cwd: root, Environ: os.Environ(), Registry: cli.New("test", Commands())}
	started := testutil.Must(proposal.Start(r.ctx, r.env, "optimize"))
	testutil.Equal(t, started.Stopped, false)
	testutil.Must(prepare(r.ctx, r.env, r.artifacts))
	return r
}
func (r *fixture) draft() doc {
	return doc{"summary": "Fit checks to CI", "owner_points": []string{}, "pr_title": "Fit repository checks", "pr_summary": "Configure the CI checks for fresh worktrees.", "changes": []doc{}, "triage": []issues.TriageItem{}}
}
func (r *fixture) write(d doc) {
	r.t.Helper()
	testutil.OK(r.t, writeJSON(file(r.artifacts, "draft.json"), d))
	input := testutil.Must(readJSON(file(r.artifacts, "input.json")))
	testutil.OK(r.t, put(file(r.artifacts, "approval.txt"), fmt.Sprintf("Everything else follows repository evidence and owner's instructions. Details: devkit proposal show %s.\nReply approve=%d, or answers naming the key to change.", r.ctx.Branch, num(input["next_proposal_version"]))))
}
func (r *fixture) changeConfig(d doc, text string) {
	r.t.Helper()
	testutil.Put(r.t, r.ctx.Cwd, ".devkit/config.json", text)
	d["changes"] = []doc{{"path": ".devkit/config.json", "reason": "Use the repository check", "evidence": "CI runs this command"}}
}
func passing() doc {
	cs := []doc{}
	for _, a := range aspects {
		cs = append(cs, doc{"aspect": a, "verdict": "pass", "checked": "Compared actual draft and fixed standards", "not_checked": ""})
	}
	return doc{"criteria": cs, "findings": []doc{}}
}
func TestMachineChecks(t *testing.T) {
	for _, tc := range []struct {
		name, config, want string
		alter              func(*fixture, doc)
	}{
		{name: "passes all checks on the base regardless of paths", config: `{"checks":[{"name":"prepared","command":"test -f prepared","paths":[],"timeout_seconds":2}]}`},
		{name: "config", config: `{"checks":"invalid"}`, want: "config:"},
		{name: "concerns", want: "concerns:", alter: func(r *fixture, d doc) {
			testutil.Put(t, r.ctx.Cwd, ".devkit/standards/concerns.json", "{}")
			d["changes"] = []doc{{"path": ".devkit/standards/concerns.json", "reason": "Fit routing", "evidence": "Repository layout"}}
		}},
		{name: "workflow validation", want: "archon validate workflows", alter: func(r *fixture, d doc) { testutil.Put(t, r.bin, "archon", "#!/bin/sh\necho invalid\nexit 1\n") }},
		{name: "workflow warning", want: "archon validate workflows", alter: func(r *fixture, d doc) { testutil.Put(t, r.bin, "archon", "#!/bin/sh\necho '1 warning'\n") }},
		{name: "base failure", config: `{"checks":[{"name":"broken","command":"echo base-failure; exit 1","timeout_seconds":2}]}`, want: "check broken on base: fail"},
		{name: "base skipped", config: `{"checks":[{"name":"unavailable","command":"echo unavailable; exit 77","timeout_seconds":2}]}`, want: "check unavailable on base: skip"},
		{name: "preparation failure", want: "base preparation", alter: func(r *fixture, d doc) {
			testutil.Put(t, r.ctx.Cwd, ".archon/config.yaml", "worktree:\n  postCreate: ['exit 1']\n")
			d["changes"] = []doc{{"path": ".archon/config.yaml", "reason": "Prepare", "evidence": "Setup command"}}
		}},
		{name: "malformed Archon config", want: "Archon config:", alter: func(r *fixture, d doc) {
			testutil.Put(t, r.ctx.Cwd, ".archon/config.yaml", "worktree: [")
			d["changes"] = []doc{{"path": ".archon/config.yaml", "reason": "Prepare", "evidence": "Setup command"}}
		}},
		{name: "title limit", want: "100 characters", alter: func(r *fixture, d doc) { d["pr_title"] = strings.Repeat("a", 101) }},
		{name: "title newline", want: "single-line", alter: func(r *fixture, d doc) { d["pr_title"] = "first\nsecond" }},
		{name: "unknown issue", want: "existing open issue", alter: func(r *fixture, d doc) {
			d["triage"] = []issues.TriageItem{{Key: "missing", Number: 99, Kind: "leave", Reason: "Not in scope", CompletionConditions: []string{}, HeldUnits: []string{}, BlockedBy: []string{}}}
		}},
		{name: "unlisted file", want: "outside optimize scope", alter: func(r *fixture, d doc) { testutil.Put(t, r.ctx.Cwd, "application.go", "package application\n") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t)
			d := r.draft()
			if tc.config != "" {
				r.changeConfig(d, tc.config)
			}
			if tc.alter != nil {
				tc.alter(r, d)
			}
			r.write(d)
			checked := testutil.Must(machineCheck(r.ctx, r.artifacts, r.env))
			if tc.want == "" {
				testutil.Equal(t, checked["valid"], true)
			} else {
				testutil.Equal(t, checked["valid"], false)
				testutil.Contains(t, strings.Join(stringsOf(checked["errors"]), "\n"), tc.want)
			}
			if strings.HasPrefix(tc.name, "base ") {
				report := testutil.Must(readJSON(file(r.artifacts, "verification.json")))
				data := testutil.Must(json.Marshal(report))
				testutil.Contains(t, string(data), "base_checks")
				logs := testutil.Must(os.ReadFile(str(items(report["base_checks"])[0]["log"])))
				if tc.name == "base failure" {
					testutil.Contains(t, string(logs), "base-failure")
				}
			}
			testutil.Equal(t, strings.Count(testutil.Git(t, r.ctx.Cwd, "worktree", "list", "--porcelain"), "worktree "), 1)
		})
	}
}
func TestReviewApprovalAndPublication(t *testing.T) {
	r := setup(t)
	d := r.draft()
	r.changeConfig(d, `{"checks":[{"name":"ready","command":"test -f prepared","timeout_seconds":2}]}`)
	r.write(d)
	_, err := approve(r.ctx, r.env)
	testutil.Fails(t, err, "approve must name")
	testutil.Equal(t, testutil.Must(machineCheck(r.ctx, r.artifacts, r.env))["valid"], true)
	testutil.Equal(t, testutil.Must(round(r.ctx, r.artifacts, passing()))["valid"], true)
	result := testutil.Must(finish(r.ctx, r.artifacts))
	testutil.Equal(t, result["outcome"], "question")
	question := str(items(result["questions"])[0]["text"])
	testutil.Equal(t, question, string(testutil.Must(os.ReadFile(file(r.artifacts, "approval.txt")))))
	for _, field := range []string{str(d["pr_title"]), str(d["summary"]), "Proposal version:", "Commit:", "Parent:", "mapping", "blocked_by", "{", "}"} {
		testutil.Equal(t, strings.Contains(question, field), false)
	}
	testutil.Equal(t, strings.Count(question, "approve=1"), 1)
	problems := testutil.Must(proposal.ApprovalErrors(r.ctx, "2", "optimize"))
	testutil.Contains(t, strings.Join(problems, "\n"), "latest proposal version")
	details := testutil.Must(readJSON(filepath.Join(r.ctx.Dir, "proposals/1.details.json")))
	testutil.Equal(t, details, testutil.Must(record.Latest(r.ctx.Dir, "proposal")))
	testutil.Equal(t, obj(details["content"])["draft"], testutil.Must(readJSON(file(r.artifacts, "draft.json"))))
	calls := string(testutil.Must(os.ReadFile(filepath.Join(r.ghlog, "calls"))))
	for _, write := range []string{"issue create", "issue edit", "label create", "pr create"} {
		testutil.Equal(t, strings.Contains(calls, write), false)
	}
	head := testutil.Git(t, r.ctx.Cwd, "rev-parse", "HEAD")
	r.ctx.RunID = "r2"
	env := r.env
	env.LookupEnv = func(k string) (string, bool) {
		if k == "INPUTS_APPROVE" {
			return "1", true
		}
		return os.LookupEnv(k)
	}
	testutil.Equal(t, testutil.Must(proposal.Start(r.ctx, env, "optimize")).Stopped, false)
	published := testutil.Must(approve(r.ctx, env))
	testutil.Equal(t, published["outcome"], "pass")
	testutil.Equal(t, published["pr_number"], 80)
	testutil.Equal(t, testutil.Git(t, r.ctx.Cwd, "rev-parse", "HEAD"), head)
	body := string(testutil.Must(os.ReadFile(filepath.Join(r.ghlog, "pr-body"))))
	testutil.Equal(t, strings.HasPrefix(body, str(d["pr_summary"])), true)
	testutil.Contains(t, body, ".devkit/config.json")
}
func TestJudgeNonConvergenceAndPostReviewEdits(t *testing.T) {
	for _, tamper := range []bool{false, true} {
		t.Run(fmt.Sprint(tamper), func(t *testing.T) {
			r := setup(t)
			rounds := 3
			if tamper {
				rounds = 1
			}
			for n := 1; n <= rounds; n++ {
				r.write(r.draft())
				testutil.Equal(t, testutil.Must(machineCheck(r.ctx, r.artifacts, r.env))["valid"], true)
				j := passing()
				if !tamper {
					items(j["criteria"])[0]["verdict"] = "fail"
				}
				testutil.Must(round(r.ctx, r.artifacts, j))
			}
			if tamper {
				testutil.OK(t, put(file(r.artifacts, "approval.txt"), "changed"))
			}
			testutil.Equal(t, testutil.Must(finish(r.ctx, r.artifacts))["outcome"], "stop")
			if !tamper {
				_, err := round(r.ctx, r.artifacts, passing())
				testutil.Fails(t, err, "three rounds")
			}
		})
	}
}
func TestWorkflowSchemaAndPlumbingFallback(t *testing.T) {
	r := setup(t)
	data := testutil.Must(os.ReadFile("../../kit/.archon/workflows/dev/optimize/optimize.yaml"))
	var workflow doc
	testutil.OK(t, yaml.Unmarshal(data, &workflow))
	testutil.Equal(t, len(obj(workflow["inputs"])), 4)
	for _, node := range items(workflow["nodes"]) {
		if node["id"] == "draft" {
			for _, child := range items(obj(node["loop_group"])["nodes"]) {
				if child["id"] == "judge" {
					var schema doc
					testutil.OK(t, json.Unmarshal(testutil.Must(schemas.ReadFile("schemas/judgment.schema.json")), &schema))
					testutil.Equal(t, child["output_format"], schema)
				}
			}
		}
	}
	bin := t.TempDir()
	for _, name := range []string{"sh", "git"} {
		source := "/bin/" + name
		if name == "git" {
			source = "/usr/bin/git"
		}
		testutil.OK(t, os.Symlink(source, filepath.Join(bin, name)))
	}
	t.Setenv("PATH", bin)
	r.env.Environ = os.Environ()
	method, problems, err := workflowCheck(r.ctx, r.env, t.TempDir())
	testutil.OK(t, err)
	testutil.Equal(t, len(problems), 0)
	testutil.Contains(t, method, "plumbing")
	testutil.Put(t, r.ctx.Cwd, ".archon/workflows/example/example.yaml", "nodes: [")
	_, problems, err = workflowCheck(r.ctx, r.env, t.TempDir())
	testutil.OK(t, err)
	testutil.Equal(t, len(problems) > 0, true)
	testutil.Put(t, r.ctx.Cwd, ".archon/workflows/example/example.yaml", "nodes:\n - bash: devkit missing command\n")
	_, problems, err = workflowCheck(r.ctx, r.env, t.TempDir())
	testutil.OK(t, err)
	testutil.Contains(t, strings.Join(problems, ","), "unregistered")
}

func TestBaseFindingLogsSurviveLaterRounds(t *testing.T) {
	r := setup(t)
	d := r.draft()
	r.changeConfig(d, `{"checks":[{"name":"ci","command":"echo original-base-failure; exit 1","timeout_seconds":2}]}`)
	r.write(d)
	testutil.Equal(t, testutil.Must(machineCheck(r.ctx, r.artifacts, r.env))["valid"], false)
	first := testutil.Must(readJSON(file(r.artifacts, "verification/1.json")))
	log := str(items(first["base_checks"])[0]["log"])
	testutil.Must(round(r.ctx, r.artifacts, passing()))
	r.changeConfig(d, `{"checks":[{"name":"ci","command":"true","timeout_seconds":2}]}`)
	d["summary"] = "Correct the CI command after the original base check failed."
	r.write(d)
	testutil.Equal(t, testutil.Must(machineCheck(r.ctx, r.artifacts, r.env))["valid"], true)
	testutil.Contains(t, string(testutil.Must(os.ReadFile(log))), "original-base-failure")
	testutil.Must(readJSON(file(r.artifacts, "verification/2.json")))
}
