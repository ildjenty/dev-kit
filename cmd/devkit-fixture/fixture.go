package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type doc = map[string]any

func Commands() []cli.Command {
	return []cli.Command{
		{Area: "fixture", Name: "github", Usage: "devkit-fixture github <args>", Run: fixtureGitHub},
		{Area: "fixture", Name: "before", Usage: "devkit-fixture before <area> <subcommand> [args]", Run: before},
		{Area: "fixture", Name: "after", Usage: "devkit-fixture after <area> <subcommand> [args]", Run: after},
	}
}

func save(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func read(path string) (doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value doc
	err = json.Unmarshal(data, &value)
	return value, err
}

func guard(env cli.Env) error {
	if env.Getenv("DEVKIT_FIXTURE") != "1" || !strings.HasPrefix(filepath.Base(env.Cwd), "fixture-exec-") || env.Getenv("ARTIFACTS_DIR") == "" || env.Getenv("STATE_DIR") == "" {
		return fmt.Errorf("fixture helpers require the test-kit scratch workspace")
	}
	return nil
}

func before(args []string, env cli.Env) error {
	if len(args) < 1 {
		return cli.Usage("devkit-fixture before <area> <subcommand> [args]")
	}
	if err := guard(env); err != nil {
		return err
	}
	marker := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "fixture.json")
	m, err := read(marker)
	if os.IsNotExist(err) {
		workflow := args[0]
		if workflow == "record" && len(args) == 3 && args[1] == "start" {
			workflow = args[2]
		}
		if err = setup(workflow, env); err != nil {
			return err
		}
		m = doc{"workflow": workflow}
		if err = save(marker, m); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	workflow := m["workflow"].(string)
	if args[0] == "checks" && (workflow == "implement" || workflow == "fix") {
		return os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\nUpdated by the stubbed author.\n"), 0644)
	}
	if len(args) > 1 && args[0] == workflow {
		switch workflow + " " + args[1] {
		case "plan check", "refine check", "retro check":
			draft := "draft.json"
			data, err := os.ReadFile(filepath.Join(env.Cwd, ".archon/workflows/dev", workflow, "fixtures/executed", draft))
			if err != nil {
				return err
			}
			if workflow == "plan" && env.Getenv("PLAN_PHASE") == "structure" {
				var d doc
				if err = json.Unmarshal(data, &d); err != nil {
					return err
				}
				for _, key := range []string{"increments", "tests", "kept_behaviors"} {
					d[key] = []any{}
				}
				data, err = json.Marshal(d)
				if err != nil {
					return err
				}
			}
			path := filepath.Join(env.Getenv("ARTIFACTS_DIR"), workflow, "draft.json")
			if err = os.WriteFile(path, data, 0644); err != nil {
				return err
			}
			if workflow == "plan" {
				return os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\nPlan skeleton.\n"), 0644)
			}
			if workflow == "retro" {
				return os.WriteFile(filepath.Join(filepath.Dir(path), "approval.txt"), []byte("Approve the traced repair with approve=1, or provide answers for changes?"), 0644)
			}
			if workflow == "refine" {
				return os.WriteFile(filepath.Join(filepath.Dir(path), "approval.txt"), []byte("Owner decision: N1. Provisional decisions: none. Unit: UN1. Start refine with approve=1, or answers to request changes."), 0644)
			}
		case "sync continue":
			return os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\nBase and topic changes.\n"), 0644)
		case "pr finish":
			state, err := read(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr/report-state.json"))
			if err != nil {
				return err
			}
			if state["passed"] != true || state["round"] != float64(1) {
				return fmt.Errorf("fixture report did not pass: %v", state)
			}
			return save(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr", "outcome.json"), doc{"outcome": "pass"})
		}
	}
	return nil
}

func setup(workflow string, env cli.Env) error {
	if _, err := record.StageOf(workflow); err != nil {
		return err
	}
	run := func(args ...string) error { _, err := gitx.Run(env.Cwd, args...); return err }
	branch := "fixture/" + filepath.Base(env.Cwd)
	if workflow == "refine" || workflow == "retro" {
		branch = workflow + "/" + filepath.Base(env.Cwd)
	}
	for _, args := range [][]string{{"switch", "-c", branch}, {"config", "user.name", "dev-kit"}, {"config", "user.email", "dev-kit@example.invalid"}, {"config", "commit.gpgsign", "false"}, {"config", "core.hooksPath", "/dev/null"}} {
		if err := run(args...); err != nil {
			return err
		}
	}
	settings, err := config.Load(env.Cwd)
	if err != nil {
		return err
	}
	if err := save(filepath.Join(env.Cwd, ".devkit/config.json"), doc{"provider": settings.Provider, "language": "en", "checks": []doc{{"name": "fixture", "command": "test -f fixture.txt", "timeout_seconds": 5}}, "skeleton_check": []string{"fixture"}, "decision_paths": []doc{}}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\n"), 0644); err != nil {
		return err
	}
	if workflow == "refine" {
		path := filepath.Join(env.Cwd, ".devkit/decisions/D-abcdef.md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("---\nid: D-abcdef\ntitle: Fixture constraint\nstatus: agreed\ndecided_by: owner\nrequest: refine/fixture\ndepends_on: []\nsupersedes: []\n---\nKeep the fixture result.\n"), 0644); err != nil {
			return err
		}
	}
	if workflow != "refine" && workflow != "retro" {
		path := filepath.Join(env.Cwd, ".devkit/decisions/D-abcdef.md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("invalid frontmatter\nDELIVERY_MUST_NOT_RECEIVE_THIS_LOG\n"), 0644); err != nil {
			return err
		}
	}
	if workflow == "retro" {
		data, err := os.ReadFile(filepath.Join(env.Cwd, ".archon/workflows/dev/retro/fixtures/executed/backlog.json"))
		if err != nil {
			return err
		}
		var backlog doc
		if err = json.Unmarshal(data, &backlog); err != nil {
			return err
		}
		if err = save(filepath.Join(env.Cwd, ".devkit/backlog/fixture.json"), backlog); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "Set up executed fixture"}, {"branch", env.Getenv("BASE_BRANCH")}} {
		if err := run(args...); err != nil {
			return err
		}
	}
	if workflow == "sync" {
		for _, target := range []string{env.Getenv("BASE_BRANCH"), branch} {
			if err := run("switch", target); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\n"+target+" changed.\n"), 0644); err != nil {
				return err
			}
			if err := run("add", "fixture.txt"); err != nil {
				return err
			}
			if err := run("commit", "-qm", "Create conflicting fixture change"); err != nil {
				return err
			}
		}
		return nil
	}
	if workflow == "requirements" || workflow == "refine" || workflow == "retro" {
		return nil
	}
	ctx, err := record.Context(env)
	if err != nil {
		return err
	}
	base, err := gitx.Run(env.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	ctx.RunID = "fixture-requirements"
	if _, err = record.Start(ctx.Dir, "requirements", record.StartOptions{RunID: ctx.RunID, Commit: base, BaseCommit: base}); err != nil {
		return err
	}
	if _, err = record.Write(ctx, "request", doc{"owner_words": "Update fixture.txt", "supplement": ""}, ""); err != nil {
		return err
	}
	if workflow == "pr" || workflow == "fix" && env.Getenv("INPUTS_REVIEW") != "" {
		if _, err = record.Write(ctx, "request", doc{"owner_words": "", "supplement": "", "issue": 42}, ""); err != nil {
			return err
		}
	}
	if _, err = record.Write(ctx, "requirements", doc{"goal": "Update fixture.txt", "completion_conditions": []doc{{"id": "C1", "text": "fixture.txt contains fixture result"}}, "constraints": []string{}, "open_points": []doc{}}, ""); err != nil {
		return err
	}
	if _, err = record.Write(ctx, "judgment", doc{"judge": "requirements", "kind": "requirements", "target": doc{"requirements_version": 1}, "materials": []string{}, "observations": []string{"Fixture prerequisite"}, "verdict": "pass", "findings": []doc{}}, ""); err != nil {
		return err
	}
	if _, err = record.WriteResult(ctx.Dir, "requirements", doc{"outcome": "pass"}, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: base}); err != nil {
		return err
	}
	if workflow == "plan" {
		return nil
	}
	ctx.RunID = "fixture-plan"
	if _, err = record.Start(ctx.Dir, "plan", record.StartOptions{RunID: ctx.RunID, Commit: base, BaseCommit: base}); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(env.Cwd, ".archon/workflows/dev/plan/fixtures/executed/draft.json"))
	if err != nil {
		return err
	}
	var plan doc
	if err = json.Unmarshal(data, &plan); err != nil {
		return err
	}
	plan["skeleton_commit"] = base
	if _, err = record.Write(ctx, "plan", plan, ""); err != nil {
		return err
	}
	if _, err = record.WriteResult(ctx.Dir, "plan", doc{"outcome": "pass"}, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: base}); err != nil {
		return err
	}
	if workflow == "implement" {
		return nil
	}
	if err = os.WriteFile(filepath.Join(env.Cwd, "fixture.txt"), []byte("fixture result\nChange under judgment.\n"), 0644); err != nil {
		return err
	}
	if err = run("add", "fixture.txt"); err != nil {
		return err
	}
	if err = run("commit", "-qm", "Change fixture input"); err != nil {
		return err
	}
	if workflow == "judge" {
		return nil
	}
	head, err := gitx.Run(env.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	ctx.RunID = "fixture-judge"
	if _, err = record.Start(ctx.Dir, "judge", record.StartOptions{RunID: ctx.RunID, Commit: head, BaseCommit: base}); err != nil {
		return err
	}
	g, err := gitx.New(env.Cwd, ctx.BaseBranch)
	if err != nil {
		return err
	}
	patch, err := g.ChangeID(head)
	if err != nil {
		return err
	}
	findings, verdict, outcome := []doc{}, "pass", doc{"outcome": "pass"}
	if workflow == "fix" && env.Getenv("INPUTS_REVIEW") == "" {
		findings = []doc{{"id": "F1", "blocking": true, "needs_decision": false, "text": "Update fixture result", "evidence": "fixture.txt"}}
		verdict, outcome = "fail", doc{"outcome": "stop", "stop": doc{"state": "Blocking fixture finding", "unresolved": []string{"scope/F1"}, "premise": ""}}
	}
	for _, judge := range []string{"test", "scope"} {
		fs, v := []doc{}, "pass"
		if judge == "scope" {
			fs, v = findings, verdict
		}
		if _, err = record.Write(ctx, "judgment", doc{"judge": judge, "kind": "code", "target": doc{"requirements_version": 1, "plan_version": 1, "base": base, "commit": head, "change_id": *patch}, "materials": []string{}, "observations": []string{"Fixture prerequisite"}, "verdict": v, "findings": fs}, ""); err != nil {
			return err
		}
	}
	if _, err = record.WriteResult(ctx.Dir, "judge", outcome, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}); err != nil {
		return err
	}
	if workflow == "fix" && env.Getenv("INPUTS_REVIEW") != "" {
		ctx.RunID = "fixture-pr"
		if _, err = record.Start(ctx.Dir, "pr", record.StartOptions{RunID: ctx.RunID, Commit: head, BaseCommit: base}); err != nil {
			return err
		}
		if err = save(filepath.Join(env.Cwd, ".devkit/backlog", record.EncodeURIComponent(ctx.Branch)+".json"), doc{"deferred": []doc{}, "carried": []doc{}, "mechanical": []doc{}, "accepted_objections": []doc{}}); err != nil {
			return err
		}
		if err = run("add", "."); err != nil {
			return err
		}
		if err = run("commit", "-qm", "chore: record review backlog"); err != nil {
			return err
		}
		head, err = gitx.Run(env.Cwd, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if _, err = record.WriteResult(ctx.Dir, "pr", doc{"outcome": "pass"}, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: head}); err != nil {
			return err
		}
	}
	return nil
}

func after(args []string, env cli.Env) error {
	if err := guard(env); err != nil {
		return err
	}
	if len(args) > 0 && args[0] != "refine" && args[0] != "retro" {
		if err := filepath.WalkDir(env.Getenv("ARTIFACTS_DIR"), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.Contains(data, []byte("DELIVERY_MUST_NOT_RECEIVE_THIS_LOG")) || bytes.Contains(data, []byte(`"agreed_decisions"`)) {
				return fmt.Errorf("decision material reached delivery input: %s", path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if len(args) < 2 || args[1] != "finish" {
		return nil
	}
	ctx, err := record.Context(env)
	if err != nil {
		return err
	}
	result, err := record.Latest(ctx.Dir, "result")
	if err != nil {
		return err
	}
	expected := "pass"
	if args[0] == "refine" || args[0] == "retro" {
		expected = "question"
	}
	if args[0] == "implement" || args[0] == "fix" {
		state, err := read(filepath.Join(env.Getenv("ARTIFACTS_DIR"), args[0], "state.json"))
		if err != nil {
			return err
		}
		if outside, ok := state["outside_requirements"].([]any); ok && len(outside) > 0 {
			expected = "back"
		}
		if owners, ok := state["owner_decisions"].([]any); ok && len(owners) > 0 {
			expected = "back"
		}
	}
	if args[0] == "requirements" && env.Getenv("INPUTS_ISSUE") != "" {
		input, err := read(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "requirements/input.json"))
		if err != nil {
			return err
		}
		issue, ok := input["issue"].(map[string]any)
		if !ok || fmt.Sprint(issue["number"]) != env.Getenv("INPUTS_ISSUE") {
			return fmt.Errorf("issue input was not loaded: %v", input)
		}
		if _, exists := input["agreed_decisions"]; exists {
			return fmt.Errorf("decision records reached delivery input")
		}
		parent, ok := input["parent_topic"].(map[string]any)
		if !ok || parent["body"] != "Who can use the result?" {
			return fmt.Errorf("parent topic text was not loaded: %v", input)
		}
	}
	if args[0] == "refine" {
		input, err := read(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "refine/input.json"))
		if err != nil {
			return err
		}
		records, ok := input["decisions"].([]any)
		if !ok || len(records) != 1 || records[0].(map[string]any)["body"] == "" {
			return fmt.Errorf("refine decision log is missing: %v", input)
		}
		if _, exists := input["relevant_decisions"]; exists {
			return fmt.Errorf("removed relevance selection reached refine")
		}
	}
	head, err := gitx.Run(env.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if result["run_id"] != ctx.RunID || result["workflow"] != args[0] || result["outcome"] != expected || result["commit"] != head {
		return fmt.Errorf("unexpected executed fixture result: %v", result)
	}
	if args[0] == "sync" {
		state, err := read(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "sync/state.json"))
		if err != nil {
			return err
		}
		if state["state"] != "done" || state["rounds"] != float64(1) {
			return fmt.Errorf("fixture conflict was not resolved: %v", state)
		}
	}
	if args[0] == "refine" || args[0] == "retro" {
		proposal, err := record.Latest(ctx.Dir, "proposal")
		if err != nil {
			return err
		}
		if proposal["version"] != float64(1) {
			return fmt.Errorf("fixture proposal was not recorded")
		}
	} else if args[0] != "requirements" && args[0] != "pr" {
		data, err := os.ReadFile(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "checks/1.jsonl"))
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), `"status":"pass"`) {
			return fmt.Errorf("fixture check did not pass: %s", data)
		}
	}
	return nil
}
