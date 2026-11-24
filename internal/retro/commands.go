package retro

import (
	"encoding/json"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/record"
)

func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"start", "prepare", "check", "round", "verify", "finish", "empty", "approve", "not-started"} {
		commands = append(commands, cli.Command{Area: "retro", Name: mode, Usage: "devkit retro " + mode, Run: func(args []string, env cli.Env) error {
			ctx, e := record.Context(env)
			if e != nil {
				return e
			}
			displayDir := ctx.Dir
			ctx.Dir = env.Path(ctx.Dir)
			artifacts := env.Getenv("ARTIFACTS_DIR")
			var out any
			if mode == "start" {
				var started record.StartOutput
				started, e = start(ctx, env)
				started.Details = displayDir
				out = started
			} else if mode == "approve" {
				out, e = approve(ctx, env.Getenv("INPUTS_APPROVE"))
			} else {
				if artifacts == "" {
					return fmt.Errorf("ARTIFACTS_DIR must be set")
				}
				switch mode {
				case "prepare":
					out, e = prepare(ctx, artifacts)
				case "check":
					out, e = check(ctx, artifacts)
				case "round":
					var j any
					e = json.Unmarshal([]byte(env.Getenv("INPUTS_JUDGMENT")), &j)
					if e == nil {
						out, e = round(ctx, artifacts, j)
					}
				case "empty":
					out, e = empty(ctx, artifacts)
				case "finish":
					if text := env.Getenv("INPUTS_VERIFICATION"); text != "" {
						var report doc
						e = json.Unmarshal([]byte(text), &report)
						if e == nil {
							e = writeJSON(file(resolve(ctx.Cwd, artifacts), "verification-text.json"), report)
						}
					}
					if e == nil {
						out, e = finish(ctx, artifacts)
					}
				case "verify":
					out, e = verify(ctx, artifacts, env)
				case "not-started":
					var s doc
					e = json.Unmarshal([]byte(env.Getenv("INPUTS_START")), &s)
					if e == nil {
						reasons := append([]any{s["reason"]}, array(s["remaining"])...)
						out = output(doc{"outcome": "not_started", "stop": doc{"unresolved": reasons}}, 0)
					}
				}
			}
			if e != nil {
				return e
			}
			return env.JSON(out)
		}})
	}
	commands = append(commands, cli.Command{Area: "retro", Usage: "devkit retro start | prepare | check | round | verify | finish | empty | approve | not-started", Run: func(args []string, env cli.Env) error {
		if _, err := record.Context(env); err != nil {
			return err
		}
		if env.Getenv("ARTIFACTS_DIR") == "" {
			return fmt.Errorf("ARTIFACTS_DIR must be set")
		}
		return &cli.ExitError{Code: 1, Message: "Usage: devkit retro start | prepare | check | round | verify | finish | empty | approve | not-started"}
	}})
	return commands
}
