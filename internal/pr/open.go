package pr

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ildjenty/dev-kit/internal/cli"
	"github.com/ildjenty/dev-kit/internal/gitx"
	"github.com/ildjenty/dev-kit/internal/record"
)

func refusals(branch, baseBranch, head string, facts doc, clean bool, judgment doc) []string {
	out := []string{}
	if branch == "HEAD" || branch == baseBranch {
		out = append(out, "refusing to open a pull request from "+branch)
	}
	if head != facts["head"] {
		out = append(out, fmt.Sprintf("HEAD moved from %s after the facts were taken", text(facts["head"])))
	}
	if !clean {
		out = append(out, "the working tree has changes that are not committed")
	}
	if judgment == nil || object(judgment["target"])["commit"] != head || !passed(judgment) {
		out = append(out, "the latest report judgment did not pass on HEAD")
	}
	return out
}
func pushKind(remote, head string, isAncestor func(string, string) bool, recorded map[string]bool) string {
	if remote == "" {
		return "new"
	}
	if remote == head || isAncestor(remote, head) {
		return "forward"
	}
	if recorded[remote] {
		return "replace"
	}
	return ""
}
func recordedCommitsOf(results, judgments []doc) map[string]bool {
	out := map[string]bool{}
	for _, r := range results {
		if s := text(r["commit"]); s != "" {
			out[s] = true
		}
	}
	for _, j := range judgments {
		if s := text(object(j["target"])["commit"]); s != "" {
			out[s] = true
		}
	}
	return out
}
func openPR(args []string, env cli.Env) error {
	ctx, err := context(env)
	if err != nil {
		return err
	}
	g, err := gitx.New(ctx.Cwd, ctx.BaseBranch, ctx.Dir)
	if err != nil {
		return err
	}
	out := env.Path(filepath.Join(env.Getenv("ARTIFACTS_DIR"), "pr"))
	facts, err := read(filepath.Join(out, "facts.json"))
	if err != nil {
		return err
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	judgments, err := record.List(ctx.Dir, "judgment")
	if err != nil {
		return err
	}
	var report doc
	for _, j := range judgments {
		if j["kind"] == "report" && j["run_id"] == ctx.RunID {
			report = j
		}
	}
	clean, err := g.Clean()
	if err != nil {
		return err
	}
	if refused := refusals(ctx.Branch, ctx.BaseBranch, head, facts, clean, report); len(refused) > 0 {
		return &record.RecordError{Message: "the pull request was not opened", Errors: refused}
	}
	state, err := optional(filepath.Join(out, "report-state.json"))
	if err != nil {
		return err
	}
	if !truth(state["passed"]) {
		return &record.RecordError{Message: "the pull request was not opened: the report must be judged after assembly"}
	}
	titleBytes, err := os.ReadFile(filepath.Join(out, "pr-title.txt"))
	if err != nil {
		return err
	}
	bodyBytes, err := os.ReadFile(filepath.Join(out, "pr.md"))
	if err != nil {
		return err
	}
	if !slices.Equal(stringsOf(report["materials"]), reportMaterials(titleBytes, bodyBytes)) {
		return &record.RecordError{Message: "the pull request was not opened: the title or body differs from the report judgment"}
	}
	run := func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = ctx.Cwd
		cmd.Env = env.Environ
		cmd.Stderr = env.Stderr
		if name == "gh" && slices.Contains(args, "--body-file") {
			cmd.Stdin = bytes.NewReader(bodyBytes)
		}
		b, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("Command failed: %s %s", name, strings.Join(args, " "))
		}
		return strings.TrimSpace(string(b)), nil
	}
	remoteOutput, err := run("git", "ls-remote", "--heads", "origin", "refs/heads/"+ctx.Branch)
	if err != nil {
		return err
	}
	remote := ""
	if fields := strings.Fields(remoteOutput); len(fields) > 0 {
		remote = fields[0]
	}
	results, err := record.List(ctx.Dir, "result")
	if err != nil {
		return err
	}
	kind := pushKind(remote, head, g.IsAncestor, recordedCommitsOf(results, judgments))
	if kind == "" {
		return &record.RecordError{Message: fmt.Sprintf("origin/%s is at %s, which HEAD does not contain and this request did not record", ctx.Branch, remote)}
	}
	head, err = commitBacklog(ctx, judgments, results)
	if err != nil {
		return err
	}
	push := []string{"push", "-u"}
	if kind == "replace" {
		push = append(push, "--force-with-lease=refs/heads/"+ctx.Branch+":"+remote)
	}
	if kind != "forward" || remote != head {
		if _, err = run("git", append(push, "origin", "HEAD:refs/heads/"+ctx.Branch)...); err != nil {
			return err
		}
	}
	title := strings.TrimSpace(string(titleBytes))
	url, err := run("gh", "pr", "list", "--head", ctx.Branch, "--state", "open", "--json", "url", "--jq", ".[0].url // empty")
	if err != nil {
		return err
	}
	if url != "" {
		_, err = run("gh", "pr", "edit", url, "--title", title, "--body-file", "-")
	} else {
		url, err = run("gh", "pr", "create", "--base", ctx.BaseBranch, "--head", ctx.Branch, "--title", title, "--body-file", "-")
	}
	if err != nil {
		return err
	}
	if err = save(filepath.Join(out, "outcome.json"), doc{"outcome": "pass"}); err != nil {
		return err
	}
	return env.JSON(doc{"url": url, "branch": ctx.Branch, "commit": head})
}
