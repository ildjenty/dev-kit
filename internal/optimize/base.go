package optimize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ildjenty/dev-kit/internal/checks"
	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
	"gopkg.in/yaml.v3"
)

var warningCount = regexp.MustCompile(`[1-9][0-9]* warnings?`)

func validateYAML(data []byte) error { var v any; return yaml.Unmarshal(data, &v) }
func validateArchonConfig(root string) error {
	data, err := os.ReadFile(filepath.Join(root, ".archon/config.yaml"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var s struct {
		Worktree struct {
			BaseBranch        string   `yaml:"baseBranch"`
			PostCreate        []string `yaml:"postCreate"`
			PostCreateTimeout *float64 `yaml:"postCreateTimeout"`
		} `yaml:"worktree"`
	}
	if err = yaml.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.Worktree.PostCreateTimeout != nil && *s.Worktree.PostCreateTimeout <= 0 {
		return fmt.Errorf("worktree.postCreateTimeout must be positive")
	}
	return nil
}
func checkBase(ctx record.ContextData, c config.Config, logs string, env cli.Env) (results []checks.Result, problems []string, err error) {
	results, problems = []checks.Result{}, []string{}
	temp, err := os.MkdirTemp("", "devkit-optimize-base-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temp)) }()
	worktree := filepath.Join(temp, "worktree")
	if _, err = gitx.Run(ctx.Cwd, "worktree", "add", "--detach", "--quiet", worktree, ctx.BaseBranch); err != nil {
		return nil, nil, err
	}
	defer func() {
		_, e := gitx.Run(ctx.Cwd, "worktree", "remove", "--force", "--force", worktree)
		err = errors.Join(err, e)
	}()
	if err = checks.PrepareWorktree(ctx.Cwd, worktree, logs, 1, env.Environ); err != nil {
		return results, []string{"base preparation: " + err.Error()}, nil
	}
	for _, check := range c.Checks {
		r, e := checks.RunCheck(check, checks.CheckOptions{Cwd: worktree, Dir: logs, Round: 1, Env: env.Environ})
		if e != nil {
			return nil, nil, e
		}
		results = append(results, r)
		if r.Status != "pass" {
			problems = append(problems, fmt.Sprintf("check %s on base: %s; log: %s", r.Name, r.Status, r.Log))
		}
	}
	return results, problems, nil
}
