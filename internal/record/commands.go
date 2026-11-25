package record

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/issues"
)

func Commands() []cli.Command {
	return []cli.Command{
		{Area: "record", Name: "start", Usage: "devkit record start <workflow>", Run: runStart},
		{Area: "record", Name: "write", Usage: "devkit record write <kind> <file>", Run: func(args []string, env cli.Env) error {
			if len(args) < 2 {
				return cli.Usage("devkit record write <kind> <file>")
			}
			ctx, err := Context(env)
			if err != nil {
				return err
			}
			originalDir := ctx.Dir
			ctx.Dir = env.Path(ctx.Dir)
			body, err := readBody(env.Path(args[1]))
			if err != nil {
				return err
			}
			entry, err := Write(ctx, args[0], body, "")
			if err != nil {
				return err
			}
			if !filepath.IsAbs(originalDir) {
				entry.Path = filepath.Join(originalDir, folders[args[0]], filepath.Base(entry.Path))
			}
			return env.JSON(struct {
				Kind   string `json:"kind"`
				Number int    `json:"number"`
				Path   string `json:"path"`
			}{entry.Kind, entry.Number, entry.Path})
		}},
		{Area: "record", Name: "latest", Usage: "devkit record latest <kind>", Run: func(args []string, env cli.Env) error {
			if len(args) < 1 || folders[args[0]] == "" || args[0] == "question" || args[0] == "answer" {
				return cli.Usage("devkit record latest <kind>")
			}
			ctx, err := Context(env)
			if err != nil {
				return err
			}
			doc, err := Latest(env.Path(ctx.Dir), args[0])
			if err != nil {
				return err
			}
			return env.JSON(doc)
		}},
		{Area: "record", Name: "dir", Usage: "devkit record dir", Run: func(args []string, env cli.Env) error {
			ctx, err := Context(env)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(env.Stdout, ctx.Dir)
			return err
		}},
		{Area: "record", Name: "result", Usage: "devkit record result <workflow> <outcome.json>", Run: func(args []string, env cli.Env) error {
			if len(args) < 2 {
				return cli.Usage("devkit record result <workflow> <outcome.json>")
			}
			ctx, err := Context(env)
			if err != nil {
				return err
			}
			body, err := readBody(env.Path(args[1]))
			if err != nil {
				return err
			}
			base, err := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ctx.BaseBranch)
			if err != nil {
				return err
			}
			commit, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			result, err := WriteResult(env.Path(ctx.Dir), args[0], body, ResultOptions{RunID: ctx.RunID, Base: base, Commit: commit})
			if err != nil {
				return err
			}
			return env.JSON(result)
		}},
		{Area: "record", Name: "limit", Usage: "devkit record limit <workflow>", Run: func(args []string, env cli.Env) error {
			if len(args) != 1 {
				return cli.Usage("devkit record limit <workflow>")
			}
			answers, err := ParseAnswers(env.Getenv("INPUTS_ANSWERS"))
			if err != nil {
				return err
			}
			c, err := config.Load(env.Cwd)
			if err != nil {
				return err
			}
			ctx, err := Context(env)
			if err != nil {
				return err
			}
			h, err := History(env.Path(ctx.Dir))
			if err != nil {
				return err
			}
			ids := []string{}
			for _, a := range answers {
				ids = append(ids, a.QuestionID)
			}
			state, err := LimitState(h, args[0], ids, c.Limits.Restarts)
			if err != nil {
				return err
			}
			return env.JSON(state)
		}},
	}
}
func nonemptyLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
func runStart(args []string, env cli.Env) error {
	if len(args) != 1 {
		return cli.Usage("devkit record start <workflow>")
	}
	ctx, err := Context(env)
	if err != nil {
		return err
	}
	reason, err := KitStartReason(env, args[0])
	if err != nil {
		return err
	}
	if reason != "" {
		return env.JSON(StartOutput{Stopped: true, Stage: "start", Reason: reason, Details: ctx.Dir, Remaining: []string{}})
	}
	c, err := config.Load(ctx.Cwd)
	if err != nil {
		return err
	}
	answers, err := ParseAnswers(env.Getenv("INPUTS_ANSWERS"))
	if err != nil {
		return err
	}
	continuation := env.Getenv("INPUTS_CONTINUE")
	if strings.TrimSpace(continuation) == "" {
		continuation = ""
	}
	commit, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, env.Path(ctx.Dir))
	if err != nil {
		return err
	}
	base, err := g.Run("rev-parse", g.Refs[0])
	if err != nil {
		return err
	}
	common, err := gitx.Run(ctx.Cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	changed, err := gitx.Run(filepath.Dir(common), "status", "--porcelain", "--", ".archon/workflows")
	if err != nil {
		return err
	}
	worktree, err := gitx.Run(ctx.Cwd, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	issueReasons := []string{}
	if args[0] == "requirements" {
		number, err := InputIssue(env, env.Path(ctx.Dir))
		if err != nil {
			issueReasons = append(issueReasons, err.Error())
		} else if number != 0 {
			if _, err := (issues.Client{Env: env}).ReadyUnit(number, c.Issues.Labels.Unit); err != nil {
				issueReasons = append(issueReasons, err.Error())
			}
		}
	}
	out, err := Start(env.Path(ctx.Dir), args[0], StartOptions{Cwd: ctx.Cwd, BaseBranch: ctx.BaseBranch, BaseBranchSource: ctx.BaseBranchSource, Review: env.Getenv("INPUTS_REVIEW"), IssueReasons: issueReasons, RunID: ctx.RunID, Commit: commit, ChangeID: func() (*string, error) {
		g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, env.Path(ctx.Dir))
		if err != nil {
			return nil, err
		}
		return g.ChangeID(commit)
	}, BaseCommit: base, Answers: answers, Continuation: continuation, Limit: &c.Limits.Restarts, WorkflowsChanged: nonemptyLines(changed), WorktreeChanged: nonemptyLines(worktree)})
	if err != nil {
		return err
	}
	out.Details = ctx.Dir
	return env.JSON(out)
}
