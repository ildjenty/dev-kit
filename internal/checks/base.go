package checks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"gopkg.in/yaml.v3"
)

func (g Gate) OnlyPreexisting() bool {
	return !g.Passed && len(g.Failing) > 0 && len(g.Failing) == len(g.Preexisting) && slices.Equal(g.Failing, g.Preexisting)
}

func BaseMessage(names []string) string {
	return "the base branch fails " + strings.Join(names, ", ") + " already: fix the base branch first, then start this workflow again"
}

func (g Gate) FailingLogs() ([]string, error) {
	data, err := os.ReadFile(g.Results)
	if err != nil {
		return nil, err
	}
	logs := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var r Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, err
		}
		if r.Status == "fail" {
			logs = append(logs, r.Name+": "+r.Log)
		}
	}
	return logs, nil
}

func compareBase(g *gitx.Git, selected []config.Check, results []Result, dir string, round int, env []string, compare func(Result) bool) (err error) {
	failing := false
	for i := range results {
		if results[i].Status == "fail" {
			results[i].Base = "unavailable"
			if compare(results[i]) {
				failing = true
			} else {
				results[i].Base = "not_compared"
			}
		}
	}
	if !failing {
		return nil
	}
	out := filepath.Join(dir, fmt.Sprintf("%d-base", round))
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	unavailable := func(e error) error { return appendFile(filepath.Join(out, "prepare.log"), e.Error()+"\n") }
	base, e := g.MergeBase("")
	if e != nil {
		return unavailable(e)
	}
	temp, e := os.MkdirTemp("", "devkit-checks-base-")
	if e != nil {
		return unavailable(e)
	}
	worktree := filepath.Join(temp, "worktree")
	defer func() { err = errors.Join(err, os.RemoveAll(temp)) }()
	if _, e := g.Run("worktree", "add", "--detach", "--quiet", worktree, base); e != nil {
		return unavailable(e)
	}
	defer func() {
		_, e := g.Run("worktree", "remove", "--force", "--force", worktree)
		err = errors.Join(err, e)
	}()
	if e := PrepareWorktree(g.Cwd, worktree, out, round, env); e != nil {
		return unavailable(e)
	}
	for i, r := range results {
		if r.Status != "fail" || r.Base == "not_compared" {
			continue
		}
		checkDir := filepath.Join(out, fmt.Sprint(i+1))
		if e := os.MkdirAll(checkDir, 0755); e != nil {
			return e
		}
		compared, e := RunCheck(selected[i], CheckOptions{Cwd: worktree, Dir: checkDir, Round: round, Env: env})
		if e != nil {
			if e := unavailable(e); e != nil {
				return e
			}
			continue
		}
		results[i].BaseLog = compared.Log
		switch compared.Status {
		case "fail":
			results[i].Base = "failed"
		case "pass":
			results[i].Base = "passed"
		}
	}
	return nil
}

func RunOnBase(cwd, base string, selected []config.Check, logs string, env []string) (results []Result, problems []string, err error) {
	results, problems = []Result{}, []string{}
	temp, err := os.MkdirTemp("", "devkit-checks-base-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(temp)) }()
	worktree := filepath.Join(temp, "worktree")
	if _, err = gitx.Run(cwd, "worktree", "add", "--detach", "--quiet", worktree, base); err != nil {
		return nil, nil, err
	}
	defer func() {
		_, e := gitx.Run(cwd, "worktree", "remove", "--force", "--force", worktree)
		err = errors.Join(err, e)
	}()
	if err = PrepareWorktree(cwd, worktree, logs, 1, env); err != nil {
		return results, []string{"base preparation: " + err.Error()}, nil
	}
	for _, check := range selected {
		r, e := RunCheck(check, CheckOptions{Cwd: worktree, Dir: logs, Round: 1, Env: env})
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

func PrepareWorktree(settingsRoot, worktree, out string, round int, env []string) error {
	var settings struct {
		Worktree struct {
			PostCreate        []string `yaml:"postCreate"`
			PostCreateTimeout float64  `yaml:"postCreateTimeout"`
		} `yaml:"worktree"`
	}
	settings.Worktree.PostCreateTimeout = 300000
	data, e := os.ReadFile(filepath.Join(settingsRoot, ".archon/config.yaml"))
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e == nil {
		if e := yaml.Unmarshal(data, &settings); e != nil {
			return e
		}
	}
	if settings.Worktree.PostCreateTimeout <= 0 {
		return fmt.Errorf("worktree.postCreateTimeout must be positive")
	}
	for i, command := range settings.Worktree.PostCreate {
		check := config.Check{Name: fmt.Sprintf("prepare-%d", i+1), Command: command, TimeoutSeconds: settings.Worktree.PostCreateTimeout / 1000}
		r, e := runCheck(check, CheckOptions{Cwd: worktree, Dir: out, Round: round, Env: env}, false)
		if e != nil {
			return e
		}
		if r.Status != "pass" {
			return fmt.Errorf("base preparation failed: %s", r.Log)
		}
	}
	return nil
}
