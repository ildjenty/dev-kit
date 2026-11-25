package pr

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func finish(args []string, env cli.Env) error {
	if err := env.Require("ARTIFACTS_DIR"); err != nil {
		return err
	}
	ctx, err := context(env)
	if err != nil {
		return err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return err
	}
	d, err := read(env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr", "outcome.json")))
	if err != nil {
		return err
	}
	if d["outcome"] == "question" {
		var output doc
		if err = json.Unmarshal([]byte(env.Getenv("INPUTS_QUESTIONS")), &output); err != nil {
			return err
		}
		questions := docs(d["questions"])
		written := docs(output["questions"])
		if len(written) != len(questions) {
			return fmt.Errorf("the question writer changed the number of changes")
		}
		seen := map[int]bool{}
		for _, q := range written {
			n, ok := q["index"].(float64)
			index := int(n)
			if !ok || math.Trunc(n) != n || index < 0 || index >= len(questions) || seen[index] || strings.TrimSpace(text(q["text"])) == "" {
				return fmt.Errorf("the question writer did not cover each change exactly once")
			}
			seen[index] = true
			questions[index]["text"] = q["text"]
		}
	}
	commit, err := g.Head()
	if err != nil {
		return err
	}
	base, err := g.MergeBase(commit)
	if err != nil {
		return err
	}
	result, err := record.WriteResult(ctx.Dir, "pr", d, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: commit})
	if err != nil {
		return err
	}
	questions := result["questions"]
	if questions == nil {
		questions = []doc{}
	}
	return env.JSON(doc{"outcome": result["outcome"], "questions": questions, "stop": result["stop"], "commit": commit})
}
