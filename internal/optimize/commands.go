package optimize

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/issues"
	"github.com/ildjenty/dev-kit/internal/proposal"
	"github.com/ildjenty/dev-kit/internal/record"
)

func approve(ctx record.ContextData, env cli.Env) (doc, error) {
	version := env.Getenv("INPUTS_APPROVE")
	if err := proposal.AuthorizedApproval(ctx, version, "optimize"); err != nil {
		return nil, err
	}
	p, err := record.Latest(ctx.Dir, "proposal")
	if err != nil {
		return nil, err
	}
	content := obj(p["content"])
	data, err := json.Marshal(content["draft"])
	if err != nil {
		return nil, err
	}
	var draft Draft
	if err = json.Unmarshal(data, &draft); err != nil {
		return nil, err
	}
	cfg, err := config.Load(ctx.Cwd)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(ctx.Dir, "issue-creation", version+".json")
	progress, err := issues.ReadCreation(path)
	if err != nil {
		return nil, err
	}
	if progress.RunID == "" {
		progress.RunID, progress.Commit = ctx.RunID, str(content["commit"])
		if err = issues.SaveCreation(path, progress); err != nil {
			return nil, err
		}
	}
	if _, err = proposal.PublishProposal(ctx, env, content, path); err != nil {
		return nil, err
	}
	progress, err = (issues.Client{Env: env}).ApplyTriage(path, draft.Triage, cfg.Issues.Labels)
	if err != nil {
		return nil, err
	}
	if err = proposal.UpdateProposal(ctx, env, content, progress); err != nil {
		return nil, err
	}
	return saveResult(ctx, doc{"outcome": "pass", "issue_numbers": progress.Numbers(), "pr_number": progress.PRNumber, "pr_url": progress.PRURL, "publication": progress.Publication}, num(p["version"]))
}

func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"start", "prepare", "check", "round", "finish", "approve", "not-started"} {
		commands = append(commands, cli.Command{Area: "optimize", Name: mode, Usage: "devkit optimize " + mode, Run: func(args []string, env cli.Env) error {
			if len(args) > 0 {
				return cli.Usage("devkit optimize " + mode)
			}
			ctx, err := record.Context(env)
			if err != nil {
				return err
			}
			display := ctx.Dir
			ctx.Dir = env.Path(ctx.Dir)
			var out any
			switch mode {
			case "start":
				var started record.StartOutput
				started, err = proposal.Start(ctx, env, "optimize")
				started.Details = display
				out = started
			case "approve":
				out, err = approve(ctx, env)
			case "not-started":
				var s doc
				err = json.Unmarshal([]byte(env.Getenv("INPUTS_START")), &s)
				if err == nil {
					out = output(doc{"outcome": "not_started", "stop": doc{"unresolved": append([]any{s["reason"]}, array(s["remaining"])...)}}, 0)
				}
			default:
				artifacts := env.Getenv("ARTIFACTS_DIR")
				if artifacts == "" {
					return fmt.Errorf("ARTIFACTS_DIR must be set")
				}
				if mode == "prepare" {
					out, err = prepare(ctx, env, artifacts)
				} else {
					artifacts = resolve(ctx.Cwd, artifacts)
					switch mode {
					case "check":
						out, err = machineCheck(ctx, artifacts, env)
					case "round":
						var judgment any = doc{}
						if text := env.Getenv("INPUTS_JUDGMENT"); text != "" {
							err = json.Unmarshal([]byte(text), &judgment)
						}
						if err == nil {
							out, err = round(ctx, artifacts, judgment)
						}
					case "finish":
						out, err = finish(ctx, artifacts)
					}
				}
			}
			if err != nil {
				return err
			}
			return env.JSON(out)
		}})
	}
	return commands
}
