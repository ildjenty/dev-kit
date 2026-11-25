package optimize

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/kitinfo"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

func changedPaths(ctx record.ContextData, base string) ([]string, error) {
	out, err := gitx.Run(ctx.Cwd, "diff", "--name-only", "--no-renames", "-z", base, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := gitx.Run(ctx.Cwd, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, p := range strings.Split(out+"\x00"+untracked, "\x00") {
		if p != "" && !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func workflowCheck(ctx record.ContextData, env cli.Env, dir string) (string, []string, error) {
	method := "Archon: archon validate workflows (warnings rejected)"
	if _, err := exec.LookPath("archon"); err != nil {
		method = "devkit plumbing check (Archon unavailable)"
		calls, e := kitinfo.Invocations(os.DirFS(ctx.Cwd), ".archon/workflows", false)
		if e != nil {
			return method, []string{e.Error()}, nil
		}
		missing := env.Registry.MissingCommands(calls)
		if len(missing) > 0 {
			return method, []string{"unregistered workflow commands: " + strings.Join(missing, ", ")}, nil
		}
		err = filepath.WalkDir(filepath.Join(ctx.Cwd, ".archon/workflows"), func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.IsDir() || strings.Contains(filepath.ToSlash(path), "/fixtures/") || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
				return nil
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			return validateYAML(data)
		})
		if err != nil {
			return method, []string{err.Error()}, nil
		}
		return method, []string{}, nil
	}
	problems := []string{}
	command := exec.Command("archon", "validate", "workflows")
	command.Dir, command.Env = ctx.Cwd, env.Environ
	data, runErr := command.CombinedOutput()
	log := filepath.Join(dir, "workflow-validation.log")
	if err := os.WriteFile(log, data, 0644); err != nil {
		return method, nil, err
	}
	if runErr != nil {
		problems = append(problems, "archon validate workflows: "+runErr.Error()+"; "+log)
	} else if strings.Contains(string(data), "WARN") || strings.Contains(string(data), "Warning") || warningCount.Match(data) {
		problems = append(problems, "archon validate workflows: warnings; "+log)
	}
	return method, problems, nil
}

func machineCheck(ctx record.ContextData, artifacts string, env cli.Env) (doc, error) {
	o, err := resultOptions(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = record.BuildResult(ctx.Dir, "optimize", doc{"outcome": "pass"}, o); err != nil {
		return nil, err
	}
	input, err := readJSON(file(artifacts, "input.json"))
	if err != nil {
		return nil, err
	}
	value, err := readDraft(artifacts)
	if err != nil {
		return nil, err
	}
	problems, err := record.ValidateSchema(file(artifacts, "draft.schema.json"), value)
	if err != nil {
		return nil, err
	}
	finish := func() (doc, error) {
		out := doc{"valid": len(problems) == 0, "errors": problems}
		return out, writeJSON(file(artifacts, "check.json"), out)
	}
	if len(problems) > 0 {
		return finish()
	}
	draft, _, err := readTypedDraft(artifacts)
	if err != nil {
		return nil, err
	}
	baseline, err := readJSON(file(artifacts, "baseline.json"))
	if err != nil {
		return nil, err
	}
	head, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	if head != baseline["commit"] {
		problems = append(problems, "writer changed HEAD; only optimize check may commit")
	}
	problems = append(problems, proposal.PRTextErrors(value)...)
	if strings.TrimSpace(draft.Summary) == "" {
		problems = append(problems, "summary must not be blank")
	}
	var existing []issues.Issue
	data, err := json.Marshal(input["issues"])
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &existing); err != nil {
		return nil, err
	}
	var initial config.Config
	data, err = json.Marshal(input["config"])
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &initial); err != nil {
		return nil, err
	}
	problems = append(problems, issues.TriageErrors(draft.Triage, existing, initial.Issues.Labels)...)
	fresh, e := (issues.Client{Env: env}).Untriaged(initial.Issues.Labels)
	if e != nil {
		return nil, e
	}
	problems = append(problems, issues.TriageErrors(draft.Triage, fresh, initial.Issues.Labels)...)
	current, err := config.Load(ctx.Cwd)
	if err != nil {
		problems = append(problems, "config: "+err.Error())
	} else {
		if _, err = concerns.Load(filepath.Join(ctx.Cwd, current.Standards)); err != nil {
			problems = append(problems, "concerns: "+err.Error())
		}
	}
	paths, err := changedPaths(ctx, str(input["proposal_base"]))
	if err != nil {
		return nil, err
	}
	named := map[string]bool{}
	for _, change := range draft.Changes {
		if named[change.Path] || !slices.Contains(paths, change.Path) || strings.TrimSpace(change.Reason) == "" || strings.TrimSpace(change.Evidence) == "" {
			problems = append(problems, "change needs a unique edited path, reason and evidence: "+change.Path)
		}
		named[change.Path] = true
	}
	for _, p := range paths {
		allowed := p == ".devkit/config.json" || p == ".archon/config.yaml" || p == "CLAUDE.md" || p == "AGENTS.md" || strings.HasPrefix(p, initial.Standards+"/") || current.Standards != "" && strings.HasPrefix(p, current.Standards+"/") || strings.TrimSpace(str(input["instructions"])) != "" && strings.HasPrefix(p, ".archon/workflows/dev/")
		if !allowed {
			problems = append(problems, "edited outside optimize scope: "+p)
		}
		if !named[p] {
			problems = append(problems, "edited path missing from changes: "+p)
		}
		info, e := os.Lstat(filepath.Join(ctx.Cwd, p))
		if e != nil && !os.IsNotExist(e) {
			return nil, e
		}
		if e == nil && !info.Mode().IsRegular() {
			problems = append(problems, "edited path must be a regular file: "+p)
		}
	}
	approval, err := os.ReadFile(file(artifacts, "approval.txt"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if strings.TrimSpace(string(approval)) == "" || !strings.Contains(string(approval), fmt.Sprintf("approve=%d", num(input["next_proposal_version"]))) || !strings.Contains(string(approval), "answers") {
		problems = append(problems, "approval.txt must give the reviewed summary, owner points, approve=<version> and answers")
	}
	if err = validateArchonConfig(ctx.Cwd); err != nil {
		problems = append(problems, "Archon config: "+err.Error())
	}
	if len(problems) > 0 {
		return finish()
	}
	lastRound, e := readJSON(file(artifacts, "rounds/latest.json"))
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	roundNumber := num(lastRound["round"]) + 1
	logs := file(artifacts, fmt.Sprintf("checks/%d", roundNumber))
	if err = os.MkdirAll(logs, 0755); err != nil {
		return nil, err
	}
	method, workflowErrors, err := workflowCheck(ctx, env, logs)
	if err != nil {
		return nil, err
	}
	problems = append(problems, workflowErrors...)
	results, baseErrors, err := checkBase(ctx, current, logs, env)
	if err != nil {
		return nil, err
	}
	problems = append(problems, baseErrors...)
	report := doc{"workflow_validation": method, "base_checks": results, "errors": problems}
	if err = writeJSON(file(artifacts, fmt.Sprintf("verification/%d.json", roundNumber)), report); err != nil {
		return nil, err
	}
	if err = writeJSON(file(artifacts, "verification.json"), report); err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return finish()
	}
	committed, previous, err := proposal.CommittedErrors(ctx, value)
	if err != nil {
		return nil, err
	}
	if len(committed) > 0 || num(previous["version"]) != num(input["next_proposal_version"]) {
		if num(previous["version"])+1 != num(input["next_proposal_version"]) {
			problems = append(problems, "proposal version already committed; start the next draft version")
			return finish()
		}
		working, err := changedPaths(ctx, head)
		if err != nil {
			return nil, err
		}
		p, err := proposal.Commit(ctx, working, fmt.Sprintf("chore: propose repository fit v%d", num(input["next_proposal_version"])), doc{"draft": value, "base_commit": input["proposal_base"], "ready": false, "verification": report})
		if err != nil {
			return nil, err
		}
		if err = writeJSON(file(artifacts, "baseline.json"), doc{"commit": obj(p["content"])["commit"]}); err != nil {
			return nil, err
		}
	}
	return finish()
}
