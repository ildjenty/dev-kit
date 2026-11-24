package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/concerns"
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/criteria"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

type State struct {
	Previous      string     `json:"previous"`
	Onto          *string    `json:"onto"`
	Ref           *string    `json:"ref"`
	Rounds        int        `json:"rounds"`
	Conflicts     [][]string `json:"conflicts"`
	Contradiction string     `json:"contradiction"`
	State         string     `json:"state"`
}
type Resolution struct {
	Resolved      bool   `json:"resolved"`
	Contradiction string `json:"contradiction"`
}
type Gate struct {
	Passed  bool     `json:"passed"`
	Failing []string `json:"failing"`
}
type git struct {
	cwd string
	env []string
}

var marker = regexp.MustCompile(`(?m)^(<{7}|>{7})( |$)|^={7}$|^\|{7}( |$)`)

func hasMarkers(text string) bool { return marker.MatchString(text) }
func (g git) command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.cwd
	cmd.Env = append(append([]string{}, g.env...), "GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true")
	return cmd
}
func (g git) run(args ...string) (string, error) {
	cmd := g.command(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Command failed: git %s\n%s", strings.Join(args, " "), strings.TrimRight(stderr.String(), "\n"))
	}
	return strings.TrimSpace(string(out)), nil
}
func (g git) ok(args ...string) bool { return g.command(args...).Run() == nil }
func (g git) rebasing() (bool, error) {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		p, err := g.run("rev-parse", "--git-path", name)
		if err != nil {
			return false, err
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(g.cwd, p)
		}
		if _, err = os.Stat(p); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}
func (g git) unmerged() ([]string, error) {
	cmd := g.command("diff", "--name-only", "--diff-filter=U", "-z")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	files := []string{}
	if err != nil {
		return files, fmt.Errorf("Command failed: git diff --name-only --diff-filter=U -z\n%s", strings.TrimRight(stderr.String(), "\n"))
	}
	seen := map[string]bool{}
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" && !seen[f] {
			files = append(files, f)
			seen[f] = true
		}
	}
	return files, nil
}
func afterRebase(g git, s State) (State, error) {
	for {
		active, err := g.rebasing()
		if err != nil {
			return s, err
		}
		if !active {
			if s.Onto == nil {
				return s, fmt.Errorf("the rebase has no recorded target commit")
			}
			if _, err = g.run("merge-base", "--is-ancestor", *s.Onto, "HEAD"); err != nil {
				return s, fmt.Errorf("the rebase did not incorporate %s: %w", *s.Onto, err)
			}
			s.State = "done"
			return s, nil
		}
		files, err := g.unmerged()
		if err != nil {
			return s, err
		}
		if len(files) > 0 {
			s.State = "conflict"
			s.Conflicts = append(s.Conflicts, files)
			return s, nil
		}
		if !g.ok("rebase", "--continue") {
			files, err = g.unmerged()
			if err != nil {
				return s, err
			}
			if len(files) == 0 {
				s.State = "conflict"
				s.Conflicts = append(s.Conflicts, []string{})
				return s, nil
			}
		}
	}
}
func start(g git, base string) (State, error) {
	previous, err := g.run("rev-parse", "HEAD")
	s := State{Previous: previous, Conflicts: [][]string{}}
	if err != nil {
		return s, err
	}
	status, err := g.run("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return s, err
	}
	if status != "" {
		s.State = "refused"
		return s, nil
	}
	ref := base
	if g.ok("remote", "get-url", "origin") {
		if _, err = g.run("fetch", "--quiet", "origin", base); err != nil {
			return s, err
		}
		ref = "origin/" + base
	}
	onto, err := g.run("rev-parse", ref+"^{commit}")
	if err != nil {
		return s, err
	}
	s.Ref = &ref
	s.Onto = &onto
	if g.ok("merge-base", "--is-ancestor", onto, previous) {
		s.State = "current"
		return s, nil
	}
	if _, err = g.run("rebase", "--quiet", onto); err != nil {
		active, stateErr := g.rebasing()
		if stateErr != nil {
			return s, stateErr
		}
		if !active {
			return s, err
		}
		files, stateErr := g.unmerged()
		if stateErr != nil {
			return s, stateErr
		}
		if len(files) == 0 {
			return s, err
		}
	}
	return afterRebase(g, s)
}
func proceed(g git, s State, r Resolution, read func(string) (*string, error)) (State, error) {
	s.Rounds++
	if !r.Resolved {
		s.State = "contradiction"
		s.Contradiction = r.Contradiction
		return s, nil
	}
	left := []string{}
	if len(s.Conflicts) > 0 {
		for _, f := range s.Conflicts[len(s.Conflicts)-1] {
			text, err := read(f)
			if err != nil {
				return s, err
			}
			if text != nil && hasMarkers(*text) {
				left = append(left, f)
			}
		}
	}
	if len(left) > 0 {
		s.State = "conflict"
		s.Conflicts = append(s.Conflicts, left)
		return s, nil
	}
	if _, err := g.run("add", "-A"); err != nil {
		return s, err
	}
	g.ok("rebase", "--continue")
	return afterRebase(g, s)
}
func settle(g git, s State) (State, error) {
	active, err := g.rebasing()
	if err != nil {
		return s, err
	}
	if s.State == "conflict" || active {
		if !g.ok("rebase", "--abort") {
			return s, fmt.Errorf("the rebase could not be aborted and restored to the original commit")
		}
		head, err := g.run("rev-parse", "HEAD")
		if err != nil || head != s.Previous {
			return s, fmt.Errorf("the rebase could not be aborted and restored to the original commit")
		}
		if s.State != "contradiction" {
			s.State = "unresolved"
		}
	}
	return s, nil
}
func outcomeOf(s State, gate *Gate) (record.Document, error) {
	ended := "cleared"
	if s.State == "unresolved" {
		ended = "max_iterations"
	}
	if s.State == "contradiction" {
		ended = "stuck"
	}
	doc := record.Document{"loops": []record.Document{{"name": "conflicts", "ended": ended, "rounds": s.Rounds}}}
	stop := func(state string, unresolved []string) (record.Document, error) {
		doc["outcome"] = "stop"
		doc["stop"] = record.Document{"state": state, "unresolved": unresolved, "premise": ""}
		return doc, nil
	}
	files := []string{}
	seen := map[string]bool{}
	for _, round := range s.Conflicts {
		for _, f := range round {
			if !seen[f] {
				files = append(files, f)
				seen[f] = true
			}
		}
	}
	switch s.State {
	case "refused":
		return stop("The working tree has uncommitted changes; the branch was not rebased", []string{"uncommitted changes"})
	case "unresolved":
		return stop(fmt.Sprintf("Conflicts remain after %d rounds; the rebase was aborted and the original commit restored", s.Rounds), files)
	case "contradiction":
		return stop("The base and branch changes cannot both hold; the rebase was aborted and the original commit restored", []string{s.Contradiction})
	case "done", "current":
	default:
		return nil, fmt.Errorf("the rebase ended in state %s", s.State)
	}
	if gate == nil || !gate.Passed {
		state := "Checks did not pass on the latest base"
		if s.State == "done" {
			state = "Checks did not pass after rebasing onto the latest base"
		}
		failing := []string{"no check result"}
		if gate != nil && len(gate.Failing) > 0 {
			failing = gate.Failing
		}
		return stop(state, failing)
	}
	doc["outcome"] = "pass"
	return doc, nil
}
func readJSON(file string, v any) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func saveJSON(file string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}
func Commands() []cli.Command {
	commands := []cli.Command{}
	for _, mode := range []string{"start", "continue", "state"} {
		commands = append(commands, cli.Command{Area: "sync", Name: mode, Usage: "devkit sync " + mode, Run: func(args []string, env cli.Env) error { return rebase(mode, env) }})
	}
	return append(commands, cli.Command{Area: "sync", Name: "finish", Usage: "devkit sync finish", Run: finish}, cli.Command{Area: "sync", Name: "output", Usage: "devkit sync output <not-started> <result>", Run: output})
}
func rebase(mode string, env cli.Env) error {
	childEnv := env.Environ
	if childEnv == nil {
		childEnv = os.Environ()
	}
	g := git{env.Cwd, childEnv}
	out := filepath.Join(env.Getenv("ARTIFACTS_DIR"), "sync")
	file := env.Path(filepath.Join(out, "state.json"))
	var s State
	var err error
	var resolver record.Document
	if mode == "start" {
		ctx, err := record.Context(env)
		if err != nil {
			return err
		}
		requests, err := record.List(env.Path(ctx.Dir), "request")
		if err != nil {
			return err
		}
		if len(requests) == 0 {
			return fmt.Errorf("no request is recorded yet")
		}
		source, err := criteria.CopyCriteria(criteria.Options{Root: env.Cwd, Base: requests[0]["criteria_commit"].(string), Dir: env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "criteria"))})
		if err != nil {
			return err
		}
		c, err := config.Load(env.Cwd)
		if err != nil {
			return err
		}
		standards := source.Paths[0]
		mapping, err := concerns.Load(filepath.Join(source.Dir, standards))
		if err != nil {
			return err
		}
		resolver = record.Document{"language": c.Language, "standards": standards, "writer_materials": mapping.Materials(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "criteria", standards), "writer:sync", "code")}

		base := env.Getenv("BASE_BRANCH")
		if base == "" {
			base = "main"
		}
		s, err = start(g, base)
		if err != nil {
			return err
		}
	} else {
		if err = readJSON(file, &s); err != nil {
			return err
		}
		if mode == "state" {
			s, err = settle(g, s)
		} else {
			var r Resolution
			if err = json.Unmarshal([]byte(env.Getenv("INPUTS_RESOLVED")), &r); err != nil {
				return err
			}
			s, err = proceed(g, s, r, func(f string) (*string, error) {
				b, e := os.ReadFile(env.Path(f))
				if os.IsNotExist(e) {
					return nil, nil
				}
				if e != nil {
					return nil, e
				}
				t := string(b)
				return &t, nil
			})
		}
	}
	if err != nil {
		return err
	}
	if err = os.MkdirAll(env.Path(out), 0755); err != nil {
		return err
	}
	if resolver != nil {
		if err = saveJSON(env.Path(filepath.Join(out, "input.json")), resolver); err != nil {
			return err
		}
	}
	if err = saveJSON(file, s); err != nil {
		return err
	}
	files := []string{}
	if len(s.Conflicts) > 0 {
		files = s.Conflicts[len(s.Conflicts)-1]
	}
	if s.State == "conflict" {
		if err = readJSON(env.Path(filepath.Join(out, "input.json")), &resolver); err != nil {
			return err
		}
		replaying := ""
		if g.ok("rev-parse", "--verify", "--quiet", "REBASE_HEAD") {
			replaying, err = g.run("log", "-1", "--format=%H %s", "REBASE_HEAD")
			if err != nil {
				return err
			}
		}
		if err = saveJSON(env.Path(filepath.Join(out, "conflict.json")), record.Document{"language": resolver["language"], "standards": resolver["standards"], "writer_materials": resolver["writer_materials"], "files": files, "replaying": replaying, "onto": s.Onto}); err != nil {
			return err
		}
	}
	head, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	return env.JSON(record.Document{"previous": s.Previous, "onto": s.Onto, "ref": s.Ref, "rounds": s.Rounds, "contradiction": s.Contradiction, "state": s.State, "files": files, "commit": head})
}
func finish(args []string, env cli.Env) error {
	ctx, err := record.Context(env)
	if err != nil {
		return err
	}
	var s State
	if err = readJSON(env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "sync", "state.json")), &s); err != nil {
		return err
	}
	var gate *Gate
	if text := env.Getenv("INPUTS_GATE"); strings.TrimSpace(text) != "" {
		if err = json.Unmarshal([]byte(text), &gate); err != nil {
			return err
		}
	}
	doc, err := outcomeOf(s, gate)
	if err != nil {
		return err
	}
	commit, err := gitx.Run(ctx.Cwd, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	ref := ctx.BaseBranch
	if s.Onto != nil {
		ref = *s.Onto
	}
	base, err := gitx.Run(ctx.Cwd, "merge-base", "HEAD", ref)
	if err != nil {
		return err
	}
	result, err := record.WriteResult(env.Path(ctx.Dir), "sync", doc, record.ResultOptions{RunID: ctx.RunID, Base: base, Commit: commit})
	if err != nil {
		return err
	}
	rounds := 0
	if s.State == "done" {
		rounds = s.Rounds
	}
	onto := ""
	if s.Onto != nil {
		onto = *s.Onto
	}
	failing := []string{}
	if gate != nil && gate.Failing != nil {
		failing = gate.Failing
	}
	return env.JSON(record.Document{"outcome": result["outcome"], "needs_judge": result["outcome"] == "pass" && rounds > 0, "conflict_rounds": rounds, "onto": onto, "previous": s.Previous, "commit": commit, "failing": failing, "stop": result["stop"]})
}
func output(args []string, env cli.Env) error {
	if len(args) != 2 {
		return cli.Usage("devkit sync output <not-started> <result>")
	}
	doc := record.Document{}
	for i, key := range []string{"not_started", "result"} {
		var value any
		if args[i] != "" {
			if err := json.Unmarshal([]byte(args[i]), &value); err != nil {
				return err
			}
		}
		doc[key] = value
	}
	return env.JSON(doc)
}
