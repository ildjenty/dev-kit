package retro

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/record"
)

func start(ctx record.ContextData, env cli.Env) (record.StartOutput, error) {
	stopped := func(reason string) record.StartOutput {
		return record.StartOutput{Stopped: true, Stage: "start", Reason: reason, Details: ctx.Dir, Remaining: []string{}}
	}
	reason, err := record.KitStartReason(env, "retro")
	if err != nil {
		return record.StartOutput{}, err
	}
	if reason != "" {
		return stopped(reason), nil
	}
	if !strings.HasPrefix(ctx.Branch, "retro/") {
		return stopped("retro requires a retro/<name> branch"), nil
	}
	approve := env.Getenv("INPUTS_APPROVE")
	previous, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return record.StartOutput{}, e
	}
	if oldProposal(previous) {
		return stopped("old-format retro proposal cannot be used; start retro again on a new retro branch"), nil
	}
	answers, e := record.ParseAnswers(env.Getenv("INPUTS_ANSWERS"))
	if e != nil {
		return record.StartOutput{}, e
	}
	if approve != "" {
		problems, e := approvalErrors(ctx, approve)
		if e != nil {
			return record.StartOutput{}, e
		}
		if len(problems) > 0 {
			return stopped(strings.Join(problems, "; ")), nil
		}
		if len(answers) > 0 {
			return stopped("approve cannot be combined with answers"), nil
		}
		r, e := record.Latest(ctx.Dir, "result")
		if e != nil {
			return record.StartOutput{}, e
		}
		qid := str(items(r["questions"])[0]["id"])
		recorded, e := record.ReadKeyed(ctx.Dir, "answer", qid)
		if e != nil {
			return record.StartOutput{}, e
		}
		if recorded == nil {
			answers = append(answers, record.Answer{QuestionID: qid, Answer: "approve=" + approve})
		}

	}
	common, e := gitx.Run(ctx.Cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return record.StartOutput{}, e
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return record.StartOutput{}, e
	}
	base, e := gitx.Run(ctx.Cwd, "rev-parse", ctx.BaseBranch)
	if e != nil {
		return record.StartOutput{}, e
	}
	c, e := config.Load(ctx.Cwd)
	if e != nil {
		return record.StartOutput{}, e
	}
	workflows, e := gitx.Run(filepath.Dir(common), "status", "--porcelain", "--", ".archon/workflows")
	if e != nil {
		return record.StartOutput{}, e
	}
	worktree, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return record.StartOutput{}, e
	}
	lines := func(s string) []string {
		if s == "" {
			return []string{}
		}
		return strings.Split(s, "\n")
	}
	problems, e := record.StartConditions(ctx.Dir, "retro", record.ConditionOptions{Commit: head})
	if e != nil {
		return record.StartOutput{}, e
	}
	for _, problem := range problems {
		answered := false
		for _, answer := range answers {
			answered = answered || strings.HasPrefix(problem, answer.QuestionID+" (")
		}
		if !answered {
			return stopped(problem), nil
		}
	}
	return record.Start(ctx.Dir, "retro", record.StartOptions{RunID: ctx.RunID, Commit: head, BaseCommit: base, ChangeID: func() (*string, error) {
		g, e := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
		if e != nil {
			return nil, e
		}
		return g.ChangeID("HEAD")
	}, Answers: answers, Continuation: env.Getenv("INPUTS_CONTINUE"), Limit: &c.Limits.Restarts, WorkflowsChanged: lines(workflows), WorktreeChanged: lines(worktree)})
}

func approvalErrors(ctx record.ContextData, version string) ([]string, error) {
	p, e := record.Latest(ctx.Dir, "proposal")
	if e != nil {
		return nil, e
	}
	if oldProposal(p) {
		return []string{"old-format retro proposal cannot be approved; start retro again on a new retro branch"}, nil
	}
	r, e := record.Latest(ctx.Dir, "result")
	if e != nil {
		return nil, e
	}
	if !regexp.MustCompile(`^[1-9]\d*$`).MatchString(version) || strconv.Itoa(num(p["version"])) != version {
		return []string{"approve must name the latest proposal version"}, nil
	}
	qs := items(r["questions"])
	if obj(p["content"])["ready"] != true || r["workflow"] != "retro" || r["run_id"] != p["run_id"] || r["outcome"] != "question" || len(qs) != 1 || obj(qs[0]["target"])["kind"] != "proposal" || obj(qs[0]["target"])["ref"] != version {
		return []string{"the latest proposal result did not ask for approval"}, nil
	}
	head, e := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if e != nil {
		return nil, e
	}
	if r["commit"] != head || obj(p["content"])["commit"] != head {
		return []string{"HEAD changed after the proposal asked for approval"}, nil
	}
	status, e := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return nil, e
	}
	if status != "" {
		return []string{"approval requires a clean worktree"}, nil
	}
	return []string{}, nil
}

func authorizedApproval(ctx record.ContextData, version string) error {
	problems, e := approvalErrors(ctx, version)
	if e != nil {
		return e
	}
	if len(problems) > 0 {
		return &record.RecordError{Message: "proposal cannot be approved", Errors: problems}
	}
	runs, e := record.List(ctx.Dir, "run")
	if e != nil {
		return e
	}
	r, e := record.Latest(ctx.Dir, "result")
	if e != nil {
		return e
	}
	q := items(r["questions"])[0]
	a, e := record.ReadKeyed(ctx.Dir, "answer", str(q["id"]))
	if e != nil {
		return e
	}
	progress, e := issues.ReadCreation(filepath.Join(ctx.Dir, "issue-creation", version+".json"))
	if e != nil {
		return e
	}
	authorizationRun := ctx.RunID
	if progress.RunID != "" {
		authorizationRun = progress.RunID
	}
	for _, run := range runs {
		if run["run_id"] == authorizationRun && run["workflow"] == "retro" && run["start"] == "answer" && a["run_id"] == authorizationRun && a["workflow"] == "retro" && a["answer"] == "approve="+version && a["commit"] == r["commit"] && obj(a["target"])["ref"] == version {
			for _, id := range stringsOf(run["answers"]) {
				if id == q["id"] {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("approval requires a retro run that passed start with the recorded approval answer")
}
